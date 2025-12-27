# JSON API 路由集成指南

## 概述

本文档说明如何将新创建的JSON API端点集成到服务器的路由配置中。

## 路由注册示例

在服务器主文件（通常是 `cmd/server/main.go` 或路由配置文件）中，添加以下路由：

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/api"
	"github.com/readflow/gateway/internal/db"
	// ... 其他导入
)

func setupRoutes(router *gin.Engine, database *db.DB, worker *worker.Worker) {
	// 创建JSON API处理器
	articlesJSONHandler := api.NewArticlesJSONHandler(database)

	// API v1 路由组（JSON格式）
	v1 := router.Group("/api/v1")
	{
		// 文章相关路由
		articles := v1.Group("/articles")
		{
			// 文章列表（支持分页、过滤、增量同步）
			// GET /api/v1/articles?source_id=1&limit=50&cursor=xxx
			articles.GET("", articlesJSONHandler.ListArticles)
			
			// 文章详情
			// GET /api/v1/articles/:id
			articles.GET("/:id", articlesJSONHandler.GetArticle)
			
			// 标记为已读
			// POST /api/v1/articles/:id/read
			articles.POST("/:id/read", articlesJSONHandler.MarkAsRead)
			
			// 切换收藏状态
			// POST /api/v1/articles/:id/favorite
			articles.POST("/:id/favorite", articlesJSONHandler.ToggleFavorite)
			
			// 更新阅读进度
			// POST /api/v1/articles/:id/progress
			// Body: {"progress": 50}
			articles.POST("/:id/progress", articlesJSONHandler.UpdateReadProgress)
		}
	}

	// 保持现有的同步API（XML格式，向后兼容）
	syncHandler := api.NewSyncHandler(database, worker)
	router.GET("/api/sync", syncHandler.Sync)
	router.GET("/api/rss", syncHandler.Sync) // 别名
}
```

## API使用示例

### 1. 获取文章列表

**请求**：
```http
GET /api/v1/articles?limit=20&source_id=1
Authorization: Bearer <token>
```

**响应**：
```json
{
  "articles": [
    {
      "id": 123,
      "source_id": 1,
      "guid": "article-guid-123",
      "title": "Example Article",
      "summary": "Article summary...",
      "content": "<html>Full content...</html>",
      "author": "John Doe",
      "cover_image": "/static/images/1/abc123.webp",
      "word_count": 500,
      "reading_time": 2,
      "published_at": "2024-01-01T12:00:00Z",
      "created_at": "2024-01-01T12:05:00Z",
      "source_title": "Tech Blog",
      "source_url": "https://example.com/feed",
      "status": 0,
      "is_favorite": false,
      "read_progress": 0,
      "updated_at": "2024-01-01T12:05:00Z"
    }
  ],
  "next_cursor": "encoded_cursor_string",
  "total": 20
}
```

### 2. 获取文章详情

**请求**：
```http
GET /api/v1/articles/123
Authorization: Bearer <token>
```

**响应**：
```json
{
  "article": {
    "id": 123,
    "title": "Example Article",
    "content": "<html>Full HTML content...</html>",
    ...
  }
}
```

### 3. 标记文章为已读

**请求**：
```http
POST /api/v1/articles/123/read
Authorization: Bearer <token>
```

**响应**：
```json
{
  "message": "已标记为已读"
}
```

### 4. 切换收藏状态

**请求**：
```http
POST /api/v1/articles/123/favorite
Authorization: Bearer <token>
```

**响应**：
```json
{
  "is_favorite": true,
  "message": "收藏状态已更新"
}
```

### 5. 更新阅读进度

**请求**：
```http
POST /api/v1/articles/123/progress
Authorization: Bearer <token>
Content-Type: application/json

{
  "progress": 75
}
```

**响应**：
```json
{
  "message": "阅读进度已更新"
}
```

## 查询参数说明

### ListArticles 查询参数

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `source_id` | integer | 否 | 按源ID过滤 |
| `since` | integer | 否 | 增量同步时间戳（Unix秒） |
| `cursor` | string | 否 | 分页游标 |
| `limit` | integer | 否 | 返回数量（默认50，最大200） |
| `offset` | integer | 否 | 偏移量（仅在无cursor/since时使用） |

## 认证说明

所有API端点都需要认证。请确保：
1. 在Gin中间件中设置用户ID：`c.Set("user_id", userID)`
2. API将通过 `c.Get("user_id")` 获取当前用户
3. 使用Bearer token或其他认证方式

```go
// 认证中间件示例
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		// 验证token并获取userID
		userID, err := validateToken(token)
		if err != nil {
			c.JSON(401, gin.H{"error": "未授权"})
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

// 在路由中使用
v1 := router.Group("/api/v1")
v1.Use(AuthMiddleware())
{
	// ... routes
}
```

## 向后兼容性

现有的XML同步API（`/api/sync`）将继续工作，不会影响旧客户端。

新客户端可以使用JSON API获得更好的性能和更丰富的数据。

## 下一步

1. 在主服务器文件中集成这些路由
2. 配置认证中间件
3. 测试所有端点
4. 更新客户端代码以使用JSON API
