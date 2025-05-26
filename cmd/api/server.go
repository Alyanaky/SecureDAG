package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/auth"
	"github.com/Alyanaky/SecureDAG/internal/metrics"
	"github.com/Alyanaky/SecureDAG/internal/p2p"
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
		s3.GET("/", s3ListBucketsHandler)
		s3.PUT("/:bucket/*key", s3PutHandler)
		s3.GET("/:bucket/*key", s3GetHandler)
		s3.DELETE("/:bucket/*key", s3DeleteHandler)
		s3.HEAD("/:bucket/*key", s3HeadHandler)
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

// ### Core API Handlers ###
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

// ### S3 API Handlers ###
func s3CreateBucketHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	if err := pgStore.CreateBucket(bucket); err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to create bucket")
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

func s3PutHandler(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")
	data, _ := c.GetRawData()

	meta := map[string]string{
		"Content-Type": c.GetHeader("Content-Type"),
		"Bucket":       bucket,
		"Key":          key,
	}

	hash, err := storageBackend.PutBlockWithMetadata(data, meta)
	if err != nil {
		abortWithError
