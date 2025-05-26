package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/auth"
	"github.com/Alyanaky/SecureDAG/internal/metrics"
	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/Alyanaky/SecureDAG/internal/s3"
	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var (
	quotaManager   *storage.QuotaManager
	pgStore        *storage.PostgresStore
	nodeManager    *p2p.NodeManager
	storageBackend *storage.BadgerStore
	deleteManager  *storage.DeletionManager
	s3Adapter      *s3.S3Adapter
)

func main() {
	metrics.Init()
	initStorage()
	initPostgres()
	initP2P()
	startAPIServer()
}

func initStorage() {
	var err error
	storageBackend, err = storage.NewBadgerStore(getStoragePath())
	if err != nil {
		panic(fmt.Sprintf("Storage init failed: %v", err))
	}
	deleteManager = storage.NewDeletionManager()
	quotaManager = storage.NewQuotaManager(storageBackend)
	s3Adapter = s3.NewS3Adapter(pgStore, storageBackend)
}

func initPostgres() {
	connStr := getPostgresURL()
	var err error
	pgStore, err = storage.NewPostgresStore(connStr)
	if err != nil {
		panic(fmt.Sprintf("Postgres init failed: %v", err))
	}
	if err := pgStore.Migrate(); err != nil {
		panic(fmt.Sprintf("Migration failed: %v", err))
	}
}

func initP2P() {
	nodeManager = p2p.NewNodeManager()
	go nodeManager.Start(context.Background())
}

func startAPIServer() {
	r := gin.Default()
	configureRoutes(r)
	r.Run(":" + getPort())
}

func getStoragePath() string {
	path := os.Getenv("STORAGE_PATH")
	if path == "" {
		path = "./data"
	}
	return path
}

func getPostgresURL() string {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://user:password@localhost:5432/securedag?sslmode=disable"
	}
	return url
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

func configureRoutes(r *gin.Engine) {
	// Core API
	api := r.Group("/api/v1")
	{
		api.POST("/auth", authHandler)
		api.Use(authMiddleware())
		api.PUT("/objects/:key", putObjectHandler)
		api.GET("/objects/:hash", getObjectHandler)
		api.DELETE("/objects/:hash", deleteObjectHandler)
		api.GET("/users/:id/quota", getQuotaHandler)
		api.PUT("/users/:id/quota", updateQuotaHandler)
	}

	// S3 Compatible API
	s3 := r.Group("/s3")
	s3.Use(authMiddleware())
	{
		s3.PUT("/:bucket", s3CreateBucketHandler)
		s3.DELETE("/:bucket", s3DeleteBucketHandler)
		s3.GET("/", s3ListBucketsHandler)
		s3.HEAD("/:bucket", s3HeadBucketHandler)
		s3.PUT("/:bucket/*key", s3PutHandler)
		s3.GET("/:bucket/*key", s3GetHandler)
		s3.DELETE("/:bucket/*key", s3DeleteHandler)
		s3.HEAD("/:bucket/*key", s3HeadObjectHandler)
	}

	// Admin API
	admin := r.Group("/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.GET("/nodes", listNodesHandler)
		admin.POST("/nodes/restart", restartNodeHandler)
		admin.GET("/stats", systemStatsHandler)
		admin.GET("/health", healthCheckHandler)
	}

	// Metrics
	r.GET("/metrics", gin.WrapH(metrics.Handler()))
}

func authHandler(c *gin.Context) {
	var creds struct{ Username, Password string }
	if err := c.BindJSON(&creds); err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	token, err := auth.GenerateToken(creds.Username, storageBackend.PublicKey())
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Token generation failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func putObjectHandler(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	data, _ := c.GetRawData()

	if ok, err := quotaManager.CheckQuota(userID, int64(len(data))); err != nil || !ok {
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to check quota")
			return
		}
		abortWithError(c, http.StatusTooManyRequests, "Storage quota exceeded")
		return
	}

	hash, err := storageBackend.PutBlock(data)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to store object")
		return
	}

	if err := quotaManager.UpdateUsage(userID, int64(len(data))); err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to update quota")
		return
	}

	c.JSON(http.StatusOK, gin.H{"hash": hash})
}

func getObjectHandler(c *gin.Context) {
	hash := c.Param("hash")
	data, err := storageBackend.GetBlock(hash)
	if err != nil {
		if errors.Is(err, storage.ErrBlockNotFound) {
			abortWithError(c, http.StatusNotFound, "Object not found")
			return
		}
		abortWithError(c, http.StatusInternalServerError, "Failed to retrieve object")
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func deleteObjectHandler(c *gin.Context) {
	hash := c.Param("hash")
	if err := deleteManager.ScheduleDeletion(hash, 5*time.Minute); err != nil {
		abortWithError(c, http.StatusConflict, "Deletion already scheduled")
		return
	}
	c.Status(http.StatusAccepted)
}

func s3CreateBucketHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	if err := pgStore.CreateBucket(bucket); err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to create bucket")
		return
	}
	c.Status(http.StatusOK)
}

func s3DeleteBucketHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	if err := pgStore.DeleteBucket(bucket); err != nil {
		if errors.Is(err, storage.ErrBucketNotFound) {
			abortWithError(c, http.StatusNotFound, "Bucket not found")
			return
		}
		abortWithError(c, http.StatusInternalServerError, "Failed to delete bucket")
		return
	}
	c.Status(http.StatusOK)
}

func s3ListBucketsHandler(c *gin.Context) {
	buckets, err := pgStore.ListBuckets()
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to list buckets")
		return
	}
	c.JSON(http.StatusOK, buckets)
}

