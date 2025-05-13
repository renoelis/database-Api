import logging
import json
from typing import Dict, Any, Tuple, Optional, List
from psycopg2.extras import DictCursor
from datetime import datetime
from app.utils.pool import postgresql_pool
from app.database.config import auth_db_config

logger = logging.getLogger("database-api")

# 数据库初始化操作
async def init_auth_tables() -> Tuple[bool, Optional[str]]:
    """初始化令牌认证系统所需的数据库表"""
    try:
        # 获取数据库连接
        config = auth_db_config
        conn, error = postgresql_pool.get_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            password=config["password"]
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return False, f"数据库连接失败: {error}"
        
        # 创建表结构
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
        
        postgresql_pool.release_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            conn=conn
        )
        
        logger.info("令牌认证系统数据库表初始化完成")
        return True, None
        
    except Exception as e:
        logger.error(f"初始化令牌认证系统数据库表失败: {str(e)}")
        return False, str(e)

# 令牌操作函数
async def create_auth_token(email: str, ws_id: str, token_type: str, access_token: str, 
                           total_calls: Optional[int] = None) -> Tuple[bool, Optional[Dict[str, Any]], Optional[str]]:
    """
    创建新的访问令牌
    
    Args:
        email: 用户邮箱
        ws_id: 工作区ID
        token_type: 令牌类型 (limited 或 unlimited)
        access_token: 访问令牌
        total_calls: 总调用次数 (对于limited类型)
        
    Returns:
        (成功标志, 创建的令牌信息, 错误信息)
    """
    try:
        # 获取数据库连接
        config = auth_db_config
        conn, error = postgresql_pool.get_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            password=config["password"]
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return False, None, f"数据库连接失败: {error}"
        
        # 检查工作区ID是否已存在令牌
        with conn.cursor(cursor_factory=DictCursor) as cursor:
            cursor.execute(
                "SELECT * FROM api_tokens WHERE ws_id = %s",
                (ws_id,)
            )
            existing_token = cursor.fetchone()
            
            if existing_token:
                postgresql_pool.release_connection(
                    host=config["host"],
                    port=config["port"],
                    database=config["database"],
                    user=config["user"],
                    conn=conn
                )
                return False, None, f"工作区ID {ws_id} 已存在关联的令牌"
            
            # 设置令牌参数
            is_unlimited = token_type == "unlimited"
            remaining_calls = None if is_unlimited else total_calls
            total_calls_value = None if is_unlimited else total_calls
            
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
                    email,
                    ws_id,
                    token_type,
                    remaining_calls,
                    total_calls_value,
                )
            )
            
            token_id = cursor.fetchone()[0]
            conn.commit()
        
        # 返回创建的令牌信息
        token_info = {
            "token_id": token_id,
            "access_token": access_token,
            "ws_id": ws_id,
            "token_type": token_type,
            "remaining_calls": remaining_calls,
            "total_calls": total_calls_value
        }
        
        postgresql_pool.release_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            conn=conn
        )
        
        return True, token_info, None
        
    except Exception as e:
        logger.error(f"创建令牌失败: {str(e)}")
        return False, None, f"服务器内部错误: {str(e)}"

async def update_auth_token(ws_id: str, operation: str, calls_value: Optional[int] = None) -> Tuple[bool, Optional[Dict[str, Any]], Optional[str]]:
    """
    更新令牌使用次数
    
    Args:
        ws_id: 工作区ID
        operation: 操作类型 (add=增加次数, set=设置次数, unlimited=设置为无限制)
        calls_value: 对于add和set操作，指定调用次数值
        
    Returns:
        (成功标志, 更新后的令牌信息, 错误信息)
    """
    try:
        # 获取数据库连接
        config = auth_db_config
        conn, error = postgresql_pool.get_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            password=config["password"]
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return False, None, f"数据库连接失败: {error}"
        
        # 检查工作区ID是否存在令牌
        with conn.cursor(cursor_factory=DictCursor) as cursor:
            cursor.execute(
                "SELECT * FROM api_tokens WHERE ws_id = %s",
                (ws_id,)
            )
            token_record = cursor.fetchone()
            
            if not token_record:
                postgresql_pool.release_connection(
                    host=config["host"],
                    port=config["port"],
                    database=config["database"],
                    user=config["user"],
                    conn=conn
                )
                return False, None, f"未找到工作区ID {ws_id} 关联的令牌"
            
            # 执行不同的更新操作
            if operation == "unlimited":
                # 设置为无限制使用
                cursor.execute(
                    """
                    UPDATE api_tokens 
                    SET token_type = 'unlimited', remaining_calls = NULL, total_calls = NULL, updated_at = CURRENT_TIMESTAMP
                    WHERE ws_id = %s
                    RETURNING token_id, ws_id, token_type, remaining_calls, total_calls
                    """,
                    (ws_id,)
                )
            elif operation == "add":
                # 增加指定次数
                if not calls_value or calls_value <= 0:
                    return False, None, "增加操作需要正数的调用次数值"
                
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
                    (calls_value, calls_value, ws_id)
                )
            elif operation == "set":
                # 设置为指定次数
                if not calls_value or calls_value < 0:
                    return False, None, "设置操作需要非负的调用次数值"
                
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
                    (calls_value, calls_value, ws_id)
                )
            else:
                return False, None, f"不支持的操作类型: {operation}"
            
            updated_token = cursor.fetchone()
            conn.commit()
        
        # 返回更新后的令牌信息
        token_info = {
            "token_id": updated_token["token_id"],
            "ws_id": updated_token["ws_id"],
            "token_type": updated_token["token_type"],
            "remaining_calls": updated_token["remaining_calls"],
            "total_calls": updated_token["total_calls"]
        }
        
        postgresql_pool.release_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            conn=conn
        )
        
        return True, token_info, None
        
    except Exception as e:
        logger.error(f"更新令牌失败: {str(e)}")
        return False, None, f"服务器内部错误: {str(e)}"

