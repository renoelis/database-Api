package utils

import (
	"github.com/gin-gonic/gin"
)

// Response 定义统一的响应结构
type Response struct {
	ErrCode int         `json:"errCode"`
	Data    interface{} `json:"data"`
	ErrMsg  interface{} `json:"errMsg"`
}

// SuccessResponse 成功响应
func SuccessResponse(data interface{}) Response {
	return Response{
		ErrCode: 0,
		Data:    data,
		ErrMsg:  nil,
	}
}

// ErrorResponse 错误响应
func ErrorResponse(errorCode int, errorMessage string) Response {
	return Response{
		ErrCode: errorCode,
		Data:    nil,
		ErrMsg:  errorMessage,
	}
}

// ResponseSuccess 返回成功响应
func ResponseSuccess(c *gin.Context, data interface{}) {
	c.JSON(200, SuccessResponse(data))
}

// ResponseError 返回错误响应
func ResponseError(c *gin.Context, httpCode, errorCode int, errorMessage string) {
	c.JSON(httpCode, ErrorResponse(errorCode, errorMessage))
} 