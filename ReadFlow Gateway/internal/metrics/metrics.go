package metrics

import (
	"runtime"
	"sync"
	"time"
)

// Metrics 监控指标
type Metrics struct {
	mu sync.RWMutex

	// API指标
	apiRequests       map[string]int64 // key: "method:path:status"
	apiDurations      []time.Duration
	apiRequestsTotal  int64
	apiErrorsTotal    int64

	// RSS指标
	rssFetchTotal    int64
	rssFetchSuccess  int64
	rssFetchFailed   int64

	// 图片处理指标
	imageProcessed   int64
	imageSuccess     int64
	imageFailed      int64

	// 业务指标
	activeUsers      int
	activeSources    int
	
	// 系统指标
	startTime        time.Time
}

var globalMetrics *Metrics
var once sync.Once

// GetMetrics 获取全局指标实例
func GetMetrics() *Metrics {
	once.Do(func() {
		globalMetrics = &Metrics{
			apiRequests:  make(map[string]int64),
			apiDurations: make([]time.Duration, 0, 1000),
			startTime:    time.Now(),
		}
	})
	return globalMetrics
}

// RecordAPIRequest 记录API请求
func (m *Metrics) RecordAPIRequest(method, path string, status int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := method + ":" + path + ":" + string(rune(status))
	m.apiRequests[key]++
	m.apiRequestsTotal++

	if status >= 500 {
		m.apiErrorsTotal++
	}

	// 记录响应时间（最多保存1000个）
	if len(m.apiDurations) < 1000 {
		m.apiDurations = append(m.apiDurations, duration)
	}
}

// RecordRSSFetch 记录RSS抓取
func (m *Metrics) RecordRSSFetch(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rssFetchTotal++
	if success {
		m.rssFetchSuccess++
	} else {
		m.rssFetchFailed++
	}
}

// RecordImageProcess 记录图片处理
func (m *Metrics) RecordImageProcess(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.imageProcessed++
	if success {
		m.imageSuccess++
	} else {
		m.imageFailed++
	}
}

// UpdateActiveUsers 更新活跃用户数
func (m *Metrics) UpdateActiveUsers(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeUsers = count
}

// UpdateActiveSources 更新活跃源数
func (m *Metrics) UpdateActiveSources(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeSources = count
}

// GetStats 获取统计数据
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"uptime_seconds": time.Since(m.startTime).Seconds(),
		"api": map[string]interface{}{
			"requests_total": m.apiRequestsTotal,
			"errors_total":   m.apiErrorsTotal,
			"error_rate":     m.calculateErrorRate(),
		},
		"rss": map[string]interface{}{
			"fetch_total":   m.rssFetchTotal,
			"fetch_success": m.rssFetchSuccess,
			"fetch_failed":  m.rssFetchFailed,
			"success_rate":  m.calculateRSSSuccessRate(),
		},
		"image": map[string]interface{}{
			"processed": m.imageProcessed,
			"success":   m.imageSuccess,
			"failed":    m.imageFailed,
		},
		"business": map[string]interface{}{
			"active_users":   m.activeUsers,
			"active_sources": m.activeSources,
		},
		"system": map[string]interface{}{
			"goroutines":    runtime.NumGoroutine(),
			"memory_alloc":  memStats.Alloc,
			"memory_sys":    memStats.Sys,
			"gc_runs":       memStats.NumGC,
		},
	}
}

func (m *Metrics) calculateErrorRate() float64 {
	if m.apiRequestsTotal == 0 {
		return 0
	}
	return float64(m.apiErrorsTotal) / float64(m.apiRequestsTotal)
}

func (m *Metrics) calculateRSSSuccessRate() float64 {
	if m.rssFetchTotal == 0 {
		return 0
	}
	return float64(m.rssFetchSuccess) / float64(m.rssFetchTotal)
}

// Reset 重置指标（测试用）
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.apiRequests = make(map[string]int64)
	m.apiDurations = make([]time.Duration, 0, 1000)
	m.apiRequestsTotal = 0
	m.apiErrorsTotal = 0
	m.rssFetchTotal = 0
	m.rssFetchSuccess = 0
	m.rssFetchFailed = 0
	m.imageProcessed = 0
	m.imageSuccess = 0
	m.imageFailed = 0
}
