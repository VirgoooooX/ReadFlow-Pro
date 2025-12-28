package api

import (
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/db"
)

// ArticleHandler 文章相关 API 处理器
type ArticleHandler struct {
	db *db.DB
}

// NewArticleHandler 创建文章处理器
func NewArticleHandler(database *db.DB) *ArticleHandler {
	return &ArticleHandler{db: database}
}

// ArticleListItem 列表项结构
type ArticleListItem struct {
	ID                int64  `json:"id"`
	Title             string `json:"title"`
	Summary           string `json:"summary"`
	ImageURL          string `json:"imageUrl"`
	ImageCaption      string `json:"imageCaption"`      // Added
	ImageCredit       string `json:"imageCredit"`       // Added
	ImagePrimaryColor string `json:"imagePrimaryColor"` // Added
	Author            string `json:"author"`
	PublishedAt       int64  `json:"publishedAt"`
	SourceID          int64  `json:"sourceId"`
	SourceName        string `json:"sourceName"`
	WordCount         int    `json:"wordCount"`
	ReadingTime       int    `json:"readingTime"`
	IsRead            bool   `json:"isRead"`
	IsFavorite        bool   `json:"isFavorite"`
	ReadProgress      int    `json:"readProgress"`
	ReadAt            *int64 `json:"readAt,omitempty"`
	UpdatedAt         int64  `json:"updatedAt"`
}

// ArticleListResponse 列表响应
type ArticleListResponse struct {
	Success    bool              `json:"success"`
	Articles   []ArticleListItem `json:"articles"`
	HasMore    bool              `json:"hasMore"`
	SyncTime   *int64            `json:"syncTime,omitempty"`   // 增量同步模式：服务端当前时间戳
	NextCursor *string           `json:"nextCursor,omitempty"` // 游标分页模式：下一页游标
}

// ArticleDetailResponse 详情响应
type ArticleDetailResponse struct {
	Success           bool   `json:"success"`
	ID                int64  `json:"id"`
	Title             string `json:"title"`
	Content           string `json:"content"`
	Summary           string `json:"summary"`
	ImageURL          string `json:"imageUrl"`
	ImageCaption      string `json:"imageCaption"`      // Added
	ImageCredit       string `json:"imageCredit"`       // Added
	ImagePrimaryColor string `json:"imagePrimaryColor"` // Added
	Author            string `json:"author"`
	PublishedAt       int64  `json:"publishedAt"`
	URL               string `json:"url"`
	SourceID          int64  `json:"sourceId"`
	SourceName        string `json:"sourceName"`
	WordCount         int    `json:"wordCount"`
	ReadingTime       int    `json:"readingTime"`
	IsFavorite        bool   `json:"isFavorite"`
	ReadProgress      int    `json:"readProgress"`
	ReadAt            *int64 `json:"readAt,omitempty"`
	UpdatedAt         int64  `json:"updatedAt"`
}

var (
	imgTagRegex = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)
	tagRegex    = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRegex  = regexp.MustCompile(`\s+`)
)

