package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Alyanaky/SecureDAG/cmd/api"
	"github.com/Alyanaky/SecureDAG/internal/auth"
	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func setupTestServer(t *testing.T) (*gin.Engine, *storage.BadgerStore, *storage.PostgresStore) {
	storageBackend, err := storage.NewBadgerStore(t.TempDir())
	require.NoError(t, err)

	connStr := "postgres://user:password@localhost:5432/securedag_test?sslmode=disable"
	pgStore, err := storage.NewPostgresStore(connStr)
	require.NoError(t, err)
	err = pgStore.Migrate()
	require.NoError(t, err)

	nodeManager := p2p.NewNodeManager()
	go nodeManager.Start(context.Background())

	r := gin.Default()
	api.ConfigureRoutes(r, storageBackend, pgStore, nodeManager)
	return r, storageBackend, pgStore
}

func TestS3CreateBucket(t *testing.T) {
	r, _, pgStore := setupTestServer(t)
	token, err := auth.GenerateToken("testuser", nil)
	require.NoError(t, err)

	req, _ := http.NewRequest("PUT", "/s3/testbucket", nil)
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	bucket, err := pgStore.GetBucket("testbucket")
	assert.NoError(t, err)
	assert.Equal(t, "testbucket", bucket.Name)
}

func TestS3PutAndGetObject(t *testing.T) {
	r, storageBackend, pgStore := setupTestServer(t)
	token, err := auth.GenerateToken("testuser", nil)
	require.NoError(t, err)

	// Create bucket
	req, _ := http.NewRequest("PUT", "/s3/testbucket", nil)
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Put object
	data := []byte("test data")
	req, _ = http.NewRequest("PUT", "/s3/testbucket/testkey", bytes.NewReader(data))
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get object
	req, _ = http.NewRequest("GET", "/s3/testbucket/testkey", nil)
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, data, w.Body.Bytes())
}

func TestS3CopyObject(t *testing.T) {
	r, storageBackend, pgStore := setupTestServer(t)
	token, err := auth.GenerateToken("testuser", nil)
	require.NoError(t, err)

	// Create source and destination buckets
	req, _ := http.NewRequest("PUT", "/s3/sourcebucket", nil)
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req, _ = http.NewRequest("PUT", "/s3/destbucket", nil)
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Put source object
	data := []byte("test data")
	req, _ = http.NewRequest("PUT", "/s3/sourcebucket/sourcekey", bytes.NewReader(data))
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Copy object
	req, _ = http.NewRequest("PUT", "/s3/destbucket/destkey", nil)
	req.Header.Set("Authorization", token)
	req.Header.Set("x-amz-copy-source", "/sourcebucket/sourcekey")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify copied object
	req, _ = http.NewRequest("GET", "/s3/destbucket/destkey", nil)
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, data, w.Body.Bytes())
}

func TestS3AbortMultipartUpload(t *testing.T) {
	r, _, pgStore := setupTestServer(t)
	token, err := auth.GenerateToken("testuser", nil)
	require.NoError(t, err)

	// Create bucket
	req, _ := http.NewRequest("PUT", "/s3/testbucket", nil)
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Initiate multipart upload
	req, _ = http.NewRequest("POST", "/s3/testbucket/testkey?uploads", nil)
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct{ UploadID string }
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	uploadID := resp.UploadID

	// Abort multipart upload
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("/s3/testbucket/testkey?uploadId=%s", uploadID), nil)
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify upload is aborted
	_, err = pgStore.GetMultipartUpload(uploadID)
	assert.Error(t, err)
}

func TestQuotaManagement(t *testing.T) {
	r, storageBackend, _ := setupTestServer(t)
	token, err := auth.GenerateToken("testuser", nil)
	require.NoError(t, err)

	// Set quota
	req, _ := http.NewRequest("PUT", "/api/v1/users/testuser/quota", bytes.NewReader([]byte(`{"quota": 1000}`)))
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Put object exceeding quota
	data := make([]byte, 2000)
	req, _ = http.NewRequest("PUT", "/api/v1/objects/testkey", bytes.NewReader(data))
	req.Header.Set("Authorization", token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}
