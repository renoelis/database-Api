package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/renoelis/database-api-go/utils"
	"github.com/sirupsen/logrus"
)

// RecoveryMiddleware 从panic中恢复，并返回500错误
func RecoveryMiddleware() gin.HandlerFunc {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 打印堆栈信息
				logger.Errorf("Panic recovered: %v\n%s", err, debug.Stack())
				
				// 返回500错误
				utils.ResponseError(c, 500, 9999, fmt.Sprintf("服务器内部错误: %v", err))
				
				// 中断请求
				c.Abort()
			}
		}()
		
		// 继续处理请求
		c.Next()
	}
} 