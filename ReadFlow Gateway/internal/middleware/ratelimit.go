package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter 限流器
type RateLimiter struct {
	limiters sync.Map // key: userID, value: *rate.Limiter
	rps      int      // 每秒请求数
	burst    int      // 突发容量
}

// NewRateLimiter 创建限流器
func NewRateLimiter(rps, burst int) *RateLimiter {
	return &RateLimiter{
		rps:   rps,
		burst: burst,
	}
}

// GetLimiter 获取用户的限流器
func (rl *RateLimiter) GetLimiter(userID int64) *rate.Limiter {
	limiter, ok := rl.limiters.Load(userID)
	if !ok {
		limiter = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
		rl.limiters.Store(userID, limiter)
	}
	return limiter.(*rate.Limiter)
}

// Middleware 限流中间件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从context获取userID（需要在认证中间件之后调用）
		userIDVal, exists := c.Get("user_id")
		if !exists {
			// 如果没有用户ID，跳过限流（可能是公开接口）
			c.Next()
			return
		}

		userID := userIDVal.(int64)
		limiter := rl.GetLimiter(userID)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "请求频率超限，请稍后重试",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// NewSyncLimiter 创建同步接口限流器（每分钟10次）
func NewSyncLimiter() *RateLimiter {
	return NewRateLimiter(10, 10) // 10 req/min
}

// NewAckLimiter 创建ACK接口限流器（每分钟20次）
func NewAckLimiter() *RateLimiter {
	return NewRateLimiter(20, 20) // 20 req/min
}

// NewVocabLimiter 创建生词本接口限流器（每分钟30次）
func NewVocabLimiter() *RateLimiter {
	return NewRateLimiter(30, 30) // 30 req/min
}

// NewSubscribeLimiter 创建订阅接口限流器（每小时100次）
func NewSubscribeLimiter() *RateLimiter {
	return NewRateLimiter(2, 10) // 约100 req/hour，突发允许10次
}
