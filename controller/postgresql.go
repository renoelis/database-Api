package controller

import (
	"context"
	"database-api-public-go/config"
	"database-api-public-go/utils"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// PostgreSQLConnectionInfo PostgreSQL连接信息
type PostgreSQLConnectionInfo struct {
	Host           string `json:"host" binding:"required"`
	Port           int    `json:"port" binding:"required"`
	Database       string `json:"database" binding:"required"`
	User           string `json:"user" binding:"required"`
	Password       string `json:"password" binding:"required"`
	SSLMode        string `json:"sslmode" binding:"omitempty"`
	ConnectTimeout int    `json:"connect_timeout" binding:"omitempty"`
}

// PostgreSQLExecuteRequest PostgreSQL执行请求
type PostgreSQLExecuteRequest struct {
	Connection PostgreSQLConnectionInfo `json:"connection" binding:"required"`
	SQL        string                   `json:"sql" binding:"required"`
	Parameters interface{}              `json:"parameters" binding:"omitempty"`
}

// PostgreSQLHandler 处理PostgreSQL请求
func PostgreSQLHandler(c *gin.Context) {
	// 记录请求开始时间
	startTime := time.Now()
	var request PostgreSQLExecuteRequest

	// 根据调试模式决定日志详细程度
	isDebug := config.IsDebugMode()
	
	log.Printf("SQL请求：开始处理PostgreSQL请求")

	// 绑定请求数据
	bindStart := time.Now()
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("SQL请求：请求数据绑定失败 耗时=%.3fms 错误=%v",
			float64(time.Since(bindStart).Microseconds())/1000.0, err)
		utils.BadRequestResponse(c, fmt.Sprintf("无效的请求数据: %v", err))
		return
	}

	// 验证SQL语句
	validateStart := time.Now()
	if err := validateSQL(request.SQL); err != nil {
		log.Printf("SQL请求：SQL验证失败 耗时=%.3fms 错误=%v",
			float64(time.Since(validateStart).Microseconds())/1000.0, err)
		utils.ErrorResponse(c, 1005, fmt.Sprintf("SQL验证错误: %v", err))
		return
	}

	// 设置默认值
	if request.Connection.SSLMode == "" {
		request.Connection.SSLMode = "disable"
	}
	if request.Connection.ConnectTimeout == 0 {
		request.Connection.ConnectTimeout = 30
	}

	// 获取数据库连接
	connectionStart := time.Now()
	if isDebug {
		log.Printf("SQL请求：获取数据库连接 数据库=%s SQL=%s", 
			request.Connection.Database, truncateSQL(request.SQL, 50))
	} else {
		log.Printf("SQL请求：获取数据库连接 数据库=%s", request.Connection.Database)
	}
	
	pool, err := utils.PostgreSQLPool.GetConnection(
		request.Connection.Host,
		request.Connection.Port,
		request.Connection.Database,
		request.Connection.User,
		request.Connection.Password,
		request.Connection.SSLMode,
	)
	
	if err != nil {
		log.Printf("SQL请求：数据库连接失败 耗时=%.3fms 错误=%v",
			float64(time.Since(connectionStart).Microseconds())/1000.0, err)
		utils.ErrorResponse(c, 1001, fmt.Sprintf("数据库连接错误: %v", err))
		return
	}
	
	connectionTime := time.Since(connectionStart)
	log.Printf("SQL请求：数据库连接成功 耗时=%.3fms", 
		float64(connectionTime.Microseconds())/1000.0)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 执行SQL
	sqlLower := strings.ToLower(strings.TrimSpace(request.SQL))
	isQuery := isQuerySQL(sqlLower)

	executionStart := time.Now()
	if isQuery {
		// 查询操作
		if isDebug {
			log.Printf("SQL请求：执行查询操作 SQL=%s", truncateSQL(request.SQL, 50))
		} else {
			log.Printf("SQL请求：执行查询操作")
		}
		
		rows, err := pool.Query(ctx, request.SQL)
		if err != nil {
			log.Printf("SQL请求：查询执行失败 耗时=%.3fms 错误=%v",
				float64(time.Since(executionStart).Microseconds())/1000.0, err)
			utils.ErrorResponse(c, 1003, fmt.Sprintf("SQL执行错误: %v", err))
			return
		}
		defer rows.Close()

		// 处理结果
		resultStart := time.Now()
		log.Printf("SQL请求：处理查询结果")
		
		// 转换结果为JSON可序列化格式
		var results []map[string]interface{}
		fieldDescriptions := rows.FieldDescriptions()
		
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				log.Printf("SQL请求：读取结果行失败 耗时=%.3fms 错误=%v",
					float64(time.Since(resultStart).Microseconds())/1000.0, err)
				utils.ErrorResponse(c, 1004, fmt.Sprintf("读取结果错误: %v", err))
				return
			}

			row := make(map[string]interface{})
			for i, fieldDesc := range fieldDescriptions {
				fieldName := string(fieldDesc.Name)
				if i < len(values) {
					row[fieldName] = convertPgValue(values[i])
				}
			}
			results = append(results, row)
		}

		if err := rows.Err(); err != nil {
			log.Printf("SQL请求：结果集错误 耗时=%.3fms 错误=%v",
				float64(time.Since(resultStart).Microseconds())/1000.0, err)
			utils.ErrorResponse(c, 1004, fmt.Sprintf("读取结果错误: %v", err))
			return
		}

		resultTime := time.Since(resultStart)
		totalTime := time.Since(startTime)
		
		log.Printf("SQL请求：查询成功完成 总耗时=%.3fms 数据库连接=%.3fms SQL执行=%.3fms 结果处理=%.3fms 行数=%d",
			float64(totalTime.Microseconds())/1000.0,
			float64(connectionTime.Microseconds())/1000.0,
			float64(resultStart.Sub(executionStart).Microseconds())/1000.0,
			float64(resultTime.Microseconds())/1000.0,
			len(results))
		
		utils.SuccessResponse(c, results)
	} else {
		// 非查询操作
		if isDebug {
			log.Printf("SQL请求：执行更新操作 SQL=%s", truncateSQL(request.SQL, 50))
		} else {
			log.Printf("SQL请求：执行更新操作")
		}
		
		tag, err := pool.Exec(ctx, request.SQL)
		if err != nil {
			log.Printf("SQL请求：更新执行失败 耗时=%.3fms 错误=%v",
				float64(time.Since(executionStart).Microseconds())/1000.0, err)
			utils.ErrorResponse(c, 1003, fmt.Sprintf("SQL执行错误: %v", err))
			return
		}

		executionTime := time.Since(executionStart)
		totalTime := time.Since(startTime)
		
		log.Printf("SQL请求：更新操作成功完成 总耗时=%.3fms 数据库连接=%.3fms SQL执行=%.3fms 影响行数=%d",
			float64(totalTime.Microseconds())/1000.0,
			float64(connectionTime.Microseconds())/1000.0,
			float64(executionTime.Microseconds())/1000.0,
			tag.RowsAffected())
		
		utils.SuccessResponse(c, tag.RowsAffected())
	}
}

