package middleware

import (
	"strconv"
	"time"

	"github.com/rayzhoull/gin-admin/internal/app/config"
	"github.com/rayzhoull/gin-admin/internal/app/errors"
	"github.com/rayzhoull/gin-admin/internal/app/ginplus"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/go-redis/redis_rate"
	"golang.org/x/time/rate"
)

// RateLimiterMiddleware 请求频率限制中间件
func RateLimiterMiddleware(skipper ...SkipperFunc) gin.HandlerFunc {
	cfg := config.GetGlobalConfig().RateLimiter
	if !cfg.Enable {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	rc := config.GetGlobalConfig().Redis
	ring := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": rc.Addr,
		},
		Password: rc.Password,
		DB:       cfg.RedisDB,
	})

	limiter := redis_rate.NewLimiter(ring)
	limiter.Fallback = rate.NewLimiter(rate.Inf, 0)

	return func(c *gin.Context) {
		if (len(skipper) > 0 && skipper[0](c)) || limiter == nil {
			c.Next()
			return
		}

		userID := ginplus.GetUserID(c)
		if userID == "" {
			c.Next()
			return
		}

		limit := cfg.Count
		rate, delay, allowed := limiter.AllowMinute(userID, limit)
		if !allowed {
			h := c.Writer.Header()
			h.Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
			h.Set("X-RateLimit-Remaining", strconv.FormatInt(limit-rate, 10))
			delaySec := int64(delay / time.Second)
			h.Set("X-RateLimit-Delay", strconv.FormatInt(delaySec, 10))
			ginplus.ResError(c, errors.ErrTooManyRequests)
			return
		}

		c.Next()
	}
}
