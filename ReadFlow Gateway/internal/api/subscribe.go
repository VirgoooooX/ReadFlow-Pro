package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/db"
)

// SubscribeHandler 订阅管理处理器
type SubscribeHandler struct {
	db *db.DB
}

// NewSubscribeHandler 创建订阅处理器
func NewSubscribeHandler(database *db.DB) *SubscribeHandler {
	return &SubscribeHandler{db: database}
}

// SubscribeRequest 订阅请求
type SubscribeRequest struct {
	URL   string `json:"url" binding:"required"`
	Title string `json:"title"`
}

// SubscribeResponse 订阅响应
type SubscribeResponse struct {
	Success     bool   `json:"success"`
	SourceID    int64  `json:"source_id,omitempty"`
	IsNewSource bool   `json:"is_new_source,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Subscribe 订阅源
func (h *SubscribeHandler) Subscribe(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, SubscribeResponse{
			Success: false,
			Message: "无效的请求参数",
		})
		return
	}

	// 检查源是否已存在
	source, err := h.db.GetSourceByURL(req.URL)
	isNewSource := false

	if err == sql.ErrNoRows {
		// 创建新源
		source, err = h.db.CreateSource(req.URL, req.Title, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, SubscribeResponse{
				Success: false,
				Message: "创建源失败",
			})
			return
		}
		isNewSource = true
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, SubscribeResponse{
			Success: false,
			Message: "查询源失败",
		})
		return
	}

	// 创建订阅关系
	if err := h.db.CreateSubscription(userID, source.ID); err != nil {
		c.JSON(http.StatusInternalServerError, SubscribeResponse{
			Success: false,
			Message: "订阅失败",
		})
		return
	}

	c.JSON(http.StatusOK, SubscribeResponse{
		Success:     true,
		SourceID:    source.ID,
		IsNewSource: isNewSource,
		Message:     "订阅成功",
	})
}

// Unsubscribe 取消订阅
func (h *SubscribeHandler) Unsubscribe(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	sourceIDStr := c.Param("source_id")
	sourceID, err := strconv.ParseInt(sourceIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的源 ID",
		})
		return
	}

	// 删除订阅关系
	if err := h.db.DeleteSubscription(userID, sourceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "取消订阅失败",
		})
		return
	}

	// 检查是否还有其他用户订阅
	count, err := h.db.GetSubscriptionCount(sourceID)
	if err != nil {
		// 忽略错误，继续
	} else if count == 0 {
		// 没有其他用户订阅，标记为非活跃
		_ = h.db.UpdateSourceActive(sourceID, false)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "取消订阅成功",
	})
}

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	SourceID      int64  `json:"source_id"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	SubscribedAt  string `json:"subscribed_at"`
	UnreadCount   int    `json:"unread_count"`
	LastFetchTime string `json:"last_fetch_time,omitempty"`
}

// GetSubscriptions 获取订阅列表
func (h *SubscribeHandler) GetSubscriptions(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	sources, err := h.db.GetUserSubscriptions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询失败",
		})
		return
	}

	// 构建响应
	subscriptions := make([]SubscriptionInfo, 0, len(sources))
	for _, source := range sources {
		unreadCount, _ := h.db.GetUnreadCount(userID, source.ID)
		
		info := SubscriptionInfo{
			SourceID:    source.ID,
			URL:         source.URL,
			Title:       source.Title,
			UnreadCount: unreadCount,
		}
		
		if source.LastFetchTime != nil {
			info.LastFetchTime = source.LastFetchTime.Format("2006-01-02T15:04:05Z")
		}
		
		subscriptions = append(subscriptions, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"subscriptions": subscriptions,
	})
}
