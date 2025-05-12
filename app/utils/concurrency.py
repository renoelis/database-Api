import asyncio
import logging
from fastapi import Request
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.types import ASGIApp

logger = logging.getLogger("database-api")

class ConcurrencyLimiterMiddleware(BaseHTTPMiddleware):
    """
    并发请求限制中间件
    
    用于控制同时处理的请求数量，防止过多并发请求导致服务器过载
    """
    
    def __init__(
        self, 
        app: ASGIApp, 
        max_concurrent_requests: int = 20,
        postgresql_max_concurrent: int = 10,
        mongodb_max_concurrent: int = 10
    ):
        """
        初始化并发限制器
        
        Args:
            app: FastAPI应用实例
            max_concurrent_requests: 最大并发请求数
            postgresql_max_concurrent: PostgreSQL最大并发请求数
            mongodb_max_concurrent: MongoDB最大并发请求数
        """
        super().__init__(app)
        self.semaphore = asyncio.Semaphore(max_concurrent_requests)
        self.postgresql_semaphore = asyncio.Semaphore(postgresql_max_concurrent)
        self.mongodb_semaphore = asyncio.Semaphore(mongodb_max_concurrent)
        self.max_concurrent_requests = max_concurrent_requests
        self.postgresql_max_concurrent = postgresql_max_concurrent
        self.mongodb_max_concurrent = mongodb_max_concurrent
        
        # 计数器
        self.current_requests = 0
        self.current_postgresql_requests = 0
        self.current_mongodb_requests = 0
        
        logger.info(f"初始化并发限制中间件: 总并发={max_concurrent_requests}, PostgreSQL={postgresql_max_concurrent}, MongoDB={mongodb_max_concurrent}")
        
    async def dispatch(self, request: Request, call_next):
        """处理请求的中间件方法"""
        path = request.url.path
        
        # 根据路径选择合适的信号量
        if "/postgresql" in path:
            semaphore = self.postgresql_semaphore
            async with semaphore:
                self.current_postgresql_requests += 1
                try:
                    logger.debug(f"PostgreSQL当前并发请求数: {self.current_postgresql_requests}/{self.postgresql_max_concurrent}")
                    return await self._process_request(request, call_next)
                finally:
                    self.current_postgresql_requests -= 1
        elif "/mongodb" in path:
            semaphore = self.mongodb_semaphore
            async with semaphore:
                self.current_mongodb_requests += 1
                try:
                    logger.debug(f"MongoDB当前并发请求数: {self.current_mongodb_requests}/{self.mongodb_max_concurrent}")
                    return await self._process_request(request, call_next)
                finally:
                    self.current_mongodb_requests -= 1
        else:
            # 其他请求使用通用信号量
            async with self.semaphore:
                self.current_requests += 1
                try:
                    logger.debug(f"当前并发请求数: {self.current_requests}/{self.max_concurrent_requests}")
                    return await call_next(request)
                finally:
                    self.current_requests -= 1
    
    async def _process_request(self, request: Request, call_next):
        """处理请求并记录执行时间"""
        # 记录请求开始时间
        import time
        start_time = time.time()
        
        # 处理请求
        response = await call_next(request)
        
        # 计算执行时间
        execution_time = time.time() - start_time
        logger.info(f"请求 {request.url.path} 执行时间: {execution_time:.3f}秒")
        
        return response 