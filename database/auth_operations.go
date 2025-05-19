package database

import (
	"context"
	"database-api-public-go/model"
	"database-api-public-go/utils"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// InitAuthTables 初始化认证系统所需的数据库表
func InitAuthTables() (bool, error) {
	// 获取数据库连接
	config := AuthDBConfig
	port, _ := strconv.Atoi(config["port"])

	pool, err := utils.PostgreSQLPool.GetConnection(
		config["host"],
		port,
		config["database"],
		config["user"],
		config["password"],
		config["sslmode"],
	)
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return false, err
	}

	// 创建表结构
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查db_api_tokens表是否已经存在
	var tableExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'db_api_tokens'
		)
	`).Scan(&tableExists)

	if err != nil {
		log.Printf("检查表是否存在失败: %v", err)
		return false, err
	}

	// 如果表已经存在，先删除ws_id的唯一约束
	if tableExists {
		log.Printf("正在移除db_api_tokens表中ws_id列的唯一约束...")
		_, err = pool.Exec(ctx, `
			ALTER TABLE IF EXISTS db_api_tokens
			DROP CONSTRAINT IF EXISTS db_api_tokens_ws_id_key;
		`)
		if err != nil {
			log.Printf("移除唯一约束失败: %v", err)
			return false, err
		}
	}

	// 创建db_api_tokens表
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS db_api_tokens (
			token_id SERIAL PRIMARY KEY,
			access_token VARCHAR(64) UNIQUE NOT NULL,
			email VARCHAR(100) NOT NULL,
			ws_id VARCHAR(50) NOT NULL,
			token_type VARCHAR(20) NOT NULL DEFAULT 'limited',
			plugin_type VARCHAR(20) NOT NULL DEFAULT 'all',
			remaining_calls INTEGER,
			total_calls INTEGER,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Printf("创建db_api_tokens表失败: %v", err)
		return false, err
	}

	// 添加ws_id和plugin_type的组合唯一约束
	_, err = pool.Exec(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint 
				WHERE conname = 'db_api_tokens_ws_id_plugin_type_key'
			) THEN
				ALTER TABLE db_api_tokens
				ADD CONSTRAINT db_api_tokens_ws_id_plugin_type_key 
				UNIQUE (ws_id, plugin_type);
			END IF;
		END
		$$;
	`)
	if err != nil {
		log.Printf("添加组合唯一约束失败: %v", err)
		return false, err
	}

	// 创建db_token_usage_logs表
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS db_token_usage_logs (
			log_id SERIAL PRIMARY KEY,
			token_id INTEGER REFERENCES db_api_tokens(token_id),
			ws_id VARCHAR(50) NOT NULL,
			operation_type VARCHAR(50) NOT NULL,
			target_database VARCHAR(50) NOT NULL,
			target_collection VARCHAR(50),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			status VARCHAR(20) NOT NULL,
			request_details JSONB
		)
	`)
	if err != nil {
		log.Printf("创建db_token_usage_logs表失败: %v", err)
		return false, err
	}

	log.Println("令牌认证系统数据库表初始化完成")
	return true, nil
}

// CreateAuthToken 创建新的访问令牌
func CreateAuthToken(email, wsID, tokenType, pluginType, accessToken string, totalCalls *int) (bool, *model.TokenInfoResponse, error) {
	// 获取数据库连接
	config := AuthDBConfig
	port, _ := strconv.Atoi(config["port"])

	pool, err := utils.PostgreSQLPool.GetConnection(
		config["host"],
		port,
		config["database"],
		config["user"],
		config["password"],
		config["sslmode"],
	)
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return false, nil, fmt.Errorf("数据库连接失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查工作区ID和插件类型的组合是否已存在令牌
	var exists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM db_api_tokens WHERE ws_id = $1 AND plugin_type = $2)", wsID, pluginType).Scan(&exists)
	if err != nil {
		log.Printf("检查工作区令牌失败: %v", err)
		return false, nil, fmt.Errorf("服务器内部错误: %v", err)
	}

	if exists {
		return false, nil, fmt.Errorf("工作区ID %s 已存在 %s 类型的令牌", wsID, pluginType)
	}

	// 验证plugin_type
	if pluginType != "postgresql" && pluginType != "mongodb" && pluginType != "all" {
		return false, nil, fmt.Errorf("插件类型必须是 postgresql, mongodb 或 all")
	}

	// 设置令牌参数
	isUnlimited := tokenType == "unlimited"
	var remainingCalls, totalCallsValue *int
	if !isUnlimited && totalCalls != nil {
		remainingCalls = totalCalls
		totalCallsValue = totalCalls
	}

	// 创建令牌记录
	var tokenID int
	err = pool.QueryRow(ctx, `
		INSERT INTO db_api_tokens 
		(access_token, email, ws_id, token_type, plugin_type, remaining_calls, total_calls, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
		RETURNING token_id
	`, accessToken, email, wsID, tokenType, pluginType, remainingCalls, totalCallsValue).Scan(&tokenID)

	if err != nil {
		log.Printf("创建令牌失败: %v", err)
		return false, nil, fmt.Errorf("服务器内部错误: %v", err)
	}

	// 返回创建的令牌信息
	tokenInfoResp := &model.TokenInfoResponse{
		TokenID:        tokenID,
		AccessToken:    &accessToken,
		WsID:           wsID,
		TokenType:      tokenType,
		PluginType:     pluginType,
		RemainingCalls: remainingCalls,
		TotalCalls:     totalCallsValue,
		UsedCalls:      new(int), // 新创建的令牌，已使用次数为0
	}

	return true, tokenInfoResp, nil
}

// UpdateAuthToken 更新令牌使用次数
func UpdateAuthToken(accessToken, operation string, callsValue *int) (bool, *model.TokenInfoResponse, error) {
	// 获取数据库连接
	config := AuthDBConfig
	port, _ := strconv.Atoi(config["port"])

	pool, err := utils.PostgreSQLPool.GetConnection(
		config["host"],
		port,
		config["database"],
		config["user"],
		config["password"],
		config["sslmode"],
	)
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return false, nil, fmt.Errorf("数据库连接失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查token是否存在
	var exists bool
	var wsID string
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM db_api_tokens WHERE access_token = $1), ws_id FROM db_api_tokens WHERE access_token = $1", accessToken).Scan(&exists, &wsID)
	if err != nil {
		log.Printf("检查令牌失败: %v", err)
		return false, nil, fmt.Errorf("服务器内部错误: %v", err)
	}

	if !exists {
		return false, nil, fmt.Errorf("未找到令牌 %s", accessToken)
	}

	// 根据操作类型更新令牌
	var query string
	var args []interface{}

	switch operation {
	case "add":
		if callsValue == nil {
			return false, nil, errors.New("add操作需要提供calls_value参数")
		}
		query = `
			UPDATE db_api_tokens 
			SET remaining_calls = COALESCE(remaining_calls, 0) + $1,
				total_calls = COALESCE(total_calls, 0) + $1,
				token_type = 'limited',
				updated_at = CURRENT_TIMESTAMP
			WHERE access_token = $2
			RETURNING token_id, ws_id, token_type, plugin_type, remaining_calls, total_calls
		`
		args = []interface{}{*callsValue, accessToken}
	case "set":
		if callsValue == nil {
			return false, nil, errors.New("set操作需要提供calls_value参数")
		}
		query = `
			UPDATE db_api_tokens 
			SET remaining_calls = $1, 
				total_calls = $1, 
				token_type = 'limited',
				updated_at = CURRENT_TIMESTAMP
			WHERE access_token = $2
			RETURNING token_id, ws_id, token_type, plugin_type, remaining_calls, total_calls
		`
		args = []interface{}{*callsValue, accessToken}
	case "unlimited":
		query = `
			UPDATE db_api_tokens 
			SET token_type = 'unlimited', remaining_calls = NULL, total_calls = NULL, updated_at = CURRENT_TIMESTAMP
			WHERE access_token = $1
			RETURNING token_id, ws_id, token_type, plugin_type, remaining_calls, total_calls
		`
		args = []interface{}{accessToken}
	default:
		return false, nil, fmt.Errorf("不支持的操作类型: %s", operation)
	}

	// 执行更新
	var tokenID int
	var tokenType, pluginType string
	var remainingCalls, totalCalls pgtype.Int4
	err = pool.QueryRow(ctx, query, args...).Scan(
		&tokenID, &wsID, &tokenType, &pluginType, &remainingCalls, &totalCalls,
	)
	if err != nil {
		log.Printf("更新令牌失败: %v", err)
		return false, nil, fmt.Errorf("服务器内部错误: %v", err)
	}

	// 处理可空整数字段
	var remainingCallsPtr, totalCallsPtr *int
	if remainingCalls.Valid {
		val := int(remainingCalls.Int32)
		remainingCallsPtr = &val
	}
	if totalCalls.Valid {
		val := int(totalCalls.Int32)
		totalCallsPtr = &val
	}

	// 计算已使用次数
	var usedCallsPtr *int
	if totalCallsPtr != nil && remainingCallsPtr != nil {
		usedCalls := *totalCallsPtr - *remainingCallsPtr
		if usedCalls < 0 {
			usedCalls = 0 // 防止出现负数的已使用次数
		}
		usedCallsPtr = &usedCalls
	}

	// 返回更新后的令牌信息
	tokenInfoResp := &model.TokenInfoResponse{
		TokenID:        tokenID,
		WsID:           wsID,
		TokenType:      tokenType,
		PluginType:     pluginType,
		RemainingCalls: remainingCallsPtr,
		TotalCalls:     totalCallsPtr,
		UsedCalls:      usedCallsPtr,
	}

	return true, tokenInfoResp, nil
}

// GetAuthTokenInfo 查询令牌信息
func GetAuthTokenInfo(wsID string, pluginType *string) (bool, interface{}, error) {
	// 获取数据库连接
	config := AuthDBConfig
	port, _ := strconv.Atoi(config["port"])

	pool, err := utils.PostgreSQLPool.GetConnection(
		config["host"],
		port,
		config["database"],
		config["user"],
		config["password"],
		config["sslmode"],
	)
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return false, nil, fmt.Errorf("数据库连接失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if pluginType != nil {
		// 如果提供了plugin_type，则根据wsID和plugin_type组合查询单个令牌
		var tokenID int
		var accessToken string
		var email string
		var tokenTypeDB string
		var pluginTypeDB string
		var remainingCalls, totalCalls pgtype.Int4
		var isActive bool
		var createdAt, updatedAt time.Time

		err = pool.QueryRow(ctx, `
			SELECT token_id, access_token, email, ws_id, token_type, plugin_type,
				remaining_calls, total_calls, is_active, created_at, updated_at 
			FROM db_api_tokens 
			WHERE ws_id = $1 AND plugin_type = $2
		`, wsID, *pluginType).Scan(
			&tokenID, &accessToken, &email, &wsID, &tokenTypeDB, &pluginTypeDB,
			&remainingCalls, &totalCalls, &isActive, &createdAt, &updatedAt,
		)

		if err != nil {
			log.Printf("查询令牌信息失败: %v", err)
			return false, nil, fmt.Errorf("未找到工作区ID %s 及插件类型 %s 的令牌", wsID, *pluginType)
		}

		// 构建返回结果
		tokenInfo := map[string]interface{}{
			"token_id":     tokenID,
			"access_token": accessToken,
			"email":        email,
			"ws_id":        wsID,
			"token_type":   tokenTypeDB,
			"plugin_type":  pluginTypeDB,
			"is_active":    isActive,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		}

		if remainingCalls.Valid {
			tokenInfo["remaining_calls"] = remainingCalls.Int32
		}
		if totalCalls.Valid {
			tokenInfo["total_calls"] = totalCalls.Int32

			// 计算已使用次数
			if remainingCalls.Valid {
				usedCalls := totalCalls.Int32 - remainingCalls.Int32
				if usedCalls < 0 {
					usedCalls = 0 // 防止负数
				}
				tokenInfo["used_calls"] = usedCalls
			}
		}

		return true, tokenInfo, nil
	} else {
		// 如果没有提供plugin_type，则查询该工作区的所有令牌
		rows, err := pool.Query(ctx, `
			SELECT token_id, access_token, email, ws_id, token_type, plugin_type,
				remaining_calls, total_calls, is_active, created_at, updated_at 
			FROM db_api_tokens 
			WHERE ws_id = $1
		`, wsID)

		if err != nil {
			log.Printf("查询令牌信息失败: %v", err)
			return false, nil, fmt.Errorf("查询工作区ID %s 的令牌失败", wsID)
		}
		defer rows.Close()

		var tokens []map[string]interface{}

		for rows.Next() {
			var tokenID int
			var accessToken string
			var email string
			var tokenTypeDB string
			var pluginTypeDB string
			var remainingCalls, totalCalls pgtype.Int4
			var isActive bool
			var createdAt, updatedAt time.Time

			err := rows.Scan(
				&tokenID, &accessToken, &email, &wsID, &tokenTypeDB, &pluginTypeDB,
				&remainingCalls, &totalCalls, &isActive, &createdAt, &updatedAt,
			)

			if err != nil {
				log.Printf("扫描令牌信息失败: %v", err)
				continue
			}

			// 构建单个令牌信息
			tokenInfo := map[string]interface{}{
				"token_id":     tokenID,
				"access_token": accessToken,
				"email":        email,
				"ws_id":        wsID,
				"token_type":   tokenTypeDB,
				"plugin_type":  pluginTypeDB,
				"is_active":    isActive,
				"created_at":   createdAt,
				"updated_at":   updatedAt,
			}

			if remainingCalls.Valid {
				tokenInfo["remaining_calls"] = remainingCalls.Int32
			}
			if totalCalls.Valid {
				tokenInfo["total_calls"] = totalCalls.Int32

				// 计算已使用次数
				if remainingCalls.Valid {
					usedCalls := totalCalls.Int32 - remainingCalls.Int32
					if usedCalls < 0 {
						usedCalls = 0 // 防止负数
					}
					tokenInfo["used_calls"] = usedCalls
				}
			}

			tokens = append(tokens, tokenInfo)
		}

		if len(tokens) == 0 {
			return false, nil, fmt.Errorf("未找到工作区ID %s 的令牌", wsID)
		}

		return true, tokens, nil
	}
}

// ValidateAuthToken 验证令牌
func ValidateAuthToken(accessToken string) (bool, map[string]interface{}) {
	// 获取数据库连接
	config := AuthDBConfig
	port, _ := strconv.Atoi(config["port"])

	pool, err := utils.PostgreSQLPool.GetConnection(
		config["host"],
		port,
		config["database"],
		config["user"],
		config["password"],
		config["sslmode"],
	)
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 查询令牌信息
	var tokenID int
	var wsID string
	var tokenType string
	var pluginType string
	var remainingCalls pgtype.Int4
	var isActive bool

	err = pool.QueryRow(ctx, `
		SELECT token_id, ws_id, token_type, plugin_type, remaining_calls, is_active 
		FROM db_api_tokens 
		WHERE access_token = $1
	`, accessToken).Scan(
		&tokenID, &wsID, &tokenType, &pluginType, &remainingCalls, &isActive,
	)

	if err != nil {
		log.Printf("验证令牌失败: %v", err)
		return false, nil
	}

	if !isActive {
		log.Printf("令牌已被禁用: %s", accessToken)
		return false, nil
	}

	// 构建返回结果
	tokenInfo := map[string]interface{}{
		"token_id":    tokenID,
		"ws_id":       wsID,
		"token_type":  tokenType,
		"plugin_type": pluginType,
	}

	if remainingCalls.Valid {
		tokenInfo["remaining_calls"] = remainingCalls.Int32
	}

	return true, tokenInfo
}

// UpdateTokenUsage 更新令牌使用次数并记录使用日志
func UpdateTokenUsage(tokenID int, wsID, operationType, targetDatabase string,
	targetCollection *string, status string, requestDetails map[string]interface{}) bool {
	// 获取数据库连接
	config := AuthDBConfig
	port, _ := strconv.Atoi(config["port"])

	pool, err := utils.PostgreSQLPool.GetConnection(
		config["host"],
		port,
		config["database"],
		config["user"],
		config["password"],
		config["sslmode"],
	)
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取令牌信息，检查类型
	var tokenType string
	err = pool.QueryRow(ctx, "SELECT token_type FROM db_api_tokens WHERE token_id = $1", tokenID).Scan(&tokenType)
	if err != nil {
		log.Printf("查询令牌类型失败: %v", err)
		return false
	}

	// 开始事务
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("开始事务失败: %v", err)
		return false
	}
	defer tx.Rollback(ctx)

	// 对于limited类型的令牌，减少调用次数
	if tokenType == "limited" && status == "success" {
		_, err = tx.Exec(ctx, `
			UPDATE db_api_tokens 
			SET remaining_calls = GREATEST(0, remaining_calls - 1), updated_at = CURRENT_TIMESTAMP 
			WHERE token_id = $1
		`, tokenID)
		if err != nil {
			log.Printf("更新令牌使用次数失败: %v", err)
			return false
		}
	}

	// 转换请求详情为JSON
	requestDetailsJSON, err := json.Marshal(requestDetails)
	if err != nil {
		log.Printf("序列化请求详情失败: %v", err)
		return false
	}

	// 记录使用日志
	_, err = tx.Exec(ctx, `
		INSERT INTO db_token_usage_logs
		(token_id, ws_id, operation_type, target_database, target_collection, status, request_details)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, tokenID, wsID, operationType, targetDatabase, targetCollection, status, requestDetailsJSON)
	if err != nil {
		log.Printf("记录令牌使用日志失败: %v", err)
		return false
	}

	// 提交事务
	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("提交事务失败: %v", err)
		return false
	}

	return true
}

// CleanupTokenUsageLogs 清理旧的令牌使用日志
func CleanupTokenUsageLogs() (bool, error) {
	// 获取数据库连接
	config := AuthDBConfig
	port, _ := strconv.Atoi(config["port"])

	pool, err := utils.PostgreSQLPool.GetConnection(
		config["host"],
		port,
		config["database"],
		config["user"],
		config["password"],
		config["sslmode"],
	)
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return false, fmt.Errorf("数据库连接失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 删除一个月前的日志
	result, err := pool.Exec(ctx, `
		DELETE FROM db_token_usage_logs
		WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '1 month'
	`)
	if err != nil {
		log.Printf("清理令牌使用日志失败: %v", err)
		return false, fmt.Errorf("清理令牌使用日志失败: %v", err)
	}

	rowsAffected := result.RowsAffected()
	log.Printf("已清理 %d 条令牌使用日志", rowsAffected)

	return true, nil
}
