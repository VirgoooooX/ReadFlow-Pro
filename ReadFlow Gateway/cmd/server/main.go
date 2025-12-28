package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/api"
	"github.com/readflow/gateway/internal/config"
	"github.com/readflow/gateway/internal/db"
	"github.com/readflow/gateway/internal/worker"
)

func main() {
	// 解析命令行参数
	flag.String("config", ".env", "Configuration file path")
	flag.Parse()

	// 加载配置
	cfg := config.Load()
	log.Printf("[INFO] Configuration loaded - DB: %s, Port: %s", cfg.DBPath, cfg.ServerPort)

	// 初始化数据库
	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize database: %v", err)
	}
	defer database.Close()
	log.Println("[INFO] Database initialized successfully")

	// 初始化 Gin 路由
	router := setupRoutes(cfg, database, nil)

	// 启动 RSS Worker（后台任务）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := worker.New(database, cfg)
	go w.Start(ctx)
	log.Printf("[INFO] RSS Worker started with interval: %d seconds", cfg.FetchInterval)

	// 更新路由中的 Worker 引用
	router = setupRoutes(cfg, database, w)

	// 启动 HTTP 服务器
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	// 优雅关闭处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[INFO] Shutdown signal received, gracefully shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("[ERROR] Server shutdown error: %v", err)
		}
	}()

	// 启动服务器
	log.Printf("[INFO] 🚀 ReadFlow Gateway Server starting on http://localhost:%s", cfg.ServerPort)
	log.Printf("[INFO] Admin Panel: http://localhost:%s/admin", cfg.ServerPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[ERROR] Server error: %v", err)
	}
	log.Println("[INFO] Server stopped")
}

// setupRoutes 设置所有路由
func setupRoutes(cfg *config.Config, database *db.DB, w *worker.Worker) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 添加请求详细日志
	router.Use(func(c *gin.Context) {
		log.Printf("[REQ] %s %s from %s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
		c.Next()
	})

	// 添加 CORS 中间件
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 创建服务实例
	authService := api.NewAuthService(database, cfg)
	syncHandler := api.NewSyncHandler(database, w)
	subscribeHandler := api.NewSubscribeHandler(database)
	ackHandler := api.NewAckHandler(database, cfg.StaticDir)
	vocabHandler := api.NewVocabHandler(database)
	adminHandler := api.NewAdminHandler(database, cfg.StaticDir, w) // 注入 Worker 用于立即刷新
	articleHandler := api.NewArticleHandler(database)

	// 认证 API
	authGroup := router.Group("/api/auth")
	{
		authGroup.POST("/login", authService.Login)
		authGroup.POST("/register", authService.Register)
	}

	// 用户 API（需要认证）
	userGroup := router.Group("/api/user")
	userGroup.Use(authService.AuthMiddleware())
	{
		userGroup.POST("/profile", authService.UpdateProfile)
	}

	// 订阅 API（需要认证）
	subscribeGroup := router.Group("/api")
	subscribeGroup.Use(authService.AuthMiddleware())
	{
		subscribeGroup.POST("/subscribe", subscribeHandler.Subscribe)
		subscribeGroup.DELETE("/subscribe/:source_id", subscribeHandler.Unsubscribe)
		subscribeGroup.GET("/subscriptions", subscribeHandler.GetSubscriptions)
	}

	// 同步 API（需要认证）
	syncGroup := router.Group("/api")
	syncGroup.Use(authService.AuthMiddleware())
	{
		syncGroup.GET("/sync", syncHandler.Sync)
	}

	// 文章 API（需要认证）
	articleGroup := router.Group("/api")
	articleGroup.Use(authService.AuthMiddleware())
	{
		// 文章查询
		articleGroup.GET("/articles", articleHandler.ListArticles)
		articleGroup.GET("/articles/:id", articleHandler.GetArticleDetail)
		// Quest 5: 阅读状态管理
		articleGroup.POST("/articles/:id/read", articleHandler.MarkArticleRead)
		articleGroup.DELETE("/articles/:id/read", articleHandler.MarkArticleUnread)
		articleGroup.POST("/articles/:id/favorite", articleHandler.AddFavorite)
		articleGroup.DELETE("/articles/:id/favorite", articleHandler.RemoveFavorite)
		articleGroup.PUT("/articles/:id/progress", articleHandler.UpdateArticleProgress)
	}

	// 确认 API（需要认证）
	ackGroup := router.Group("/api")
	ackGroup.Use(authService.AuthMiddleware())
	{
		ackGroup.POST("/ack", ackHandler.Acknowledge)
	}

	// 生词本 API（需要认证）
	vocabGroup := router.Group("/api/vocab")
	vocabGroup.Use(authService.AuthMiddleware())
	{
		vocabGroup.POST("/push", vocabHandler.Push)
		vocabGroup.GET("/pull", vocabHandler.Pull)
	}

	// 管理后台 Web UI（无需认证）
	router.GET("/admin", func(c *gin.Context) {
		c.File("internal/api/admin.html")
	})

	// 静态文件服务（图片缓存）
	router.Static("/static", cfg.StaticDir)

	// 管理 API - 无需认证
	adminGroup := router.Group("/api/admin")
	{
		adminGroup.GET("/dashboard", adminHandler.Dashboard)
		adminGroup.GET("/users", adminHandler.UserSubscriptions)
		adminGroup.GET("/sources", adminHandler.SourceDetails)
		adminGroup.GET("/cache-stats", adminHandler.CacheStats)
		adminGroup.GET("/metrics", adminHandler.SystemMetrics)
		// 配置管理接口
		adminGroup.GET("/config", adminHandler.GetConfig)
		adminGroup.POST("/config", adminHandler.UpdateConfig)
		// 用户管理接口
		adminGroup.DELETE("/users", adminHandler.DeleteUser)
		// 源管理接口
		adminGroup.POST("/sources/refresh", adminHandler.RefreshSource)
		adminGroup.POST("/sources/clear-items", adminHandler.ClearSourceItems)
	}

	// 健康检查 (支持 GET 和 HEAD)
	router.Match([]string{"GET", "HEAD"}, "/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"time":   time.Now(),
		})
	})

	return router
}
