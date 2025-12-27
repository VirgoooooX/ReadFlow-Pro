package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/readflow/gateway/internal/db"
)

// VocabHandler 生词本处理器
type VocabHandler struct {
	db *db.DB
}

// NewVocabHandler 创建生词本处理器
func NewVocabHandler(database *db.DB) *VocabHandler {
	return &VocabHandler{db: database}
}

// VocabWord 客户端上传的生词结构
type VocabWord struct {
	ID                 string `json:"id" binding:"required"`         // 词条ID
	Word               string `json:"word" binding:"required"`       // 单词
	Definition         string `json:"definition"`                    // 定义（JSON）
	Translation        string `json:"translation"`                   // 翻译
	Example            string `json:"example"`                       // 例句
	Context            string `json:"context"`                       // 上下文
	SourceArticleID    string `json:"source_article_id"`             // 来源文章ID
	SourceArticleTitle string `json:"source_article_title"`          // 来源文章标题
	ArticleID          int64  `json:"article_id"`                    // 文章ID
	ReviewCount        int64  `json:"review_count"`                  // 复习次数
	CorrectCount       int64  `json:"correct_count"`                 // 正确次数
	LastReviewAt       int64  `json:"last_review_at"`                // 最后复习时间（Unix时间戳）
	NextReviewAt       int64  `json:"next_review_at"`                // 下次复习时间（Unix时间戳）
	MasteryLevel       int64  `json:"mastery_level"`                 // 掌握程度
	Difficulty         string `json:"difficulty"`                    // 难度
	Tags               string `json:"tags"`                          // 标签（JSON）
	Notes              string `json:"notes"`                         // 备注
	AddedAt            int64  `json:"added_at" binding:"required"`   // 添加时间（Unix时间戳）
	UpdatedAt          int64  `json:"updated_at" binding:"required"` // 更新时间（Unix时间戳）
	IsDeleted          bool   `json:"is_deleted"`                    // 删除标记
}

// VocabWordFull 服务端返回的完整生词结构
type VocabWordFull struct {
	ID                 string `json:"id"`                   // 词条ID
	Word               string `json:"word"`                 // 单词
	Definition         string `json:"definition"`           // 定义（JSON）
	Translation        string `json:"translation"`          // 翻译
	Example            string `json:"example"`              // 例句
	Context            string `json:"context"`              // 上下文
	SourceArticleID    string `json:"source_article_id"`    // 来源文章ID
	SourceArticleTitle string `json:"source_article_title"` // 来源文章标题
	ArticleID          int64  `json:"article_id"`           // 文章ID
	ReviewCount        int64  `json:"review_count"`         // 复习次数
	CorrectCount       int64  `json:"correct_count"`        // 正确次数
	LastReviewAt       int64  `json:"last_review_at"`       // 最后复习时间（Unix时间戳）
	NextReviewAt       int64  `json:"next_review_at"`       // 下次复习时间（Unix时间戳）
	MasteryLevel       int64  `json:"mastery_level"`        // 掌握程度
	Difficulty         string `json:"difficulty"`           // 难度
	Tags               string `json:"tags"`                 // 标签（JSON）
	Notes              string `json:"notes"`                // 备注
	AddedAt            int64  `json:"added_at"`             // 添加时间（Unix时间戳）
	CreatedAt          int64  `json:"created_at"`           // 创建时间（Unix时间戳）
	UpdatedAt          int64  `json:"updated_at"`           // 更新时间（Unix时间戳）
	IsDeleted          bool   `json:"is_deleted"`           // 删除标记
}

// PushRequest Push请求
type PushRequest struct {
	Words []VocabWord `json:"words" binding:"required"`
}

// PushResponse Push响应
type PushResponse struct {
	Success    bool      `json:"success"`
	Synced     int       `json:"synced"`
	Conflicts  int       `json:"conflicts"`
	ServerTime time.Time `json:"server_time"`
}

// PullResponse Pull响应
type PullResponse struct {
	Words      []VocabWordFull `json:"words"`
	HasMore    bool            `json:"has_more"`
	ServerTime time.Time       `json:"server_time"`
}

