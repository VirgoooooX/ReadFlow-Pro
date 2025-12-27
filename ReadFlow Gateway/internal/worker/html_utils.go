package worker

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	// HTML 标签正则（预编译优化性能）
	tagRegex = regexp.MustCompile(`(?s)<[^>]*>`)
	// 空白字符正则
	spaceRegex = regexp.MustCompile(`\s+`)
	// img 标签正则（支持 figure 和独立 img）
	figureRegex = regexp.MustCompile(`(?s)<figure[^>]*>.*?<img[^>]+src\s*=\s*["']([^"']+)["'][^>]*(?:alt\s*=\s*["']([^"']*)["'])?[^>]*>.*?<figcaption[^>]*>(.*?)</figcaption>.*?</figure>`)
	imgRegex    = regexp.MustCompile(`<img[^>]+src\s*=\s*["']([^"']+)["'][^>]*(?:alt\s*=\s*["']([^"']*)["'])?[^>]*>`)
	imgRegex2   = regexp.MustCompile(`<img[^>]*alt\s*=\s*["']([^"']*)["'][^>]+src\s*=\s*["']([^"']+)["'][^>]*>`)
)

// ImageWithCaption 图片及其说明信息
type ImageWithCaption struct {
	URL     string
	Caption string
	Alt     string
	Source  string // "figure", "img", "media"
}

// DecodeHTMLEntities 解码 HTML 实体
func DecodeHTMLEntities(text string) string {
	if text == "" {
		return ""
	}
	// 使用标准库解码
	decoded := html.UnescapeString(text)
	return decoded
}

// CleanHTMLTags 清理 HTML 标签，只保留文本
func CleanHTMLTags(htmlText string) string {
	if htmlText == "" {
		return ""
	}
	
	// 替换 &nbsp; 为空格
	s := strings.ReplaceAll(htmlText, "&nbsp;", " ")
	// 移除所有 HTML 标签
	s = tagRegex.ReplaceAllString(s, " ")
	// 解码 HTML 实体
	s = html.UnescapeString(s)
	// 合并多个空白字符
	s = spaceRegex.ReplaceAllString(s, " ")
	
	return strings.TrimSpace(s)
}

// ExtractImageFromHTML 从 HTML 内容中提取图片及其说明
// skipFirst: 是否跳过第一张图片（用于过滤占位图）
func ExtractImageFromHTML(content string, skipFirst bool) *ImageWithCaption {
	if content == "" {
		return nil
	}
	
	// 先解码 HTML 实体
	decoded := DecodeHTMLEntities(content)
	
	// 1. 优先提取 figure 结构（带 figcaption）
	figureMatches := figureRegex.FindAllStringSubmatch(decoded, -1)
	if len(figureMatches) > 0 {
		startIdx := 0
		if skipFirst && len(figureMatches) > 1 {
			startIdx = 1
		}
		
		for i := startIdx; i < len(figureMatches); i++ {
			match := figureMatches[i]
			if len(match) >= 4 {
				imgURL := match[1]
				alt := match[2]
				caption := CleanHTMLTags(match[3])
				
				if imgURL != "" && (strings.HasPrefix(imgURL, "http") || strings.HasPrefix(imgURL, "/")) {
					return &ImageWithCaption{
						URL:     imgURL,
						Caption: caption,
						Alt:     alt,
						Source:  "figure",
					}
				}
			}
		}
	}
	
	// 2. 回退：提取普通 img 标签
	var imgMatches [][]string
	imgMatches = append(imgMatches, imgRegex.FindAllStringSubmatch(decoded, -1)...)
	imgMatches = append(imgMatches, imgRegex2.FindAllStringSubmatch(decoded, -1)...)
	
	if len(imgMatches) > 0 {
		startIdx := 0
		if skipFirst && len(imgMatches) > 1 {
			startIdx = 1
		}
		
		for i := startIdx; i < len(imgMatches); i++ {
			match := imgMatches[i]
			if len(match) >= 3 {
				var imgURL, alt string
				// 判断是哪种正则匹配
				if strings.Contains(match[0], "src") && strings.Index(match[0], "src") < strings.Index(match[0], "alt") {
					// imgRegex: src 在前
					imgURL = match[1]
					alt = match[2]
				} else {
					// imgRegex2: alt 在前
					alt = match[1]
					imgURL = match[2]
				}
				
				if imgURL != "" && (strings.HasPrefix(imgURL, "http") || strings.HasPrefix(imgURL, "/")) {
					return &ImageWithCaption{
						URL:     imgURL,
						Caption: alt,
						Alt:     alt,
						Source:  "img",
					}
				}
			}
		}
	}
	
	return nil
}

// ProcessImageURL 处理特殊格式的图片 URL
// 例如 Engadget 的 CDN URL 解码
func ProcessImageURL(imgURL string) string {
	if imgURL == "" {
		return imgURL
	}
	
	// 处理 Engadget 格式的 URL: o.aolcdn.com/images/dims?...&image_uri=xxx
	if strings.Contains(imgURL, "o.aolcdn.com/images/dims") && strings.Contains(imgURL, "image_uri=") {
		u, err := url.Parse(imgURL)
		if err == nil {
			if imageURI := u.Query().Get("image_uri"); imageURI != "" {
				// 解码 image_uri 参数
				if decoded, err := url.QueryUnescape(imageURI); err == nil {
					return decoded
				}
			}
		}
	}
	
	return imgURL
}
