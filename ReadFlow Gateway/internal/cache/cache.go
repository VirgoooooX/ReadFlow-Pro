package cache

import (
	"sync"
	"time"
)

// TokenCache Token缓存
type TokenCache struct {
	data    sync.Map // key: token, value: userID
	expiry  sync.Map // key: token, value: expiryTime
	ttl     time.Duration
	stopCh  chan struct{}
}

// NewTokenCache 创建Token缓存
func NewTokenCache(ttl time.Duration) *TokenCache {
	tc := &TokenCache{
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go tc.startCleanup()
	return tc
}

// Get 获取缓存
func (tc *TokenCache) Get(token string) (int64, bool) {
	// 检查是否过期
	if exp, ok := tc.expiry.Load(token); ok {
		if time.Now().After(exp.(time.Time)) {
			tc.Delete(token)
			return 0, false
		}
	}

	val, ok := tc.data.Load(token)
	if !ok {
		return 0, false
	}

	return val.(int64), true
}

// Set 设置缓存
func (tc *TokenCache) Set(token string, userID int64, ttl time.Duration) {
	if ttl == 0 {
		ttl = tc.ttl
	}

	tc.data.Store(token, userID)
	tc.expiry.Store(token, time.Now().Add(ttl))
}

// Delete 删除缓存
func (tc *TokenCache) Delete(token string) {
	tc.data.Delete(token)
	tc.expiry.Delete(token)
}

// startCleanup 启动清理协程
func (tc *TokenCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.cleanup()
		case <-tc.stopCh:
			return
		}
	}
}

// cleanup 清理过期条目
func (tc *TokenCache) cleanup() {
	now := time.Now()
	tc.expiry.Range(func(key, value interface{}) bool {
		if now.After(value.(time.Time)) {
			token := key.(string)
			tc.Delete(token)
		}
		return true
	})
}

// Stop 停止清理协程
func (tc *TokenCache) Stop() {
	close(tc.stopCh)
}

// SourceCache RSS源缓存
type SourceCache struct {
	data   sync.Map // key: sourceID, value: source metadata
	expiry sync.Map // key: sourceID, value: expiryTime
	ttl    time.Duration
}

// NewSourceCache 创建源缓存
func NewSourceCache(ttl time.Duration) *SourceCache {
	return &SourceCache{
		ttl: ttl,
	}
}

// Get 获取源缓存
func (sc *SourceCache) Get(sourceID int64) (interface{}, bool) {
	// 检查是否过期
	if exp, ok := sc.expiry.Load(sourceID); ok {
		if time.Now().After(exp.(time.Time)) {
			sc.Delete(sourceID)
			return nil, false
		}
	}

	return sc.data.Load(sourceID)
}

// Set 设置源缓存
func (sc *SourceCache) Set(sourceID int64, data interface{}) {
	sc.data.Store(sourceID, data)
	sc.expiry.Store(sourceID, time.Now().Add(sc.ttl))
}

// Delete 删除源缓存
func (sc *SourceCache) Delete(sourceID int64) {
	sc.data.Delete(sourceID)
	sc.expiry.Delete(sourceID)
}
