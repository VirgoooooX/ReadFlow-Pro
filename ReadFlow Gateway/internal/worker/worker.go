package worker

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/readflow/gateway/internal/config"
	"github.com/readflow/gateway/internal/db"
	"github.com/readflow/gateway/internal/image"
	"github.com/readflow/gateway/internal/utils"
)

// 常量定义
const (
	// RSS 抓取超时时间
	fetchTimeout = 60 * time.Second
	// 单个源处理超时时间
	sourceTimeout = 120 * time.Second
	// HTTP 请求超时
	httpTimeout = 30 * time.Second
)

// Worker RSS 抓取工作器
type Worker struct {
	db               *db.DB
	config           *config.Config
	parser           *gofeed.Parser
	imageProcessor   *image.Processor
	imageExtractor   *ImageExtractor
	contentExtractor *ContentExtractor
	staticDir        string
	fetching         sync.Mutex // 防止并发抓取
}

// New 创建新的 Worker
func New(database *db.DB, cfg *config.Config) *Worker {
	// 创建 HTTP 客户端（带超时）
	httpClient := &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// 创建 RSS Parser
	parser := gofeed.NewParser()
	parser.Client = httpClient

	// 创建图片处理器
	imgProcessor := image.NewProcessor(cfg)

	// 创建智能图片提取器
	imgExtractor := NewImageExtractor()

	// 创建内容提取器
	contentExtractor := NewContentExtractor()

	return &Worker{
		db:               database,
		config:           cfg,
		parser:           parser,
		imageProcessor:   imgProcessor,
		imageExtractor:   imgExtractor,
		contentExtractor: contentExtractor,
		staticDir:        cfg.StaticDir,
	}
}

// Start 启动 Worker
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(w.config.FetchInterval) * time.Second)
	defer ticker.Stop()

	// 清理任务每初执行一次，間隔为抬取间隔的 1/3
	cleanupTicker := time.NewTicker(time.Duration(w.config.FetchInterval/3) * time.Second)
	defer cleanupTicker.Stop()

	log.Println("RSS Worker started")

	// 启动时立即执行一次
	w.FetchAll()

	for {
		select {
		case <-ctx.Done():
			log.Println("RSS Worker stopped")
			return
		case <-ticker.C:
			w.FetchAll()
		case <-cleanupTicker.C:
			w.CleanupExpiredItems()
		}
	}
}

// FetchAll 抓取所有活跃的订阅源
func (w *Worker) FetchAll() {
	// 防止并发抓取
	if !w.fetching.TryLock() {
		log.Println("[WORKER] Previous fetch still running, skipping this round")
		return
	}
	defer w.fetching.Unlock()

	// 添加 panic 恢复
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WORKER] Recovered from panic in FetchAll: %v", r)
		}
	}()

	sources, err := w.db.GetActiveSources()
	if err != nil {
		log.Printf("Failed to get active sources: %v", err)
		return
	}

	log.Printf("Fetching %d active sources", len(sources))

	for _, source := range sources {
		// 检查是否应该抓取
		if !w.shouldFetch(source) {
			continue
		}

		// 为每个源设置超时
		if err := w.fetchSourceWithTimeout(source); err != nil {
			log.Printf("Failed to fetch source %s: %v", source.URL, err)
			w.db.UpdateSourceError(source.ID, err.Error())
		} else {
			w.db.UpdateSourceFetchTime(source.ID)
		}
	}
}

// fetchSourceWithTimeout 带超时的源抓取
func (w *Worker) fetchSourceWithTimeout(source *db.Source) error {
	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), sourceTimeout)
	defer cancel()

	// 使用 channel 接收结果
	errChan := make(chan error, 1)

	go func() {
		// 添加 panic 恢复
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic: %v", r)
			}
		}()
		errChan <- w.fetchSource(source)
	}()

	// 等待结果或超时
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timeout after %v", sourceTimeout)
	}
}

// shouldFetch 判断是否应该抓取该源
func (w *Worker) shouldFetch(source *db.Source) bool {
	if source.LastFetchTime == nil {
		return true
	}

	elapsed := time.Since(*source.LastFetchTime)
	interval := time.Duration(source.FetchInterval) * time.Second

	return elapsed >= interval
}