// ListArticles 获取文章列表（按用户视角）
// 支持三种模式：
// 1. 增量同步：since 参数，返回该时间之后发布的文章
// 2. 游标分页：cursor 参数，翻页历史文章
// 3. 默认模式：offset 分页（兼容旧逻辑）
func (h *ArticleHandler) ListArticles(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	// 解析 limit 参数
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 200 {
		limit = 50
	}

	// 解析 offset 参数（默认模式）
	offsetStr := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// 解析 source_id 参数
	var sourceIDPtr *int64
	if sourceIDStr := c.Query("source_id"); sourceIDStr != "" {
		if sid, err := strconv.ParseInt(sourceIDStr, 10, 64); err == nil && sid > 0 {
			sourceIDPtr = &sid
		}
	}

	// 解析 since 参数（增量同步）
	var sinceTimePtr *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		if sinceTimestamp, err := strconv.ParseInt(sinceStr, 10, 64); err == nil && sinceTimestamp > 0 {
			t := time.Unix(sinceTimestamp, 0)
			sinceTimePtr = &t
		}
	}

	// 解析 cursor 参数（游标分页）
	var cursorPtr *string
	if cursorStr := c.Query("cursor"); cursorStr != "" {
		cursorPtr = &cursorStr
	}

	// 调用数据库层
	userArticles, nextCursor, err := h.db.GetUserArticles(userID, sourceIDPtr, sinceTimePtr, cursorPtr, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询失败",
		})
		return
	}

	// 构建响应
	items := make([]ArticleListItem, 0, len(userArticles))
	for _, ua := range userArticles {
		// 直接使用结构化字段，不需要解析 xml_content
		summary := ua.Summary
		imageURL := ua.CoverImage
		wordCount := ua.WordCount
		readingTime := ua.ReadingTime

		// 如果结构化字段为空（旧数据），回退到解析 xml_content
		if summary == "" || imageURL == "" || wordCount == 0 {
			desc, contentHTML, _ := parseXMLFields(ua.XMLContent)

			if summary == "" {
				summary = generateSummaryFromHTML(desc, 200)
				if summary == "" {
					summary = generateSummaryFromHTML(contentHTML, 200)
				}
			}

			if imageURL == "" {
				imageURL = extractFirstImageURL(contentHTML)
			}

			if wordCount == 0 {
				wordCount = countWordsFromHTML(contentHTML)
				if wordCount > 0 {
					readingTime = (wordCount + 199) / 200
				}
			}
		}

		var publishedAt int64
		if ua.PublishedAt != nil {
			publishedAt = ua.PublishedAt.Unix()
		}

		var readAt *int64
		if ua.ReadAt != nil {
			t := ua.ReadAt.Unix()
			readAt = &t
		}

		items = append(items, ArticleListItem{
			ID:                ua.ID,
			Title:             ua.Title,
			Summary:           summary,
			ImageURL:          imageURL,
			ImageCaption:      ua.ImageCaption,
			ImageCredit:       ua.ImageCredit,
			ImagePrimaryColor: ua.ImagePrimaryColor,
			Author:            ua.Author,
			PublishedAt:       publishedAt,
			SourceID:          ua.SourceID,
			SourceName:        ua.SourceTitle,
			WordCount:         wordCount,
			ReadingTime:       readingTime,
			IsRead:            ua.Status != 0,
			IsFavorite:        ua.IsFavorite,
			ReadProgress:      ua.ReadProgress,
			ReadAt:            readAt,
			UpdatedAt:         ua.UpdatedAt.Unix(),
		})
	}

	// 构建响应对象
	response := ArticleListResponse{
		Success:  true,
		Articles: items,
		HasMore:  nextCursor != nil,
	}

	// 根据请求模式添加相应字段
	if sinceTimePtr != nil {
		// 增量同步模式：返回 syncTime
		syncTime := time.Now().Unix()
		response.SyncTime = &syncTime
	} else if cursorPtr != nil || nextCursor != nil {
		// 游标分页模式：返回 nextCursor
		response.NextCursor = nextCursor
	}

	c.JSON(http.StatusOK, response)
}

// GetArticleDetail 获取文章详情
func (h *ArticleHandler) GetArticleDetail(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}
	_ = userID // 当前只用于鉴权，不做额外校验

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文章 ID",
		})
		return
	}

	item, err := h.db.GetItemByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "文章不存在",
		})
		return
	}

	source, err := h.db.GetSourceByID(item.SourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询订阅源失败",
		})
		return
	}

	desc, contentHTML, link := parseXMLFields(item.XMLContent)

	// 直接使用结构化字段
	content := item.CleanContent
	summary := item.Summary
	imageURL := item.CoverImage
	wordCount := item.WordCount
	readingTime := item.ReadingTime

	// 如果结构化字段为空（旧数据），回退到解析
	if content == "" {
		content = contentHTML
		if content == "" {
			content = desc
		}
	}

	if summary == "" {
		summary = generateSummaryFromHTML(desc, 200)
		if summary == "" {
			summary = generateSummaryFromHTML(contentHTML, 200)
		}
	}

	if imageURL == "" {
		imageURL = extractFirstImageURL(contentHTML)
	}

	if wordCount == 0 {
		wordCount = countWordsFromHTML(contentHTML)
		if wordCount > 0 {
			readingTime = (wordCount + 199) / 200
		}
	}

	var publishedAt int64
	if item.PublishedAt != nil {
		publishedAt = item.PublishedAt.Unix()
	}

	c.JSON(http.StatusOK, ArticleDetailResponse{
		Success:      true,
		ID:           item.ID,
		Title:        item.Title,
		Content:      content,
		Summary:      summary,
		ImageURL:     imageURL,
		ImageCaption: item.ImageCaption,
		ImageCredit:  item.ImageCredit,
		Author:       item.Author,
		PublishedAt:  publishedAt,
		URL:          link,
		SourceID:     source.ID,
		SourceName:   source.Title,
		WordCount:    wordCount,
		ReadingTime:  readingTime,
	})
}

