package config

import (
	"sync"
)

// RuntimeConfig 运行时可修改的配置
type RuntimeConfig struct {
	mu sync.RWMutex

	// RSS 抓取间隔（秒）
	FetchInterval int

	// 图片处理配置
	ImageMaxWidth   int
	ImageQuality    int
	ImageConcurrent int

	// 图片缓存过期时间（秒），默认 86400（1 天）
	ImageCacheExpiration int

	// 文章保留时间（秒），悾清理文章需要等待的时间，默认 86400（1 天）
	ItemRetentionTime int

	// 日志级别
	LogLevel string

	// 其他运行时配置
	MaxItemsPerFetch int // 每次抓取最多保留的文章数
	MaxRetries       int // 最大重试次数
	ReadTimeout      int // 读取超时（秒）
	ConnectTimeout   int // 连接超时（秒）
}

var runtimeConfig *RuntimeConfig
var once sync.Once

// GetRuntimeConfig 获取全局运行时配置实例
func GetRuntimeConfig() *RuntimeConfig {
	once.Do(func() {
		runtimeConfig = &RuntimeConfig{
			FetchInterval:        900, // 15 分钟
			ImageMaxWidth:        1080,
			ImageQuality:         75,
			ImageConcurrent:      2,
			ImageCacheExpiration: 86400, // 1 天
			ItemRetentionTime:    86400, // 1 天
			LogLevel:             "info",
			MaxItemsPerFetch:     500,
			MaxRetries:           3,
			ReadTimeout:          30,
			ConnectTimeout:       10,
		}
	})
	return runtimeConfig
}

// GetFetchInterval 获取 RSS 抓取间隔
func (rc *RuntimeConfig) GetFetchInterval() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.FetchInterval
}

// SetFetchInterval 设置 RSS 抓取间隔
func (rc *RuntimeConfig) SetFetchInterval(interval int) {
	if interval < 60 {
		interval = 60 // 最少 60 秒
	}
	if interval > 86400 {
		interval = 86400 // 最多 24 小时
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.FetchInterval = interval
}

// GetImageQuality 获取图片质量
func (rc *RuntimeConfig) GetImageQuality() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ImageQuality
}

// SetImageQuality 设置图片质量
func (rc *RuntimeConfig) SetImageQuality(quality int) {
	if quality < 10 {
		quality = 10
	}
	if quality > 100 {
		quality = 100
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.ImageQuality = quality
}

// GetImageMaxWidth 获取图片最大宽度
func (rc *RuntimeConfig) GetImageMaxWidth() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ImageMaxWidth
}

// SetImageMaxWidth 设置图片最大宽度
func (rc *RuntimeConfig) SetImageMaxWidth(width int) {
	if width < 300 {
		width = 300 // 最小 300px
	}
	if width > 4000 {
		width = 4000 // 最大 4000px
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.ImageMaxWidth = width
}

// GetImageConcurrent 获取图片并发数
func (rc *RuntimeConfig) GetImageConcurrent() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ImageConcurrent
}

// SetImageConcurrent 设置图片并发数
func (rc *RuntimeConfig) SetImageConcurrent(concurrent int) {
	if concurrent < 1 {
		concurrent = 1
	}
	if concurrent > 10 {
		concurrent = 10
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.ImageConcurrent = concurrent
}

// GetLogLevel 获取日志级别
func (rc *RuntimeConfig) GetLogLevel() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.LogLevel
}

// SetLogLevel 设置日志级别
func (rc *RuntimeConfig) SetLogLevel(level string) {
	if level != "debug" && level != "info" && level != "warn" && level != "error" {
		level = "info" // 默认为 info
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.LogLevel = level
}

// GetMaxItemsPerFetch 获取每次抓取最多保留的文章数
func (rc *RuntimeConfig) GetMaxItemsPerFetch() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.MaxItemsPerFetch
}

// SetMaxItemsPerFetch 设置每次抓取最多保留的文章数
func (rc *RuntimeConfig) SetMaxItemsPerFetch(count int) {
	if count < 10 {
		count = 10
	}
	if count > 5000 {
		count = 5000
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.MaxItemsPerFetch = count
}

// GetItemRetentionTime 获取文章保留时间
func (rc *RuntimeConfig) GetItemRetentionTime() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ItemRetentionTime
}

// SetItemRetentionTime 设置文章保留时间
func (rc *RuntimeConfig) SetItemRetentionTime(seconds int) {
	if seconds < 3600 {
		seconds = 3600 // 最少 1 小时
	}
	if seconds > 2592000 {
		seconds = 2592000 // 最多 30 天
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.ItemRetentionTime = seconds
}

// GetImageCacheExpiration 获取图片缓存过期时间
func (rc *RuntimeConfig) GetImageCacheExpiration() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ImageCacheExpiration
}

// SetImageCacheExpiration 设置图片缓存过期时间
func (rc *RuntimeConfig) SetImageCacheExpiration(seconds int) {
	if seconds < 3600 {
		seconds = 3600 // 最少 1 小时
	}
	if seconds > 2592000 {
		seconds = 2592000 // 最多 30 天
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.ImageCacheExpiration = seconds
}

// GetAllConfig 获取所有运行时配置
func (rc *RuntimeConfig) GetAllConfig() map[string]interface{} {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return map[string]interface{}{
		"fetch_interval":         rc.FetchInterval,
		"image_max_width":        rc.ImageMaxWidth,
		"image_quality":          rc.ImageQuality,
		"image_concurrent":       rc.ImageConcurrent,
		"image_cache_expiration": rc.ImageCacheExpiration,
		"item_retention_time":    rc.ItemRetentionTime,
		"log_level":              rc.LogLevel,
		"max_items_per_fetch":    rc.MaxItemsPerFetch,
		"max_retries":            rc.MaxRetries,
		"read_timeout":           rc.ReadTimeout,
		"connect_timeout":        rc.ConnectTimeout,
	}
}

// UpdateConfig 批量更新配置
func (rc *RuntimeConfig) UpdateConfig(updates map[string]interface{}) map[string]string {
	errors := make(map[string]string)

	for key, value := range updates {
		switch key {
		case "fetch_interval":
			if v, ok := value.(float64); ok {
				rc.SetFetchInterval(int(v))
			} else {
				errors[key] = "必须是整数"
			}
		case "image_quality":
			if v, ok := value.(float64); ok {
				rc.SetImageQuality(int(v))
			} else {
				errors[key] = "必须是整数"
			}
		case "image_max_width":
			if v, ok := value.(float64); ok {
				rc.SetImageMaxWidth(int(v))
			} else {
				errors[key] = "必须是整数"
			}
		case "image_concurrent":
			if v, ok := value.(float64); ok {
				rc.SetImageConcurrent(int(v))
			} else {
				errors[key] = "必须是整数"
			}
		case "item_retention_time":
			if v, ok := value.(float64); ok {
				rc.SetItemRetentionTime(int(v))
			} else {
				errors[key] = "必须是整数"
			}
		case "image_cache_expiration":
			if v, ok := value.(float64); ok {
				rc.SetImageCacheExpiration(int(v))
			} else {
				errors[key] = "必须是整数"
			}
		case "log_level":
			if v, ok := value.(string); ok {
				rc.SetLogLevel(v)
			} else {
				errors[key] = "必须是字符串"
			}
		case "max_items_per_fetch":
			if v, ok := value.(float64); ok {
				rc.SetMaxItemsPerFetch(int(v))
			} else {
				errors[key] = "必须是整数"
			}
		default:
			errors[key] = "未知的配置项"
		}
	}

	return errors
}