func s3HeadBucketHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	_, err := pgStore.GetBucket(bucket)
	if err != nil {
		if errors.Is(err, storage.ErrBucketNotFound) {
			c.Status(http.StatusNotFound)
		} else {
			abortWithError(c, http.StatusInternalServerError, "Failed to head bucket")
		}
		return
	}
	c.Status(http.StatusOK)
}

func s3PutHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")[1:] // Удаляем ведущий слэш
	copySource := c.GetHeader("x-amz-copy-source")
	if copySource != "" {
		// Handle Copy Object
		err := s3Adapter.CopyObject(bucket, key, copySource)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to copy object")
			return
		}
		c.Status(http.StatusOK)
	} else {
		// Handle Put Object
		data, err := c.GetRawData()
		if err != nil {
			abortWithError(c, http.StatusBadRequest, "Failed to read request body")
			return
		}
		userID := c.MustGet("user_id").(string)
		if ok, err := quotaManager.CheckQuota(userID, int64(len(data))); err != nil || !ok {
			if err != nil {
				abortWithError(c, http.StatusInternalServerError, "Failed to check quota")
				return
			}
			abortWithError(c, http.StatusTooManyRequests, "Storage quota exceeded")
			return
		}
		meta := extractMetadata(c)
		hash, err := storageBackend.PutBlockWithMetadata(data, meta)
		if err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to store object")
			return
		}
		obj := &storage.Object{
			Bucket: bucket,
			Key:    key,
			Hash:   hash,
			Metadata: meta,
		}
		if err := pgStore.PutObject(obj); err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to save object metadata")
			return
		}
		if err := quotaManager.UpdateUsage(userID, int64(len(data))); err != nil {
			abortWithError(c, http.StatusInternalServerError, "Failed to update quota")
			return
		}
		c.Status(http.StatusOK)
	}
}

func s3GetHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")[1:] // Удаляем ведущий слэш
	versionID := c.Query("versionId")
	obj, err := pgStore.GetObject(bucket, key, versionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			abortWithError(c, http.StatusNotFound, "Object not found")
			return
		}
		abortWithError(c, http.StatusInternalServerError, "Failed to retrieve object")
		return
	}
	data, err := storageBackend.GetBlock(obj.Hash)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to retrieve object data")
		return
	}
	for k, v := range obj.Metadata {
		c.Header(k, v)
	}
	c.Data(http.StatusOK, obj.Metadata["Content-Type"], data)
}

func s3DeleteHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")[1:] // Удаляем ведущий слэш
	uploadID := c.Query("uploadId")
	if uploadID != "" {
		// Abort Multipart Upload
		err := s3Adapter.AbortMultipartUpload(bucket, key, uploadID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				abortWithError(c, http.StatusNotFound, "Upload not found")
			} else {
				abortWithError(c, http.StatusInternalServerError, "Failed to abort upload")
			}
			return
		}
		c.Status(http.StatusOK)
	} else {
		// Delete Object
		versionID := c.Query("versionId")
		err := s3Adapter.DeleteObject(bucket, key, versionID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				abortWithError(c, http.StatusNotFound, "Object not found")
			} else {
				abortWithError(c, http.StatusInternalServerError, "Failed to delete object")
			}
			return
		}
		c.Status(http.StatusOK)
	}
}

func s3HeadObjectHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")[1:] // Удаляем ведущий слэш
	versionID := c.Query("versionId")
	obj, err := pgStore.GetObject(bucket, key, versionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.Status(http.StatusNotFound)
		} else {
			abortWithError(c, http.StatusInternalServerError, "Failed to head object")
		}
		return
	}
	for k, v := range obj.Metadata {
		c.Header(k, v)
	}
	c.Status(http.StatusOK)
}

func getQuotaHandler(c *gin.Context) {
	userID := c.Param("id")
	quota, err := quotaManager.GetQuota(userID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to get quota")
		return
	}
	c.JSON(http.StatusOK, gin.H{"quota": quota})
}

func updateQuotaHandler(c *gin.Context) {
	userID := c.Param("id")
	var req struct{ Quota int64 }
	if err := c.BindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}
	if err := quotaManager.UpdateQuota(userID, req.Quota); err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to update quota")
		return
	}
	c.Status(http.StatusOK)
}

func listNodesHandler(c *gin.Context) {
	nodes := nodeManager.ListNodes()
	c.JSON(http.StatusOK, nodes)
}

func restartNodeHandler(c *gin.Context) {
	var req struct{ NodeID string }
	if err := c.BindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}
	if err := nodeManager.RestartNode(req.NodeID); err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to restart node")
		return
	}
	c.Status(http.StatusOK)
}

func systemStatsHandler(c *gin.Context) {
	stats := metrics.GetSystemStats()
	c.JSON(http.StatusOK, stats)
}

func healthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			abortWithError(c, http.StatusUnauthorized, "Missing authorization token")
			return
		}
		userID, err := auth.ValidateToken(token)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, "Invalid token")
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			abortWithError(c, http.StatusUnauthorized, "Missing authorization token")
			return
		}
		userID, err := auth.ValidateToken(token)
		if err != nil || !auth.IsAdmin(userID) {
			abortWithError(c, http.StatusForbidden, "Admin access required")
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func abortWithError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{"error": message})
}

func extractMetadata(c *gin.Context) map[string]string {
	meta := make(map[string]string)
	for k, v := range c.Request.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-amz-meta-") || k == "Content-Type" {
			meta[k] = v[0]
		}
	}
	return meta
}
