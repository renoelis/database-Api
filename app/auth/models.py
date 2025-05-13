from pydantic import BaseModel, Field, EmailStr
from typing import Optional, Union, List, Dict, Any
from datetime import datetime


class TokenCreate(BaseModel):
    """创建令牌请求模型"""
    email: str
    ws_id: str
    token_type: str = "limited"  # limited 或 unlimited
    total_calls: Optional[int] = None


class TokenUpdate(BaseModel):
    """更新令牌请求模型"""
    ws_id: str
    operation: str  # add, set, unlimited
    calls_value: Optional[int] = None


class TokenInfo(BaseModel):
    """令牌信息响应模型"""
    token_id: int
    access_token: str
    email: str
    ws_id: str
    token_type: str
    remaining_calls: Optional[int]
    total_calls: Optional[int]
    is_active: bool
    created_at: datetime
    updated_at: datetime


class TokenInfoResponse(BaseModel):
    """令牌信息简化响应模型，用于创建和更新操作"""
    token_id: int
    access_token: Optional[str] = None
    ws_id: str
    token_type: str
    remaining_calls: Optional[int]
    total_calls: Optional[int]


class TokenUsageLog(BaseModel):
    """令牌使用日志模型"""
    token_id: int
    ws_id: str
    operation_type: str
    target_database: str
    target_collection: Optional[str] = None
    status: str
    request_details: Dict[str, Any] 