package main

import (
	"database/sql"
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
	quotaManager  *storage.QuotaManager
	pgStore       *storage.PostgresStore
	nodeManager   *p2p.NodeManager
	loadBalancer  *p2p.LoadBalancer
	deleteManager *storage.DeletionManager
)

func main() {
	metrics.Init()
	
	// Инициализация PostgreSQL
	connStr := getPostgresURL()
	pgStore = initPostgres(connStr)
	
	// Инициализация P2P
	nodeManager = p2p.NewNodeManager()
	loadBalancer = p2p.NewLoadBalancer()
	go nodeManager.Start()
	
	// Инициализация менеджеров
	quotaManager = storage.NewQuotaManager()
	deleteManager = storage.NewDeletionManager()
	
	// Запуск API
	StartAPIServer(storage.NewBadgerStore("data"))
}

func initPostgres(connStr string) *storage.PostgresStore {
	store, err := storage.NewPostgresStore(connStr)
	if err != nil {
		panic(fmt.Sprintf("Postgres init failed: %v", err))
	}
	if err := store.Migrate(); err != nil {
		panic(fmt.Sprintf("Migration failed: %v", err))
	}
	return store
}

func StartAPIServer(store *storage.BadgerStore) {
	r := gin.Default()
	
	// Метрики Prometheus
	r.GET("/metrics", gin.WrapH(metrics.Handler()))
	
	// Аутентификация
	r.POST("/login", loginHandler(store))
	
	// Основные эндпоинты
	api := r.Group("/v1")
	api.Use(authMiddleware(store))
	{
		api.PUT("/objects/:key", putObjectHandler(store))
		api.GET("/objects/:hash", getObjectHandler(store))
		api.DELETE("/objects/:hash", deleteObjectHandler(store))
	}
	
	// Администрирование
	admin := api.Group("/admin")
	admin.Use(adminAuthMiddleware())
	{
		admin.GET("/nodes", listNodesHandler)
		admin.POST("/nodes/restart", restartNodeHandler)
		admin.PUT("/users/:id/quota", updateQuotaHandler)
		admin.GET("/stats", systemStatsHandler)
	}
	
	// Запуск сервера
	port := getPort()
	r.Run(":" + port)
}

// Обработчики запросов
func loginHandler(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var creds struct{ Username, Password string }
		if err := c.BindJSON(&creds); err != nil {
			sendError(c, http.StatusBadRequest, "Invalid request format")
			return
		}
		
		token, err := auth.GenerateToken(creds.Username, store.PrivateKey())
		if err != nil {
			sendError(c, http.StatusInternalServerError, "Token generation failed")
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func putObjectHandler(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		defer metrics.ObserveRequest(c, start)
		
		// Проверка квоты
		userID := c.MustGet("user_id").(string)
		data, _ := c.GetRawData()
		
		if !quotaManager.CheckQuota(userID, int64(len(data))) {
			sendError(c, http.StatusTooManyRequests, "Quota exceeded")
			return
		}
		
		// Балансировка нагрузки
		targetNodes := loadBalancer.SelectNodes(3)
		if len(targetNodes) == 0 {
			sendError(c, http.StatusServiceUnavailable, "No available nodes")
			return
		}
		
		// Сохранение данных
		hash, err := store.PutBlock(data)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "Storage error")
			return
		}
		
		// Репликация
		go p2p.ReplicateToNodes(hash, data, targetNodes)
		quotaManager.UpdateUsage(userID, int64(len(data)))
		
		c.JSON(http.StatusOK, gin.H{
			"hash": hash,
			"nodes": targetNodes,
		})
	}
}

func deleteObjectHandler(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		hash := c.Param("hash")
		if err := deleteManager.ScheduleDeletion(hash, 5*time.Minute); err != nil {
			sendError(c, http.StatusConflict, "Deletion already scheduled")
			return
		}
		c.Status(http.StatusAccepted)
	}
}

// Middleware
func authMiddleware(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		claims, err := auth.ValidateToken(token, store.PublicKey())
		if err != nil {
			sendError(c, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

func adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("user_id").(string)
		if !isAdmin(userID) {
			sendError(c, http.StatusForbidden, "Admin access required")
			return
		}
		c.Next()
	}
}

// Вспомогательные функции
func getPostgresURL() string {
	if url := os.Getenv("POSTGRES_URL"); url != "" {
		return url
	}
	return "postgres://user:pass@localhost:5432/secure-dag?sslmode=disable"
}

func getPort() string {
	if port := os.Getenv("API_PORT"); port != "" {
		return port
	}
	return "8080"
}

func isAdmin(userID string) bool {
	var role string
	err := pgStore.db.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	return err == nil && role == "admin"
}

func sendError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{"error": message})
}

// Административные обработчики
func listNodesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"nodes": nodeManager.GetNodes()})
}

func restartNodeHandler(c *gin.Context) {
	nodeID := c.Query("id")
	if err := nodeManager.RestartNode(nodeID); err != nil {
		sendError(c, http.StatusNotFound, "Node not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func updateQuotaHandler(c *gin.Context) {
	userID := c.Param("id")
	var req struct{ Quota int64 `json:"quota"` }
	
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request format")
		return
	}
	
	if err := quotaManager.SetQuota(userID, req.Quota); err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update quota")
		return
	}
	
	c.Status(http.StatusOK)
}

func systemStatsHandler(c *gin.Context) {
	stats := gin.H{
		"nodes":       nodeManager.GetNodeStats(),
		"storage":     storage.GetGlobalUsage(),
		"throughput":  metrics.GetThroughput(),
		"active_jobs": deleteManager.ActiveOperations(),
	}
	c.JSON(http.StatusOK, stats)
}
