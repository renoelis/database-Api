from fastapi import Request
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse
import logging
import json
import os
from pathlib import Path
from app.utils.response import error_response

# 配置日志
logger = logging.getLogger("database-api")

# 定义配置文件路径
BASE_DIR = Path(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
CONFIG_DIR = BASE_DIR / "config"
TOKEN_CONFIG_FILE = CONFIG_DIR / "access_token.json"

class AuthMiddleware(BaseHTTPMiddleware):
    """
    身份验证中间件，验证请求头中的访问令牌
    """
    
    async def dispatch(self, request: Request, call_next):
        """
        处理请求，验证访问令牌
        """
        # 排除根路径和文档路径，不需要验证
        if request.url.path in ["/", "/docs", "/redoc", "/openapi.json", "/apiDatabase/token"]:
            return await call_next(request)
        
        # 从请求头获取访问令牌
        token = request.headers.get("accessToken")
        if not token:
            logger.warning("请求缺少accessToken头")
            return JSONResponse(
                status_code=401,
                content=error_response(1401, "缺少accessToken头")
            )
        
        # 从配置文件读取有效令牌
        expected_token = None
        try:
            if TOKEN_CONFIG_FILE.exists():
                with open(TOKEN_CONFIG_FILE, "r") as f:
                    config = json.load(f)
                    expected_token = config.get("accessToken")
        except Exception as e:
            logger.error(f"读取配置文件失败: {str(e)}")
            return JSONResponse(
                status_code=500,
                content=error_response(9001, "服务器配置错误")
            )
            
        if not expected_token:
            logger.error("无法获取有效的访问令牌")
            return JSONResponse(
                status_code=500,
                content=error_response(9002, "服务器配置错误")
            )
        
        # 验证令牌
        if token != expected_token:
            logger.warning("无效的访问令牌")
            return JSONResponse(
                status_code=401,
                content=error_response(1402, "无效的访问令牌")
            )
        
        # 验证通过，继续处理请求
        try:
            return await call_next(request)
        except Exception as e:
            logger.error(f"请求处理过程中发生错误: {str(e)}", exc_info=True)
            return JSONResponse(
                status_code=500,
                content=error_response(9999, "内部服务器错误")
            ) 