async def get_auth_token_info(ws_id: str) -> Tuple[bool, Optional[Dict[str, Any]], Optional[str]]:
    """
    查询令牌信息
    
    Args:
        ws_id: 工作区ID
        
    Returns:
        (成功标志, 令牌信息, 错误信息)
    """
    try:
        # 获取数据库连接
        config = auth_db_config
        conn, error = postgresql_pool.get_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            password=config["password"]
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return False, None, f"数据库连接失败: {error}"
        
        # 查询令牌信息
        with conn.cursor(cursor_factory=DictCursor) as cursor:
            cursor.execute(
                "SELECT * FROM api_tokens WHERE ws_id = %s",
                (ws_id,)
            )
            token_record = cursor.fetchone()
            
            if not token_record:
                postgresql_pool.release_connection(
                    host=config["host"],
                    port=config["port"],
                    database=config["database"],
                    user=config["user"],
                    conn=conn
                )
                return False, None, f"未找到工作区ID {ws_id} 关联的令牌"
            
            # 转换为字典
            token_info = dict(token_record)
            
            # 确保 datetime 对象正确序列化
            for field in ["created_at", "updated_at"]:
                if field in token_info and token_info[field]:
                    token_info[field] = token_info[field].isoformat()
        
        postgresql_pool.release_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            conn=conn
        )
        
        return True, token_info, None
        
    except Exception as e:
        logger.error(f"查询令牌信息失败: {str(e)}")
        return False, None, f"服务器内部错误: {str(e)}"

async def validate_auth_token(access_token: str) -> Tuple[bool, Optional[Dict[str, Any]]]:
    """
    验证访问令牌是否有效
    
    Args:
        access_token: 访问令牌
        
    Returns:
        (有效标志, 令牌信息)
    """
    try:
        # 获取数据库连接
        config = auth_db_config
        conn, error = postgresql_pool.get_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            password=config["password"]
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return False, None
        
        # 查询令牌信息
        with conn.cursor(cursor_factory=DictCursor) as cursor:
            cursor.execute(
                "SELECT * FROM api_tokens WHERE access_token = %s AND is_active = TRUE",
                (access_token,)
            )
            token_record = cursor.fetchone()
        
        postgresql_pool.release_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            conn=conn
        )
        
        if not token_record:
            return False, None
        
        # 转换为字典
        token_info = dict(token_record)
        return True, token_info
        
    except Exception as e:
        logger.error(f"令牌验证过程中发生错误: {str(e)}")
        return False, None

