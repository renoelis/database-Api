import logging
import secrets
import string
from fastapi import APIRouter, Depends, HTTPException, Query
from app.utils.pool import postgresql_pool
from app.utils.response import success_response, error_response
from app.auth.models import TokenCreate, TokenUpdate, TokenInfoResponse
from psycopg2.extras import DictCursor
from datetime import datetime

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
    try:
        # 获取数据库连接
        conn, error = postgresql_pool.get_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            password="renoelis02@gmail.com"
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return error_response(1000, f"数据库连接失败: {error}")
        
        # 检查工作区ID是否已存在令牌
        with conn.cursor(cursor_factory=DictCursor) as cursor:
            cursor.execute(
                "SELECT * FROM api_tokens WHERE ws_id = %s",
                (token_data.ws_id,)
            )
            existing_token = cursor.fetchone()
            
            if existing_token:
                postgresql_pool.release_connection(
                    host="120.46.147.53",
                    port=5432, 
                    database="pro_db",
                    user="renoelis",
                    conn=conn
                )
                return error_response(1050, f"工作区ID {token_data.ws_id} 已存在关联的令牌")
            
            # 生成访问令牌
            access_token = generate_access_token()
            
            # 设置令牌参数
            is_unlimited = token_data.token_type == "unlimited"
            remaining_calls = None if is_unlimited else token_data.total_calls
            total_calls = None if is_unlimited else token_data.total_calls
            
            # 创建令牌记录
            cursor.execute(
                """
                INSERT INTO api_tokens 
                (access_token, email, ws_id, token_type, remaining_calls, total_calls, is_active)
                VALUES (%s, %s, %s, %s, %s, %s, TRUE)
                RETURNING token_id
                """,
                (
                    access_token,
                    token_data.email,
                    token_data.ws_id,
                    token_data.token_type,
                    remaining_calls,
                    total_calls,
                )
            )
            
            token_id = cursor.fetchone()[0]
            conn.commit()
        
        # 返回创建的令牌信息
        response_data = TokenInfoResponse(
            token_id=token_id,
            access_token=access_token,
            ws_id=token_data.ws_id,
            token_type=token_data.token_type,
            remaining_calls=remaining_calls,
            total_calls=total_calls
        )
        
        postgresql_pool.release_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            conn=conn
        )
        
        return success_response(response_data.dict())
        
    except Exception as e:
        logger.error(f"创建令牌失败: {str(e)}")
        return error_response(9999, f"服务器内部错误: {str(e)}")


@router.post("/token/update", summary="更新令牌使用次数")
async def update_token(update_data: TokenUpdate):
    """
    更新令牌使用次数
    
    - **ws_id**: 工作区ID
    - **operation**: 操作类型 (add=增加次数, set=设置次数, unlimited=设置为无限制)
    - **calls_value**: 对于add和set操作，指定调用次数值
    """
    try:
        # 获取数据库连接
        conn, error = postgresql_pool.get_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            password="renoelis02@gmail.com"
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return error_response(1000, f"数据库连接失败: {error}")
        
        # 检查工作区ID是否存在令牌
        with conn.cursor(cursor_factory=DictCursor) as cursor:
            cursor.execute(
                "SELECT * FROM api_tokens WHERE ws_id = %s",
                (update_data.ws_id,)
            )
            token_record = cursor.fetchone()
            
            if not token_record:
                postgresql_pool.release_connection(
                    host="120.46.147.53",
                    port=5432, 
                    database="pro_db",
                    user="renoelis",
                    conn=conn
                )
                return error_response(1051, f"未找到工作区ID {update_data.ws_id} 关联的令牌")
            
            # 执行不同的更新操作
            if update_data.operation == "unlimited":
                # 设置为无限制使用
                cursor.execute(
                    """
                    UPDATE api_tokens 
                    SET token_type = 'unlimited', remaining_calls = NULL, total_calls = NULL, updated_at = CURRENT_TIMESTAMP
                    WHERE ws_id = %s
                    RETURNING token_id, ws_id, token_type, remaining_calls, total_calls
                    """,
                    (update_data.ws_id,)
                )
            elif update_data.operation == "add":
                # 增加指定次数
                if not update_data.calls_value or update_data.calls_value <= 0:
                    return error_response(1400, "增加操作需要正数的调用次数值")
                
                cursor.execute(
                    """
                    UPDATE api_tokens 
                    SET token_type = 'limited', 
                        remaining_calls = COALESCE(remaining_calls, 0) + %s,
                        total_calls = COALESCE(total_calls, 0) + %s,
                        updated_at = CURRENT_TIMESTAMP
                    WHERE ws_id = %s
                    RETURNING token_id, ws_id, token_type, remaining_calls, total_calls
                    """,
                    (update_data.calls_value, update_data.calls_value, update_data.ws_id)
                )
            elif update_data.operation == "set":
                # 设置为指定次数
                if not update_data.calls_value or update_data.calls_value < 0:
                    return error_response(1400, "设置操作需要非负的调用次数值")
                
                cursor.execute(
                    """
                    UPDATE api_tokens 
                    SET token_type = 'limited', 
                        remaining_calls = %s,
                        total_calls = %s,
                        updated_at = CURRENT_TIMESTAMP
                    WHERE ws_id = %s
                    RETURNING token_id, ws_id, token_type, remaining_calls, total_calls
                    """,
                    (update_data.calls_value, update_data.calls_value, update_data.ws_id)
                )
            else:
                return error_response(1400, f"不支持的操作类型: {update_data.operation}")
            
            updated_token = cursor.fetchone()
            conn.commit()
        
        # 返回更新后的令牌信息
        response_data = TokenInfoResponse(
            token_id=updated_token["token_id"],
            ws_id=updated_token["ws_id"],
            token_type=updated_token["token_type"],
            remaining_calls=updated_token["remaining_calls"],
            total_calls=updated_token["total_calls"]
        )
        
        postgresql_pool.release_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            conn=conn
        )
        
        return success_response(response_data.dict())
        
    except Exception as e:
        logger.error(f"更新令牌失败: {str(e)}")
        return error_response(9999, f"服务器内部错误: {str(e)}")


@router.get("/token/info", summary="查询令牌信息")
async def get_token_info(ws_id: str = Query(..., description="工作区ID")):
    """
    查询令牌信息
    
    - **ws_id**: 工作区ID
    """
    try:
        # 获取数据库连接
        conn, error = postgresql_pool.get_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            password="renoelis02@gmail.com"
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return error_response(1000, f"数据库连接失败: {error}")
        
        # 查询令牌信息
        with conn.cursor(cursor_factory=DictCursor) as cursor:
            cursor.execute(
                "SELECT * FROM api_tokens WHERE ws_id = %s",
                (ws_id,)
            )
            token_record = cursor.fetchone()
            
            if not token_record:
                postgresql_pool.release_connection(
                    host="120.46.147.53",
                    port=5432, 
                    database="pro_db",
                    user="renoelis",
                    conn=conn
                )
                return error_response(1051, f"未找到工作区ID {ws_id} 关联的令牌")
            
            # 转换日期时间格式
            token_info = dict(token_record)
            
            # 确保 datetime 对象正确序列化
            for field in ["created_at", "updated_at"]:
                if field in token_info and token_info[field]:
                    token_info[field] = token_info[field].isoformat()
        
        postgresql_pool.release_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            conn=conn
        )
        
        return success_response(token_info)
        
    except Exception as e:
        logger.error(f"查询令牌信息失败: {str(e)}")
        return error_response(9999, f"服务器内部错误: {str(e)}") 