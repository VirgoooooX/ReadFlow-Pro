package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/db"
	"github.com/readflow/gateway/internal/image"
)

// AckHandler ACK 确认处理器
type AckHandler struct {
	db        *db.DB
	staticDir string
}

// NewAckHandler 创建 ACK 处理器
func NewAckHandler(database *db.DB, staticDir string) *AckHandler {
	return &AckHandler{
		db:        database,
		staticDir: staticDir,
	}
}

// AckRequest ACK 请求
type AckRequest struct {
	ItemIDs []int64 `json:"item_ids" binding:"required"`
}

// AckResponse ACK 响应
type AckResponse struct {
	Success      bool   `json:"success"`
	Acknowledged int    `json:"acknowledged"`
	Cleaned      int    `json:"cleaned"`
	Message      string `json:"message,omitempty"`
}

// Acknowledge 确认文章已接收
func (h *AckHandler) Acknowledge(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	var req AckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AckResponse{
			Success: false,
			Message: "无效的请求参数",
		})
		return
	}

	if len(req.ItemIDs) == 0 {
		c.JSON(http.StatusOK, AckResponse{
			Success:      true,
			Acknowledged: 0,
			Cleaned:      0,
		})
		return
	}

	// 更新投递状态为已投递
	if err := h.db.BatchUpdateDeliveryStatus(userID, req.ItemIDs, 1); err != nil {
		c.JSON(http.StatusInternalServerError, AckResponse{
			Success: false,
			Message: "更新状态失败",
		})
		return
	}

	// 不再立即删除，等待后台定時任务清理
	c.JSON(http.StatusOK, AckResponse{
		Success:      true,
		Acknowledged: len(req.ItemIDs),
		Cleaned:      0, // 不立即清理
	})
}

// shouldCleanItem 判断是否应该清理文章
func (h *AckHandler) shouldCleanItem(itemID int64) bool {
	total, acked, err := h.db.GetDeliveryStats(itemID)
	if err != nil {
		log.Printf("Failed to get delivery stats for item %d: %v", itemID, err)
		return false
	}

	// 所有用户都已确认接收
	return total > 0 && total == acked
}

// cleanItem 清理文章及相关资源
func (h *AckHandler) cleanItem(itemID int64) error {
	// 获取文章信息
	item, err := h.db.GetItemByID(itemID)
	if err != nil {
		return err
	}

	// 删除图片文件
	if item.ImagePaths != "" && item.ImagePaths != "[]" {
		if err := image.DeleteImageFiles(h.staticDir, item.ImagePaths); err != nil {
			log.Printf("Failed to delete image files for item %d: %v", itemID, err)
			// 继续执行，不中断流程
		}

		// 检查并删除空目录
		imageDir := image.GetImageDirPath(h.staticDir, item.SourceID)
		if err := image.RemoveEmptyDir(imageDir); err != nil {
			log.Printf("Failed to remove empty dir %s: %v", imageDir, err)
		}
	}

	// 删除投递记录
	if err := h.db.DeleteUserDeliveries(itemID); err != nil {
		return err
	}

	// 删除文章
	if err := h.db.DeleteItem(itemID); err != nil {
		return err
	}

	log.Printf("Cleaned item %d successfully", itemID)
	return nil
}