async def update_token_usage(token_id: int, ws_id: str, operation_type: str, 
                            target_database: str, target_collection: Optional[str], 
                            status: str, request_details: Optional[Dict[str, Any]]) -> bool:
    """
    更新令牌使用次数并记录使用日志
    
    Args:
        token_id: 令牌ID
        ws_id: 工作区ID
        operation_type: 操作类型
        target_database: 目标数据库
        target_collection: 目标集合/表
        status: 状态 (success 或 error)
        request_details: 请求详情
        
    Returns:
        操作是否成功
    """
    # 确保输入参数不为None
    target_database = target_database or ""
    target_collection = target_collection or ""
    
    # 减少日志，只在DEBUG级别记录详细信息
    logger.debug(f"执行令牌使用记录: token_id={token_id}, operation={operation_type}")
    
    try:
        # 获取数据库连接
        config = auth_db_config
        conn, error = postgresql_pool.get_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            password=config["password"]
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return False
        
        # 保存原始自动提交状态
        original_autocommit = conn.autocommit
        new_remaining = None
        
        try:
            # 设置为手动提交模式
            conn.autocommit = False
            
            with conn.cursor() as cursor:
                # 1. 获取当前令牌类型和剩余次数
                cursor.execute(
                    "SELECT token_type, remaining_calls FROM api_tokens WHERE token_id = %s FOR UPDATE",
                    (token_id,)
                )
                result = cursor.fetchone()
                if not result:
                    logger.error(f"未找到令牌信息: token_id={token_id}")
                    conn.rollback()
                    return False
                
                token_type, remaining_calls = result
                logger.debug(f"令牌信息: token_id={token_id}, token_type={token_type}, remaining_calls={remaining_calls}")
                
                # 2. 更新令牌使用次数（如果是limited类型）
                new_remaining = remaining_calls
                if token_type == "limited":
                    cursor.execute(
                        """
                        UPDATE api_tokens 
                        SET remaining_calls = GREATEST(remaining_calls - 1, 0), 
                            updated_at = CURRENT_TIMESTAMP 
                        WHERE token_id = %s 
                        RETURNING remaining_calls
                        """,
                        (token_id,)
                    )
                    new_remaining = cursor.fetchone()[0]
                    
                    # 只记录类型为limited且实际减少次数时的信息
                    if remaining_calls != new_remaining:
                        logger.info(f"令牌使用次数更新: token_id={token_id}, remaining_calls={new_remaining}")
                
                # 3. 记录使用日志（对所有类型令牌都记录）
                request_json = None
                if request_details:
                    try:
                        request_json = json.dumps(request_details)
                    except:
                        logger.error("无法序列化请求详情为JSON")
                        request_json = json.dumps({"error": "无法序列化原始请求详情"})
                
                cursor.execute(
                    """
                    INSERT INTO token_usage_logs 
                    (token_id, ws_id, operation_type, target_database, target_collection, status, request_details)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    RETURNING log_id
                    """,
                    (
                        token_id,
                        ws_id,
                        operation_type,
                        target_database,
                        target_collection,
                        status,
                        request_json
                    )
                )
                
                log_id = cursor.fetchone()[0]
                logger.debug(f"令牌使用记录已创建: log_id={log_id}, token_id={token_id}, token_type={token_type}")
                
                # 显式提交事务
                conn.commit()
            
            return True
            
        except Exception as e:
            # 回滚事务
            conn.rollback()
            logger.error(f"更新令牌使用信息失败: {str(e)}")
            return False
            
        finally:
            # 恢复原始自动提交状态
            conn.autocommit = original_autocommit
            # 归还连接到连接池
            postgresql_pool.release_connection(
                host=config["host"],
                port=config["port"],
                database=config["database"],
                user=config["user"],
                conn=conn
            )
        
    except Exception as e:
        logger.error(f"处理令牌使用记录时发生错误: {str(e)}")
        return False

async def cleanup_token_usage_logs() -> Tuple[bool, Optional[str]]:
    """
    清理旧的令牌使用日志记录
    
    保留最近1个月的日志，删除更早的记录
    
    Returns:
        (成功标志, 错误信息)
    """
    try:
        # 获取数据库连接
        config = auth_db_config
        conn, error = postgresql_pool.get_connection(
            host=config["host"],
            port=config["port"],
            database=config["database"],
            user=config["user"],
            password=config["password"]
        )
        
        if error:
            logger.error(f"数据库连接失败: {error}")
            return False, f"数据库连接失败: {error}"
        
        try:
            # 查询1个月前的日志数量，用于记录
            with conn.cursor() as cursor:
                cursor.execute(
                    "SELECT COUNT(*) FROM token_usage_logs WHERE created_at < NOW() - INTERVAL '1 month'"
                )
                count = cursor.fetchone()[0]
            
            if count == 0:
                logger.debug("没有需要清理的旧日志记录")
                return True, None
            
            # 删除1个月前的日志记录
            with conn.cursor() as cursor:
                cursor.execute(
                    "DELETE FROM token_usage_logs WHERE created_at < NOW() - INTERVAL '1 month'"
                )
                conn.commit()
                
            logger.info(f"已清理 {count} 条1个月前的令牌使用日志")
            return True, None
            
        except Exception as e:
            logger.error(f"清理令牌使用日志失败: {str(e)}")
            return False, f"清理令牌使用日志失败: {str(e)}"
            
        finally:
            # 归还连接到连接池
            postgresql_pool.release_connection(
                host=config["host"],
                port=config["port"],
                database=config["database"],
                user=config["user"],
                conn=conn
            )
        
    except Exception as e:
        logger.error(f"清理令牌使用日志过程中发生错误: {str(e)}")
        return False, str(e) 