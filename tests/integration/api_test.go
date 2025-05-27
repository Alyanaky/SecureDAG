package integration

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"

    "github.com/Alyanaky/SecureDAG/internal/auth"
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

    r := gin.Default()
    r.PUT("/s3/:bucket", func(c *gin.Context) {
        c.Status(http.StatusOK)
    })
    r.PUT("/s3/:bucket/:key", func(c *gin.Context) {
        bucket := c.Param("bucket")
        key := c.Param("key")
        data, err := c.GetRawData()
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        err = storageBackend.PutObject(bucket, key, data)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.Status(http.StatusOK)
    })
    r.GET("/s3/:bucket/:key", func(c *gin.Context) {
        bucket := c.Param("bucket")
        key := c.Param("key")
        data, err := storageBackend.GetObject(bucket, key)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.Data(http.StatusOK, "application/octet-stream", data)
    })
    return r, storageBackend, pgStore
}

func TestS3CreateBucket(t *testing.T) {
    r, _, _ := setupTestServer(t)
    token, err := auth.GenerateToken("testuser", nil)
    require.NoError(t, err)

    req, _ := http.NewRequest("PUT", "/s3/testbucket", nil)
    req.Header.Set("Authorization", token)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestS3PutAndGetObject(t *testing.T) {
    r, _, _ := setupTestServer(t)
    token, err := auth.GenerateToken("testuser", nil)
    require.NoError(t, err)

    req, _ := http.NewRequest("PUT", "/s3/testbucket", nil)
    req.Header.Set("Authorization", token)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)

    data := []byte("test data")
    req, _ = http.NewRequest("PUT", "/s3/testbucket/testkey", bytes.NewReader(data))
    req.Header.Set("Authorization", token)
    req.Header.Set("Content-Type", "text/plain")
    w = httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)

    req, _ = http.NewRequest("GET", "/s3/testbucket/testkey", nil)
    req.Header.Set("Authorization", token)
    w = httptest.NewRecorder()
    r.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Equal(t, data, w.Body.Bytes())
}

func TestS3CopyObject(t *testing.T) {
    // Заглушка для теста
}

func TestS3AbortMultipartUpload(t *testing.T) {
    // Заглушка для теста
}

func TestQuotaManagement(t *testing.T) {
    // Заглушка для теста
}
