package model

import "time"

// TokenCreate 创建令牌请求模型
type TokenCreate struct {
	Email      string `json:"email" binding:"required"`
	WsID       string `json:"ws_id" binding:"required"`
	TokenType  string `json:"token_type" binding:"omitempty"`
	TotalCalls *int   `json:"total_calls" binding:"omitempty"`
}

// TokenUpdate 更新令牌请求模型
type TokenUpdate struct {
	WsID       string `json:"ws_id" binding:"required"`
	Operation  string `json:"operation" binding:"required"` // add, set, unlimited
	CallsValue *int   `json:"calls_value" binding:"omitempty"`
}

// TokenInfo 令牌信息响应模型
type TokenInfo struct {
	TokenID       int        `json:"token_id"`
	AccessToken   string     `json:"access_token"`
	Email         string     `json:"email"`
	WsID          string     `json:"ws_id"`
	TokenType     string     `json:"token_type"`
	RemainingCalls *int       `json:"remaining_calls,omitempty"`
	TotalCalls    *int       `json:"total_calls,omitempty"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TokenInfoResponse 令牌信息简化响应模型，用于创建和更新操作
type TokenInfoResponse struct {
	TokenID       int     `json:"token_id"`
	AccessToken   *string  `json:"access_token,omitempty"`
	WsID          string  `json:"ws_id"`
	TokenType     string  `json:"token_type"`
	RemainingCalls *int    `json:"remaining_calls,omitempty"`
	TotalCalls    *int    `json:"total_calls,omitempty"`
}

// TokenUsageLog 令牌使用日志模型
type TokenUsageLog struct {
	TokenID         int                    `json:"token_id"`
	WsID            string                 `json:"ws_id"`
	OperationType   string                 `json:"operation_type"`
	TargetDatabase  string                 `json:"target_database"`
	TargetCollection *string                `json:"target_collection,omitempty"`
	Status          string                 `json:"status"`
	RequestDetails  map[string]interface{} `json:"request_details"`
} 