package worker

import (
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

// ImageCandidate 图片候选项
type ImageCandidate struct {
	URL    string
	Source string // "media:content" | "media:thumbnail" | "enclosure" | "content_html"
	Width  int
	Height int
	Medium string
	Alt    string
	Credit string // Added
	Score  int    // 评分（用于排序）
}

// ImageExtractor 智能图片提取器
type ImageExtractor struct {
	// 最小尺寸要求
	MinWidth  int
	MinHeight int
}

// NewImageExtractor 创建图片提取器
func NewImageExtractor() *ImageExtractor {
	return &ImageExtractor{
		MinWidth:  300,
		MinHeight: 200,
	}
}

// ExtractBestImage 从RSS item和内容中提取最佳图片
// 这是主入口函数，复刻客户端的智能提取逻辑
func (e *ImageExtractor) ExtractBestImage(feedItem *gofeed.Item, contentHTML string) *ImageCandidate {
	candidates := []ImageCandidate{}

	// 1. 增强的RSS item (media:content, media:thumbnail)
	enhanced := EnhanceItem(feedItem)
	if enhanced != nil {
		candidates = append(candidates, e.extractFromMediaContent(enhanced)...)
		candidates = append(candidates, e.extractFromMediaThumbnail(enhanced)...)
		candidates = append(candidates, e.extractFromEnclosures(enhanced)...)
	}

	// 2. 从HTML内容提取
	if contentHTML != "" {
		candidates = append(candidates, e.extractFromHTML(contentHTML)...)
	}

	// 3. 过滤占位图
	filtered := []ImageCandidate{}
	for _, candidate := range candidates {
		if !e.isPlaceholderImage(candidate.URL, candidate.Alt) {
			filtered = append(filtered, candidate)
		}
	}

	// 4. 评分排序
	for i := range filtered {
		filtered[i].Score = e.scoreImage(&filtered[i])
	}

	// 5. 选择最高分
	var best *ImageCandidate
	for i := range filtered {
		if best == nil || filtered[i].Score > best.Score {
			best = &filtered[i]
		}
	}

	if best != nil {
		best.URL = e.processImageURL(best.URL)
		log.Printf("[ImageExtractor] Selected best image: %s (source: %s, score: %d)", best.URL, best.Source, best.Score)
		return best
	}

	return nil
}

// extractFromMediaContent 从 media:content 提取
func (e *ImageExtractor) extractFromMediaContent(enhanced *EnhancedItem) []ImageCandidate {
	var candidates []ImageCandidate

	if len(enhanced.MediaContent) == 0 {
		return candidates
	}

	// 优先选择 medium="image"
	for _, media := range enhanced.MediaContent {
		if media.Medium == "image" && media.URL != "" {
			candidate := ImageCandidate{
				URL:    media.URL,
				Source: "media:content",
				Medium: media.Medium,
				Alt:    media.Description,
				Credit: media.Credit,
			}
			// 解析尺寸
			if media.Width != "" {
				if w, err := strconv.Atoi(media.Width); err == nil {
					candidate.Width = w
				}
			}
			if media.Height != "" {
				if h, err := strconv.Atoi(media.Height); err == nil {
					candidate.Height = h
				}
			}
			candidates = append(candidates, candidate)
		}
	}

	// 如果没有 medium="image"，使用所有有URL的
	if len(candidates) == 0 {
		for _, media := range enhanced.MediaContent {
			if media.URL != "" {
				candidate := ImageCandidate{
					URL:    media.URL,
					Source: "media:content",
					Medium: media.Medium,
					Alt:    media.Description,
				}
				if media.Width != "" {
					if w, err := strconv.Atoi(media.Width); err == nil {
						candidate.Width = w
					}
				}
				if media.Height != "" {
					if h, err := strconv.Atoi(media.Height); err == nil {
						candidate.Height = h
					}
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates
}

// extractFromMediaThumbnail 从 media:thumbnail 提取
func (e *ImageExtractor) extractFromMediaThumbnail(enhanced *EnhancedItem) []ImageCandidate {
	var candidates []ImageCandidate

	if enhanced.MediaThumbnail != nil && enhanced.MediaThumbnail.URL != "" {
		candidate := ImageCandidate{
			URL:    enhanced.MediaThumbnail.URL,
			Source: "media:thumbnail",
		}
		if enhanced.MediaThumbnail.Width != "" {
			if w, err := strconv.Atoi(enhanced.MediaThumbnail.Width); err == nil {
				candidate.Width = w
			}
		}
		if enhanced.MediaThumbnail.Height != "" {
			if h, err := strconv.Atoi(enhanced.MediaThumbnail.Height); err == nil {
				candidate.Height = h
			}
		}
		candidates = append(candidates, candidate)
	}

	return candidates
}

// extractFromEnclosures 从 enclosures 提取图片
func (e *ImageExtractor) extractFromEnclosures(enhanced *EnhancedItem) []ImageCandidate {
	var candidates []ImageCandidate

	if enhanced.Enclosures != nil {
		for _, enc := range enhanced.Enclosures {
			if enc.Type != "" && strings.HasPrefix(enc.Type, "image/") && enc.URL != "" {
				candidate := ImageCandidate{
					URL:    enc.URL,
					Source: "enclosure",
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates
}

// extractFromHTML 从HTML内容中提取图片
func (e *ImageExtractor) extractFromHTML(contentHTML string) []ImageCandidate {
	var candidates []ImageCandidate

	// 解析HTML
	doc, err := html.Parse(strings.NewReader(contentHTML))
	if err != nil {
		return candidates
	}

	// 遍历查找 img 标签
	position := 0
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			var src, alt string
			var width, height int

			for _, attr := range n.Attr {
				switch attr.Key {
				case "src":
					src = attr.Val
				case "alt":
					alt = attr.Val
				case "width":
					if w, err := strconv.Atoi(attr.Val); err == nil {
						width = w
					}
				case "height":
					if h, err := strconv.Atoi(attr.Val); err == nil {
						height = h
					}
				}
			}

			if src != "" && e.isValidImageURL(src) {
				candidate := ImageCandidate{
					URL:    src,
					Source: "content_html",
					Width:  width,
					Height: height,
					Alt:    alt,
				}
				candidates = append(candidates, candidate)
			}
			position++
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return candidates
}

// isPlaceholderImage 检测占位图
// 复刻客户端逻辑：检查URL和alt属性中的占位图特征
func (e *ImageExtractor) isPlaceholderImage(url string, alt string) bool {
	if url == "" {
		return true
	}

	urlLower := strings.ToLower(url)
	altLower := strings.ToLower(alt)

	// 占位图URL特征
	placeholderPatterns := []string{
		"placeholder",
		"loading",
		"grey-placeholder",
		"gray-placeholder",
		"dummy",
		"blank",
		"default.png",
		"default.jpg",
		"spacer",
		"1x1",
		"pixel",
		"transparent",
		"avatar-default",
	}

	for _, pattern := range placeholderPatterns {
		if strings.Contains(urlLower, pattern) {
			log.Printf("[ImageExtractor] Detected placeholder image (URL pattern: %s): %s", pattern, url)
			return true
		}
	}

	// alt 属性特征
	if altLower == "loading" || altLower == "image unavailable" || altLower == "placeholder" {
		log.Printf("[ImageExtractor] Detected placeholder image (alt: %s)", alt)
		return true
	}

	return false
}

// processImageURL 处理特殊格式的图片URL
// 复刻客户端逻辑：处理Engadget等特殊格式
func (e *ImageExtractor) processImageURL(url string) string {
	if url == "" {
		return ""
	}

	// Engadget 格式处理：https://example.com/image.jpg?w=300&h=200
	// 提取原始URL，移除查询参数中的尺寸限制
	if strings.Contains(url, "engadget") || strings.Contains(url, "?w=") {
		// 保留URL，但可能需要调整尺寸参数
		// 简单实现：返回原始URL
		return url
	}

	return url
}

// isValidImageURL 验证图片URL有效性
func (e *ImageExtractor) isValidImageURL(url string) bool {
	if url == "" {
		return false
	}

	// 必须是http或https
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}

	// 检查是否为图片格式
	urlLower := strings.ToLower(url)
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg"}

	// 移除查询参数检查文件扩展名
	urlPath := urlLower
	if idx := strings.Index(urlPath, "?"); idx != -1 {
		urlPath = urlPath[:idx]
	}

	hasImageExt := false
	for _, ext := range imageExts {
		if strings.HasSuffix(urlPath, ext) {
			hasImageExt = true
			break
		}
	}

	// 即使没有明确扩展名，如果URL看起来像图片也接受
	if !hasImageExt {
		// 检查URL中是否包含图片相关关键词
		imagePatterns := []string{"image", "img", "photo", "picture", "thumb", "avatar"}
		for _, pattern := range imagePatterns {
			if strings.Contains(urlLower, pattern) {
				return true
			}
		}
		return false
	}

	return true
}

// isAntiHotlinkDomain 检查是否是防盗链域名
func (e *ImageExtractor) isAntiHotlinkDomain(url string) bool {
	// 常见防盗链域名列表
	antiHotlinkDomains := []string{
		"weibo.com",
		"zhihu.com",
		"bilibili.com",
	}

	urlLower := strings.ToLower(url)
	for _, domain := range antiHotlinkDomains {
		if strings.Contains(urlLower, domain) {
			return true
		}
	}

	return false
}

// scoreImage 为图片评分
// 评分标准：尺寸、来源、位置等
func (e *ImageExtractor) scoreImage(candidate *ImageCandidate) int {
	score := 0

	// 1. 来源评分（优先级）
	switch candidate.Source {
	case "media:content":
		score += 100
		if candidate.Medium == "image" {
			score += 50 // medium="image" 额外加分
		}
	case "media:thumbnail":
		score += 80
	case "enclosure":
		score += 70
	case "content_html":
		score += 50
	}

	// 2. 尺寸评分
	if candidate.Width > 0 && candidate.Height > 0 {
		// 有明确尺寸加分
		score += 20

		// 符合最小尺寸要求加分
		if candidate.Width >= e.MinWidth && candidate.Height >= e.MinHeight {
			score += 30
		}

		// 大尺寸额外加分（但有上限，避免过大图片）
		if candidate.Width >= 800 && candidate.Width <= 2000 {
			score += 20
		}
		if candidate.Height >= 600 && candidate.Height <= 2000 {
			score += 20
		}

		// 宽高比合理（避免过窄或过宽）
		ratio := float64(candidate.Width) / float64(candidate.Height)
		if ratio >= 0.5 && ratio <= 2.5 {
			score += 10
		}
	}

	// 3. URL质量评分
	urlLower := strings.ToLower(candidate.URL)

	// 包含高质量关键词
	qualityKeywords := []string{"cover", "featured", "hero", "main", "og:image"}
	for _, kw := range qualityKeywords {
		if strings.Contains(urlLower, kw) {
			score += 15
			break
		}
	}

	// 包含低质量关键词（减分）
	lowQualityKeywords := []string{"avatar", "icon", "logo", "ads", "banner"}
	for _, kw := range lowQualityKeywords {
		if strings.Contains(urlLower, kw) {
			score -= 20
			break
		}
	}

	// 4. 防盗链域名（减分）
	if e.isAntiHotlinkDomain(candidate.URL) {
		score -= 30
	}

	return score
}

// ImageWithCaption (in html_utils.go)
// ExtractImageFromHTML (in html_utils.go)

// HTML实体解码的正则表达式
var htmlEntityRegex = regexp.MustCompile(`&([a-zA-Z]+|#\d+|#x[0-9a-fA-F]+);`)

// decodeHTMLEntities 解码HTML实体
func decodeHTMLEntities(text string) string {
	// 简化版本，处理常见实体
	replacements := map[string]string{
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&#39;":   "'",
		"&apos;":  "'",
		"&nbsp;":  " ",
		"&mdash;": "—",
		"&ndash;": "–",
	}

	result := text
	for entity, replacement := range replacements {
		result = strings.ReplaceAll(result, entity, replacement)
	}

	return result
}
