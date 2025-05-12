import logging
import time
import threading
from typing import Dict, Any, Optional, List, Tuple
import psycopg2
from psycopg2.pool import ThreadedConnectionPool
from psycopg2.extras import RealDictCursor
import pymongo
from pymongo.mongo_client import MongoClient

logger = logging.getLogger("database-api")

class PostgreSQLConnectionPool:
    """PostgreSQL连接池管理器，使用ThreadedConnectionPool实现"""
    
    _instance = None
    _lock = threading.Lock()
    _pools: Dict[str, ThreadedConnectionPool] = {}
    
    def __new__(cls):
        with cls._lock:
            if cls._instance is None:
                cls._instance = super(PostgreSQLConnectionPool, cls).__new__(cls)
                cls._instance._pools = {}
                # 添加清理线程
                cleanup_thread = threading.Thread(target=cls._instance._cleanup_idle_pools, daemon=True)
                cleanup_thread.start()
            return cls._instance
    
    def get_connection(self, host: str, port: int, database: str, user: str, 
                       password: str, sslmode: str = "prefer", connect_timeout: int = 30) -> Tuple[Any, str]:
        """获取数据库连接，如果连接池不存在则创建新的连接池"""
        pool_key = f"{host}:{port}:{database}:{user}"
        
        # 检查连接池是否存在，不存在则创建
        if pool_key not in self._pools:
            with self._lock:
                if pool_key not in self._pools:
                    try:
                        # 创建新的连接池
                        logger.info(f"创建PostgreSQL连接池: {host}:{port}/{database}")
                        dsn = f"host={host} port={port} dbname={database} user={user} password={password} sslmode={sslmode} connect_timeout={connect_timeout}"
                        pool = ThreadedConnectionPool(
                            minconn=1,  # 最小连接数，设为1保持至少一个连接
                            maxconn=30,  # 最大连接数，增加到30
                            dsn=dsn
                        )
                        self._pools[pool_key] = {
                            "pool": pool,
                            "last_used": time.time()
                        }
                    except Exception as e:
                        logger.error(f"创建PostgreSQL连接池失败: {str(e)}")
                        return None, str(e)
        
        # 更新最后使用时间
        self._pools[pool_key]["last_used"] = time.time()
        
        try:
            # 从连接池获取连接
            conn = self._pools[pool_key]["pool"].getconn()
            return conn, None
        except Exception as e:
            logger.error(f"从PostgreSQL连接池获取连接失败: {str(e)}")
            return None, str(e)
    
    def release_connection(self, host: str, port: int, database: str, user: str, conn):
        """释放连接回连接池"""
        pool_key = f"{host}:{port}:{database}:{user}"
        if pool_key in self._pools:
            try:
                self._pools[pool_key]["pool"].putconn(conn)
                # 更新最后使用时间
                self._pools[pool_key]["last_used"] = time.time()
            except Exception as e:
                logger.error(f"释放PostgreSQL连接失败: {str(e)}")
                # 如果释放失败，关闭连接
                try:
                    conn.close()
                except:
                    pass
    
    def _cleanup_idle_pools(self):
        """清理长时间未使用的连接池"""
        while True:
            time.sleep(300)  # 每5分钟检查一次
            current_time = time.time()
            pools_to_close = []
            
            # 找出超过10分钟未使用的连接池
            for pool_key, pool_info in self._pools.items():
                if current_time - pool_info["last_used"] > 600:  # 10分钟
                    pools_to_close.append(pool_key)
            
            # 关闭并移除这些连接池
            with self._lock:
                for pool_key in pools_to_close:
                    try:
                        logger.info(f"关闭闲置PostgreSQL连接池: {pool_key}")
                        self._pools[pool_key]["pool"].closeall()
                        del self._pools[pool_key]
                    except Exception as e:
                        logger.error(f"关闭PostgreSQL连接池失败: {str(e)}")


class MongoDBConnectionPool:
    """MongoDB连接池管理器"""
    
    _instance = None
    _lock = threading.Lock()
    _clients: Dict[str, Dict[str, Any]] = {}
    
    def __new__(cls):
        with cls._lock:
            if cls._instance is None:
                cls._instance = super(MongoDBConnectionPool, cls).__new__(cls)
                cls._instance._clients = {}
                # 添加清理线程
                cleanup_thread = threading.Thread(target=cls._instance._cleanup_idle_clients, daemon=True)
                cleanup_thread.start()
            return cls._instance
    
    def get_client(self, host: str, port: int, database: str, 
                  username: Optional[str] = None, password: Optional[str] = None, 
                  auth_source: str = "admin", connect_timeout_ms: int = 30000) -> Tuple[Any, str]:
        """获取MongoDB客户端连接，如果不存在则创建新的客户端"""
        
        client_key = f"{host}:{port}:{database}"
        if username and password:
            client_key += f":{username}"
        
        # 检查客户端是否存在，不存在则创建
        if client_key not in self._clients:
            with self._lock:
                if client_key not in self._clients:
                    try:
                        # 构建连接字符串
                        connection_string = f"mongodb://"
                        if username and password:
                            connection_string += f"{username}:{password}@"
                        connection_string += f"{host}:{port}/{database}"
                        if auth_source:
                            connection_string += f"?authSource={auth_source}"
                        
                        # 连接选项
                        connect_options = {
                            "connectTimeoutMS": connect_timeout_ms,  # 连接超时
                            "serverSelectionTimeoutMS": connect_timeout_ms,  # 服务器选择超时
                            "maxPoolSize": 30,  # 连接池最大连接数，增加到30
                            "minPoolSize": 1,   # 连接池最小连接数，设为1保持至少一个连接
                            "maxIdleTimeMS": 10000,  # 连接最大空闲时间，减少到10000毫秒(10秒)
                            "waitQueueTimeoutMS": 30000  # 等待队列超时时间，设置为30秒
                        }
                        
                        # 创建新的客户端
                        logger.info(f"创建MongoDB客户端连接: {host}:{port}/{database}")
                        client = pymongo.MongoClient(connection_string, **connect_options)
                        
                        # 测试连接
                        client.admin.command('ping')
                        
                        self._clients[client_key] = {
                            "client": client,
                            "last_used": time.time()
                        }
                    except Exception as e:
                        logger.error(f"创建MongoDB客户端连接失败: {str(e)}")
                        return None, str(e)
        
        # 更新最后使用时间
        self._clients[client_key]["last_used"] = time.time()
        
        return self._clients[client_key]["client"], None
    
    def _cleanup_idle_clients(self):
        """清理长时间未使用的客户端连接"""
        while True:
            time.sleep(300)  # 每5分钟检查一次
            current_time = time.time()
            clients_to_close = []
            
            # 找出超过10分钟未使用的客户端
            for client_key, client_info in self._clients.items():
                if current_time - client_info["last_used"] > 600:  # 10分钟
                    clients_to_close.append(client_key)
            
            # 关闭并移除这些客户端
            with self._lock:
                for client_key in clients_to_close:
                    try:
                        logger.info(f"关闭闲置MongoDB客户端: {client_key}")
                        self._clients[client_key]["client"].close()
                        del self._clients[client_key]
                    except Exception as e:
                        logger.error(f"关闭MongoDB客户端失败: {str(e)}")


# 创建连接池单例实例
postgresql_pool = PostgreSQLConnectionPool()
mongodb_pool = MongoDBConnectionPool() 