import logging
import secrets
import string
from fastapi import APIRouter, Depends, HTTPException, Query, Response, status
from app.utils.response import success_response, error_response
from app.auth.models import TokenCreate, TokenUpdate, TokenInfoResponse
from app.database.auth_operations import create_auth_token, update_auth_token, get_auth_token_info, cleanup_token_usage_logs

logger = logging.getLogger("database-api")
router = APIRouter(prefix="/auth", tags=["认证"])

def generate_access_token(length=48):
    """生成随机访问令牌"""
    alphabet = string.ascii_letters + string.digits
    return ''.join(secrets.choice(alphabet) for _ in range(length))


@router.post("/token", summary="创建访问令牌")
async def create_token(token_data: TokenCreate):
    """
    创建与工作区关联的访问令牌
    
    - **email**: 用户邮箱
    - **ws_id**: 工作区ID (唯一)
    - **token_type**: 令牌类型 (limited 或 unlimited)
    - **total_calls**: 如果是limited类型，指定总调用次数
    """
    # 生成访问令牌
    access_token = generate_access_token()
    
    # 创建令牌
    success, token_info, error_msg = await create_auth_token(
        email=token_data.email,
        ws_id=token_data.ws_id,
        token_type=token_data.token_type,
        access_token=access_token,
        total_calls=token_data.total_calls
    )
    
    if not success:
        error_code = 1050 if "已存在关联的令牌" in error_msg else 9999
        return error_response(error_code, error_msg)
    
    # 返回创建的令牌信息
    response_data = TokenInfoResponse(
        token_id=token_info["token_id"],
        access_token=token_info["access_token"],
        ws_id=token_info["ws_id"],
        token_type=token_info["token_type"],
        remaining_calls=token_info["remaining_calls"],
        total_calls=token_info["total_calls"]
    )
    
    return success_response(response_data.dict())


@router.post("/token/update", summary="更新令牌使用次数")
async def update_token(update_data: TokenUpdate):
    """
    更新令牌使用次数
    
    - **ws_id**: 工作区ID
    - **operation**: 操作类型 (add=增加次数, set=设置次数, unlimited=设置为无限制)
    - **calls_value**: 对于add和set操作，指定调用次数值
    """
    # 更新令牌
    success, token_info, error_msg = await update_auth_token(
        ws_id=update_data.ws_id,
        operation=update_data.operation,
        calls_value=update_data.calls_value
    )
    
    if not success:
        error_code = 1051 if "未找到工作区ID" in error_msg else 1400 if "操作需要" in error_msg else 9999
        return error_response(error_code, error_msg)
    
    # 返回更新后的令牌信息
    response_data = TokenInfoResponse(
        token_id=token_info["token_id"],
        ws_id=token_info["ws_id"],
        token_type=token_info["token_type"],
        remaining_calls=token_info["remaining_calls"],
        total_calls=token_info["total_calls"]
    )
    
    return success_response(response_data.dict())


@router.get("/token/info", summary="查询令牌信息")
async def get_token_info(ws_id: str = Query(..., description="工作区ID")):
    """
    查询令牌信息
    
    - **ws_id**: 工作区ID
    """
    # 查询令牌信息
    success, token_info, error_msg = await get_auth_token_info(ws_id)
    
    if not success:
        error_code = 1051 if "未找到工作区ID" in error_msg else 1000 if "数据库连接失败" in error_msg else 9999
        return error_response(error_code, error_msg)
    
    return success_response(token_info)


@router.post("/logs/cleanup", summary="清理旧的令牌使用日志", status_code=status.HTTP_202_ACCEPTED)
async def cleanup_logs(response: Response):
    """
    手动触发清理旧的令牌使用日志
    
    清理1个月前的所有令牌使用日志记录
    """
    # 执行清理操作
    success, error_msg = await cleanup_token_usage_logs()
    
    if not success:
        response.status_code = status.HTTP_500_INTERNAL_SERVER_ERROR
        return error_response(1100, f"清理令牌使用日志失败: {error_msg}")
    
    return success_response({"message": "令牌使用日志清理任务已完成"}) 