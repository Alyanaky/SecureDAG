package main

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/Alyanaky/SecureDAG/internal/auth"
	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var (
	quotaManager *storage.QuotaManager
	pgStore      *storage.PostgresStore
)

func main() {
	connStr := os.Getenv("POSTGRES_URL")
	store, _ := storage.NewPostgresStore(connStr)
	store.Migrate()
	
	quotaManager = storage.NewQuotaManager()
	StartAPIServer(storage.NewBadgerStore("data"))
}

func StartAPIServer(store *storage.BadgerStore) {
	r := gin.Default()

	r.POST("/login", loginHandler)
	
	authGroup := r.Group("/")
	authGroup.Use(authMiddleware(store))
	
	authGroup.PUT("/objects/:key", putObjectHandler(store))
	authGroup.GET("/objects/:hash", getObjectHandler(store))
	
	adminGroup := authGroup.Group("/admin")
	adminGroup.GET("/users", adminGetUsersHandler)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}

func loginHandler(c *gin.Context) {
	var creds struct{ Username, Password string }
	c.BindJSON(&creds)
	
	token, _ := auth.GenerateToken(creds.Username, storage.GetPrivateKey())
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func authMiddleware(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		claims, err := auth.ValidateToken(token, store.PublicKey())
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		
		role := getRoleFromDB(claims.UserID)
		if !auth.HasPermission(auth.Role(role), c.FullPath(), c.Request.Method) {
			c.AbortWithStatus(http.StatusForbidden)
		}
		
		c.Set("claims", claims)
		c.Set("role", role)
	}
}

func putObjectHandler(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := c.MustGet("claims").(*auth.Claims)
		data, _ := c.GetRawData()
		
		if !quotaManager.CheckQuota(claims.UserID, int64(len(data))) {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		
		hash, _ := store.PutBlock(data)
		quotaManager.UpdateUsage(claims.UserID, int64(len(data)))
		c.JSON(http.StatusOK, gin.H{"hash": hash})
	}
}

func getObjectHandler(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := store.GetBlock(c.Param("hash"))
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", data)
	}
}

func adminGetUsersHandler(c *gin.Context) {
	rows, _ := pgStore.db.Query("SELECT id, role FROM users")
	defer rows.Close()
	
	var users []map[string]interface{}
	for rows.Next() {
		var id, role string
		rows.Scan(&id, &role)
		users = append(users, map[string]interface{}{"id": id, "role": role})
	}
	c.JSON(http.StatusOK, users)
}

func getRoleFromDB(userID string) string {
	var role string
	pgStore.db.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	return role
}
