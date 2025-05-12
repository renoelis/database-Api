from typing import Any, Optional, Dict, List, Union

def success_response(data: Union[List[Dict[str, Any]], int]) -> Dict[str, Any]:
    """
    成功响应格式化
    
    Args:
        data: 查询结果列表或受影响的行数
        
    Returns:
        统一格式的响应字典
    """
    return {
        "errCode": 0,
        "data": data,
        "errMsg": None
    }

def error_response(error_code: int, error_message: str) -> Dict[str, Any]:
    """
    错误响应格式化
    
    Args:
        error_code: 错误代码
        error_message: 错误信息
        
    Returns:
        统一格式的响应字典
    """
    return {
        "errCode": error_code,
        "data": None,
        "errMsg": error_message
    } 