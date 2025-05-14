package database

import "os"

// AuthDBConfig 认证数据库配置
var AuthDBConfig = map[string]string{
	"host":     getEnv("AUTH_DB_HOST", "196.185.170.10"),
	"port":     getEnv("AUTH_DB_PORT", "5432"),
	"database": getEnv("AUTH_DB_NAME", "4444"),
	"user":     getEnv("AUTH_DB_USER", "123456"),
	"password": getEnv("AUTH_DB_PASSWORD", "123456"),
	"sslmode":  getEnv("AUTH_DB_SSLMODE", "disable"),
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
} 