package models

import (
	"errors"
	"strings"
)

// MongoDBConnectionInfo MongoDB连接信息
type MongoDBConnectionInfo struct {
	Host            string `json:"host" binding:"required"`
	Port            int    `json:"port" binding:"required"`
	Database        string `json:"database" binding:"required"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	AuthSource      string `json:"auth_source"`
	ConnectTimeoutMS int    `json:"connect_timeout_ms"`
}

// MongoDBExecuteRequest MongoDB执行请求
type MongoDBExecuteRequest struct {
	Connection MongoDBConnectionInfo   `json:"connection" binding:"required"`
	Collection string                  `json:"collection" binding:"required"`
	Operation  string                  `json:"operation" binding:"required"`
	Filter     map[string]interface{}  `json:"filter"`
	Update     map[string]interface{}  `json:"update"`
	Document   map[string]interface{}  `json:"document"`
	Documents  []map[string]interface{} `json:"documents"`
	Projection map[string]interface{}  `json:"projection"`
	Sort       [][]interface{}         `json:"sort"`
	Limit      int                     `json:"limit"`
	Skip       int                     `json:"skip"`
	Pipeline   []map[string]interface{} `json:"pipeline"`
}

// ValidateRequest 验证请求的必要字段
func (r *MongoDBExecuteRequest) ValidateRequest() error {
	// 验证集合名
	if strings.TrimSpace(r.Collection) == "" {
		return errors.New("MongoDB集合名不能为空")
	}

	// 验证操作类型
	operation := strings.ToLower(strings.TrimSpace(r.Operation))
	if operation == "" {
		return errors.New("MongoDB操作类型不能为空")
	}

	// 检查支持的操作类型
	validOperations := map[string]bool{
		"find":       true,
		"findone":    true,
		"insert":     true,
		"insertmany": true,
		"update":     true,
		"updatemany": true,
		"delete":     true,
		"deletemany": true,
		"aggregate":  true,
		"count":      true,
	}

	if !validOperations[operation] {
		return errors.New("不支持的操作类型: " + operation + "。支持的操作类型有: find, findone, insert, insertmany, update, updatemany, delete, deletemany, aggregate, count")
	}

	// 根据操作类型验证必要的字段
	switch operation {
	case "insert":
		if r.Document == nil {
			return errors.New("插入操作缺少document字段，请提供要插入的文档")
		}
	case "insertmany":
		if r.Documents == nil || len(r.Documents) == 0 {
			return errors.New("批量插入操作缺少documents字段，请提供要插入的文档列表")
		}
	case "update", "updatemany":
		if r.Filter == nil {
			return errors.New("更新操作缺少filter字段，请提供查询条件")
		}
		if r.Update == nil {
			return errors.New("更新操作缺少update字段，请提供更新内容")
		}

		// 检查更新操作是否包含操作符
		if r.Update != nil {
			hasOperator := false
			for key := range r.Update {
				if strings.HasPrefix(key, "$") {
					hasOperator = true
					break
				}
			}
			if !hasOperator {
				return errors.New("更新操作的update字段格式不正确，应包含至少一个更新操作符如 $set, $unset 等")
			}
		}
	case "delete", "deletemany":
		if r.Filter == nil {
			return errors.New("删除操作缺少filter字段，请提供查询条件")
		}
	case "aggregate":
		if r.Pipeline == nil || len(r.Pipeline) == 0 {
			return errors.New("聚合操作缺少pipeline字段，请提供聚合管道")
		}
	}

	return nil
} 