// 截断SQL语句，用于日志显示
func truncateSQL(sql string, maxLength int) string {
	if len(sql) <= maxLength {
		return sql
	}
	return sql[:maxLength] + "..."
}

// 验证SQL语句
func validateSQL(sql string) error {
	if sql == "" {
		return fmt.Errorf("SQL语句不能为空")
	}

	sqlLower := strings.ToLower(strings.TrimSpace(sql))

	// 检查SELECT语句是否有FROM子句
	if strings.HasPrefix(sqlLower, "select") {
		if !strings.Contains(sqlLower, " from ") {
			return fmt.Errorf("SELECT语句缺少FROM子句")
		}

		// 检查是否指定了列
		selectFromParts := strings.Split(sqlLower, " from ")[0]
		if selectFromParts == "select" || strings.TrimSpace(selectFromParts) == "select" {
			return fmt.Errorf("SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
		}

		// 检查select和from之间是否有非空白字符
		selectPartMatch, _ := regexp.MatchString(`select\s+from`, sqlLower)
		if selectPartMatch {
			return fmt.Errorf("SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
		}
	}

	// 检查基本的括号匹配
	if strings.Count(sql, "(") != strings.Count(sql, ")") {
		return fmt.Errorf("SQL语句括号不匹配")
	}

	// 检查WHERE后是否有条件
	wherePattern, _ := regexp.Compile(`where\s+;`)
	if wherePattern.MatchString(sql) {
		return fmt.Errorf("WHERE子句后缺少条件")
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
			return fmt.Errorf("SQL语句可能存在拼写错误: 检查 '%s'", correction)
		}
	}

	return nil
}

// 判断是否是查询SQL
func isQuerySQL(sql string) bool {
	queryKeywords := []string{"select", "show", "explain", "with"}
	for _, keyword := range queryKeywords {
		if strings.HasPrefix(sql, keyword) {
			return true
		}
	}
	return false
}

// 转换PostgreSQL值为JSON可序列化格式
func convertPgValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	// 根据类型进行转换
	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case []byte:
		return string(v)
	default:
		return v
	}
} 