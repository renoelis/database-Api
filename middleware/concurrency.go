package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/renoelis/database-api-go/config"
	"github.com/renoelis/database-api-go/utils"
	"github.com/sirupsen/logrus"
)

var (
	// 全局并发控制计数器和锁
	generalCounter     int
	postgresqlCounter  int
	mongodbCounter     int
	counterMutex       sync.Mutex
	
	// 并发限制
	maxConcurrentRequests   int
	postgresqlMaxConcurrent int
	mongodbMaxConcurrent    int
)

// ConcurrencyLimiterMiddleware 返回并发限制中间件
func ConcurrencyLimiterMiddleware() gin.HandlerFunc {
	// 从环境变量获取并发限制配置
	maxConcurrentRequests = config.GetEnvInt("MAX_CONCURRENT_REQUESTS", 200)
	postgresqlMaxConcurrent = config.GetEnvInt("POSTGRESQL_MAX_CONCURRENT", 100)
	mongodbMaxConcurrent = config.GetEnvInt("MONGODB_MAX_CONCURRENT", 100)

	logger := logrus.New()
	logger.Infof("配置并发控制: 总并发=%d, PostgreSQL=%d, MongoDB=%d", 
		maxConcurrentRequests, postgresqlMaxConcurrent, mongodbMaxConcurrent)

	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		logger := logrus.New()

		// 根据路径选择合适的计数器
		if strings.Contains(path, "/postgresql") {
			// PostgreSQL请求
			counterMutex.Lock()
			if postgresqlCounter >= postgresqlMaxConcurrent {
				counterMutex.Unlock()
				utils.ResponseError(c, 429, 1429, "PostgreSQL请求并发数已达上限，请稍后重试")
				c.Abort()
				return
			}
			postgresqlCounter++
			counterMutex.Unlock()

			// 确保处理完请求后减少计数
			defer func() {
				counterMutex.Lock()
				postgresqlCounter--
				counterMutex.Unlock()
				logger.Debugf("PostgreSQL当前并发请求数: %d/%d", postgresqlCounter, postgresqlMaxConcurrent)
				executionTime := time.Since(startTime).Seconds()
				logger.Infof("请求 %s 执行时间: %.3f秒", path, executionTime)
			}()

			logger.Debugf("PostgreSQL当前并发请求数: %d/%d", postgresqlCounter, postgresqlMaxConcurrent)
		} else if strings.Contains(path, "/mongodb") {
			// MongoDB请求
			counterMutex.Lock()
			if mongodbCounter >= mongodbMaxConcurrent {
				counterMutex.Unlock()
				utils.ResponseError(c, 429, 1429, "MongoDB请求并发数已达上限，请稍后重试")
				c.Abort()
				return
			}
			mongodbCounter++
			counterMutex.Unlock()

			// 确保处理完请求后减少计数
			defer func() {
				counterMutex.Lock()
				mongodbCounter--
				counterMutex.Unlock()
				logger.Debugf("MongoDB当前并发请求数: %d/%d", mongodbCounter, mongodbMaxConcurrent)
				executionTime := time.Since(startTime).Seconds()
				logger.Infof("请求 %s 执行时间: %.3f秒", path, executionTime)
			}()

			logger.Debugf("MongoDB当前并发请求数: %d/%d", mongodbCounter, mongodbMaxConcurrent)
		} else {
			// 其他请求
			counterMutex.Lock()
			if generalCounter >= maxConcurrentRequests {
				counterMutex.Unlock()
				utils.ResponseError(c, 429, 1429, "请求并发数已达上限，请稍后重试")
				c.Abort()
				return
			}
			generalCounter++
			counterMutex.Unlock()

			// 确保处理完请求后减少计数
			defer func() {
				counterMutex.Lock()
				generalCounter--
				counterMutex.Unlock()
				logger.Debugf("当前并发请求数: %d/%d", generalCounter, maxConcurrentRequests)
			}()

			logger.Debugf("当前并发请求数: %d/%d", generalCounter, maxConcurrentRequests)
		}

		// 继续处理请求
		c.Next()
	}
} 