import os
import logging
from typing import Dict, Any

logger = logging.getLogger("database-api")

# PostgreSQL配置（用于存储令牌信息）
DEFAULT_PG_CONFIG = {
    "host": "xxxx",
    "port": 5432,
    "database": "xxxx",
    "user": "xxxx",
    "password": "xxxxx" ,
    "sslmode": "prefer",
    "connect_timeout": 30
}

# 从环境变量加载数据库配置
def load_pg_config() -> Dict[str, Any]:
    """从环境变量加载PostgreSQL配置，如果没有则使用默认值"""
    config = DEFAULT_PG_CONFIG.copy()
    
    # 环境变量覆盖默认值
    if os.getenv("PG_HOST"):
        config["host"] = os.getenv("PG_HOST")
    if os.getenv("PG_PORT"):
        config["port"] = int(os.getenv("PG_PORT"))
    if os.getenv("PG_DATABASE"):
        config["database"] = os.getenv("PG_DATABASE")
    if os.getenv("PG_USER"):
        config["user"] = os.getenv("PG_USER")
    if os.getenv("PG_PASSWORD"):
        config["password"] = os.getenv("PG_PASSWORD")
    if os.getenv("PG_SSLMODE"):
        config["sslmode"] = os.getenv("PG_SSLMODE")
    if os.getenv("PG_CONNECT_TIMEOUT"):
        config["connect_timeout"] = int(os.getenv("PG_CONNECT_TIMEOUT"))
    
    return config

# 获取令牌认证系统使用的数据库配置
def get_auth_db_config() -> Dict[str, Any]:
    """获取令牌认证系统使用的数据库配置"""
    return load_pg_config()

# 加载配置
pg_config = load_pg_config()
auth_db_config = get_auth_db_config()

logger.info(f"PostgreSQL配置加载完成: {pg_config['host']}:{pg_config['port']}/{pg_config['database']}") 