// FetchSource 抽取单个订阅源的文章 - 公开方法，供 Sync API 调用
func (w *Worker) FetchSource(source *db.Source) error {
	if source == nil {
		return fmt.Errorf("source is nil")
	}
	// 使用带超时的方法
	return w.fetchSourceWithTimeout(source)
}

// FetchAllSourcesForUser 并发抓取用户订阅的所有源（供 Sync API 刷新模式调用）
func (w *Worker) FetchAllSourcesForUser(userID int64) error {
	sources, err := w.db.GetUserSubscriptions(userID)
	if err != nil {
		return fmt.Errorf("get user sources failed: %w", err)
	}

	if len(sources) == 0 {
		log.Printf("[Worker] 用户 %d 没有订阅源", userID)
		return nil
	}

	log.Printf("[Worker] 开始为用户 %d 并发刷新 %d 个源", userID, len(sources))

	var wg sync.WaitGroup
	errChan := make(chan error, len(sources))

	// 并发限制：最多同时抓取 3 个源
	semaphore := make(chan struct{}, 3)

	for _, source := range sources {
		wg.Add(1)
		go func(s *db.Source) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 添加 panic 恢复
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic while fetching source %s: %v", s.URL, r)
				}
			}()

			if err := w.fetchSourceWithTimeout(s); err != nil {
				log.Printf("[Worker] 源 %s 抓取失败: %v", s.URL, err)
				w.db.UpdateSourceError(s.ID, err.Error())
				errChan <- err
			} else {
				w.db.UpdateSourceFetchTime(s.ID)
			}
		}(source)
	}

	wg.Wait()
	close(errChan)

	// 统计错误数量
	errorCount := 0
	for err := range errChan {
		if err != nil {
			errorCount++
		}
	}

	log.Printf("[Worker] 用户 %d 源刷新完成，%d 个成功，%d 个失败", userID, len(sources)-errorCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("%d sources failed to fetch", errorCount)
	}
	return nil
}

// fetchSource 抓取单个源
func (w *Worker) fetchSource(source *db.Source) error {
	url := source.URL
	log.Printf("Fetching source: %s", url)

	// 处理 rsshub:// 协议
	if strings.HasPrefix(url, "rsshub://") {
		rsshubHost := "https://rsshub.app"
		// 如果将来有配置，可以从 w.config 获取
		url = rsshubHost + "/" + strings.TrimPrefix(url, "rsshub://")
		log.Printf("[WORKER] Transforming rsshub:// to %s", url)
	}

	// 解析 RSS
	feed, err := w.parser.ParseURL(url)
	if err != nil {
		return fmt.Errorf("parse RSS failed: %w", err)
	}

	// 更新源信息
	if source.Title == "" && feed.Title != "" {
		// 这里可以更新源的标题和描述
	}

	// 获取订阅该源的用户列表
	userIDs, err := w.db.GetSubscribedUserIDs(source.ID)
	if err != nil {
		return fmt.Errorf("get subscribed users failed: %w", err)
	}

	if len(userIDs) == 0 {
		log.Printf("No subscribers for source %s, skipping", source.URL)
		return nil
	}

	// 处理每篇文章
	newItemsCount := 0
	for _, feedItem := range feed.Items {
		// 创建新文章
		if err := w.processItem(source.ID, feedItem, userIDs); err != nil {
			log.Printf("Failed to process item %s: %v", feedItem.GUID, err)
			continue
		}

		newItemsCount++
	}

	log.Printf("Fetched %d new items from source %s", newItemsCount, source.URL)
	return nil
}

