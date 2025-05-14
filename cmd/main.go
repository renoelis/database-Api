package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renoelis/database-api-go/config"
	"github.com/renoelis/database-api-go/routers"
	"github.com/sirupsen/logrus"
)

func main() {
	// 初始化配置
	config.InitConfig()

	// 初始化日志
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)
	logger.Info("数据库API服务启动中...")

	// 获取端口号，默认3011
	port := config.GetEnvInt("PORT", 3011)

	// 初始化认证系统
	config.InitAuth()

	// 设置路由
	router := routers.SetupRouter()

	// 创建服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	// 在goroutine中启动服务器
	go func() {
		logger.Infof("服务器监听端口: %d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("启动服务器错误: %s", err)
		}
	}()

	// 等待中断信号优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("关闭服务器...")

	// 设置5秒超时来关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("服务器强制关闭: %s", err)
	}

	logger.Info("服务器已关闭")
} 