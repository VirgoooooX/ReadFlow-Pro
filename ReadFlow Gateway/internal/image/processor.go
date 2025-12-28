package image

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/readflow/gateway/internal/config"
	"golang.org/x/net/html"
)

// Processor 图片处理器
type Processor struct {
	config     *config.Config
	httpClient *http.Client
	semaphore  chan struct{}
	baseURL    string
	refererMap map[string]string
}

// NewProcessor 创建图片处理器
func NewProcessor(cfg *config.Config) *Processor {
	// 初始化 vips
	vips.LoggingSettings(nil, vips.LogLevelError)
	vips.Startup(nil)

	// 初始化 Referer 映射表（防盗链绕过）
	refererMap := map[string]string{
		// cnBeta
		"cnbetacdn.com":        "https://www.cnbeta.com.tw/",
		"static.cnbetacdn.com": "https://www.cnbeta.com.tw/",
		// Engadget / Yahoo
		"yimg.com":       "https://www.engadget.com/",
		"s.yimg.com":     "https://www.engadget.com/",
		"aolcdn.com":     "https://www.engadget.com/",
		"o.aolcdn.com":   "https://www.engadget.com/",
		"cloudfront.net": "https://www.engadget.com/",
		// Twitter
		"twimg.com":     "https://twitter.com/",
		"pbs.twimg.com": "https://twitter.com/",
		// Facebook/Instagram
		"fbcdn.net":        "https://www.facebook.com/",
		"cdninstagram.com": "https://www.instagram.com/",
		// Medium
		"medium.com":      "https://medium.com/",
		"miro.medium.com": "https://medium.com/",
		// Imgur
		"imgur.com":   "https://imgur.com/",
		"i.imgur.com": "https://imgur.com/",
		// WordPress
		"wp.com":    "https://wordpress.com/",
		"i0.wp.com": "https://wordpress.com/",
		"i1.wp.com": "https://wordpress.com/",
		"i2.wp.com": "https://wordpress.com/",
		// GitHub
		"githubusercontent.com":     "https://github.com/",
		"raw.githubusercontent.com": "https://github.com/",
		// Unsplash
		"unsplash.com":        "https://unsplash.com/",
		"images.unsplash.com": "https://unsplash.com/",
		// Flickr
		"staticflickr.com": "https://www.flickr.com/",
		// Giphy
		"giphy.com":       "https://giphy.com/",
		"media.giphy.com": "https://giphy.com/",
		// Reddit
		"redd.it":         "https://www.reddit.com/",
		"i.redd.it":       "https://www.reddit.com/",
		"preview.redd.it": "https://www.reddit.com/",
		// Client Identified Domains
		"sspai.com":    "https://sspai.com/",
		"ifanr.com":    "https://www.ifanr.com/",
		"ifanr.cn":     "https://www.ifanr.com/",
		"36kr.com":     "https://36kr.com/",
		"weibo.com":    "https://weibo.com/",
		"sinaimg.cn":   "https://weibo.com/",
		"zhihu.com":    "https://www.zhihu.com/",
		"zhimg.com":    "https://www.zhihu.com/",
		"bilibili.com": "https://www.bilibili.com/",
		"hdslb.com":    "https://www.bilibili.com/",
		"douban.com":   "https://www.douban.com/",
		"doubanio.com": "https://www.douban.com/",
	}

	return &Processor{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
			},
		},
		semaphore:  make(chan struct{}, cfg.ImageConcurrent),
		baseURL:    fmt.Sprintf("http://localhost:%s", cfg.ServerPort),
		refererMap: refererMap,
	}
}

