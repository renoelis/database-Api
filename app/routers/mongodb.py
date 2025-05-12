from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field, validator
from typing import Dict, Any, List, Optional, Union
import logging
import json
from bson import json_util, ObjectId
import pymongo
from pymongo.errors import PyMongoError
import re
from app.utils.response import success_response, error_response
from app.utils.pool import mongodb_pool

router = APIRouter(tags=["MongoDB"])
logger = logging.getLogger("database-api")

class MongoDBConnectionInfo(BaseModel):
    host: str
    port: int = 27017
    database: str
    username: Optional[str] = None
    password: Optional[str] = None
    auth_source: Optional[str] = "admin"
    connect_timeout_ms: int = 30000

    @validator('host')
    def validate_host(cls, v):
        if not v or not v.strip():
            raise ValueError("MongoDB主机地址不能为空")
        return v

    @validator('database')
    def validate_database(cls, v):
        if not v or not v.strip():
            raise ValueError("MongoDB数据库名不能为空")
        return v

class MongoDBExecuteRequest(BaseModel):
    connection: MongoDBConnectionInfo
    collection: str
    operation: str
    filter: Optional[Dict[str, Any]] = None
    update: Optional[Dict[str, Any]] = None
    document: Optional[Dict[str, Any]] = None
    documents: Optional[List[Dict[str, Any]]] = None
    projection: Optional[Dict[str, Any]] = None
    sort: Optional[List[tuple]] = None
    limit: Optional[int] = None
    skip: Optional[int] = None
    pipeline: Optional[List[Dict[str, Any]]] = None

    @validator('collection')
    def validate_collection(cls, v):
        if not v or not v.strip():
            raise ValueError("MongoDB集合名不能为空")
        return v

    @validator('operation')
    def validate_operation(cls, v):
        if not v or not v.strip():
            raise ValueError("MongoDB操作类型不能为空")
        
        valid_operations = [
            'find', 'findone', 'insert', 'insertmany', 
            'update', 'updatemany', 'delete', 'deletemany',
            'aggregate', 'count'
        ]
        
        if v.lower() not in valid_operations:
            raise ValueError(f"不支持的操作类型: {v}。支持的操作类型有: {', '.join(valid_operations)}")
        
        return v.lower()

# 将MongoDB的BSON对象转换为JSON可序列化的格式
def parse_json(data):
    return json.loads(json_util.dumps(data))

