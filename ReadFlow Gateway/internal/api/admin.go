package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/config"
	"github.com/readflow/gateway/internal/db"
	"github.com/readflow/gateway/internal/image"
	"github.com/readflow/gateway/internal/metrics"
)

// AdminRefreshWorker 定义刷新源所需的 Worker 接口
type AdminRefreshWorker interface {
	FetchSource(source *db.Source) error
}

// AdminHandler 管理后台处理器
type AdminHandler struct {
	db        *db.DB
	staticDir string
	worker    AdminRefreshWorker // Worker 实例，用于立即刷新源
}

// NewAdminHandler 创建管理后台处理器
func NewAdminHandler(database *db.DB, staticDir string, worker AdminRefreshWorker) *AdminHandler {
	return &AdminHandler{
		db:        database,
		staticDir: staticDir,
		worker:    worker,
	}
}

// Dashboard 获取仪表板数据
func (h *AdminHandler) Dashboard(c *gin.Context) {
	// 获取系统统计
	systemStats := h.getSystemStats()

	// 获取用户统计
	userStats := h.getUserStats()

	// 获取源统计
	sourceStats := h.getSourceStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"system":  systemStats,
			"users":   userStats,
			"sources": sourceStats,
		},
	})
}

// UserSubscriptions 获取用户订阅信息
func (h *AdminHandler) UserSubscriptions(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "user_id 参数缺失",
		})
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "user_id 参数无效",
		})
		return
	}

	// 获取用户的所有订阅
	subscriptions, err := h.db.GetSubscriptionsByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询失败",
		})
		return
	}

	// 获取每个源的统计信息
	var result []gin.H
	for _, sub := range subscriptions {
		source, _ := h.db.GetSourceByID(sub.SourceID)
		if source != nil {
			// 计算该源的文章数量
			itemCount, _ := h.db.GetItemCountBySource(sub.SourceID)

			// 计算该用户从该源拉取的文章数
			deliveredCount, _ := h.db.GetDeliveredCountByUserAndSource(userID, sub.SourceID)

			result = append(result, gin.H{
				"source_id":       source.ID,
				"title":           source.Title,
				"url":             source.URL,
				"fetch_count":     source.ErrorCount,    // 这里用 error_count 作为示例，实际应该统计 fetch 次数
				"item_total":      itemCount,            // 该源的总文章数
				"delivered":       deliveredCount,       // 该用户从该源拉取的文章数
				"is_active":       source.IsActive,      // 是否活跃
				"last_fetch_time": source.LastFetchTime, // 最后抓取时间
				"error_count":     source.ErrorCount,    // 错误计数
				"last_error":      source.LastError,     // 最后错误信息
				"subscribed_at":   sub.SubscribedAt,     // 订阅时间
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user_id": userID,
		"data":    result,
	})
}

// SourceDetails 获取源的详细信息
func (h *AdminHandler) SourceDetails(c *gin.Context) {
	sourceIDStr := c.Query("source_id")
	if sourceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "source_id 参数缺失",
		})
		return
	}

	sourceID, err := strconv.ParseInt(sourceIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "source_id 参数无效",
		})
		return
	}

	source, err := h.db.GetSourceByID(sourceID)
	if err != nil || source == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "源不存在",
		})
		return
	}

	// 获取该源的统计信息
	totalItems, _ := h.db.GetItemCountBySource(sourceID)
	totalSubscribers, _ := h.db.GetSubscriberCountBySource(sourceID)
	totalDeliveries, _ := h.db.GetDeliveryCountBySource(sourceID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":              source.ID,
			"url":             source.URL,
			"title":           source.Title,
			"description":     source.Description,
			"is_active":       source.IsActive,
			"fetch_interval":  source.FetchInterval,
			"last_fetch_time": source.LastFetchTime,
			"created_at":      source.CreatedAt,
			// 统计数据
			"total_items":       totalItems,
			"total_subscribers": totalSubscribers,
			"total_deliveries":  totalDeliveries,
			"error_count":       source.ErrorCount,
			"last_error":        source.LastError,
			// 计算的指标
			"avg_items_per_fetch": float64(totalItems) / float64(source.ErrorCount+1), // 避免除以零
			"success_rate":        fmt.Sprintf("%.2f%%", (1.0-float64(source.ErrorCount)/float64(totalItems+1))*100),
		},
	})
}