// ProcessContent 处理HTML内容中的图片
func (p *Processor) ProcessContent(sourceID int64, htmlContent string) (processedHTML string, imagePaths string, err error) {
	if htmlContent == "" {
		return htmlContent, "", nil
	}

	// 解析HTML
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		log.Printf("HTML parse failed: %v", err)
		return htmlContent, "", nil
	}

	// 提取图片URL
	imageURLs := p.extractImageURLs(doc)
	if len(imageURLs) == 0 {
		return htmlContent, "", nil
	}

	log.Printf("Found %d images in source %d", len(imageURLs), sourceID)

	// 处理图片并建立URL映射
	urlMapping := p.processImages(sourceID, imageURLs)

	// 替换HTML中的图片链接
	p.replaceImageURLs(doc, urlMapping)

	// 渲染新的HTML，但只提取 body 中的内容
	var buf strings.Builder
	var bodyFound bool
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			bodyFound = true
			// 遍历 body 的所有子节点
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				html.Render(&buf, c)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// 如果没有找到 body（例如输入不是完整的HTML），则直接渲染整个文档
	if !bodyFound {
		buf.Reset()
		if err := html.Render(&buf, doc); err != nil {
			log.Printf("HTML render failed: %v", err)
			return htmlContent, "", nil
		}
	}

	// 构建image_paths JSON
	var paths []string
	for _, localPath := range urlMapping {
		if localPath != "" {
			paths = append(paths, localPath)
		}
	}

	var imagePathsJSON string
	if len(paths) > 0 {
		pathsBytes, _ := json.Marshal(paths)
		imagePathsJSON = string(pathsBytes)
	}

	return buf.String(), imagePathsJSON, nil
}

// extractImageURLs 提取所有图片URL
func (p *Processor) extractImageURLs(n *html.Node) []string {
	var urls []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					url := strings.TrimSpace(attr.Val)
					if p.isValidImageURL(url) {
						urls = append(urls, url)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return urls
}

// isValidImageURL 检查是否是有效的图片URL
func (p *Processor) isValidImageURL(url string) bool {
	if url == "" {
		return false
	}
	// 过滤data URI、blob等
	if strings.HasPrefix(url, "data:") || strings.HasPrefix(url, "blob:") {
		return false
	}
	// 允许 http/https 以及协议相对的 //url 形式
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return true
	}
	if strings.HasPrefix(url, "//") {
		return true
	}
	return false
}

// processImages 并发处理图片
func (p *Processor) processImages(sourceID int64, imageURLs []string) map[string]string {
	urlMapping := make(map[string]string)
	resultChan := make(chan struct {
		url       string
		localPath string
	}, len(imageURLs))

	// 并发处理每个图片
	for _, url := range imageURLs {
		go func(imgURL string) {
			p.semaphore <- struct{}{}        // 获取许可
			defer func() { <-p.semaphore }() // 释放许可

			localPath, err := p.processImage(sourceID, imgURL)
			if err != nil {
				log.Printf("Process image failed: url=%s, error=%v", imgURL, err)
				localPath = "" // 失败时保留原始URL
			}

			resultChan <- struct {
				url       string
				localPath string
			}{imgURL, localPath}
		}(url)
	}

	// 收集结果
	for i := 0; i < len(imageURLs); i++ {
		result := <-resultChan
		urlMapping[result.url] = result.localPath
	}

	return urlMapping
}

// processImage 处理单个图片
func (p *Processor) processImage(sourceID int64, url string) (string, error) {
	// 下载图片
	imageData, err := p.downloadImage(url)
	if err != nil {
		return "", err
	}

	// 计算哈希
	hash := p.calculateHash(imageData)

	// 生成文件路径
	fileName := hash[:12] + ".webp"
	localPath := fmt.Sprintf("/static/images/%d/%s", sourceID, fileName)
	fullPath := filepath.Join(p.config.StaticDir, "images", fmt.Sprintf("%d", sourceID), fileName)

	// 检查文件是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		// 文件已存在，直接返回
		return localPath, nil
	}

	// 压缩图片
	webpData, err := p.compressImage(imageData)
	if err != nil {
		return "", err
	}

	// 保存到磁盘
	if err := p.saveImage(fullPath, webpData); err != nil {
		return "", err
	}

	log.Printf("Image processed: %s -> %s", url, localPath)
	return localPath, nil
}

