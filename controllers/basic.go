package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/renoelis/database-api-go/config"
	"github.com/renoelis/database-api-go/utils"
	"github.com/sirupsen/logrus"
)

// GetRoot 处理根路径请求
func GetRoot(c *gin.Context) {
	utils.ResponseSuccess(c, map[string]string{
		"message": "数据库API服务已启动",
	})
}

// GetToken 获取当前配置的访问令牌
func GetToken(c *gin.Context) {
	logger := logrus.New()

	// 读取配置文件中的令牌
	configFile := filepath.Join(".", "config", "access_token.json")

	// 检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		logger.Error("配置文件不存在")
		utils.ResponseError(c, 404, 1201, "配置文件不存在")
		return
	}

	// 读取配置文件内容
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Errorf("读取配置文件失败: %v", err)
		utils.ResponseError(c, 500, 1202, fmt.Sprintf("读取配置文件失败: %v", err))
		return
	}

	// 解析JSON
	var config config.TokenConfig
	if err := json.Unmarshal(data, &config); err != nil {
		logger.Errorf("解析配置文件失败: %v", err)
		utils.ResponseError(c, 500, 1203, fmt.Sprintf("解析配置文件失败: %v", err))
		return
	}

	// 检查令牌是否存在
	if config.AccessToken == "" {
		logger.Error("未找到有效的访问令牌")
		utils.ResponseError(c, 404, 1204, "未找到有效的访问令牌")
		return
	}

	// 返回令牌
	utils.ResponseSuccess(c, map[string]string{
		"accessToken": config.AccessToken,
	})
} 