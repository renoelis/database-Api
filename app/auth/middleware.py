import logging
from fastapi import Request, HTTPException
from starlette.middleware.base import BaseHTTPMiddleware
from fastapi.responses import JSONResponse
import json
from app.utils.response import error_response
from app.database.auth_operations import validate_auth_token, update_token_usage

logger = logging.getLogger("database-api")

class TokenAuthMiddleware(BaseHTTPMiddleware):
    """令牌认证中间件，用于验证API请求中的访问令牌"""
    
    async def dispatch(self, request: Request, call_next):
        # 保存请求体以便多次读取
        body_content = None
        if request.method != "GET":
            try:
                body_content = await request.body()
                # 创建一个新的请求体
                async def receive():
                    return {"type": "http.request", "body": body_content}
                request._receive = receive
            except Exception as e:
                logger.error(f"保存请求体失败: {str(e)}")
        
        # 不需要验证的路径
        exclude_paths = [
            "/apiDatabase/auth/token",
            "/apiDatabase/auth/token/update",
            "/apiDatabase/auth/token/info",
            "/apiDatabase/auth/logs/cleanup",
            "/",
            "/docs",
            "/openapi.json",
            "/redoc"
        ]
        
        # 检查是否需要跳过验证
        path = request.url.path
        if any(path == excluded or (excluded != "/" and path.startswith(excluded + "/")) for excluded in exclude_paths):
            # 减少日志记录，仅在调试级别记录
            logger.debug(f"跳过验证路径: {path}")
            return await call_next(request)
        
        # 获取访问令牌
        access_token = None
        
        # 从请求头获取令牌
        if "accessToken" in request.headers:
            access_token = request.headers.get("accessToken")
        
        # 从查询参数获取令牌
        if not access_token and "access_token" in request.query_params:
            access_token = request.query_params.get("access_token")
        
        # 如果没有令牌，返回错误
        if not access_token:
            return JSONResponse(
                status_code=401,
                content=error_response(1401, "未提供访问令牌")
            )
        
        # 验证令牌
        token_valid, token_info = await validate_auth_token(access_token)
        if not token_valid:
            return JSONResponse(
                status_code=401,
                content=error_response(1402, "无效的访问令牌")
            )
        
        # 严格检查写操作 - POST, PUT, PATCH, DELETE方法
        is_write_operation = request.method in ["POST", "PUT", "PATCH", "DELETE"]
        
        # 检查令牌调用次数（仅对有限制的令牌进行检查）
        if is_write_operation and token_info["token_type"] == "limited":
            if token_info["remaining_calls"] <= 0:
                return JSONResponse(
                    status_code=403,
                    content=error_response(1403, "令牌调用次数已用完")
                )
        
        # 将令牌信息存储在请求状态中
        request.state.token_info = token_info
        
        # 继续处理请求
        response = await call_next(request)
        
        # 完成请求后，记录令牌使用日志（对所有写操作，不管是限制性还是无限制令牌）
        if is_write_operation:
            # 简化日志，只记录路径和方法
            logger.debug(f"处理令牌使用: {request.method} {path}")
            success = await self._update_token_usage(
                token_id=token_info["token_id"],
                ws_id=token_info["ws_id"],
                token_type=token_info["token_type"],
                request=request,
                response=response,
                body_content=body_content
            )
            if not success:
                logger.error(f"令牌使用记录失败: token_id={token_info['token_id']}")
        
        return response
    
    async def _update_token_usage(self, token_id, ws_id, token_type, request, response, body_content):
        """更新令牌使用次数并记录使用日志"""
        try:
            # 获取请求和响应的详细信息
            request_body = None
            try:
                # 尝试解析请求体
                if body_content:
                    request_body = json.loads(body_content)
            except Exception as e:
                logger.error(f"解析请求体失败: {str(e)}")
                # 即使请求体解析失败，仍然需要记录令牌使用
            
            # 记录使用日志需要的信息
            operation_type = request.url.path.split("/")[-1]
            target_database = ""
            target_collection = ""
            
            # 提取目标数据库和集合信息
            if request_body and isinstance(request_body, dict):
                # 针对不同结构的请求体尝试提取信息
                if "connection" in request_body:
                    if isinstance(request_body["connection"], dict):
                        target_database = request_body["connection"].get("database", "")
                
                # 针对MongoDB操作，提取集合名
                if "collection" in request_body:
                    target_collection = request_body.get("collection", "")
                
                # 针对SQL操作
                if "table" in request_body:
                    target_collection = request_body.get("table", "")
            
            # 响应状态
            status = "success" if response.status_code < 400 else "error"
            
            # 使用封装的函数更新令牌使用情况
            success = await update_token_usage(
                token_id=token_id,
                ws_id=ws_id,
                operation_type=operation_type,
                target_database=target_database,
                target_collection=target_collection,
                status=status,
                request_details=request_body
            )
            
            if not success:
                logger.error(f"更新令牌使用记录失败！token_id={token_id}, ws_id={ws_id}, operation={operation_type}")
                return False
                
            return True
                
        except Exception as e:
            logger.error(f"更新令牌使用记录时发生错误: {str(e)}")
            return False 