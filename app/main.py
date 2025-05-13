from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
import uvicorn
import logging
import os
import asyncio
import datetime

from app.routers import postgresql, mongodb
from app.auth.routes import router as auth_router
from app.auth.middleware import TokenAuthMiddleware
from app.utils.pool import postgresql_pool, mongodb_pool
from app.utils.concurrency import ConcurrencyLimiterMiddleware
from app.utils.error_handler import setup_error_handlers
from app.database.auth_operations import init_auth_tables, cleanup_token_usage_logs

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

# 令牌使用日志清理任务
async def scheduled_cleanup_token_logs():
    """定期清理令牌使用日志的任务"""
    while True:
        try:
            # 计算距离下一个凌晨3点的时间
            now = datetime.datetime.now()
            next_run = now.replace(hour=3, minute=0, second=0, microsecond=0)
            if now >= next_run:
                next_run = next_run + datetime.timedelta(days=1)
            
            # 计算需要等待的秒数
            wait_seconds = (next_run - now).total_seconds()
            # 只在首次启动时记录下一次执行时间
            logger.debug(f"令牌使用日志清理计划: {next_run.strftime('%Y-%m-%d %H:%M:%S')}")
            
            # 等待到下一次执行时间
            await asyncio.sleep(wait_seconds)
            
            # 执行清理任务
            logger.info(f"开始执行令牌使用日志清理任务: {datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
            success, error = await cleanup_token_usage_logs()
            if not success:
                logger.error(f"令牌使用日志清理失败: {error}")
            
        except Exception as e:
            logger.error(f"令牌使用日志清理任务异常: {str(e)}")
            # 出错后等待一小时再次尝试
            await asyncio.sleep(3600)

@app.on_event("startup")
async def startup_event():
    """应用启动时执行的事件，初始化连接池和数据库表"""
    logger.info("数据库API服务启动，初始化连接池")
    
    # 初始化令牌认证系统数据库表
    success, error = await init_auth_tables()
    if not success:
        logger.error(f"初始化令牌认证系统数据库表失败: {error}")
    else:
        logger.info("初始化令牌认证系统数据库表成功")
    
    # 启动定时清理任务
    asyncio.create_task(scheduled_cleanup_token_logs())
    logger.info("令牌使用日志清理定时任务已启动")

@app.on_event("shutdown")
async def shutdown_event():
    """应用关闭时执行的事件，清理资源"""
    logger.info("数据库API服务关闭，清理资源")
    # 连接池会自动清理

if __name__ == "__main__":
    # 获取端口，默认3010
    port = int(os.environ.get("PORT", 3010))
    uvicorn.run("app.main:app", host="0.0.0.0", port=port, reload=True) 