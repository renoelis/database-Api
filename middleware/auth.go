package middleware

import (
	"database-api-public-go/database"
	"database-api-public-go/utils"
	"encoding/json"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

// TokenAuthMiddleware 令牌认证中间件
func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 不需要验证的路径
		excludePaths := []string{
			"/apiDatabase/auth/token",
			"/apiDatabase/auth/token/update",
			"/apiDatabase/auth/token/info",
			"/apiDatabase/auth/logs/cleanup",
			"/",
			"/docs",
			"/swagger",
			"/redoc",
		}

		// 检查是否需要跳过验证
		path := c.Request.URL.Path
		for _, excludePath := range excludePaths {
			if path == excludePath || (excludePath != "/" && strings.HasPrefix(path, excludePath+"/")) {
				log.Printf("跳过验证路径: %s", path)
				c.Next()
				return
			}
		}

		// 获取访问令牌
		var accessToken string

		// 从请求头获取令牌
		accessToken = c.GetHeader("accessToken")

		// 从查询参数获取令牌
		if accessToken == "" {
			accessToken = c.Query("access_token")
		}

		// 如果没有令牌，返回错误
		if accessToken == "" {
			utils.ErrorResponse(c, 1401, "未提供访问令牌")
			c.Abort()
			return
		}

		// 验证令牌
		valid, tokenInfo := database.ValidateAuthToken(accessToken)
		if !valid {
			utils.ErrorResponse(c, 1402, "无效的访问令牌")
			c.Abort()
			return
		}

		// 检查plugin_type权限
		pluginType, _ := tokenInfo["plugin_type"].(string)

		// 检查路径是否匹配令牌的plugin_type
		if strings.Contains(path, "/apiDatabase/postgresql") && pluginType != "postgresql" && pluginType != "all" {
			utils.ErrorResponse(c, 1404, "该令牌无权访问PostgreSQL接口")
			c.Abort()
			return
		}

		if strings.Contains(path, "/apiDatabase/mongodb") && pluginType != "mongodb" && pluginType != "all" {
			utils.ErrorResponse(c, 1404, "该令牌无权访问MongoDB接口")
			c.Abort()
			return
		}

		// 严格检查写操作 - POST, PUT, PATCH, DELETE方法
		isWriteOperation := c.Request.Method == "POST" || c.Request.Method == "PUT" ||
			c.Request.Method == "PATCH" || c.Request.Method == "DELETE"

		// 检查令牌调用次数（仅对有限制的令牌进行检查）
		if isWriteOperation && tokenInfo["token_type"] == "limited" {
			remainingCalls, ok := tokenInfo["remaining_calls"].(int32)
			if ok && remainingCalls <= 0 {
				utils.ErrorResponse(c, 1403, "令牌调用次数已用完")
				c.Abort()
				return
			}
		}

		// 将令牌信息存储在上下文中
		c.Set("tokenInfo", tokenInfo)

		// 保存请求体以供后续使用
		requestBody, _ := c.GetRawData()
		if len(requestBody) > 0 {
			// 将请求体设置回context，以便后续处理程序能够访问
			c.Request.Body = utils.NewReadCloser(requestBody)
			// 存储请求体供后续使用
			c.Set("rawRequestBody", requestBody)
		}

		// 处理请求
		c.Next()

		// 完成请求后，记录令牌使用日志（对所有写操作，不管是限制性还是无限制令牌）
		if isWriteOperation {
			updateTokenUsage(c, tokenInfo)
		}
	}
}

// 更新令牌使用次数并记录使用日志
func updateTokenUsage(c *gin.Context, tokenInfo map[string]interface{}) {
	tokenID := int(tokenInfo["token_id"].(int))
	wsID := tokenInfo["ws_id"].(string)

	// 获取操作类型
	operationType := c.FullPath()
	parts := strings.Split(operationType, "/")
	if len(parts) > 0 {
		operationType = parts[len(parts)-1]
	}

	// 提取请求体
	var requestBody map[string]interface{}
	var targetDatabase string
	var targetCollection *string

	// 尝试从存储的原始请求体解析
	if rawBody, exists := c.Get("rawRequestBody"); exists {
		if bodyBytes, ok := rawBody.([]byte); ok && len(bodyBytes) > 0 {
			json.Unmarshal(bodyBytes, &requestBody)
		}
	}

	// 提取目标数据库和集合信息
	if requestBody != nil {
		// 针对不同结构的请求体尝试提取信息
		if connection, ok := requestBody["connection"].(map[string]interface{}); ok {
			if db, exists := connection["database"]; exists {
				targetDatabase = db.(string)
			}
		}

		// 针对MongoDB操作，提取集合名
		if collection, exists := requestBody["collection"]; exists {
			if collStr, ok := collection.(string); ok {
				targetCollection = &collStr
			}
		}

		// 针对SQL操作
		if table, exists := requestBody["table"]; exists {
			if tableStr, ok := table.(string); ok {
				targetCollection = &tableStr
			}
		}
	}

	// 响应状态
	status := "success"
	if c.Writer.Status() >= 400 {
		status = "error"
	}

	// 更新令牌使用情况
	success := database.UpdateTokenUsage(
		tokenID,
		wsID,
		operationType,
		targetDatabase,
		targetCollection,
		status,
		requestBody,
	)

	if !success {
		log.Printf("更新令牌使用记录失败！token_id=%d, ws_id=%s, operation=%s", tokenID, wsID, operationType)
	}
}
