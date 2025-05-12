from fastapi import FastAPI, Request, status
from fastapi.responses import JSONResponse
from fastapi.exceptions import RequestValidationError
from starlette.exceptions import HTTPException
import logging
import traceback
from typing import Union, Dict, Any

from app.utils.response import error_response

logger = logging.getLogger("database-api")

def setup_error_handlers(app: FastAPI) -> None:
    """配置全局错误处理器，确保所有错误都以统一格式返回"""
    
    @app.exception_handler(HTTPException)
    async def http_exception_handler(request: Request, exc: HTTPException) -> JSONResponse:
        """处理HTTP异常，返回统一格式"""
        logger.error(f"HTTP异常: {exc.detail}")
        return JSONResponse(
            content=error_response(exc.status_code, str(exc.detail)),
            status_code=exc.status_code,
            headers=getattr(exc, "headers", None)
        )
    
    @app.exception_handler(RequestValidationError)
    async def validation_exception_handler(request: Request, exc: RequestValidationError) -> JSONResponse:
        """处理请求验证错误，返回统一格式"""
        errors = exc.errors()
        error_details = []
        for error in errors:
            error_details.append({
                "loc": error.get("loc", []),
                "msg": error.get("msg", ""),
                "type": error.get("type", "")
            })
        
        logger.error(f"请求验证错误: {error_details}")
        return JSONResponse(
            content=error_response(
                1400, 
                f"请求参数验证失败: {'; '.join([e['msg'] for e in error_details])}"
            ),
            status_code=status.HTTP_422_UNPROCESSABLE_ENTITY
        )
    
    @app.exception_handler(Exception)
    async def unhandled_exception_handler(request: Request, exc: Exception) -> JSONResponse:
        """处理所有未捕获的异常，返回统一格式"""
        error_msg = f"未处理的异常: {str(exc)}"
        logger.error(error_msg)
        logger.error(traceback.format_exc())
        
        return JSONResponse(
            content=error_response(9999, "服务器内部错误"),
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR
        ) 