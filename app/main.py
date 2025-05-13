from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
import uvicorn
import logging
import os

from app.routers import postgresql, mongodb
from app.auth.routes import router as auth_router
from app.auth.middleware import TokenAuthMiddleware
from app.utils.pool import postgresql_pool, mongodb_pool
from app.utils.concurrency import ConcurrencyLimiterMiddleware
from app.utils.error_handler import setup_error_handlers

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("database-api")

# 创建FastAPI应用
app = FastAPI(
    title="数据库API服务",
    description="用于连接并操作PostgreSQL和MongoDB数据库的API服务",
    version="1.0.0",
)

# 添加CORS中间件
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 从环境变量读取并发限制配置
max_concurrent_requests = int(os.environ.get("MAX_CONCURRENT_REQUESTS", 200))
postgresql_max_concurrent = int(os.environ.get("POSTGRESQL_MAX_CONCURRENT", 100))
mongodb_max_concurrent = int(os.environ.get("MONGODB_MAX_CONCURRENT", 100))

# 添加并发控制中间件
app.add_middleware(
    ConcurrencyLimiterMiddleware,
    max_concurrent_requests=max_concurrent_requests,
    postgresql_max_concurrent=postgresql_max_concurrent,
    mongodb_max_concurrent=mongodb_max_concurrent,
)

# 添加令牌认证中间件
app.add_middleware(TokenAuthMiddleware)

logger.info(f"配置并发控制: 总并发={max_concurrent_requests}, PostgreSQL={postgresql_max_concurrent}, MongoDB={mongodb_max_concurrent}")

# 注册路由
app.include_router(postgresql.router, prefix="/apiDatabase")
app.include_router(mongodb.router, prefix="/apiDatabase")
app.include_router(auth_router, prefix="/apiDatabase")

# 设置全局错误处理器
setup_error_handlers(app)

@app.get("/")
async def root():
    return {"message": "数据库API服务已启动"}

@app.on_event("startup")
async def startup_event():
    """应用启动时执行的事件，初始化连接池和数据库表"""
    logger.info("数据库API服务启动，初始化连接池")
    
    # 初始化数据库表
    try:
        # 创建表结构
        conn, error = postgresql_pool.get_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            password="renoelis02@gmail.com"
        )
        
        if not error and conn:
            with conn.cursor() as cursor:
                # 创建api_tokens表
                cursor.execute("""
                CREATE TABLE IF NOT EXISTS api_tokens (
                    token_id SERIAL PRIMARY KEY,
                    access_token VARCHAR(64) UNIQUE NOT NULL,
                    email VARCHAR(100) NOT NULL,
                    ws_id VARCHAR(50) NOT NULL UNIQUE,
                    token_type VARCHAR(20) NOT NULL DEFAULT 'limited',
                    remaining_calls INTEGER,
                    total_calls INTEGER,
                    is_active BOOLEAN DEFAULT TRUE,
                    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
                )
                """)
                
                # 创建token_usage_logs表
                cursor.execute("""
                CREATE TABLE IF NOT EXISTS token_usage_logs (
                    log_id SERIAL PRIMARY KEY,
                    token_id INTEGER REFERENCES api_tokens(token_id),
                    ws_id VARCHAR(50) NOT NULL,
                    operation_type VARCHAR(50) NOT NULL,
                    target_database VARCHAR(50) NOT NULL,
                    target_collection VARCHAR(50),
                    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                    status VARCHAR(20) NOT NULL,
                    request_details JSONB
                )
                """)
                
                conn.commit()
                logger.info("数据库表初始化完成")
            
            postgresql_pool.release_connection(
                host="120.46.147.53",
                port=5432, 
                database="pro_db",
                user="renoelis",
                conn=conn
            )
        else:
            logger.error(f"数据库初始化失败: {error}")
    except Exception as e:
        logger.error(f"数据库初始化过程中发生错误: {str(e)}")

@app.on_event("shutdown")
async def shutdown_event():
    """应用关闭时执行的事件，清理资源"""
    logger.info("数据库API服务关闭，清理资源")
    # 连接池会自动清理

if __name__ == "__main__":
    # 获取端口，默认3010
    port = int(os.environ.get("PORT", 3010))
    uvicorn.run("app.main:app", host="0.0.0.0", port=port, reload=True) 