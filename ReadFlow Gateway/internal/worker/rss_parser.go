package worker

import (
	"strings"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
)

// MediaContentInfo 媒体内容信息（来自 media:content）
type MediaContentInfo struct {
	URL         string
	Width       string
	Height      string
	Medium      string
	Type        string
	Description string // media:description
	Credit      string // media:credit
	Title       string // media:title
}

// MediaThumbnailInfo 媒体缩略图信息（来自 media:thumbnail）
type MediaThumbnailInfo struct {
	URL    string
	Width  string
	Height string
}

// EnhancedItem 增强的 RSS 条目
type EnhancedItem struct {
	*gofeed.Item
	MediaContent   []MediaContentInfo
	MediaThumbnail *MediaThumbnailInfo
}

// ExtractMediaContent 从 gofeed.Item 中提取 media:content 信息
func ExtractMediaContent(item *gofeed.Item) []MediaContentInfo {
	if item == nil || item.Extensions == nil {
		return nil
	}

	var mediaContents []MediaContentInfo

	// 获取 media 扩展
	mediaExts, ok := item.Extensions["media"]
	if !ok {
		return nil
	}

	// 提取 item 级别的 media 元数据
	itemMediaDesc := getMapExtensionText(mediaExts, "description")
	itemMediaCredit := getMapExtensionText(mediaExts, "credit")
	itemMediaTitle := getMapExtensionText(mediaExts, "title")

	// 获取所有 media:content 节点
	contents, ok := mediaExts["content"]
	if !ok {
		return nil
	}

	for _, content := range contents {
		mediaInfo := MediaContentInfo{
			URL:    getAttr(content.Attrs, "url"),
			Width:  getAttr(content.Attrs, "width"),
			Height: getAttr(content.Attrs, "height"),
			Medium: getAttr(content.Attrs, "medium"),
			Type:   getAttr(content.Attrs, "type"),
		}

		// 尝试从 media:content 内部获取元数据（优先级更高）
		if content.Children != nil {
			for key, children := range content.Children {
				if len(children) > 0 {
					text := strings.TrimSpace(children[0].Value)
					switch key {
					case "description":
						mediaInfo.Description = text
					case "credit":
						mediaInfo.Credit = text
					case "title":
						mediaInfo.Title = text
					}
				}
			}
		}

		// 如果内部没有，使用 item 级别的
		if mediaInfo.Description == "" {
			mediaInfo.Description = itemMediaDesc
		}
		if mediaInfo.Credit == "" {
			mediaInfo.Credit = itemMediaCredit
		}
		if mediaInfo.Title == "" {
			mediaInfo.Title = itemMediaTitle
		}

		// 只保留有 URL 的 media:content
		if mediaInfo.URL != "" {
			mediaContents = append(mediaContents, mediaInfo)
		}
	}

	return mediaContents
}

// ExtractMediaThumbnail 从 gofeed.Item 中提取 media:thumbnail 信息
func ExtractMediaThumbnail(item *gofeed.Item) *MediaThumbnailInfo {
	if item == nil || item.Extensions == nil {
		return nil
	}

	mediaExts, ok := item.Extensions["media"]
	if !ok {
		return nil
	}

	thumbnails, ok := mediaExts["thumbnail"]
	if !ok || len(thumbnails) == 0 {
		return nil
	}

	thumbnail := thumbnails[0]
	url := getAttr(thumbnail.Attrs, "url")
	if url == "" {
		return nil
	}

	return &MediaThumbnailInfo{
		URL:    url,
		Width:  getAttr(thumbnail.Attrs, "width"),
		Height: getAttr(thumbnail.Attrs, "height"),
	}
}

// EnhanceItem 增强 gofeed.Item，提取 media 信息
func EnhanceItem(item *gofeed.Item) *EnhancedItem {
	if item == nil {
		return nil
	}

	enhanced := &EnhancedItem{
		Item:           item,
		MediaContent:   ExtractMediaContent(item),
		MediaThumbnail: ExtractMediaThumbnail(item),
	}

	return enhanced
}

// getMapExtensionText 从 map类型的扩展中获取文本内容
func getMapExtensionText(exts map[string][]ext.Extension, key string) string {
	if items, ok := exts[key]; ok && len(items) > 0 {
		return strings.TrimSpace(items[0].Value)
	}
	return ""
}

// getAttr 从属性映射中获取值
func getAttr(attrs map[string]string, key string) string {
	if val, ok := attrs[key]; ok {
		return strings.TrimSpace(val)
	}
	return ""
}