// Push 上传生词本（客户端 -> 服务端）
func (h *VocabHandler) Push(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	var req PushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	// 验证数组长度
	if len(req.Words) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "words 数组不能为空",
		})
		return
	}

	if len(req.Words) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "words 数组长度不能超过 500",
		})
		return
	}

	// 处理每个单词
	synced := 0
	conflicts := 0
	for _, word := range req.Words {
		// 验证必填字段
		if word.Word == "" || word.ID == "" {
			log.Printf("Skip word with empty word or id for user %d", userID)
			continue
		}

		// 构建 Vocabulary 对象
		vocab := &db.Vocabulary{
			ID:                 word.ID,
			UserID:             userID,
			Word:               word.Word,
			Definition:         word.Definition,
			Translation:        word.Translation,
			Example:            word.Example,
			Context:            word.Context,
			SourceArticleID:    word.SourceArticleID,
			SourceArticleTitle: word.SourceArticleTitle,
			ArticleID:          word.ArticleID,
			ReviewCount:        word.ReviewCount,
			CorrectCount:       word.CorrectCount,
			LastReviewAt:       word.LastReviewAt,
			NextReviewAt:       word.NextReviewAt,
			MasteryLevel:       word.MasteryLevel,
			Difficulty:         word.Difficulty,
			Tags:               word.Tags,
			Notes:              word.Notes,
			AddedAt:            word.AddedAt,
			UpdatedAt:          word.UpdatedAt,
			IsDeleted:          word.IsDeleted,
		}

		// 如果客户端没有提供 CreatedAt，使用当前时间
		if vocab.AddedAt == 0 {
			vocab.AddedAt = int64(time.Now().Unix())
		}
		if vocab.CreatedAt == 0 {
			vocab.CreatedAt = int64(time.Now().Unix())
		}

		// Upsert 操作
		err := h.db.UpsertVocabulary(vocab)

		if err != nil {
			log.Printf("Failed to upsert vocabulary for user %d, word %s: %v", userID, word.Word, err)
			continue
		}

		synced++
	}

	c.JSON(http.StatusOK, PushResponse{
		Success:    true,
		Synced:     synced,
		Conflicts:  conflicts,
		ServerTime: time.Now(),
	})
}

// Pull 下载生词本（服务端 -> 客户端）
func (h *VocabHandler) Pull(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	// 解析查询参数
	sinceStr := c.Query("since")
	limit := 500 // 默认值

	if limitStr := c.Query("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
			limit = 500
		}
	}

	// 限制最大值
	if limit > 1000 {
		limit = 1000
	}

	// 解析since时间戳（客户端提供Unix时间戳）
	var sinceTimestamp int64 = 0
	if sinceStr != "" {
		if _, err := fmt.Sscanf(sinceStr, "%d", &sinceTimestamp); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "无效的since参数格式，应为Unix时间戳",
			})
			return
		}
	}

	// 查询生词
	vocabs, err := h.db.GetVocabulariesSince(userID, sinceTimestamp)
	if err != nil {
		log.Printf("Failed to get vocabularies for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询失败",
		})
		return
	}

	// 转换为响应格式
	words := make([]VocabWordFull, 0, len(vocabs))
	for _, vocab := range vocabs {
		words = append(words, VocabWordFull{
			ID:                 vocab.ID,
			Word:               vocab.Word,
			Definition:         vocab.Definition,
			Translation:        vocab.Translation,
			Example:            vocab.Example,
			Context:            vocab.Context,
			SourceArticleID:    vocab.SourceArticleID,
			SourceArticleTitle: vocab.SourceArticleTitle,
			ArticleID:          vocab.ArticleID,
			ReviewCount:        vocab.ReviewCount,
			CorrectCount:       vocab.CorrectCount,
			LastReviewAt:       vocab.LastReviewAt,
			NextReviewAt:       vocab.NextReviewAt,
			MasteryLevel:       vocab.MasteryLevel,
			Difficulty:         vocab.Difficulty,
			Tags:               vocab.Tags,
			Notes:              vocab.Notes,
			AddedAt:            vocab.AddedAt,
			UpdatedAt:          vocab.UpdatedAt,
			IsDeleted:          vocab.IsDeleted,
			CreatedAt:          vocab.CreatedAt,
		})

		// 限制返回数量
		if len(words) >= limit {
			break
		}
	}

	c.JSON(http.StatusOK, PullResponse{
		Words:      words,
		HasMore:    len(vocabs) > len(words),
		ServerTime: time.Now(),
	})
}
