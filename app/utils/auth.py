import os
import secrets
import json
import logging
from pathlib import Path
from fastapi import HTTPException, Header
from typing import Optional

# 配置日志
logger = logging.getLogger("database-api")

# 定义配置文件路径 - 使用绝对路径
BASE_DIR = Path(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
CONFIG_DIR = BASE_DIR / "config"
TOKEN_CONFIG_FILE = CONFIG_DIR / "access_token.json"

# 全局变量存储加载的访问令牌
ACCESS_TOKEN = None

def generate_access_token() -> str:
    """生成一个随机的访问令牌"""
    return secrets.token_hex(32)  # 生成一个64字符的十六进制字符串

def save_access_token(token: str) -> None:
    """将访问令牌保存到配置文件"""
    # 确保配置目录存在
    CONFIG_DIR.mkdir(exist_ok=True)
    
    # 保存令牌到配置文件
    with open(TOKEN_CONFIG_FILE, "w") as f:
        json.dump({"accessToken": token}, f, indent=2)
    
    logger.info(f"访问令牌已保存到 {TOKEN_CONFIG_FILE}")

def load_access_token() -> str:
    """从配置文件加载访问令牌，如果不存在则生成一个新的"""
    try:
        with open(TOKEN_CONFIG_FILE, "r") as f:
            config = json.load(f)
            token = config.get("accessToken")
            if token:
                logger.info("已从配置文件加载访问令牌")
                return token
            else:
                logger.warning("配置文件中没有找到accessToken字段")
    except (FileNotFoundError, json.JSONDecodeError) as e:
        logger.warning(f"加载访问令牌失败: {str(e)}")
    
    # 如果配置文件不存在或无效，生成一个新的令牌
    token = generate_access_token()
    save_access_token(token)
    logger.info("已生成新的访问令牌")
    return token

def initialize_auth():
    """初始化认证系统，加载或生成访问令牌"""
    global ACCESS_TOKEN
    ACCESS_TOKEN = load_access_token()
    logger.info("认证系统初始化完成")

async def verify_access_token(access_token: Optional[str] = Header(None, alias="accessToken")) -> bool:
    """验证请求头中的访问令牌"""
    if not access_token:
        logger.warning("缺少accessToken头")
        raise HTTPException(
            status_code=401,
            detail="缺少accessToken头",
        )
        
    if access_token != ACCESS_TOKEN:
        logger.warning("无效的访问令牌")
        raise HTTPException(
            status_code=401,
            detail="无效的访问令牌",
        )
    return True 