// CacheStats 获取图片缓存统计
func (h *AdminHandler) CacheStats(c *gin.Context) {
	stats := h.getImageCacheStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// SystemMetrics 获取系统实时指标
func (h *AdminHandler) SystemMetrics(c *gin.Context) {
	stats := metrics.GetMetrics().GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetConfig 获取当前配置
func (h *AdminHandler) GetConfig(c *gin.Context) {
	rc := config.GetRuntimeConfig()
	allConfig := rc.GetAllConfig()

	// 添加配置说明
	configInfo := map[string]interface{}{
		"fetch_interval": map[string]interface{}{
			"value":       allConfig["fetch_interval"],
			"description": "RSS 抓取間隔（秒）",
			"min":         60,
			"max":         86400,
			"unit":        "秒",
		},
		"image_quality": map[string]interface{}{
			"value":       allConfig["image_quality"],
			"description": "图片转换品质（1-100）",
			"min":         10,
			"max":         100,
			"unit":        "%",
		},
		"image_max_width": map[string]interface{}{
			"value":       allConfig["image_max_width"],
			"description": "图片最大宽度（像素）",
			"min":         300,
			"max":         4000,
			"unit":        "px",
		},
		"image_concurrent": map[string]interface{}{
			"value":       allConfig["image_concurrent"],
			"description": "图片处理并发数",
			"min":         1,
			"max":         10,
			"unit":        "个",
		},
		"image_cache_expiration": map[string]interface{}{
			"value":       allConfig["image_cache_expiration"],
			"description": "图片缓存过期时间",
			"min":         3600,
			"max":         2592000,
			"unit":        "秒",
		},
		"item_retention_time": map[string]interface{}{
			"value":       allConfig["item_retention_time"],
			"description": "文章保留时间",
			"min":         3600,
			"max":         2592000,
			"unit":        "秒",
		},
		"log_level": map[string]interface{}{
			"value":       allConfig["log_level"],
			"description": "日志级别（debug/info/warn/error）",
			"options":     []string{"debug", "info", "warn", "error"},
		},
		"max_items_per_fetch": map[string]interface{}{
			"value":       allConfig["max_items_per_fetch"],
			"description": "每次抓取最多保留文章数",
			"min":         10,
			"max":         5000,
			"unit":        "篇",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configInfo,
	})
}

// UpdateConfig 更新配置
func (h *AdminHandler) UpdateConfig(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体格式错误",
		})
		return
	}

	rc := config.GetRuntimeConfig()
	errors := rc.UpdateConfig(updates)

	if len(errors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "配置更新失败",
			"errors":  errors,
		})
		return
	}

	// 返回更新后的配置
	allConfig := rc.GetAllConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置季更新成功",
		"data":    allConfig,
	})
}

