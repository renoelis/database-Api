package models

import (
	"errors"
	"regexp"
	"strings"
)

// PostgreSQLConnectionInfo PostgreSQL连接信息
type PostgreSQLConnectionInfo struct {
	Host            string `json:"host" binding:"required"`
	Port            int    `json:"port" binding:"required"`
	Database        string `json:"database" binding:"required"`
	User            string `json:"user" binding:"required"`
	Password        string `json:"password" binding:"required"`
	SSLMode         string `json:"sslmode"` // 支持的模式: disable, require, verify-ca, verify-full (默认为disable)
	ConnectTimeout  int    `json:"connect_timeout"`
}

// PostgreSQLExecuteRequest PostgreSQL执行请求
type PostgreSQLExecuteRequest struct {
	Connection PostgreSQLConnectionInfo      `json:"connection" binding:"required"`
	SQL        string                        `json:"sql" binding:"required"`
	Parameters interface{}                   `json:"parameters"`
}

// ValidateSQL 验证SQL语句的基本语法
func (r *PostgreSQLExecuteRequest) ValidateSQL() error {
	if strings.TrimSpace(r.SQL) == "" {
		return errors.New("SQL语句不能为空")
	}

	sqlLower := strings.ToLower(strings.TrimSpace(r.SQL))

	// 检查SELECT语句是否有FROM子句
	if strings.HasPrefix(sqlLower, "select") {
		if !strings.Contains(sqlLower, " from ") {
			return errors.New("SELECT语句缺少FROM子句")
		}

		// 检查是否指定了列
		selectFromParts := strings.Split(sqlLower, " from ")[0]
		if selectFromParts == "select" || strings.TrimSpace(selectFromParts) == "select" {
			return errors.New("SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
		}

		// 检查select和from之间是否有非空白字符
		matched, _ := regexp.MatchString(`select\s+from`, sqlLower)
		if matched {
			return errors.New("SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
		}
	}

	// 检查基本的括号匹配
	if strings.Count(r.SQL, "(") != strings.Count(r.SQL, ")") {
		return errors.New("SQL语句括号不匹配")
	}

	// 检查WHERE后是否有条件
	wherePattern := regexp.MustCompile(`where\s+;`)
	if wherePattern.MatchString(r.SQL) {
		return errors.New("WHERE子句后缺少条件")
	}

	// 检查常见的拼写错误
	commonTypos := map[string]string{
		`\bslect\b`:         "select",
		`\bform\b`:          "from",
		`\bwhere\s+and\b`:   "where",
		`\bgroup\s+order\b`: "group by ... order",
	}

	for typo, correction := range commonTypos {
		matched, _ := regexp.MatchString(typo, sqlLower)
		if matched {
			return errors.New("SQL语句可能存在拼写错误: 检查 '" + correction + "'")
		}
	}

	return nil
} 