// processItem 处理单篇文章（增强版）
// 集成智能图片提取、内容处理、字数统计等功能
func (w *Worker) processItem(sourceID int64, feedItem *gofeed.Item, userIDs []int64) error {
	if feedItem == nil {
		return fmt.Errorf("feedItem is nil")
	}

	// GUID 去重（基于 source 和 GUID）
	guid := feedItem.GUID
	if guid == "" {
		guid = feedItem.Link
	}
	if guid == "" {
		return fmt.Errorf("item missing both GUID and Link")
	}

	// 检查是否已存在
	_, err := w.db.GetItemByGUID(sourceID, guid)
	if err == nil {
		// 已存在，跳过
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	// 不存在，继续创建
	exists := false
	if exists {
		return nil // 文章已存在，跳过
	}

	// 提取内容
	content := feedItem.Content
	if content == "" {
		content = feedItem.Description
	}

	// 【新增】使用智能图片提取器
	log.Printf("[Worker] Extracting best image for item: %s", feedItem.Title)
	var finalCoverImageURL string
	var imageCaption string
	var imageCredit string

	imageCandidate := w.imageExtractor.ExtractBestImage(feedItem, content)
	if imageCandidate != nil {
		finalCoverImageURL = imageCandidate.URL
		imageCaption = imageCandidate.Alt
		imageCredit = imageCandidate.Credit
	} else {
		// Fallback到原有逻辑
		finalCoverImageURL = w.extractBestImageURL(feedItem)
	}

	// 处理内容中的图片（下载+压缩+替换）
	processedContent := content
	var imagePaths string

	if content != "" {
		var err error
		processedContent, imagePaths, err = w.imageProcessor.ProcessContent(sourceID, content)
		if err != nil {
			log.Printf("[Worker] Failed to process images for item %s: %v", guid, err)
			processedContent = content
		}
	}

	// 【新增】文本处理
	textProcessor := utils.NewTextProcessor()

	// 计算字数
	wordCount := textProcessor.CountWords(processedContent)

	// 估算阅读时间
	readingTime := textProcessor.EstimateReadingTime(wordCount)

	// 生成摘要
	summary := textProcessor.GenerateSummary(processedContent, 200)

	// 计算难度（之后可用于扩展字段）
	_ = textProcessor.CalculateDifficulty(processedContent)

	// 【新增】完整内容字段（替代clean_content）
	// fullContent := processedContent // fullContent placeholder, now used

	// 可选：尝试从原始URL提取完整内容
	// if feedItem.Link != "" && len(processedContent) < 500 {
	// 	log.Printf("[Worker] Content too short, attempting full content extraction from: %s", feedItem.Link)
	// 	if extracted, err := w.contentExtractor.ExtractFullContentWithTimeout(feedItem.Link, 30*time.Second); err == nil && extracted != "" {
	// 		fullContent = extracted
	// 		log.Printf("[Worker] Successfully extracted full content (%d bytes)", len(fullContent))
	// 	}
	// }

	// 构建 XML content（兼容现有客户端）
	xmlContent := w.buildXMLContent(feedItem, processedContent)

	// 计算内容哈希（用于去重）
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(feedItem.Title+content)))

	// 【新增】提取分类（从source或feedItem）
	// category := "" // category placeholder, now used
	// if len(feedItem.Categories) > 0 {
	// 	category = feedItem.Categories[0]
	// }

	// 保存到 items 表（使用扩展字段）
	var publishedAt *time.Time
	if feedItem.PublishedParsed != nil {
		publishedAt = feedItem.PublishedParsed
	} else if feedItem.UpdatedParsed != nil {
		publishedAt = feedItem.UpdatedParsed
	}

	// 使用CreateItem方法的正确signature
	item, err := w.db.CreateItem(
		sourceID,
		guid,
		feedItem.Title,
		xmlContent,
		imagePaths,
		publishedAt,
		summary,
		wordCount,
		readingTime,
		finalCoverImageURL,
		getAuthor(feedItem),
		processedContent,
		content, // Original content
		contentHash,
		imageCaption,
		imageCredit,
	)
	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	log.Printf("[Worker] Item processed: id=%d, title=%s, words=%d, reading_time=%d min",
		item.ID, feedItem.Title, wordCount, readingTime)

	// 为所有订阅该源的用户创建投递记录
	for _, userID := range userIDs {
		if err := w.db.CreateUserDelivery(userID, item.ID); err != nil {
			log.Printf("[Worker] Failed to create delivery for user %d, item %d: %v", userID, item.ID, err)
		}
	}

	return nil
}

func getAuthor(feedItem *gofeed.Item) string {
	if len(feedItem.Authors) > 0 && feedItem.Authors[0] != nil {
		return feedItem.Authors[0].Name
	}
	return ""
}

