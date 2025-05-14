package controllers

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/renoelis/database-api-go/models"
	"github.com/renoelis/database-api-go/services"
	"github.com/renoelis/database-api-go/utils"
	"github.com/sirupsen/logrus"
)

// ExecutePostgreSQL 处理PostgreSQL SQL执行请求
func ExecutePostgreSQL(c *gin.Context) {
	logger := logrus.New()

	// 绑定请求数据
	var request models.PostgreSQLExecuteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Errorf("解析PostgreSQL请求失败: %v", err)
		utils.ResponseError(c, 400, 1000, fmt.Sprintf("请求参数错误: %v", err))
		return
	}

	// 验证SQL语句语法
	if err := request.ValidateSQL(); err != nil {
		logger.Errorf("SQL验证错误: %v", err)
		utils.ResponseError(c, 400, 1005, fmt.Sprintf("SQL验证错误: %v", err))
		return
	}

	// 额外的SQL语法验证
	sqlLower := strings.ToLower(strings.TrimSpace(request.SQL))
	if strings.HasPrefix(sqlLower, "select") {
		selectParts := strings.Split(sqlLower, " from ")[0]
		if selectParts == "select" || regexp.MustCompile(`select\s*$`).MatchString(selectParts) {
			utils.ResponseError(c, 400, 1005, "SQL验证错误: SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
			return
		}
	}

	// 获取连接池实例
	pgPool := services.GetPostgreSQLInstance()

	// 从连接池获取连接
	conn, err := pgPool.GetConnection(
		request.Connection.Host,
		request.Connection.Port,
		request.Connection.Database,
		request.Connection.User,
		request.Connection.Password,
		request.Connection.SSLMode,
		request.Connection.ConnectTimeout,
	)

	if err != nil {
		logger.Errorf("从连接池获取连接失败: %v", err)
		utils.ResponseError(c, 500, 1001, fmt.Sprintf("数据库连接错误: %v", err))
		return
	}

	logger.Infof("成功从连接池获取连接: %s:%d/%s", 
		request.Connection.Host, 
		request.Connection.Port, 
		request.Connection.Database)

	// 执行SQL
	logger.Infof("执行SQL: %s", request.SQL)
	
	// 处理不同类型的SQL语句
	sqlType := strings.ToUpper(strings.Fields(strings.TrimSpace(request.SQL))[0])

	// 如果是查询操作
	if sqlType == "SELECT" || sqlType == "WITH" || sqlType == "SHOW" || sqlType == "EXPLAIN" {
		// 查询数据
		rows, err := executeQueryWithParams(conn, request.SQL, request.Parameters)
		if err != nil {
			handleSQLError(c, err)
			return
		}
		defer rows.Close()

		// 转换结果为JSON
		result, err := convertRowsToJSON(rows)
		if err != nil {
			logger.Errorf("转换结果集失败: %v", err)
			utils.ResponseError(c, 500, 1006, fmt.Sprintf("处理查询结果失败: %v", err))
			return
		}

		// 检查结果是否为空
		if len(result) == 0 {
			logger.Info("SQL查询未返回任何结果")
		}

		// 返回结果
		utils.ResponseSuccess(c, result)
	} else {
		// 执行非查询操作
		result, err := executeNonQueryWithParams(conn, request.SQL, request.Parameters)
		if err != nil {
			handleSQLError(c, err)
			return
		}

		// 检查影响的行数
		if result == 0 {
			logger.Info("SQL操作未影响任何行")
		}

		// 返回影响行数
		utils.ResponseSuccess(c, result)
	}

	// 释放连接
	pgPool.ReleaseConnection(
		request.Connection.Host,
		request.Connection.Port,
		request.Connection.Database,
		request.Connection.User,
	)
	logger.Info("数据库连接已归还到连接池")
}

// executeQueryWithParams 执行带参数的查询
func executeQueryWithParams(db *sql.DB, query string, params interface{}) (*sql.Rows, error) {
	// 根据参数类型执行不同的查询
	switch p := params.(type) {
	case []interface{}:
		// 数组参数
		return db.Query(query, p...)
	case map[string]interface{}:
		// 命名参数，需要转换
		return executeNamedQuery(db, query, p)
	default:
		// 无参数
		return db.Query(query)
	}
}

