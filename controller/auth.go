package controller

import (
	"crypto/rand"
	"database-api-public-go/database"
	"database-api-public-go/model"
	"database-api-public-go/utils"
	"fmt"
	"math/big"

	"github.com/gin-gonic/gin"
)

// 生成随机访问令牌
func generateAccessToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetLength := big.NewInt(int64(len(charset)))
	
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		randomIndex, _ := rand.Int(rand.Reader, charsetLength)
		result[i] = charset[randomIndex.Int64()]
	}
	
	return string(result)
}

// CreateTokenHandler 创建访问令牌
func CreateTokenHandler(c *gin.Context) {
	var tokenData model.TokenCreate
	
	if err := c.ShouldBindJSON(&tokenData); err != nil {
		utils.BadRequestResponse(c, fmt.Sprintf("无效的请求数据: %v", err))
		return
	}
	
	// 设置默认值
	if tokenData.TokenType == "" {
		tokenData.TokenType = "limited"
	}
	
	// 检查令牌类型
	if tokenData.TokenType != "limited" && tokenData.TokenType != "unlimited" {
		utils.ErrorResponse(c, 1400, "令牌类型必须是 limited 或 unlimited")
		return
	}
	
	// 检查有限制令牌的总调用次数
	if tokenData.TokenType == "limited" && tokenData.TotalCalls == nil {
		utils.ErrorResponse(c, 1400, "limited类型的令牌必须指定total_calls参数")
		return
	}
	
	// 生成访问令牌
	accessToken := generateAccessToken(48)
	
	// 创建令牌
	success, tokenInfo, err := database.CreateAuthToken(
		tokenData.Email,
		tokenData.WsID,
		tokenData.TokenType,
		accessToken,
		tokenData.TotalCalls,
	)
	
	if !success {
		var errorCode int
		if err != nil && err.Error() == fmt.Sprintf("工作区ID %s 已存在关联的令牌", tokenData.WsID) {
			errorCode = 1050
		} else {
			errorCode = 9999
		}
		utils.ErrorResponse(c, errorCode, err.Error())
		return
	}
	
	// 返回创建的令牌信息
	utils.SuccessResponse(c, tokenInfo)
}

// UpdateTokenHandler 更新令牌使用次数
func UpdateTokenHandler(c *gin.Context) {
	var updateData model.TokenUpdate
	
	if err := c.ShouldBindJSON(&updateData); err != nil {
		utils.BadRequestResponse(c, fmt.Sprintf("无效的请求数据: %v", err))
		return
	}
	
	// 验证操作类型
	if updateData.Operation != "add" && updateData.Operation != "set" && updateData.Operation != "unlimited" {
		utils.ErrorResponse(c, 1400, "操作类型必须是 add, set 或 unlimited")
		return
	}
	
	// 检查调用次数参数
	if (updateData.Operation == "add" || updateData.Operation == "set") && updateData.CallsValue == nil {
		utils.ErrorResponse(c, 1400, fmt.Sprintf("%s操作需要提供calls_value参数", updateData.Operation))
		return
	}
	
	// 更新令牌
	success, tokenInfo, err := database.UpdateAuthToken(
		updateData.WsID,
		updateData.Operation,
		updateData.CallsValue,
	)
	
	if !success {
		var errorCode int
		if err != nil {
			errorMessage := err.Error()
			if errorMessage == fmt.Sprintf("未找到工作区ID %s 的令牌", updateData.WsID) {
				errorCode = 1051
			} else if errorMessage == fmt.Sprintf("add操作需要提供calls_value参数") || 
				      errorMessage == fmt.Sprintf("set操作需要提供calls_value参数") {
				errorCode = 1400
			} else {
				errorCode = 9999
			}
			utils.ErrorResponse(c, errorCode, errorMessage)
		} else {
			utils.ErrorResponse(c, 9999, "更新令牌失败")
		}
		return
	}
	
	// 返回更新后的令牌信息
	utils.SuccessResponse(c, tokenInfo)
}

// GetTokenInfoHandler 查询令牌信息
func GetTokenInfoHandler(c *gin.Context) {
	wsID := c.Query("ws_id")
	
	if wsID == "" {
		utils.BadRequestResponse(c, "缺少必需的ws_id查询参数")
		return
	}
	
	// 查询令牌信息
	success, tokenInfo, err := database.GetAuthTokenInfo(wsID)
	
	if !success {
		var errorCode int
		if err != nil {
			errorMessage := err.Error()
			if errorMessage == fmt.Sprintf("未找到工作区ID %s 的令牌", wsID) {
				errorCode = 1051
			} else if errorMessage == "数据库连接失败" {
				errorCode = 1000
			} else {
				errorCode = 9999
			}
			utils.ErrorResponse(c, errorCode, errorMessage)
		} else {
			utils.ErrorResponse(c, 9999, "查询令牌信息失败")
		}
		return
	}
	
	// 返回令牌信息
	utils.SuccessResponse(c, tokenInfo)
}

// CleanupLogsHandler 清理旧的令牌使用日志
func CleanupLogsHandler(c *gin.Context) {
	// 执行清理操作
	success, err := database.CleanupTokenUsageLogs()
	
	if !success {
		utils.ErrorResponse(c, 1100, fmt.Sprintf("清理令牌使用日志失败: %v", err))
		return
	}
	
	// 返回成功信息
	utils.SuccessResponse(c, map[string]interface{}{
		"message": "令牌使用日志清理任务已完成",
	})
} 