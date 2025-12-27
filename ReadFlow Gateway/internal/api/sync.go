package api

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/db"
)

// SyncHandler 同步接口处理器
type SyncHandler struct {
	db     *db.DB
	worker SyncWorker // 工作器接口
}

// SyncWorker 定义刷新源所需的工作器接口
type SyncWorker interface {
	FetchSource(source *db.Source) error
	FetchAllSourcesForUser(userID int64) error // 并发抓取用户所有订阅源
}

// NewSyncHandler 创建同步处理器
func NewSyncHandler(database *db.DB, worker SyncWorker) *SyncHandler {
	return &SyncHandler{
		db:     database,
		worker: worker,
	}
}

// Sync 处理同步请求
// 支持两种模式：
// 1. mode=sync (默认): 仅同步，返回数据库中已有的文章
// 2. mode=refresh: 刷新源并同步，先抓取最新文章再同步
// 可选参数：
// - source_url: 指定源URL，只处理该源
func (h *SyncHandler) Sync(c *gin.Context) {
	// 获取当前用户 ID
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	// 获取同步模式，默认为 "sync"
	mode := c.DefaultQuery("mode", "sync")
	// 获取指定源URL（可选）
	sourceURL := c.Query("source_url")
	// 获取输出格式，默认为 "xml"
	format := c.DefaultQuery("format", "xml")

	// 获取图片压缩选项，默认为 "true"
	imageCompressionStr := c.DefaultQuery("image_compression", "true")
	imageCompression := imageCompressionStr == "true"

	log.Printf("[SYNC] 用户=%d, mode=%s, source_url=%s, format=%s, compression=%v", userID, mode, sourceURL, format, imageCompression)

	// 如果是刷新模式，先执行刷新
	if mode == "refresh" {
		if h.worker != nil {
			if sourceURL != "" {
				// 只刷新指定源
				h.refreshSingleSource(userID, sourceURL)
			} else {
				// 刷新所有源
				if err := h.worker.FetchAllSourcesForUser(userID); err != nil {
					log.Printf("[SYNC] 刷新用户 %d 的源失败: %v", userID, err)
				}
			}
		} else {
			log.Println("[SYNC] Worker 未初始化，跳过刷新")
		}
	}

	// 获取查询参数
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 200 {
		limit = 50
	}

	// 查询待投递的文章
	var items []*db.Item
	if sourceURL != "" {
		// 只返回指定源的文章
		items, err = h.db.GetPendingDeliveriesBySourceURL(userID, sourceURL, limit)
	} else {
		// 返回所有源的文章
		items, err = h.db.GetPendingDeliveries(userID, limit)
	}

	if err != nil {
		log.Printf("[SYNC] 查询失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询失败",
		})
		return
	}

	// 处理图片压缩选项
	if !imageCompression {
		for _, item := range items {
			// 如果用户选择不压缩，使用原始内容覆盖 CleanContent
			// 客户端主要使用 CleanContent 进行渲染
			if item.Content != "" {
				item.CleanContent = item.Content
				// 注意：这里没有重新构建 XMLContent，因为客户端主要使用 JSON 格式
				// 如果需要支持 XML 格式的非压缩模式，需要重新构建 XMLContent
			}
		}
	}

	// 如果请求 JSON 格式
	if strings.ToLower(format) == "json" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"items":   items, // 注意：db.Item 包含 xml_content，可能比较大
			"count":   len(items),
		})
		return
	}

	// 默认返回 XML 格式
	xml := h.buildRSSXML(userID, items)

	// 返回 XML 格式
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, xml)
}

// refreshSingleSource 刷新单个源
func (h *SyncHandler) refreshSingleSource(userID int64, sourceURL string) {
	source, err := h.db.GetUserSourceByURL(userID, sourceURL)
	if err != nil {
		log.Printf("[SYNC] 找不到源 %s: %v", sourceURL, err)
		return
	}

	log.Printf("[SYNC] 刷新单个源: %s", sourceURL)
	if err := h.worker.FetchSource(source); err != nil {
		log.Printf("[SYNC] 刷新失败: %v", err)
		h.db.UpdateSourceError(source.ID, err.Error())
	} else {
		h.db.UpdateSourceFetchTime(source.ID)
	}
}

// buildRSSXML 构建 RSS XML 格式
func (h *SyncHandler) buildRSSXML(userID int64, items []*db.Item) string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(`<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">`)
	sb.WriteString("\n")
	sb.WriteString("  <channel>\n")
	sb.WriteString("    <title>ReadFlow Private Feed</title>\n")
	sb.WriteString("    <description>Private RSS Gateway</description>\n")
	sb.WriteString(fmt.Sprintf("    <lastBuildDate>%s</lastBuildDate>\n", time.Now().Format(time.RFC1123Z)))

	// 缓存源信息（服务端源）
	sourceCache := make(map[int64]*db.Source)

	for _, item := range items {
		var sourceName, sourceURL string
		var displayLocalID int64 = item.SourceID // 默认使用服务端 source_id

		if item.SourceID != 0 {
			// 查询服务端源信息
			src, ok := sourceCache[item.SourceID]
			if !ok {
				loaded, err := h.db.GetSourceByID(item.SourceID)
				if err != nil {
					log.Printf("[SYNC] failed to load source %d: %v", item.SourceID, err)
				}
				src = loaded
				sourceCache[item.SourceID] = src
			}

			// 使用源的标题和URL
			if src != nil {
				sourceName = src.Title
				sourceURL = src.URL
				// 使用源ID作为本地ID（保持兼容性）
				displayLocalID = src.ID
			}
		}

		// 输出带源信息的 item 标签
		sb.WriteString(fmt.Sprintf(
			"    <item data-item-id=\"%d\" data-source-id=\"%d\" data-source-name=\"%s\" data-source-url=\"%s\">\n",
			item.ID,
			displayLocalID, // 使用本地源 ID（或默认服务端 ID）
			html.EscapeString(sourceName),
			html.EscapeString(sourceURL),
		))

		sb.WriteString(fmt.Sprintf("      <title><![CDATA[%s]]></title>\n", item.Title))
		sb.WriteString(fmt.Sprintf("      <guid>%s</guid>\n", item.GUID))

		if item.PublishedAt != nil {
			sb.WriteString(fmt.Sprintf("      <pubDate>%s</pubDate>\n", item.PublishedAt.Format(time.RFC1123Z)))
		}

		// 嵌入 XML 内容
		sb.WriteString("      ")
		sb.WriteString(item.XMLContent)
		sb.WriteString("\n")

		sb.WriteString("    </item>\n")
	}

	sb.WriteString("  </channel>\n")
	sb.WriteString("</rss>")

	return sb.String()
}