// executeNonQueryWithParams 执行带参数的非查询
func executeNonQueryWithParams(db *sql.DB, query string, params interface{}) (int64, error) {
	var result sql.Result
	var err error

	switch p := params.(type) {
	case []interface{}:
		// 数组参数
		result, err = db.Exec(query, p...)
	case map[string]interface{}:
		// 命名参数，需要转换
		result, err = executeNamedExec(db, query, p)
	default:
		// 无参数
		result, err = db.Exec(query)
	}

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// executeNamedQuery 执行命名参数查询
func executeNamedQuery(db *sql.DB, query string, params map[string]interface{}) (*sql.Rows, error) {
	// 这里需要将命名参数转换为位置参数
	// 简单实现，生产环境应使用更健壮的实现
	var args []interface{}
	for _, v := range params {
		args = append(args, v)
	}
	return db.Query(query, args...)
}

// executeNamedExec 执行命名参数非查询
func executeNamedExec(db *sql.DB, query string, params map[string]interface{}) (sql.Result, error) {
	// 同上，简单实现
	var args []interface{}
	for _, v := range params {
		args = append(args, v)
	}
	return db.Exec(query, args...)
}

// convertRowsToJSON 将SQL结果转换为JSON格式
func convertRowsToJSON(rows *sql.Rows) ([]map[string]interface{}, error) {
	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// 创建结果集
	var result []map[string]interface{}

	// 预分配值接收对象
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// 逐行处理结果
	for rows.Next() {
		// 扫描当前行到对象
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// 创建一个新的行对象
		row := make(map[string]interface{})

		// 填充行数据
		for i, col := range columns {
			var v interface{}
			val := values[i]

			// 检查值类型，并适当转换
			switch value := val.(type) {
			case nil:
				v = nil
			case []byte:
				// 尝试转换为字符串
				v = string(value)
			case time.Time:
				// 格式化时间为ISO字符串
				v = value.Format(time.RFC3339)
			default:
				v = value
			}

			row[col] = v
		}

		// 添加到结果集
		result = append(result, row)
	}

	// 检查行遍历是否有错误
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// handleSQLError 处理SQL执行错误
func handleSQLError(c *gin.Context, err error) {
	logger := logrus.New()
	logger.Errorf("SQL执行错误: %v", err)

	// 处理不同类型的PostgreSQL错误
	if pqErr, ok := err.(*pq.Error); ok {
		// 根据PostgreSQL错误代码返回不同的错误消息
		switch pqErr.Code {
		case "42P01": // 表不存在
			// 提取表名
			match := regexp.MustCompile(`relation "([^"]+)" does not exist`).FindStringSubmatch(pqErr.Message)
			if len(match) > 1 {
				tableName := match[1]
				utils.ResponseError(c, 400, 1002, fmt.Sprintf("表不存在: %s", tableName))
				return
			}
			utils.ResponseError(c, 400, 1002, fmt.Sprintf("表不存在: %v", pqErr.Message))
		case "42601": // 语法错误
			utils.ResponseError(c, 400, 1003, fmt.Sprintf("SQL语法错误: %v", pqErr.Message))
		case "42703": // 列不存在
			match := regexp.MustCompile(`column "([^"]+)" does not exist`).FindStringSubmatch(pqErr.Message)
			if len(match) > 1 {
				column := match[1]
				utils.ResponseError(c, 400, 1003, fmt.Sprintf("列 '%s' 不存在，请检查列名是否正确或表是否存在此列", column))
				return
			}
			utils.ResponseError(c, 400, 1003, fmt.Sprintf("SQL执行错误: %v", pqErr.Message))
		case "25P02": // 事务失败
			utils.ResponseError(c, 400, 1004, fmt.Sprintf("SQL事务失败: %v", pqErr.Message))
		default:
			utils.ResponseError(c, 400, 1000, fmt.Sprintf("数据库错误: %v", pqErr.Message))
		}
	} else {
		utils.ResponseError(c, 400, 1000, fmt.Sprintf("数据库错误: %v", err.Error()))
	}
} 