package utils

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Response 通用响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// SuccessResponse 返回成功响应
func SuccessResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "成功",
		Data:    data,
	})
}

// ErrorResponse 返回错误响应
func ErrorResponse(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// BadRequestResponse 返回请求错误响应
func BadRequestResponse(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    400,
		Message: message,
		Data:    nil,
	})
}

// InternalErrorResponse 返回服务器内部错误响应
func InternalErrorResponse(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Message: message,
		Data:    nil,
	})
} 