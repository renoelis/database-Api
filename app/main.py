from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
import uvicorn
import logging
import os
import json
from pathlib import Path

from app.routers import postgresql, mongodb
from app.utils.pool import postgresql_pool, mongodb_pool
from app.utils.concurrency import ConcurrencyLimiterMiddleware
from app.utils.error_handler import setup_error_handlers
from app.utils.auth import initialize_auth
from app.utils.auth_middleware import AuthMiddleware

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger("database-api")

# 初始化认证系统
initialize_auth()

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

# 添加身份验证中间件
app.add_middleware(AuthMiddleware)

logger.info(f"配置并发控制: 总并发={max_concurrent_requests}, PostgreSQL={postgresql_max_concurrent}, MongoDB={mongodb_max_concurrent}")

# 注册路由
app.include_router(postgresql.router, prefix="/apiDatabase")
app.include_router(mongodb.router, prefix="/apiDatabase")

# 设置全局错误处理器
setup_error_handlers(app)

@app.get("/")
async def root():
    return {"message": "数据库API服务已启动"}

@app.get("/apiDatabase/token")
async def get_token():
    """获取当前配置的访问令牌"""
    try:
        # 读取配置文件中的令牌
        base_dir = Path(os.path.dirname(os.path.abspath(__file__))).parent
        config_file = base_dir / "config" / "access_token.json"
        
        if not config_file.exists():
            return {"error": "配置文件不存在"}
            
        with open(config_file, "r") as f:
            config = json.load(f)
            token = config.get("accessToken")
            
        if not token:
            return {"error": "未找到有效的访问令牌"}
            
        return {"accessToken": token}
    except Exception as e:
        logger.error(f"获取令牌时出错: {str(e)}")
        return {"error": f"获取令牌失败: {str(e)}"}

@app.on_event("startup")
async def startup_event():
    """应用启动时执行的事件，初始化连接池"""
    logger.info("数据库API服务启动，初始化连接池")
    # 连接池已在模块导入时初始化

@app.on_event("shutdown")
async def shutdown_event():
    """应用关闭时执行的事件，清理资源"""
    logger.info("数据库API服务关闭，清理资源")
    # 连接池会自动清理

if __name__ == "__main__":
    # 获取端口，默认3010
    port = int(os.environ.get("PORT", 3010))
    uvicorn.run("app.main:app", host="0.0.0.0", port=port, reload=True) 