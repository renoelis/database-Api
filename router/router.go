package router

import (
	"database-api-public-go/config"
	"database-api-public-go/controller"
	"database-api-public-go/middleware"
	"database-api-public-go/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 添加CORS中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "AccessToken", "accessToken"},
		AllowCredentials: true,
	}))

	// 添加令牌认证中间件（优先进行认证）
	r.Use(middleware.TokenAuthMiddleware())
	
	// 添加并发控制中间件（认证后进行并发控制）
	r.Use(middleware.ConcurrencyMiddleware())

	// 注册API路由
	api := r.Group("/apiDatabase")
	{
		// PostgreSQL路由
		api.POST("/postgresql", controller.PostgreSQLHandler)
		
		// MongoDB路由
		api.POST("/mongodb", controller.MongoDBHandler)

		// 认证路由
		auth := api.Group("/auth")
		{
			auth.POST("/token", controller.CreateTokenHandler)
			auth.POST("/token/update", controller.UpdateTokenHandler)
			auth.GET("/token/info", controller.GetTokenInfoHandler)
			auth.POST("/logs/cleanup", controller.CleanupLogsHandler)
		}
		
		// 系统控制路由
		sys := api.Group("/system")
		{
			// 添加系统接口的renoelis授权检查中间件
			sys.Use(func(c *gin.Context) {
				// 获取令牌信息
				tokenInfo, exists := c.Get("tokenInfo")
				if !exists {
					utils.ErrorResponse(c, 1405, "未找到令牌信息")
					c.Abort()
					return
				}
				
				// 检查ws_id是否为renoelis
				tokenMap, ok := tokenInfo.(map[string]interface{})
				if !ok || tokenMap["ws_id"] != "renoelis" {
					utils.ErrorResponse(c, 1406, "无权访问系统控制接口，仅允许renoelis管理员令牌")
					c.Abort()
					return
				}
				
				c.Next()
			})
			
			// 禁用并发限制
			sys.GET("/concurrency/disable", func(c *gin.Context) {
				middleware.DisableConcurrencyLimit()
				c.JSON(200, gin.H{
					"message": "并发控制已禁用",
				})
			})
			
			// 启用并发限制
			sys.GET("/concurrency/enable", func(c *gin.Context) {
				middleware.EnableConcurrencyLimit()
				c.JSON(200, gin.H{
					"message": "并发控制已启用",
				})
			})
			
			// 查询系统配置
			sys.GET("/config", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"max_concurrent_requests": config.GetMaxConcurrentRequests(),
					"postgresql_max_concurrent": config.GetPostgreSQLMaxConcurrent(),
					"mongodb_max_concurrent": config.GetMongoDBMaxConcurrent(),
					"debug_mode": config.IsDebugMode(),
					"message": "系统配置查询成功",
				})
			})
		}
	}

	// 首页路由
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "数据库API服务已启动",
		})
	})

	return r
} 