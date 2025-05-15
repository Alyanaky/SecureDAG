package main

import (
	"net/http"

	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/gin-gonic/gin"
)

func StartAPIServer(store *storage.BadgerStore) {
	r := gin.Default()

	r.PUT("/objects/:key", func(c *gin.Context) {
		data, _ := c.GetRawData()
		hash, err := store.PutBlock(data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"hash": hash})
	})

	r.GET("/objects/:hash", func(c *gin.Context) {
		data, err := store.GetBlock(c.Param("hash"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "block not found"})
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", data)
	})

	r.Run(":8080")
}
