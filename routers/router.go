package routers

import (
	"github.com/gin-gonic/gin"
	"github.com/renoelis/database-api-go/controllers"
	"github.com/renoelis/database-api-go/middleware"
)

// SetupRouter 设置所有路由
func SetupRouter() *gin.Engine {
	// 设置为生产环境模式
	gin.SetMode(gin.ReleaseMode)

	// 初始化Gin
	router := gin.Default()

	// 使用自定义中间件
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RecoveryMiddleware())

	// 获取并发限制配置
	router.Use(middleware.ConcurrencyLimiterMiddleware())

	// 根路由，不需要认证
	router.GET("/", controllers.GetRoot)
	router.GET("/apiDatabase/token", controllers.GetToken)

	// API路由组，需要认证
	api := router.Group("/apiDatabase")
	api.Use(middleware.AuthMiddleware())
	{
		// PostgreSQL路由
		api.POST("/postgresql", controllers.ExecutePostgreSQL)

		// MongoDB路由
		api.POST("/mongodb", controllers.ExecuteMongoDB)
	}

	return router
}
