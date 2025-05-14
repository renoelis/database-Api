package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"
)

var (
	// 全局变量，存储访问令牌
	AccessToken string
	logger      *logrus.Logger
)

// 令牌配置结构
type TokenConfig struct {
	AccessToken string `json:"accessToken"`
}

// 初始化配置
func InitConfig() {
	// 初始化日志
	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	// 确保配置目录存在
	ensureConfigDir()
}

// 初始化认证系统
func InitAuth() {
	// 加载或生成访问令牌
	AccessToken = loadAccessToken()
	logger.Info("认证系统初始化完成")
}

// 确保配置目录存在
func ensureConfigDir() {
	configDir := filepath.Join(".", "config")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			logger.Fatalf("创建配置目录失败: %v", err)
		}
	}
}

// 加载访问令牌
func loadAccessToken() string {
	configFile := filepath.Join(".", "config", "access_token.json")
	
	// 检查文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// 文件不存在，生成新令牌
		token := generateAccessToken()
		saveAccessToken(token)
		return token
	}

	// 读取配置文件
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Warnf("读取令牌配置文件失败: %v", err)
		token := generateAccessToken()
		saveAccessToken(token)
		return token
	}

	var config TokenConfig
	if err := json.Unmarshal(data, &config); err != nil {
		logger.Warnf("解析令牌配置文件失败: %v", err)
		token := generateAccessToken()
		saveAccessToken(token)
		return token
	}

	if config.AccessToken == "" {
		logger.Warn("配置文件中未找到有效的访问令牌")
		token := generateAccessToken()
		saveAccessToken(token)
		return token
	}

	logger.Info("已从配置文件加载访问令牌")
	return config.AccessToken
}

// 生成访问令牌
func generateAccessToken() string {
	// 使用加密安全的随机数生成器
	// 生成32字节（256位）的随机数，转换为64个十六进制字符
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		// 如果随机数生成失败，使用备用方法
		logger.Warnf("生成安全随机令牌失败: %v，使用备用方法", err)
		return generateFallbackToken()
	}
	
	// 将随机字节转换为十六进制字符串
	token := hex.EncodeToString(tokenBytes)
	logger.Info("已生成新的安全访问令牌")
	return token
}

// 备用令牌生成方法
func generateFallbackToken() string {
	// 比原来的方法稍微复杂一些，但仍不如加密随机数安全
	token := fmt.Sprintf("%d", os.Getpid())
	for i := 0; i < 30; i++ {
		token = token + fmt.Sprintf("%x", (os.Getpid()*i)^(i*7919)) // 7919是一个质数
	}
	logger.Warn("使用备用方法生成访问令牌")
	return token
}

// 保存访问令牌
func saveAccessToken(token string) {
	configFile := filepath.Join(".", "config", "access_token.json")
	config := TokenConfig{
		AccessToken: token,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Fatalf("序列化令牌配置失败: %v", err)
	}

	if err := ioutil.WriteFile(configFile, data, 0644); err != nil {
		logger.Fatalf("保存令牌配置文件失败: %v", err)
	}

	logger.Infof("访问令牌已保存到 %s", configFile)
}

// 获取环境变量，如果不存在则使用默认值
func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// 获取整型环境变量，如果不存在或无法转换则使用默认值
func GetEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		logger.Warnf("环境变量 %s 转换为整数失败: %v，使用默认值 %d", key, err, defaultValue)
		return defaultValue
	}

	return intValue
} 