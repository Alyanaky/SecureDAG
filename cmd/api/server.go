package main

import (
    "context"
    "log"

    "github.com/Alyanaky/SecureDAG/internal/s3"
    "github.com/Alyanaky/SecureDAG/internal/storage"
    "github.com/gin-gonic/gin"
)

func main() {
    ctx := context.Background()

    storageBackend, err := storage.NewBadgerStore("/tmp/securedag")
    if err != nil {
        log.Fatal(err)
    }
    defer storageBackend.Close()

    connStr := "postgres://user:password@localhost:5432/securedag?sslmode=disable"
    pgStore, err := storage.NewPostgresStore(connStr)
    if err != nil {
        log.Fatal(err)
    }
    if err := pgStore.Migrate(); err != nil {
        log.Fatal(err)
    }

    s3Adapter := s3.NewS3Adapter(storageBackend, pgStore)
    quotaManager := storage.NewQuotaManager(storageBackend)

    r := gin.Default()

    r.PUT("/s3/:bucket/:key", func(c *gin.Context) {
        bucket := c.Param("bucket")
        key := c.Param("key")
        data, err := c.GetRawData()
        if err != nil {
            c.JSON(400, gin.H{"error": err.Error()})
            return
        }

        size := int64(len(data))
        if err := quotaManager.CheckQuota(ctx, bucket, size); err != nil {
            c.JSON(403, gin.H{"error": "quota exceeded"})
            return
        }

        input := &s3.PutObjectInput{
            Bucket: &bucket,
            Key:    &key,
            Body:   bytes.NewReader(data),
        }
        if _, err := s3Adapter.PutObject(ctx, input); err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        c.Status(200)
    })

    r.GET("/s3/:bucket/:key", func(c *gin.Context) {
        bucket := c.Param("bucket")
        key := c.Param("key")
        input := &s3.GetObjectInput{
            Bucket: &bucket,
            Key:    &key,
        }
        output, err := s3Adapter.GetObject(ctx, input)
        if err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        data, err := io.ReadAll(output.Body)
        if err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        c.Data(200, "application/octet-stream", data)
    })

    r.DELETE("/s3/:bucket/:key", func(c *gin.Context) {
        bucket := c.Param("bucket")
        key := c.Param("key")
        input := &s3.DeleteObjectInput{
            Bucket: &bucket,
            Key:    &key,
        }
        if _, err := s3Adapter.DeleteObject(ctx, input); err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        c.Status(204)
    })

    if err := r.Run(":8080"); err != nil {
        log.Fatal(err)
    }
}
