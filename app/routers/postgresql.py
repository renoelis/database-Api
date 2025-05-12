from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel, Field, validator
from typing import Dict, Any, List, Optional, Union
import logging
import psycopg2
from psycopg2.extras import RealDictCursor
import psycopg2.errors
import re
from app.utils.response import success_response, error_response
from app.utils.pool import postgresql_pool

router = APIRouter(tags=["PostgreSQL"])
logger = logging.getLogger("database-api")

class PostgreSQLConnectionInfo(BaseModel):
    host: str
    port: int = 5432
    database: str
    user: str
    password: str
    sslmode: Optional[str] = "prefer"
    connect_timeout: Optional[int] = 30

class PostgreSQLExecuteRequest(BaseModel):
    connection: PostgreSQLConnectionInfo
    sql: str
    parameters: Optional[Union[List[Any], Dict[str, Any]]] = []

    @validator('sql')
    def validate_sql(cls, v):
        """基本的SQL语法验证"""
        if not v or not v.strip():
            raise ValueError("SQL语句不能为空")
        
        # 检查常见SQL语法错误
        sql_lower = v.lower().strip()
        
        # 检查SELECT语句是否有FROM子句
        if sql_lower.startswith('select'):
            if ' from ' not in sql_lower:
                raise ValueError("SELECT语句缺少FROM子句")
            
            # 检查是否指定了列
            select_from_parts = sql_lower.split(' from ')[0]
            if select_from_parts == 'select' or select_from_parts.strip() == 'select':
                raise ValueError("SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
            
            # 检查select和from之间是否有非空白字符
            select_part_match = re.match(r'select\s+from', sql_lower)
            if select_part_match:
                raise ValueError("SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
            
        # 检查基本的括号匹配
        if v.count('(') != v.count(')'):
            raise ValueError("SQL语句括号不匹配")
            
        # 检查WHERE后是否有条件
        where_pattern = re.compile(r'where\s+;', re.IGNORECASE)
        if where_pattern.search(v):
            raise ValueError("WHERE子句后缺少条件")
            
        # 检查常见的拼写错误
        common_typos = {
            r'\bslect\b': 'select', 
            r'\bform\b': 'from',
            r'\bwhere\s+and\b': 'where',
            r'\bgroup\s+order\b': 'group by ... order'
        }
        
        for typo, correction in common_typos.items():
            if re.search(typo, sql_lower):
                raise ValueError(f"SQL语句可能存在拼写错误: 检查 '{correction}'")
                
        return v

@router.post("/postgresql")
async def execute_postgresql(request: PostgreSQLExecuteRequest):
    """
    执行PostgreSQL SQL语句
    
    - 支持查询、插入、更新、删除操作
    - 返回查询结果或影响的行数
    """
    conn = None
    try:
        # 额外的SQL语法验证
        sql_lower = request.sql.lower().strip()
        
        # 特别检查SELECT语句是否包含列指定
        if sql_lower.startswith('select'):
            select_parts = sql_lower.split(' from ')[0].strip()
            if select_parts == 'select' or re.match(r'select\s*$', select_parts):
                return error_response(1005, "SQL验证错误: SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列")
        
        # 从连接池获取连接
        conn, error = postgresql_pool.get_connection(
            host=request.connection.host,
            port=request.connection.port,
            database=request.connection.database,
            user=request.connection.user,
            password=request.connection.password,
            sslmode=request.connection.sslmode,
            connect_timeout=request.connection.connect_timeout
        )
        
        if error:
            logger.error(f"从连接池获取连接失败: {error}")
            return error_response(1001, f"数据库连接错误: {error}")
        
        logger.info(f"成功从连接池获取连接: {request.connection.host}:{request.connection.port}/{request.connection.database}")
        
        # 创建游标，使用RealDictCursor返回字典格式结果
        with conn.cursor(cursor_factory=RealDictCursor) as cursor:
            # 执行SQL
            logger.info(f"执行SQL: {request.sql}")
            
            try:
                cursor.execute(request.sql, request.parameters)
            except psycopg2.ProgrammingError as e:
                # 捕获SQL执行错误，提供更详细的错误信息
                error_msg = str(e)
                
                # 尝试提取更有用的错误信息
                if "column" in error_msg and "does not exist" in error_msg:
                    match = re.search(r'column "([^"]+)" does not exist', error_msg)
                    if match:
                        column = match.group(1)
                        error_msg = f"列 '{column}' 不存在，请检查列名是否正确或表是否存在此列"
                elif "missing FROM-clause" in error_msg:
                    error_msg = "SQL语句缺少FROM子句，SELECT语句需要指定查询的表"
                elif "syntax error at or near" in error_msg:
                    match = re.search(r'syntax error at or near "([^"]+)"', error_msg)
                    if match:
                        problematic_part = match.group(1)
                        error_msg = f"SQL语法错误，问题出现在: '{problematic_part}'"
                elif "SELECT列表中的表达式" in error_msg or "column reference" in error_msg or "no columns specified" in error_msg:
                    error_msg = "SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列"
                
                logger.error(f"SQL执行错误: {error_msg}")
                return error_response(1003, f"SQL执行错误: {error_msg}")
            
            # 检查结果是否为空
            sql_type = request.sql.strip().upper().split(None, 1)[0]
            
            if sql_type in ("SELECT", "WITH", "SHOW", "EXPLAIN"):
                # 查询操作，返回结果集
                result = cursor.fetchall()
                
                # 将结果转换为普通字典列表
                result_list = []
                
                for row in result:
                    row_dict = {}
                    # RealDictRow实际上是类似字典的对象，可以直接迭代其key-value对
                    for key in row.keys():
                        value = row[key]
                        # 处理特殊数据类型
                        if hasattr(value, 'isoformat'):  # 日期/时间类型
                            row_dict[key] = value.isoformat()
                        elif isinstance(value, (int, float, str, bool, type(None))):
                            row_dict[key] = value
                        else:
                            row_dict[key] = str(value)  # 其他类型转为字符串
                    
                    result_list.append(row_dict)
                
                # 检查结果是否为空
                if not result_list:
                    logger.info("SQL查询未返回任何结果")
                
                return success_response(result_list)
            else:
                # 非查询操作，提交事务并返回影响行数
                conn.commit()
                affected_rows = cursor.rowcount
                
                # 检查影响的行数
                if affected_rows == 0:
                    logger.info("SQL操作未影响任何行")
                
                return success_response(affected_rows)
                
    except psycopg2.OperationalError as e:
        logger.error(f"PostgreSQL连接错误: {str(e)}")
        return error_response(1001, f"数据库连接错误: {str(e)}")
    except psycopg2.errors.UndefinedTable as e:
        # 提取表名
        error_msg = str(e)
        match = re.search(r'relation "([^"]+)" does not exist', error_msg)
        if match:
            table_name = match.group(1)
            error_msg = f"表 '{table_name}' 不存在"
        
        logger.error(f"表不存在: {error_msg}")
        return error_response(1002, f"表不存在: {error_msg}")
    except psycopg2.errors.SyntaxError as e:
        error_msg = str(e)
        
        # 特殊处理SELECT语句缺少列
        if "SELECT" in error_msg and ("column reference" in error_msg or "target lists" in error_msg):
            error_msg = "SELECT语句缺少列名，请指定要查询的列或使用 SELECT * 查询所有列"
        
        logger.error(f"SQL语法错误: {error_msg}")
        return error_response(1003, f"SQL语法错误: {error_msg}")
    except psycopg2.errors.InFailedSqlTransaction as e:
        logger.error(f"SQL事务失败: {str(e)}")
        if conn:
            conn.rollback()
        return error_response(1004, f"SQL事务失败: {str(e)}")
    except psycopg2.Error as e:
        logger.error(f"PostgreSQL错误: {str(e)}")
        if conn:
            conn.rollback()
        return error_response(1000, f"数据库错误: {str(e)}")
    except ValueError as e:
        # 处理SQL验证错误
        logger.error(f"SQL验证错误: {str(e)}")
        return error_response(1005, f"SQL验证错误: {str(e)}")
    except Exception as e:
        logger.error(f"未预期的错误: {str(e)}")
        if conn:
            conn.rollback()
        return error_response(9999, f"服务器错误: {str(e)}")
    finally:
        # 释放连接回连接池，而不是关闭连接
        if conn:
            try:
                postgresql_pool.release_connection(
                    host=request.connection.host, 
                    port=request.connection.port, 
                    database=request.connection.database, 
                    user=request.connection.user, 
                    conn=conn
                )
                logger.info("数据库连接已归还到连接池")
            except Exception as e:
                logger.error(f"归还连接到连接池失败: {str(e)}")
                try:
                    conn.close()
                    logger.info("无法归还到连接池，已关闭连接")
                except:
                    pass 