// parseXMLFields 从 xml_content 中解析 description、content:encoded 和 link
func parseXMLFields(xmlContent string) (description, contentHTML, link string) {
	description = between(xmlContent, "<description><![CDATA[", "]]></description>")
	contentHTML = between(xmlContent, "<content:encoded><![CDATA[", "]]></content:encoded>")
	link = between(xmlContent, "<link>", "</link>")
	return
}

// between 返回 start 和 end 中间的子串
func between(s, start, end string) string {
	startIdx := strings.Index(s, start)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(start)
	endIdx := strings.Index(s[startIdx:], end)
	if endIdx == -1 {
		return ""
	}
	return s[startIdx : startIdx+endIdx]
}

// generateSummaryFromHTML 从 HTML 生成摘要
func generateSummaryFromHTML(htmlText string, maxLength int) string {
	text := stripHTML(htmlText)
	if len(text) <= maxLength {
		return text
	}

	truncated := text[:maxLength]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 {
		return truncated[:lastSpace] + "..."
	}
	return truncated + "..."
}

// stripHTML 去掉 HTML 标签并做简单清洗
func stripHTML(htmlText string) string {
	if htmlText == "" {
		return ""
	}

	s := strings.ReplaceAll(htmlText, "&nbsp;", " ")
	s = tagRegex.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = spaceRegex.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// extractFirstImageURL 从 HTML 中提取第一张图片的 URL
func extractFirstImageURL(contentHTML string) string {
	if contentHTML == "" {
		return ""
	}
	matches := imgTagRegex.FindStringSubmatch(contentHTML)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// countWordsFromHTML 统计 HTML 文本的“字数”（中英文混合）
func countWordsFromHTML(htmlText string) int {
	plain := stripHTML(htmlText)
	if plain == "" {
		return 0
	}

	var chineseCount int
	for _, r := range plain {
		if r >= 0x4e00 && r <= 0x9fff {
			chineseCount++
		}
	}

	var b strings.Builder
	for _, r := range plain {
		if r >= 0x4e00 && r <= 0x9fff {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}

	englishWords := strings.Fields(b.String())
	englishCount := len(englishWords)

	return chineseCount + englishCount
}

// Quest 5: 阅读状态管理 API

// MarkArticleRead 标记文章为已读
func (h *ArticleHandler) MarkArticleRead(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文章 ID",
		})
		return
	}

	if err := h.db.MarkArticleAsRead(userID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "操作失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// MarkArticleUnread 标记文章为未读
func (h *ArticleHandler) MarkArticleUnread(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文章 ID",
		})
		return
	}

	if err := h.db.MarkArticleAsUnread(userID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "操作失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AddFavorite 添加文章到收藏
func (h *ArticleHandler) AddFavorite(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文章 ID",
		})
		return
	}

	if err := h.db.SetFavorite(userID, id, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "操作失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RemoveFavorite 取消文章收藏
func (h *ArticleHandler) RemoveFavorite(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文章 ID",
		})
		return
	}

	if err := h.db.SetFavorite(userID, id, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "操作失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ToggleFavorite 切换文章收藏状态
func (h *ArticleHandler) ToggleFavorite(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文章 ID",
		})
		return
	}

	// 使用 DB 层提供的 ToggleFavorite 方法
	newState, err := h.db.ToggleFavorite(userID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "操作失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"is_favorite": newState,
		"message":     "收藏状态已更新",
	})
}

// UpdateArticleProgress 更新文章阅读进度
func (h *ArticleHandler) UpdateArticleProgress(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的文章 ID",
		})
		return
	}

	var req struct {
		Progress int `json:"progress"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	if err := h.db.UpdateReadProgress(userID, id, req.Progress); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "操作失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
