package main

import (
	"database-api-public-go/config"
	"database-api-public-go/database"
	"database-api-public-go/middleware"
	"database-api-public-go/router"
	"database-api-public-go/utils"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// 自定义日志格式
type LogWriter struct {
	io.Writer
	timeFormat string
}

// Write 实现io.Writer接口
func (w LogWriter) Write(b []byte) (n int, err error) {
	return w.Writer.Write(formatLogEntry(b, w.timeFormat))
}

// formatLogEntry 格式化日志条目
func formatLogEntry(b []byte, timeFormat string) []byte {
	// 分割日志内容
	parts := strings.SplitN(string(b), " ", 3)
	if len(parts) < 3 {
		return b
	}

	// 提取文件和行信息
	filePart := parts[1]
	fileParts := strings.Split(filePart, ":")
	fileInfo := fileParts[0]
	
	// 简化文件路径信息
	if idx := strings.LastIndex(fileInfo, "/"); idx >= 0 {
		fileInfo = fileInfo[idx+1:]
	}
	
	// 添加时间戳和格式化
	timestamp := time.Now().Format(timeFormat)
	
	// 构建新的日志条目
	msg := parts[2]
	formattedMsg := fmt.Sprintf("[%s] [%s] %s\n", timestamp, fileInfo, strings.TrimSpace(msg))
	return []byte(formattedMsg)
}

// 初始化日志配置
func setupLogger() {
	// 设置自定义日志格式
	if !config.IsDebugMode() {
		// 非调试模式下，使用简化日志格式
		log.SetFlags(0) // 清除默认的时间和文件前缀
		log.SetOutput(LogWriter{os.Stdout, "2006-01-02 15:04:05"})
		
		// 设置Gin为发布模式，减少日志输出
		gin.SetMode(gin.ReleaseMode)
	} else {
		// 调试模式下，保留详细的文件和行号信息
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	}
}

// 初始化认证系统
func initAuthSystem() {
	success, err := database.InitAuthTables()
	if !success {
		log.Printf("初始化令牌认证系统数据库表失败: %v", err)
	} else {
		log.Println("初始化令牌认证系统数据库表成功")
	}
}

// 预热数据库连接池
func warmupDatabasePools() {
	log.Println("开始预热数据库连接池...")
	
	// 预热认证数据库连接池
	host := database.AuthDBConfig["host"]
	portStr := database.AuthDBConfig["port"]
	dbName := database.AuthDBConfig["database"]
	user := database.AuthDBConfig["user"]
	password := database.AuthDBConfig["password"]
	sslmode := database.AuthDBConfig["sslmode"]
	
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("解析端口失败: %v，使用默认端口5432", err)
		port = 5432
	}
	
	// 预热PostgreSQL连接池
	success := utils.PostgreSQLPool.WarmupPostgreSQLPool(host, port, dbName, user, password, sslmode)
	if success {
		log.Println("预热PostgreSQL连接池成功")
	} else {
		log.Println("预热PostgreSQL连接池失败")
	}
}

// 定期清理令牌使用日志
func scheduleTokenLogsCleanup() {
	go func() {
		for {
			// 计算到下一个凌晨3点的时间
			now := time.Now()
			next := now.Add(24 * time.Hour)
			next = time.Date(next.Year(), next.Month(), next.Day(), 3, 0, 0, 0, next.Location())
			
			// 计算需要等待的时间
			duration := next.Sub(now)
			log.Printf("计划下次令牌使用日志清理时间: %s", next.Format("2006-01-02 15:04:05"))
			
			// 等待到指定时间
			time.Sleep(duration)
			
			// 执行清理任务
			log.Printf("开始执行令牌使用日志清理任务: %s", time.Now().Format("2006-01-02 15:04:05"))
			success, err := database.CleanupTokenUsageLogs()
			if !success {
				log.Printf("令牌使用日志清理失败: %v", err)
			}
		}
	}()
}

func main() {
	// 加载配置
	config.LoadConfig()
	
	// 设置日志格式
	setupLogger()
	
	log.Println("数据库API服务启动中...")
	
	// 避免重复输出配置信息，因为config.LoadConfig()已经输出了一次
	
	// 初始化认证系统
	initAuthSystem()
	
	// 预热数据库连接池
	warmupDatabasePools()
	
	// 启动定时清理任务
	scheduleTokenLogsCleanup()
	
	// 初始化并发控制器
	middleware.InitConcurrencyLimiter()

	// 设置路由
	r := router.SetupRouter()

	// 创建通道来接收信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 启动协程监听系统信号
	go func() {
		<-quit
		log.Println("正在关闭服务...")

		// 清理资源
		log.Println("正在关闭数据库连接...")
		utils.PostgreSQLPool.CloseAll()
		utils.MongoDBPool.CloseAll()

		log.Println("服务已安全关闭")
		os.Exit(0)
	}()

	// 获取端口
	port := config.GetPort()
	log.Printf("数据库API服务启动，监听端口: %s", port)

	// 启动服务
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
} 