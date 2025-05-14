package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/renoelis/database-api-go/config"
	"github.com/renoelis/database-api-go/utils"
	"github.com/sirupsen/logrus"
)

// AuthMiddleware 认证中间件，验证请求头中的accessToken
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logrus.New()

		// 从请求头获取令牌
		token := c.GetHeader("accessToken")
		if token == "" {
			logger.Warning("请求缺少accessToken头")
			utils.ResponseError(c, 401, 1401, "缺少accessToken头")
			c.Abort()
			return
		}

		// 验证令牌
		if token != config.AccessToken {
			logger.Warning("无效的访问令牌")
			utils.ResponseError(c, 401, 1402, "无效的访问令牌")
			c.Abort()
			return
		}

		// 验证通过，继续处理请求
		c.Next()
	}
} 