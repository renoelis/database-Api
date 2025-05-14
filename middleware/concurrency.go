package middleware

import (
	"database-api-public-go/config"
	"database-api-public-go/utils"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// 并发控制结构
type ConcurrencyLimiter struct {
	mutex                   sync.Mutex
	currentRequests         int
	postgresqlRequests      int
	mongodbRequests         int
	maxConcurrentRequests   int
	postgresqlMaxConcurrent int
	mongodbMaxConcurrent    int
	enabled                 bool  // 添加一个开关，用于启用/禁用并发控制
}

// 全局并发控制器
var concurrencyLimiter = &ConcurrencyLimiter{
	currentRequests:         0,
	postgresqlRequests:      0,
	mongodbRequests:         0,
	maxConcurrentRequests:   0,  // 初始化时不设置具体值，避免在配置加载前使用错误的值
	postgresqlMaxConcurrent: 0,  // 初始化时不设置具体值
	mongodbMaxConcurrent:    0,  // 初始化时不设置具体值
	enabled:                 true,  // 默认启用
}

// 初始化并发控制器
func InitConcurrencyLimiter() {
	concurrencyLimiter.mutex.Lock()
	defer concurrencyLimiter.mutex.Unlock()
	
	// 从配置中获取最新值
	concurrencyLimiter.maxConcurrentRequests = config.GetMaxConcurrentRequests()
	concurrencyLimiter.postgresqlMaxConcurrent = config.GetPostgreSQLMaxConcurrent()
	concurrencyLimiter.mongodbMaxConcurrent = config.GetMongoDBMaxConcurrent()
	
	// 简化日志输出，与配置日志保持一致的格式
	log.Printf("并发控制器初始化: 最大并发=%d PostgreSQL=%d MongoDB=%d",
		concurrencyLimiter.maxConcurrentRequests,
		concurrencyLimiter.postgresqlMaxConcurrent,
		concurrencyLimiter.mongodbMaxConcurrent)
}

// ConcurrencyMiddleware 并发控制中间件
func ConcurrencyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		
		// 如果并发控制被禁用，直接跳过
		if !concurrencyLimiter.enabled {
			if config.IsDebugMode() {
				log.Printf("并发控制已禁用，放行请求: %s", path)
			}
			c.Next()
			return
		}
		
		// 跳过非数据库API请求和认证相关API请求
		if !strings.Contains(path, "/apiDatabase") || 
		   strings.Contains(path, "/apiDatabase/auth/") ||
		   strings.Contains(path, "/apiDatabase/system/") {
			c.Next()
			return
		}

		var allowed bool
		var rejectReason string

		concurrencyLimiter.mutex.Lock()

		// 获取当前配置的限制值（可能已通过环境变量更新）
		concurrencyLimiter.maxConcurrentRequests = config.GetMaxConcurrentRequests()
		concurrencyLimiter.postgresqlMaxConcurrent = config.GetPostgreSQLMaxConcurrent()
		concurrencyLimiter.mongodbMaxConcurrent = config.GetMongoDBMaxConcurrent()

		// 总体并发限制
		if concurrencyLimiter.currentRequests >= concurrencyLimiter.maxConcurrentRequests && concurrencyLimiter.maxConcurrentRequests > 0 {
			allowed = false
			rejectReason = "服务器正忙，请稍后再试"
		} else {
			// 特定类型数据库请求的并发限制
			if strings.Contains(path, "/apiDatabase/postgresql") && 
				concurrencyLimiter.postgresqlRequests >= concurrencyLimiter.postgresqlMaxConcurrent && 
				concurrencyLimiter.postgresqlMaxConcurrent > 0 {
				allowed = false
				rejectReason = "PostgreSQL服务器正忙，请稍后再试"
			} else if strings.Contains(path, "/apiDatabase/mongodb") && 
				concurrencyLimiter.mongodbRequests >= concurrencyLimiter.mongodbMaxConcurrent && 
				concurrencyLimiter.mongodbMaxConcurrent > 0 {
				allowed = false
				rejectReason = "MongoDB服务器正忙，请稍后再试"
			} else {
				allowed = true
				concurrencyLimiter.currentRequests++
				
				if strings.Contains(path, "/apiDatabase/postgresql") {
					concurrencyLimiter.postgresqlRequests++
				} else if strings.Contains(path, "/apiDatabase/mongodb") {
					concurrencyLimiter.mongodbRequests++
				}
			}
		}

		// 记录当前并发请求数
		currentTotal := concurrencyLimiter.currentRequests
		currentPostgres := concurrencyLimiter.postgresqlRequests
		currentMongo := concurrencyLimiter.mongodbRequests
		maxTotal := concurrencyLimiter.maxConcurrentRequests
		maxPostgres := concurrencyLimiter.postgresqlMaxConcurrent
		maxMongo := concurrencyLimiter.mongodbMaxConcurrent
		
		concurrencyLimiter.mutex.Unlock()

		if !allowed {
			// 打印简洁的拒绝信息
			log.Printf("请求被拒绝: %s (总请求: %d, PostgreSQL: %d, MongoDB: %d)",
				path, currentTotal, currentPostgres, currentMongo)
				
			utils.ErrorResponse(c, http.StatusTooManyRequests, rejectReason)
			c.Abort()
			return
		}

		// 仅在调试模式下记录请求接受信息
		if config.IsDebugMode() {
			log.Printf("请求已接受: %s (总请求: %d/%d, PostgreSQL: %d/%d, MongoDB: %d/%d)", 
				path, currentTotal, maxTotal, currentPostgres, maxPostgres, currentMongo, maxMongo)
		}

		// 请求处理完成后减少计数
		defer func() {
			concurrencyLimiter.mutex.Lock()
			concurrencyLimiter.currentRequests--
			if strings.Contains(path, "/apiDatabase/postgresql") {
				concurrencyLimiter.postgresqlRequests--
			} else if strings.Contains(path, "/apiDatabase/mongodb") {
				concurrencyLimiter.mongodbRequests--
			}
			
			// 仅在调试模式下记录完成信息
			if config.IsDebugMode() {
				log.Printf("请求完成: %s (当前并发=%d)", path, concurrencyLimiter.currentRequests)
			}
			
			concurrencyLimiter.mutex.Unlock()
		}()

		c.Next()
	}
}

// DisableConcurrencyLimit 禁用并发限制（用于测试或特殊情况）
func DisableConcurrencyLimit() {
	concurrencyLimiter.mutex.Lock()
	defer concurrencyLimiter.mutex.Unlock()
	concurrencyLimiter.enabled = false
	log.Println("并发控制已禁用")
}

// EnableConcurrencyLimit 启用并发限制
func EnableConcurrencyLimit() {
	concurrencyLimiter.mutex.Lock()
	defer concurrencyLimiter.mutex.Unlock()
	concurrencyLimiter.enabled = true
	log.Println("并发控制已启用")
} 