// replaceImageURLs 替换HTML中的图片URL
func (p *Processor) replaceImageURLs(n *html.Node, urlMapping map[string]string) {
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for i, attr := range n.Attr {
				if attr.Key == "src" {
					if localPath, ok := urlMapping[attr.Val]; ok && localPath != "" {
						// 替换为本地URL
						n.Attr[i].Val = p.baseURL + localPath
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}

// downloadImage 下载图片
func (p *Processor) downloadImage(url string) ([]byte, error) {
	// 处理协议相对的 URL，例如 //example.com/image.jpg，默认使用 https
	if strings.HasPrefix(url, "//") {
		url = "https:" + url
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "image/*,*/*")

	// 设置 Referer 防盗链
	if referer := p.getReferer(url); referer != "" {
		req.Header.Set("Referer", referer)
		log.Printf("[Image] Set Referer: %s for %s", referer, url)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 限制最大10MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}

	return data, nil
}

// compressImage 压缩图片为WebP
func (p *Processor) compressImage(imageData []byte) ([]byte, error) {
	// 加载图片
	img, err := vips.NewImageFromBuffer(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to load image: %w", err)
	}
	defer img.Close()

	// 如果宽度超过设定值，等比缩放
	if img.Width() > p.config.ImageMaxWidth {
		scale := float64(p.config.ImageMaxWidth) / float64(img.Width())
		if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
			return nil, fmt.Errorf("failed to resize image: %w", err)
		}
	}

	// 转换为WebP
	ep := vips.NewWebpExportParams()
	ep.Quality = p.config.ImageQuality
	ep.StripMetadata = true

	webpBytes, _, err := img.ExportWebp(ep)
	if err != nil {
		return nil, fmt.Errorf("failed to export webp: %w", err)
	}

	return webpBytes, nil
}

// calculateHash 计算SHA256哈希
func (p *Processor) calculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// saveImage 保存图片到磁盘
func (p *Processor) saveImage(fullPath string, data []byte) error {
	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入文件
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return err
	}

	return nil
}

// getReferer 根据域名获取合适的 Referer
func (p *Processor) getReferer(targetURL string) string {
	u, err := url.Parse(targetURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Host)

	// 精确匹配
	if ref, ok := p.refererMap[host]; ok {
		return ref
	}

	// 部分匹配
	for domain, referer := range p.refererMap {
		if strings.HasSuffix(host, domain) || strings.Contains(host, domain) {
			return referer
		}
	}

	// 默认返回空，不设置 Referer
	return ""
}

// Shutdown 关闭处理器
func (p *Processor) Shutdown() {
	vips.Shutdown()
}

// GetDominantColorFromURL 下载图片并提取主色调
func (p *Processor) GetDominantColorFromURL(url string) (string, error) {
	if url == "" {
		return "", nil
	}
	data, err := p.downloadImage(url)
	if err != nil {
		return "", err
	}
	return p.extractDominantColor(data)
}

// extractDominantColor 从图片数据中提取主色调
func (p *Processor) extractDominantColor(data []byte) (string, error) {
	img, err := vips.NewImageFromBuffer(data)
	if err != nil {
		return "", err
	}
	defer img.Close()

	// Resize to 10x10 to reduce processing
	if err := img.Thumbnail(10, 10, vips.InterestingCentre); err != nil {
		return "", err
	}

	// Export as JPEG for easy decoding
	ep := vips.NewJpegExportParams()
	ep.Quality = 80
	jpgBytes, _, err := img.ExportJpeg(ep)
	if err != nil {
		return "", err
	}

	// Decode with Go standard lib
	goImg, err := jpeg.Decode(bytes.NewReader(jpgBytes))
	if err != nil {
		return "", err
	}

	// Calculate Average
	bounds := goImg.Bounds()
	var r, g, b, count uint64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pr, pg, pb, _ := goImg.At(x, y).RGBA()
			r += uint64(pr)
			g += uint64(pg)
			b += uint64(pb)
			count++
		}
	}

	if count == 0 {
		return "", fmt.Errorf("empty image")
	}

	// RGBA returns 0-65535, we want 0-255
	r = (r / count) >> 8
	g = (g / count) >> 8
	b = (b / count) >> 8

	return fmt.Sprintf("#%02x%02x%02x", r, g, b), nil
}
