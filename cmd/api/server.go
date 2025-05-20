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
	quotaManager *storage.QuotaManager
	pgStore      *storage.PostgresStore
	nodeManager  *p2p.NodeManager
)

func main() {
	metrics.Init()
	
	// Инициализация PostgreSQL
	connStr := os.Getenv("POSTGRES_URL")
	pgStore = initPostgres(connStr)
	
	// Инициализация P2P
	nodeManager = p2p.NewNodeManager()
	go nodeManager.Start()
	
	// Запуск API
	StartAPIServer(storage.NewBadgerStore("data"))
}

func initPostgres(connStr string) *storage.PostgresStore {
	store, err := storage.NewPostgresStore(connStr)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to PostgreSQL: %v", err))
	}
	if err := store.Migrate(); err != nil {
		panic(fmt.Sprintf("Migration failed: %v", err))
	}
	return store
}

func StartAPIServer(store *storage.BadgerStore) {
	r := gin.Default()
	
	// Метрики
	r.GET("/metrics", gin.WrapH(metrics.Handler()))
	
	// Аутентификация
	r.POST("/login", loginHandler(store))
	
	// Авторизованные эндпоинты
	authGroup := r.Group("/")
	authGroup.Use(authMiddleware(store))
	{
		authGroup.PUT("/objects/:key", putObjectHandler(store))
		authGroup.GET("/objects/:hash", getObjectHandler(store))
	}
	
	// Админские эндпоинты
	adminGroup := authGroup.Group("/admin")
	adminGroup.Use(adminAuthMiddleware()))
	{
		adminGroup.GET("/nodes", listNodesHandler)
		adminGroup.POST("/nodes/restart", restartNodeHandler)
		adminGroup.GET("/stats", systemStatsHandler)
	}

	port := getPort()
	r.Run(":" + port)
}

// Обработчики
func loginHandler(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var creds struct{ Username, Password string }
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
	}
}

func authMiddleware(store *storage.BadgerStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		claims, err := auth.ValidateToken(token, store.PublicKey())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.Next()
	}
}

// Реализация P2P функций в internal/p2p/network.go
package p2p

import (
	"context"
	"sync"
	
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p-kad-dht"
)

type NodeManager struct {
	host  host.Host
	dht   *dht.IpfsDHT
	nodes map[string]*Node
	mu    sync.RWMutex
}

type Node struct {
	ID      string
	Address string
	Status  string
}

func NewNodeManager() *NodeManager {
	return &NodeManager{
		nodes: make(map[string]*Node),
	}
}

func (m *NodeManager) Start() {
	ctx := context.Background()
	var err error
	
	m.host, err = libp2p.New()
	if err != nil {
		panic(err)
	}
	
	m.dht, err = dht.New(ctx, m.host)
	if err != nil {
		panic(err)
	}
	
	go m.discoverNodes(ctx)
}

func (m *NodeManager) discoverNodes(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			peers := m.dht.RoutingTable().ListPeers()
			m.mu.Lock()
			for _, pid := range peers {
				id := pid.String()
				if _, exists := m.nodes[id]; !exists {
					m.nodes[id] = &Node{
						ID:      id,
						Address: fmt.Sprintf("%s/p2p/%s", m.host.Addrs()[0], id),
						Status:  "active",
					}
				}
			}
			m.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (m *NodeManager) GetNodes() []Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	nodes := make([]Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, *n)
	}
	return nodes
}

func (m *NodeManager) RestartNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if node, exists := m.nodes[nodeID]; exists {
		node.Status = "restarting"
		go func() {
			time.Sleep(5 * time.Second) // Имитация перезапуска
			node.Status = "active"
		}()
		return nil
	}
	return fmt.Errorf("node %s not found", nodeID)
}

// Вспомогательные функции
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

func listNodesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"nodes": nodeManager.GetNodes()})
}

func restartNodeHandler(c *gin.Context) {
	nodeID := c.Query("id")
	if err := nodeManager.RestartNode(nodeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func systemStatsHandler(c *gin.Context) {
	stats := gin.H{
		"nodes":    len(nodeManager.GetNodes()),
		"storage":  storage.GetUsageStats(),
		"requests": metrics.GetRequestStats(),
	}
	c.JSON(http.StatusOK, stats)
}