@router.post("/mongodb")
async def execute_mongodb(request: MongoDBExecuteRequest):
    """
    执行MongoDB原生操作
    
    - 支持find, findOne, insert, insertMany, update, updateMany, delete, deleteMany等操作
    - 返回查询结果或操作影响的文档数量
    """
    client = None
    try:
        # 验证操作特定的参数
        operation = request.operation.lower()
        
        # 验证查询操作所需参数
        if operation in ["find", "findone"]:
            if request.filter is None:
                logger.warning("查询操作未提供filter参数，将使用空过滤条件")
        
        # 验证插入操作所需参数
        if operation == "insert" and not request.document:
            return error_response(1101, "插入操作缺少document字段，请提供要插入的文档")
            
        if operation == "insertmany" and not request.documents:
            return error_response(1102, "批量插入操作缺少documents字段，请提供要插入的文档列表")
        
        # 验证更新操作所需参数    
        if operation in ["update", "updatemany"]:
            if not request.filter:
                return error_response(1103, "更新操作缺少filter字段，请提供查询条件")
            if not request.update:
                return error_response(1104, "更新操作缺少update字段，请提供更新内容")
                
            # 检查更新操作是否包含操作符
            if request.update and not any(key.startswith('$') for key in request.update.keys()):
                return error_response(1105, "更新操作的update字段格式不正确，应包含至少一个更新操作符如 $set, $unset 等")
        
        # 验证删除操作所需参数
        if operation in ["delete", "deletemany"] and not request.filter:
            return error_response(1106, "删除操作缺少filter字段，请提供查询条件")
            
        # 验证聚合操作所需参数
        if operation == "aggregate" and not request.pipeline:
            return error_response(1107, "聚合操作缺少pipeline字段，请提供聚合管道")
        
        # 从连接池获取MongoDB客户端
        client, error = mongodb_pool.get_client(
            host=request.connection.host,
            port=request.connection.port,
            database=request.connection.database, 
            username=request.connection.username,
            password=request.connection.password,
            auth_source=request.connection.auth_source,
            connect_timeout_ms=request.connection.connect_timeout_ms
        )
        
        if error:
            logger.error(f"从连接池获取MongoDB客户端失败: {error}")
            error_msg = error
            # 提供更友好的连接错误消息
            if "timed out" in error_msg:
                error_msg = f"连接MongoDB服务器超时，请检查主机地址和端口是否正确: {request.connection.host}:{request.connection.port}"
            elif "not authorized" in error_msg:
                error_msg = "MongoDB认证失败，请检查用户名和密码是否正确"
            elif "Authentication failed" in error_msg:
                error_msg = "MongoDB认证失败，请检查用户名和密码是否正确"
            
            return error_response(1001, f"数据库连接错误: {error_msg}")
            
        logger.info(f"成功从连接池获取MongoDB客户端: {request.connection.host}:{request.connection.port}/{request.connection.database}")
        
        # 获取数据库和集合
        db = client[request.connection.database]
        
        try:
            # 先检查集合是否存在
            collection_exists = request.collection in db.list_collection_names()
            if not collection_exists:
                logger.warning(f"集合不存在: {request.collection}")
                # 所有操作在集合不存在时都返回错误信息
                return error_response(1120, f"集合 '{request.collection}' 不存在")
                
            collection = db[request.collection]
        except Exception as e:
            logger.error(f"获取集合失败: {str(e)}")
            return error_response(1002, f"获取集合 {request.collection} 失败，请检查集合名是否正确")
        
        # 执行操作
        try:
            # 查询操作
            if operation == "find":
                filter_dict = request.filter or {}
                projection_dict = request.projection or None
                cursor = collection.find(filter_dict, projection_dict)
                
                # 应用排序、跳过和限制
                if request.sort:
                    cursor = cursor.sort(request.sort)
                if request.skip:
                    cursor = cursor.skip(request.skip)
                if request.limit:
                    cursor = cursor.limit(request.limit)
                    
                # 获取结果并转换为JSON
                results = list(cursor)
                
                # 检查结果是否为空
                if not results:
                    logger.info(f"查询未返回任何结果: {filter_dict}")
                
                return success_response(parse_json(results))
                
            elif operation == "findone":
                filter_dict = request.filter or {}
                projection_dict = request.projection or None
                result = collection.find_one(filter_dict, projection_dict)
                
                if result:
                    return success_response([parse_json(result)])
                else:
                    logger.info(f"查询未返回任何结果: {filter_dict}")
                    return success_response([])
            
            # 插入操作
            elif operation == "insert":
                # 已在前面验证document存在
                result = collection.insert_one(request.document)
                logger.info(f"成功插入文档，ID: {result.inserted_id}")
                return success_response(1)
                
            elif operation == "insertmany":
                # 已在前面验证documents存在
                result = collection.insert_many(request.documents)
                logger.info(f"成功插入{len(result.inserted_ids)}个文档")
                return success_response(len(result.inserted_ids))
            
            # 更新操作
            elif operation == "update":
                # 已在前面验证filter和update存在
                result = collection.update_one(request.filter, request.update)
                
                if result.modified_count == 0:
                    logger.info(f"更新操作未影响任何文档: {request.filter}")
                else:
                    logger.info(f"成功更新{result.modified_count}个文档")
                
                return success_response(result.modified_count)
                
            elif operation == "updatemany":
                # 已在前面验证filter和update存在
                result = collection.update_many(request.filter, request.update)
                
                if result.modified_count == 0:
                    logger.info(f"批量更新操作未影响任何文档: {request.filter}")
                else:
                    logger.info(f"成功更新{result.modified_count}个文档")
                
                return success_response(result.modified_count)
            
            # 删除操作
            elif operation == "delete":
                # 已在前面验证filter存在
                result = collection.delete_one(request.filter)
                
                if result.deleted_count == 0:
                    logger.info(f"删除操作未影响任何文档: {request.filter}")
                else:
                    logger.info(f"成功删除{result.deleted_count}个文档")
                
                return success_response(result.deleted_count)
                
            elif operation == "deletemany":
                # 已在前面验证filter存在
                result = collection.delete_many(request.filter)
                
                if result.deleted_count == 0:
                    logger.info(f"批量删除操作未影响任何文档: {request.filter}")
                else:
                    logger.info(f"成功删除{result.deleted_count}个文档")
                
                return success_response(result.deleted_count)
                
            # 聚合操作
            elif operation == "aggregate":
                # 已在前面验证pipeline存在
                try:
                    results = list(collection.aggregate(request.pipeline))
                    
                    if not results:
                        logger.info("聚合操作未返回任何结果")
                    
                    return success_response(parse_json(results))
                except pymongo.errors.OperationFailure as e:
                    error_msg = str(e)
                    
                    if "pipeline" in error_msg:
                        error_msg = f"聚合管道格式错误: {error_msg}"
                    elif "unknown operator" in error_msg:
                        match = re.search(r'unknown operator: ([^\s]+)', error_msg)
                        if match:
                            op = match.group(1)
                            error_msg = f"聚合管道中使用了未知操作符: {op}"
                    elif "accumulator" in error_msg:
                        error_msg = f"聚合累加器错误: {error_msg}"
                    
                    logger.error(f"聚合操作失败: {error_msg}")
                    return error_response(1108, f"聚合操作失败: {error_msg}")
                
            # 计数操作
            elif operation == "count":
                filter_dict = request.filter or {}
                count = collection.count_documents(filter_dict)
                return success_response(count)
            
            else:
                # 虽然前面已验证，但保留兜底处理
                return error_response(1109, f"不支持的操作类型: {request.operation}")
                
        except pymongo.errors.DuplicateKeyError as e:
            logger.error(f"MongoDB插入重复键错误: {str(e)}")
            # 尝试提取重复键的字段名
            error_msg = str(e)
            match = re.search(r'duplicate key error .* key: \{ ([^:]+)', error_msg)
            field = match.group(1).strip() if match else "未知字段"
            return error_response(1110, f"插入操作失败，文档中的 {field} 与已有文档重复")
            
        except pymongo.errors.BulkWriteError as e:
            logger.error(f"MongoDB批量写入错误: {str(e)}")
            return error_response(1111, f"批量操作失败: {str(e)}")
            
        except pymongo.errors.WriteError as e:
            logger.error(f"MongoDB写入错误: {str(e)}")
            return error_response(1112, f"写入操作失败: {str(e)}")
            
        except pymongo.errors.InvalidOperation as e:
            logger.error(f"MongoDB无效操作: {str(e)}")
            error_msg = str(e)
            if "cannot use an empty filter" in error_msg:
                error_msg = "不能使用空的过滤条件，请提供查询条件"
            return error_response(1113, f"无效操作: {error_msg}")
            
        except pymongo.errors.DocumentTooLarge as e:
            logger.error(f"MongoDB文档过大: {str(e)}")
            return error_response(1114, "文档大小超过MongoDB限制")
            
        except Exception as e:
            logger.error(f"MongoDB操作执行错误: {str(e)}")
            return error_response(1115, f"操作执行错误: {str(e)}")
                
    except pymongo.errors.OperationFailure as e:
        logger.error(f"MongoDB操作失败: {str(e)}")
        error_msg = str(e)
        
        if "authentication failed" in error_msg.lower():
            error_msg = "MongoDB认证失败，请检查用户名和密码是否正确"
        elif "not authorized" in error_msg.lower():
            error_msg = "没有操作权限，请检查用户是否有足够的权限"
        
        return error_response(1002, f"数据库操作错误: {error_msg}")
        
    except PyMongoError as e:
        logger.error(f"MongoDB错误: {str(e)}")
        return error_response(1000, f"数据库错误: {str(e)}")
        
    except ValueError as e:
        logger.error(f"参数验证错误: {str(e)}")
        return error_response(1116, f"参数验证错误: {str(e)}")
        
    except Exception as e:
        logger.error(f"未预期的错误: {str(e)}")
        return error_response(9999, f"服务器错误: {str(e)}")
        
    finally:
        # 注意：连接池模式下不需要关闭MongoDB客户端连接
        # 客户端连接会被池管理器自动管理和重用
        logger.info("MongoDB操作完成") 