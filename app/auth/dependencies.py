import logging
from fastapi import Depends, HTTPException, Request
from typing import Optional, Dict, Any

logger = logging.getLogger("database-api")

async def get_current_token_info(request: Request) -> Dict[str, Any]:
    """
    从请求中获取当前令牌信息
    可在需要令牌信息的路由中作为依赖使用
    
    例如:
    
    @router.get("/example")
    async def example_endpoint(token_info: Dict[str, Any] = Depends(get_current_token_info)):
        return {"token_wsid": token_info["ws_id"]}
    """
    if not hasattr(request.state, "token_info"):
        raise HTTPException(status_code=401, detail="未提供有效的访问令牌")
    return request.state.token_info 