// extractBestImageURL 从 RSS 元数据中提取最合适的封面图 URL
func (w *Worker) extractBestImageURL(feedItem *gofeed.Item) string {
	if feedItem == nil {
		return ""
	}

	// 1. 优先使用 gofeed 解析到的 Image 字段
	if feedItem.Image != nil && feedItem.Image.URL != "" {
		return feedItem.Image.URL
	}

	// 2. 尝试从 media:content 扩展中获取
	if mediaExts, ok := feedItem.Extensions["media"]; ok {
		if contents, ok := mediaExts["content"]; ok {
			for _, ext := range contents {
				if url := strings.TrimSpace(ext.Attrs["url"]); url != "" {
					return url
				}
			}
		}
		// 3. 再尝试 media:thumbnail
		if thumbs, ok := mediaExts["thumbnail"]; ok {
			for _, ext := range thumbs {
				if url := strings.TrimSpace(ext.Attrs["url"]); url != "" {
					return url
				}
			}
		}
	}

	// 4. 尝试从 enclosure 中挑选图片类型
	for _, enc := range feedItem.Enclosures {
		if strings.HasPrefix(enc.Type, "image/") && strings.TrimSpace(enc.URL) != "" {
			return strings.TrimSpace(enc.URL)
		}
	}

	return ""
}

// buildXMLContent 构建 XML 内容片段
func (w *Worker) buildXMLContent(feedItem *gofeed.Item, content string) string {
	var sb strings.Builder

	// description
	sb.WriteString("<description><![CDATA[")
	sb.WriteString(feedItem.Description)
	sb.WriteString("]]></description>\n")

	// link
	if feedItem.Link != "" {
		sb.WriteString("      <link>")
		sb.WriteString(feedItem.Link)
		sb.WriteString("</link>\n")
	}

	// content:encoded
	if content != "" {
		sb.WriteString("      <content:encoded><![CDATA[")
		sb.WriteString(content)
		sb.WriteString("]]></content:encoded>")
	}

	return sb.String()
}

// CleanupExpiredItems 清理已超时的文章
func (w *Worker) CleanupExpiredItems() {
	// 添加 panic 恢复
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[CLEANUP] Recovered from panic: %v", r)
		}
	}()

	rc := config.GetRuntimeConfig()
	retentionTime := int64(rc.GetItemRetentionTime())
	nowUnix := time.Now().Unix()
	expiryTime := nowUnix - retentionTime

	log.Printf("[CLEANUP] Starting cleanup task, expiry threshold: %d seconds ago", retentionTime)

	// 获取所有已常新的文章（status=1）
	deliveredItems, err := w.db.GetDeliveredItems()
	if err != nil {
		log.Printf("[CLEANUP] Failed to get delivered items: %v", err)
		return
	}

	if len(deliveredItems) == 0 {
		log.Printf("[CLEANUP] No delivered items to clean")
		return
	}

	cleaned := 0
	for _, itemID := range deliveredItems {
		// 获取文章的最近投递时间
		deliveredTime, err := w.db.GetItemDeliveredTime(itemID)
		if err != nil {
			log.Printf("[CLEANUP] Failed to get delivered time for item %d: %v", itemID, err)
			continue
		}

		// 判断是否超时
		if deliveredTime.Unix() < expiryTime {
			if err := w.cleanupItem(itemID); err != nil {
				log.Printf("[CLEANUP] Failed to cleanup item %d: %v", itemID, err)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		log.Printf("[CLEANUP] Successfully cleaned %d items", cleaned)
	}
}

// cleanupItem 清理文章及相关资源
func (w *Worker) cleanupItem(itemID int64) error {
	// 获取文章信息
	item, err := w.db.GetItemByID(itemID)
	if err != nil {
		return err
	}

	// 删除图片文件
	if item.ImagePaths != "" && item.ImagePaths != "[]" {
		if err := image.DeleteImageFiles(w.staticDir, item.ImagePaths); err != nil {
			log.Printf("[CLEANUP] Failed to delete image files for item %d: %v", itemID, err)
			// 继续execution，不中断流程
		}

		// 检查并删除空目录
		imageDir := image.GetImageDirPath(w.staticDir, item.SourceID)
		if err := image.RemoveEmptyDir(imageDir); err != nil {
			log.Printf("[CLEANUP] Failed to remove empty dir %s: %v", imageDir, err)
		}
	}

	// 删除投递记录
	if err := w.db.DeleteUserDeliveries(itemID); err != nil {
		return err
	}

	// 删除文章
	if err := w.db.DeleteItem(itemID); err != nil {
		return err
	}

	log.Printf("[CLEANUP] Cleaned item %d successfully", itemID)
	return nil
}
