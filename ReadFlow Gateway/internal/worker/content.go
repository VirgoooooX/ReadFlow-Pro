package worker

import (
	"log"
	"strings"
)

// SelectBestImage 从增强的 RSS item 中选择最佳图片
// 优先级：media:content > media:thumbnail > enclosures > HTML content
func SelectBestImage(enhanced *EnhancedItem) string {
	if enhanced == nil {
		return ""
	}
	
	// 1. 优先检查 media:content（优先选择 medium="image"）
	if len(enhanced.MediaContent) > 0 {
		// 先找 medium="image"
		for _, media := range enhanced.MediaContent {
			if media.Medium == "image" && media.URL != "" {
				if !IsPlaceholderImage(media.URL, "") {
					url := ProcessImageURL(media.URL)
					log.Printf("[Content] Selected image from media:content (medium=image): %s", url)
					return url
				}
			}
		}
		
		// 如果没有 medium="image"，使用第一个有 URL 的
		for _, media := range enhanced.MediaContent {
			if media.URL != "" {
				if !IsPlaceholderImage(media.URL, "") {
					url := ProcessImageURL(media.URL)
					log.Printf("[Content] Selected image from media:content: %s", url)
					return url
				}
			}
		}
	}
	
	// 2. 检查 media:thumbnail
	if enhanced.MediaThumbnail != nil && enhanced.MediaThumbnail.URL != "" {
		if !IsPlaceholderImage(enhanced.MediaThumbnail.URL, "") {
			log.Printf("[Content] Selected image from media:thumbnail: %s", enhanced.MediaThumbnail.URL)
			return enhanced.MediaThumbnail.URL
		}
	}
	
	// 3. 检查 enclosures 中的图片
	if enhanced.Enclosures != nil && len(enhanced.Enclosures) > 0 {
		for _, enc := range enhanced.Enclosures {
			if enc.Type != "" && strings.HasPrefix(enc.Type, "image/") && enc.URL != "" {
				if !IsPlaceholderImage(enc.URL, "") {
					log.Printf("[Content] Selected image from enclosure: %s", enc.URL)
					return enc.URL
				}
			}
		}
	}
	
	// 4. 从内容中提取图片
	content := enhanced.Content
	if content == "" {
		content = enhanced.Description
	}
	
	if content != "" {
		imgInfo := ExtractImageFromHTML(content, false)
		if imgInfo != nil && imgInfo.URL != "" {
			if !IsPlaceholderImage(imgInfo.URL, imgInfo.Alt) {
				url := ProcessImageURL(imgInfo.URL)
				log.Printf("[Content] Selected image from HTML content: %s", url)
				return url
			}
		}
	}
	
	return ""
}

// IsPlaceholderImage 判断 URL 是否为占位图
// 占位图特征：
// - URL 包含 placeholder、loading、grey-placeholder 等关键字
// - alt 属性为 "loading" 或 "image unavailable"
func IsPlaceholderImage(url string, alt string) bool {
	if url == "" {
		return false
	}
	
	urlLower := strings.ToLower(url)
	altLower := strings.ToLower(alt)
	
	// 检查 URL 特征
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
	}
	
	for _, pattern := range placeholderPatterns {
		if strings.Contains(urlLower, pattern) {
			log.Printf("[Content] Detected placeholder image (URL pattern: %s): %s", pattern, url)
			return true
		}
	}
	
	// 检查 alt 属性特征
	if altLower == "loading" || altLower == "image unavailable" {
		log.Printf("[Content] Detected placeholder image (alt: %s)", alt)
		return true
	}
	
	return false
}