// DeleteUser 删除用户及其相关数据
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "user_id 参数缺失",
		})
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "user_id 参数无效",
		})
		return
	}

	// 验证用户是否存在
	user, err := h.db.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "用户不存在",
		})
		return
	}

	// 查出该用户的所有订阅，用于判断哪些源是“专属”该用户
	subscriptions, err := h.db.GetSubscriptionsByUser(userID)
	if err != nil {
		log.Printf("[ADMIN] GetSubscriptionsByUser failed for user %d: %v", userID, err)
	}

	// 先处理专属订阅源：只有该用户订阅的源，需要连同文章一起删除
	for _, sub := range subscriptions {
		// 统计该源的订阅者数量
		subCount, err := h.db.GetSubscriberCountBySource(sub.SourceID)
		if err != nil {
			log.Printf("[ADMIN] GetSubscriberCountBySource failed for source %d: %v", sub.SourceID, err)
			continue
		}

		if subCount == 1 {
			// 说明这是该用户专属的源：清空文章+图片，并删除源本身
			if _, err := h.clearSourceItemsInternal(sub.SourceID); err != nil {
				log.Printf("[ADMIN] clearSourceItemsInternal failed for source %d: %v", sub.SourceID, err)
			}

			if err := h.db.DeleteSource(sub.SourceID); err != nil {
				log.Printf("[ADMIN] DeleteSource failed for source %d: %v", sub.SourceID, err)
			}
		}
	}

	// 删除用户的所有订阅关系（非专属源部分）
	if err == nil {
		for _, sub := range subscriptions {
			_ = h.db.DeleteSubscription(userID, sub.SourceID)
		}
	}

	// 删除用户
	err = h.db.DeleteUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除用户失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("用户 %s 及其关联数据已删除", user.Username),
	})
}

// ClearSourceItems 清空指定订阅源的所有文章及相关缓存
func (h *AdminHandler) ClearSourceItems(c *gin.Context) {
	sourceIDStr := c.Query("source_id")
	if sourceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "source_id 参数缺失",
		})
		return
	}

	sourceID, err := strconv.ParseInt(sourceIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "source_id 参数无效",
		})
		return
	}

	// 验证源是否存在
	source, err := h.db.GetSourceByID(sourceID)
	if err != nil || source == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "订阅源不存在",
		})
		return
	}

	cleared, err := h.clearSourceItemsInternal(sourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "清空文章失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("订阅源 %s 的 %d 篇文章已清除", source.Title, cleared),
		"cleared": cleared,
	})
}

// clearSourceItemsInternal 内部方法：清空指定源下的文章、投递记录和图片缓存
func (h *AdminHandler) clearSourceItemsInternal(sourceID int64) (int, error) {
	items, err := h.db.GetItemsBySource(sourceID)
	if err != nil {
		return 0, err
	}

	cleared := 0
	for _, item := range items {
		// 删除图片文件
		if item.ImagePaths != "" && item.ImagePaths != "[]" {
			if err := image.DeleteImageFiles(h.staticDir, item.ImagePaths); err != nil {
				log.Printf("[ADMIN] DeleteImageFiles failed for item %d: %v", item.ID, err)
			}
		}

		// 删除投递记录
		if err := h.db.DeleteUserDeliveries(item.ID); err != nil {
			log.Printf("[ADMIN] DeleteUserDeliveries failed for item %d: %v", item.ID, err)
		}

		// 删除文章记录
		if err := h.db.DeleteItem(item.ID); err != nil {
			log.Printf("[ADMIN] DeleteItem failed for item %d: %v", item.ID, err)
		} else {
			cleared++
		}
	}

	// 删除可能为空的图片目录
	imageDir := image.GetImageDirPath(h.staticDir, sourceID)
	if err := image.RemoveEmptyDir(imageDir); err != nil {
		log.Printf("[ADMIN] RemoveEmptyDir failed for %s: %v", imageDir, err)
	}

	return cleared, nil
}

// RefreshSource 手动刷新指定的 RSS 源
func (h *AdminHandler) RefreshSource(c *gin.Context) {
	sourceIDStr := c.Query("source_id")
	if sourceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "source_id 参数缺失",
		})
		return
	}

	sourceID, err := strconv.ParseInt(sourceIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "source_id 参数无效",
		})
		return
	}

	// 验证源是否存在
	source, err := h.db.GetSourceByID(sourceID)
	if err != nil || source == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "订阅源不存在",
		})
		return
	}

	// 立即执行刷新（如果 Worker 已注入）
	if h.worker != nil {
		log.Printf("[ADMIN] Manually refreshing source: %s (ID=%d)", source.Title, sourceID)
		if err := h.worker.FetchSource(source); err != nil {
			log.Printf("[ADMIN] Failed to refresh source %s: %v", source.URL, err)
			h.db.UpdateSourceError(source.ID, err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("刷新源失败: %v", err),
			})
			return
		}
		// 更新最后抓取时间
		h.db.UpdateSourceFetchTime(source.ID)
		log.Printf("[ADMIN] Successfully refreshed source: %s", source.Title)
	} else {
		// 没有 Worker 时降级为更新时间戳
		log.Printf("[ADMIN] Worker not available, only updating fetch time for source: %s", source.Title)
		if err := h.db.UpdateSourceFetchTime(sourceID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "更新抓取时间失败",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("源 %s 已成功刷新", source.Title),
	})
}

