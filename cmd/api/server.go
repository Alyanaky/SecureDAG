package main

import (
	"net/http"
	"os"

	"github.com/Alyanaky/SecureDAG/internal/auth"
	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// @title SecureDAG Storage API
// @version 1.0
// @description Децентрализованное защищенное хранилище файлов

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

var (
	quotaManager = storage.NewQuotaManager()
)

func StartAPIServer(store *storage.BadgerStore) {
	r := gin.Default()

	// @Summary Аутентификация пользователя
	// @Tags Auth
	// @Accept json
	// @Produce json
	// @Param credentials body object{username=string,password=string} true "Данные для входа"
	// @Success 200 {object} object{token=string}
	// @Router /login [post]
	r.POST("/login", func(c *gin.Context) {
		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		
		if err := c.BindJSON(&creds); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		token, err := auth.GenerateToken(creds.Username, store.PrivateKey())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	authGroup := r.Group("/")
	authGroup.Use(func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := auth.ValidateToken(tokenString, store.PublicKey())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("claims", claims)
	})

	// @Summary Загрузить объект
	// @Tags Storage
	// @Security BearerAuth
	// @Accept octet-stream
	// @Produce json
	// @Param key path string true "Идентификатор объекта"
	// @Param data body string true "Данные объекта"
	// @Success 200 {object} object{hash=string}
	// @Failure 429 {object} object{error=string}
	// @Router /objects/{key} [put]
	authGroup.PUT("/objects/:key", func(c *gin.Context) {
		claims := c.MustGet("claims").(*auth.Claims)
		data, _ := c.GetRawData()

		if !quotaManager.CheckQuota(claims.UserID, int64(len(data))) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "quota exceeded"})
			return
		}

		hash, err := store.PutBlock(data)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "storage failed"})
			return
		}

		quotaManager.UpdateUsage(claims.UserID, int64(len(data)))
		c.JSON(http.StatusOK, gin.H{"hash": hash})
	})

	// @Summary Получить объект
	// @Tags Storage
	// @Security BearerAuth
	// @Produce octet-stream
	// @Param hash path string true "Хеш объекта"
	// @Success 200 {string} binary
	// @Router /objects/{hash} [get]
	authGroup.GET("/objects/:hash", func(c *gin.Context) {
		data, err := store.GetBlock(c.Param("hash"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "object not found"})
			return
		}
		
		c.Data(http.StatusOK, "application/octet-stream", data)
	})

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
