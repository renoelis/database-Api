package config

import (
	"log"
	"os"
	"strconv"
)

// AppConfig 应用配置结构
type AppConfig struct {
	Port                    string
	MaxConcurrentRequests   int
	PostgreSQLMaxConcurrent int
	MongoDBMaxConcurrent    int
	DebugMode               bool
}

// Config 全局配置实例
var Config AppConfig

// LoadConfig 加载应用配置
func LoadConfig() {
	// 设置默认值
	Config = AppConfig{
		Port:                    "3010",
		MaxConcurrentRequests:   200,
		PostgreSQLMaxConcurrent: 100,
		MongoDBMaxConcurrent:    100,
		DebugMode:               false,
	}

	// 从环境变量加载配置
	if port := os.Getenv("PORT"); port != "" {
		Config.Port = port
	}

	if maxConcurrentRequests := os.Getenv("MAX_CONCURRENT_REQUESTS"); maxConcurrentRequests != "" {
		if value, err := strconv.Atoi(maxConcurrentRequests); err == nil {
			Config.MaxConcurrentRequests = value
		}
	}

	if postgresqlMaxConcurrent := os.Getenv("POSTGRESQL_MAX_CONCURRENT"); postgresqlMaxConcurrent != "" {
		if value, err := strconv.Atoi(postgresqlMaxConcurrent); err == nil {
			Config.PostgreSQLMaxConcurrent = value
		}
	}

	if mongodbMaxConcurrent := os.Getenv("MONGODB_MAX_CONCURRENT"); mongodbMaxConcurrent != "" {
		if value, err := strconv.Atoi(mongodbMaxConcurrent); err == nil {
			Config.MongoDBMaxConcurrent = value
		}
	}
	
	// 调试模式
	if debugMode := os.Getenv("DEBUG_MODE"); debugMode == "true" || debugMode == "1" {
		Config.DebugMode = true
	}
	
	// 输出配置信息
	log.Printf("配置加载成功: 端口=%s, 最大并发请求=%d, PostgreSQL最大并发=%d, MongoDB最大并发=%d, 调试模式=%v",
		Config.Port, Config.MaxConcurrentRequests, Config.PostgreSQLMaxConcurrent, 
		Config.MongoDBMaxConcurrent, Config.DebugMode)
}

// GetPort 获取应用端口
func GetPort() string {
	return Config.Port
}

// GetMaxConcurrentRequests 获取最大并发请求数
func GetMaxConcurrentRequests() int {
	return Config.MaxConcurrentRequests
}

// GetPostgreSQLMaxConcurrent 获取PostgreSQL最大并发请求数
func GetPostgreSQLMaxConcurrent() int {
	return Config.PostgreSQLMaxConcurrent
}

// GetMongoDBMaxConcurrent 获取MongoDB最大并发请求数
func GetMongoDBMaxConcurrent() int {
	return Config.MongoDBMaxConcurrent
}

// IsDebugMode 是否为调试模式
func IsDebugMode() bool {
	return Config.DebugMode
} 