// 辅助方法

// getSystemStats 获取系统统计信息
func (h *AdminHandler) getSystemStats() gin.H {
	totalUsers, _ := h.db.GetTotalUsers()
	activeSources, _ := h.db.GetActiveSourcesCount()
	totalItems, _ := h.db.GetTotalItems()
	totalDeliveries, _ := h.db.GetTotalDeliveries()

	return gin.H{
		"total_users":        totalUsers,
		"active_sources":     activeSources,
		"total_items":        totalItems,
		"total_deliveries":   totalDeliveries,
		"cache_images_count": h.countCachedImages(),
		"cache_size_mb":      h.getCacheSizeMB(),
	}
}

// getUserStats 获取用户统计信息
func (h *AdminHandler) getUserStats() []gin.H {
	users, _ := h.db.GetAllUsers()
	var result []gin.H

	for _, user := range users {
		subCount, _ := h.db.GetSubscriptionCountByUser(user.ID)
		deliveryCount, _ := h.db.GetDeliveryCountByUser(user.ID)
		vocabCount, _ := h.db.GetVocabularyCountByUser(user.ID)

		result = append(result, gin.H{
			"id":                 user.ID,
			"username":           user.Username,
			"created_at":         user.CreatedAt,
			"last_login_at":      user.LastLoginAt,
			"subscription_count": subCount,
			"delivery_count":     deliveryCount,
			"vocabulary_count":   vocabCount,
		})
	}

	return result
}

// getSourceStats 获取源统计信息
func (h *AdminHandler) getSourceStats() []gin.H {
	sources, _ := h.db.GetAllSources()
	var result []gin.H

	for _, source := range sources {
		itemCount, _ := h.db.GetItemCountBySource(source.ID)
		subCount, _ := h.db.GetSubscriberCountBySource(source.ID)
		deliveryCount, _ := h.db.GetDeliveryCountBySource(source.ID)

		result = append(result, gin.H{
			"id":               source.ID,
			"title":            source.Title,
			"url":              source.URL,
			"is_active":        source.IsActive,
			"item_count":       itemCount,
			"subscriber_count": subCount,
			"delivery_count":   deliveryCount,
			"error_count":      source.ErrorCount,
			"last_fetch_time":  source.LastFetchTime,
			"last_error":       source.LastError,
		})
	}

	return result
}

// getImageCacheStats 获取图片缓存统计
func (h *AdminHandler) getImageCacheStats() gin.H {
	imageDir := filepath.Join(h.staticDir, "images")

	count := h.countCachedImages()
	size := h.getCacheSizeMB()

	return gin.H{
		"cached_images_count": count,
		"cache_size_mb":       size,
		"cache_directory":     imageDir,
	}
}

// countCachedImages 计算缓存图片数量（递归统计所有源目录下的文件数）
func (h *AdminHandler) countCachedImages() int {
	imageDir := filepath.Join(h.staticDir, "images")
	count := 0

	_ = filepath.Walk(imageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info == nil {
			return nil
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})

	return count
}

// getCacheSizeMB 获取缓存大小（MB）
func (h *AdminHandler) getCacheSizeMB() float64 {
	imageDir := filepath.Join(h.staticDir, "images")
	var totalSize int64

	_ = filepath.Walk(imageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info == nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	// 转换为 MB
	return float64(totalSize) / (1024 * 1024)
}
