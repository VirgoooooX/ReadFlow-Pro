package worker

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

// ContentExtractor 完整内容提取器
// 使用 Mozilla Readability 算法从原始URL提取干净的文章内容
type ContentExtractor struct {
	httpClient *http.Client
	userAgent  string
}

// NewContentExtractor 创建内容提取器
func NewContentExtractor() *ContentExtractor {
	return &ContentExtractor{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 5,
			},
		},
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}

// ExtractFullContent 从URL提取完整内容
// 使用 Readability 算法提取干净的文章正文
func (e *ContentExtractor) ExtractFullContent(urlStr string) (string, error) {
	if urlStr == "" {
		return "", fmt.Errorf("empty URL")
	}

	log.Printf("[ContentExtractor] Extracting full content from: %s", urlStr)

	// 1. 获取HTML内容（带重试）
	htmlContent, err := e.fetchWithRetry(urlStr, 2)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}

	// 2. 使用 Readability 提取
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// 如果解析失败，尝试直接传递 nil 或者基础 URL
		// 但 readability 可能需要 Base URL 解析相对路径
		log.Printf("[ContentExtractor] Failed to parse URL %s: %v", urlStr, err)
	}

	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err != nil {
		log.Printf("[ContentExtractor] Readability failed: %v", err)
		return "", fmt.Errorf("readability extraction failed: %w", err)
	}

	// 3. 清理HTML
	cleanedContent := e.cleanHTML(article.Content)

	log.Printf("[ContentExtractor] Successfully extracted content (%d bytes)", len(cleanedContent))
	return cleanedContent, nil
}

// ExtractFullContentWithTimeout 带超时的内容提取
func (e *ContentExtractor) ExtractFullContentWithTimeout(url string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		content string
		err     error
	}

	ch := make(chan result, 1)

	go func() {
		content, err := e.ExtractFullContent(url)
		ch <- result{content, err}
	}()

	select {
	case res := <-ch:
		return res.content, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("content extraction timeout after %v", timeout)
	}
}

// fetchWithRetry 带重试的HTTP请求
func (e *ContentExtractor) fetchWithRetry(url string, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[ContentExtractor] Retry attempt %d/%d for %s", attempt, maxRetries, url)
			time.Sleep(time.Duration(attempt) * time.Second) // 递增延迟
		}

		content, err := e.fetch(url)
		if err == nil {
			return content, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("all %d attempts failed: %w", maxRetries+1, lastErr)
}

// fetch 执行HTTP请求
func (e *ContentExtractor) fetch(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// 设置 User-Agent 避免被反爬
	req.Header.Set("User-Agent", e.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

// cleanHTML 清理HTML内容
func (e *ContentExtractor) cleanHTML(htmlContent string) string {
	// 简单清理：移除脚本和样式标签
	cleaned := htmlContent

	// 移除 <script> 标签
	cleaned = removeTag(cleaned, "script")

	// 移除 <style> 标签
	cleaned = removeTag(cleaned, "style")

	// 移除注释
	cleaned = removeHTMLComments(cleaned)

	return cleaned
}

// removeTag 移除指定的HTML标签及其内容
func removeTag(html string, tagName string) string {
	// 简化实现：使用字符串替换
	// 更严谨的实现应该使用HTML parser
	for {
		startTag := fmt.Sprintf("<%s", tagName)
		endTag := fmt.Sprintf("</%s>", tagName)

		startIdx := strings.Index(strings.ToLower(html), strings.ToLower(startTag))
		if startIdx == -1 {
			break
		}

		// 查找标签结束
		tagEndIdx := strings.Index(html[startIdx:], ">")
		if tagEndIdx == -1 {
			break
		}

		// 查找闭合标签
		closeIdx := strings.Index(strings.ToLower(html[startIdx:]), strings.ToLower(endTag))
		if closeIdx == -1 {
			// 自闭合标签
			html = html[:startIdx] + html[startIdx+tagEndIdx+1:]
		} else {
			// 完整标签
			endIdx := startIdx + closeIdx + len(endTag)
			html = html[:startIdx] + html[endIdx:]
		}
	}

	return html
}

// removeHTMLComments 移除HTML注释
func removeHTMLComments(html string) string {
	for {
		startIdx := strings.Index(html, "<!--")
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(html[startIdx:], "-->")
		if endIdx == -1 {
			break
		}

		html = html[:startIdx] + html[startIdx+endIdx+3:]
	}

	return html
}

// shouldUseCorsProxy 判断是否需要使用CORS代理
func (e *ContentExtractor) shouldUseCorsProxy(url string) bool {
	// 某些域名可能有CORS限制，需要代理
	corsRestrictedDomains := []string{
		// 可以根据实际情况添加
	}

	urlLower := strings.ToLower(url)
	for _, domain := range corsRestrictedDomains {
		if strings.Contains(urlLower, domain) {
			return true
		}
	}

	return false
}

// Close 关闭提取器，清理资源
func (e *ContentExtractor) Close() {
	if e.httpClient != nil {
		e.httpClient.CloseIdleConnections()
	}
}
