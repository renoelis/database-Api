# 数据库API服务

这是一个用于连接并操作PostgreSQL和MongoDB数据库的API服务。通过简单的HTTP请求，可以执行数据库查询、插入、更新和删除操作。

## 功能特点

- 支持PostgreSQL数据库的SQL操作
- 支持MongoDB的原生操作
- 统一的响应格式
- 数据库版本兼容性处理
- 容器化部署支持
- **高性能数据库连接池** - 优化数据库连接性能
- **高并发请求控制** - 保护服务器资源不被过载
- **全局统一错误处理** - 确保所有错误响应格式一致
- **自动数据库集合验证** - 验证集合/表是否存在

## 快速开始

### 本地开发

1. 创建并激活虚拟环境:

```bash
python3 -m venv venv
source venv/bin/activate  # Linux/macOS
# 或
venv\Scripts\activate  # Windows
```

2. 安装依赖:

```bash
pip install -r requirements.txt
```

3. 启动服务:

```bash
python run.py
```

服务将在 http://localhost:3010 启动，并可通过Swagger UI访问API文档: http://localhost:3010/docs

### Docker部署

1. 构建Docker镜像:

```bash
docker build -t database-api:latest .
```

2. 运行Docker容器:

```bash
docker run -d -p 3010:3010 --name database-api database-api:latest
```

或使用docker-compose:

```bash
docker-compose -f docker-compose-database-api.yml up -d
```

## API文档

### PostgreSQL API

#### 执行SQL

```
POST /apiDatabase/postgresql
```

请求示例:

```json
{
  "connection": {
    "host": "localhost",
    "port": 5432,
    "database": "your_database",
    "user": "postgres",
    "password": "your_password"
  },
  "sql": "SELECT * FROM your_table LIMIT 10",
  "parameters": []
}
```

### MongoDB API

#### 执行原生操作

```
POST /apiDatabase/mongodb
```

请求示例 (查询):

```json
{
  "connection": {
    "host": "localhost",
    "port": 27017,
    "database": "your_database",
    "username": "mongodb_user",
    "password": "your_password"
  },
  "collection": "your_collection",
  "operation": "find",
  "filter": {"status": "active"},
  "limit": 10
}
```

#### 支持的MongoDB操作

- `find`: 查询文档
- `findone`: 查询单个文档
- `insert`: 插入单个文档
- `insertmany`: 批量插入文档
- `update`: 更新单个文档
- `updatemany`: 批量更新文档
- `delete`: 删除单个文档
- `deletemany`: 批量删除文档
- `aggregate`: 聚合操作
- `count`: 计数操作

#### 分页查询示例

```json
{
  "connection": {
    "host": "localhost",
    "port": 27017,
    "database": "your_database",
    "username": "mongodb_user",
    "password": "your_password"
  },
  "collection": "your_collection",
  "operation": "find",
  "filter": {"status": "active"},
  "sort": [["createdAt", -1]], 
  "skip": 20,  // 跳过前20条
  "limit": 10  // 显示10条
}
```

## 响应格式

所有API都使用统一的响应格式，包括成功和错误情况:

```json
{
  "errCode": 0,          // 0表示成功，其他值表示错误
  "data": [...],         // 查询结果或影响的行数，错误时为null
  "errMsg": null         // 成功为null，失败时包含错误信息
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1000-1099 | 数据库通用错误 |
| 1100-1199 | MongoDB特定错误 |
| 1120 | MongoDB集合不存在 |
| 1400 | 请求参数验证失败 |
| 9999 | 服务器内部错误 |

## 连接池与并发控制

### 连接池管理

本服务实现了高效的数据库连接池，可以降低频繁创建和销毁连接的开销，提高性能：

- **PostgreSQL连接池**：使用`ThreadedConnectionPool`实现，每个不同的连接参数组合创建独立的连接池
- **MongoDB连接池**：使用`MongoClient`内置连接池机制，为每个不同的连接参数组合维护连接池

连接池特性：
- 自动管理连接的创建和复用
- 长时间未使用的连接池会被自动清理（默认10分钟）
- 连接池配置：
  - 最小连接数：1（保持至少一个活跃连接）
  - 最大连接数：30（每个连接池最多30个连接）
  - 空闲清理时间：10分钟
  - 连接超时：30秒

### 并发控制

为了防止服务器资源被大量并发请求耗尽，实现了请求限流机制：

- **全局并发限制**：控制同时处理的所有请求总数（默认200个）
- **PostgreSQL并发限制**：控制同时处理的PostgreSQL请求数（默认100个）
- **MongoDB并发限制**：控制同时处理的MongoDB请求数（默认100个）

可以通过环境变量配置并发限制：
```
MAX_CONCURRENT_REQUESTS=200
POSTGRESQL_MAX_CONCURRENT=100
MONGODB_MAX_CONCURRENT=100
```

## 错误处理

系统实现了全局统一的错误处理机制，确保所有错误响应都遵循相同的格式：

- **HTTP异常**：处理HTTP状态码错误，如404、500等
- **请求验证错误**：处理请求参数验证失败
- **数据库错误**：处理数据库连接、查询等相关错误
- **未捕获异常**：处理所有其他未预期的异常

对于MongoDB操作，系统会验证集合是否存在并提供相应错误信息，避免对不存在的集合进行操作。

## 环境要求

- Python 3.8+
- 支持的PostgreSQL版本: 9.6+
- 支持的MongoDB版本: 4.0+
- 建议服务器规格: 4核CPU, 8GB内存 (大规模部署)
- 小规模部署最低要求: 2核CPU, 2GB内存 

## 令牌认证系统

为了提供安全的API访问，数据库API服务现在集成了基于令牌的认证系统。此系统可以控制API的访问并跟踪使用情况。

### 数据库初始化流程

在首次部署服务时，需要执行以下数据库初始化步骤：

#### 1. 检查并创建数据库

如果需要新建数据库：

```sql
CREATE DATABASE pro_db;
```

如果数据库已存在，直接连接到现有数据库：

```bash
psql -h [host] -p 5432 -U [username] -d pro_db
```

#### 2. 创建令牌认证表

连接到数据库后，创建令牌认证所需的表结构：

```sql
-- 创建API令牌表
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
);

-- 创建令牌使用日志表
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
);
```

#### 3. 启动与验证

创建完表结构后，启动服务并验证表是否创建成功：

```bash
# 启动服务
python run.py

# 验证表是否创建成功
psql -h [host] -p 5432 -U [username] -d pro_db -c "\dt"
```

应当看到`api_tokens`和`token_usage_logs`两个表在输出结果中。

#### 4. 自动初始化功能

为了简化部署流程，服务在启动时会自动检查并创建所需的表结构。该功能是通过在`app/main.py`的启动事件中实现的：

```python
@app.on_event("startup")
async def startup_event():
    """应用启动时执行的事件，初始化连接池和数据库表"""
    logger.info("数据库API服务启动，初始化连接池")
    
    # 初始化数据库表
    try:
        # 创建表结构
        conn, error = postgresql_pool.get_connection(
            host="120.46.147.53",
            port=5432, 
            database="pro_db",
            user="renoelis",
            password="renoelis02@gmail.com"
        )
        
        if not error and conn:
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
                logger.info("数据库表初始化完成")
            
            postgresql_pool.release_connection(conn)
        else:
            logger.error(f"数据库初始化失败: {error}")
    except Exception as e:
        logger.error(f"数据库初始化过程中发生错误: {str(e)}")

### 令牌特性

- **每工作区唯一令牌**: 每个工作区(wsId)只能创建一个唯一的访问令牌
- **按使用次数计费**: 支持有限次数调用和无限次调用两种类型
- **自动扣减次数**: 每次数据库写入操作自动扣减一次调用次数
- **次数管理**: 支持增加、设置和无限制三种次数管理方式

### 令牌API

#### 1. 创建令牌

```
POST /apiDatabase/auth/token
```

请求示例:

```json
{
  "email": "user@example.com",
  "ws_id": "workspace123",
  "token_type": "limited",
  "total_calls": 1000
}
```

响应示例:

```json
{
  "errCode": 0,
  "data": {
    "token_id": 1,
    "access_token": "yZVKdhfL3ajfQ9XcvbnmPzcl5KPJSd8vwMZ9ks7JhSfKkd72",
    "ws_id": "workspace123",
    "token_type": "limited",
    "remaining_calls": 1000,
    "total_calls": 1000
  },
  "errMsg": null
}
```

#### 2. 更新令牌使用次数

```
POST /apiDatabase/auth/token/update
```

请求示例:

```json
{
  "ws_id": "workspace123",
  "operation": "add",
  "calls_value": 500
}
```

支持的操作:
- `add`: 增加指定次数
- `set`: 设置为指定次数
- `unlimited`: 设置为无限制使用

响应示例:

```json
{
  "errCode": 0,
  "data": {
    "token_id": 1,
    "ws_id": "workspace123",
    "token_type": "limited",
    "remaining_calls": 1500,
    "total_calls": 1500
  },
  "errMsg": null
}
```

#### 3. 查询令牌信息

```
GET /apiDatabase/auth/token/info?ws_id=workspace123
```

响应示例:

```json
{
  "errCode": 0,
  "data": {
    "token_id": 1,
    "access_token": "yZVKdhfL3ajfQ9XcvbnmPzcl5KPJSd8vwMZ9ks7JhSfKkd72",
    "email": "user@example.com",
    "ws_id": "workspace123",
    "token_type": "limited",
    "remaining_calls": 995,
    "total_calls": 1000,
    "is_active": true,
    "created_at": "2024-06-20T08:30:00.000Z",
    "updated_at": "2024-06-20T09:15:30.000Z"
  },
  "errMsg": null
}
```

### 令牌使用流程

令牌认证系统的设计目的是控制和管理对数据库API的访问，特别是针对PostgreSQL和MongoDB数据库操作接口的调用权限和使用限额。完整的使用流程如下：

#### 1. 令牌创建与管理

1. 首先创建一个与工作区ID关联的访问令牌：
```bash
curl -X POST http://localhost:3010/apiDatabase/auth/token \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","ws_id":"workspace123","token_type":"limited","total_calls":1000}'
```

2. 系统返回访问令牌，记录下`access_token`的值：
```json
{
  "errCode": 0,
  "data": {
    "token_id": 1,
    "access_token": "yZVKdhfL3ajfQ9XcvbnmPzcl5KPJSd8vwMZ9ks7JhSfKkd72",
    "ws_id": "workspace123",
    "token_type": "limited",
    "remaining_calls": 1000,
    "total_calls": 1000
  },
  "errMsg": null
}
```

#### 2. 使用令牌调用数据库API

使用上一步获取的令牌，可以调用PostgreSQL和MongoDB相关接口：

##### PostgreSQL查询示例：

```bash
curl -X POST http://localhost:3010/apiDatabase/postgresql \
  -H "accessToken: yZVKdhfL3ajfQ9XcvbnmPzcl5KPJSd8vwMZ9ks7JhSfKkd72" \
  -H "Content-Type: application/json" \
  -d '{
    "connection": {
      "host": "120.46.147.53",
      "port": 5432,
      "database": "pro_db",
      "user": "renoelis",
      "password": "renoelis02@gmail.com"
    },
    "sql": "SELECT * FROM test LIMIT 5",
    "parameters": []
  }'
```

##### MongoDB查询示例：

```bash
curl -X POST http://localhost:3010/apiDatabase/mongodb \
  -H "accessToken: yZVKdhfL3ajfQ9XcvbnmPzcl5KPJSd8vwMZ9ks7JhSfKkd72" \
  -H "Content-Type: application/json" \
  -d '{
    "connection": {
      "host": "120.46.147.53",
      "port": 27017,
      "database": "test_db",
      "username": "mongo_user",
      "password": "mongo_password"
    },
    "collection": "users",
    "operation": "find",
    "filter": {"active": true},
    "limit": 10
  }'
```

#### 3. 令牌使用计费机制

- 每次调用PostgreSQL或MongoDB的写操作（非GET请求）会自动扣减一次令牌的使用次数
- 查询操作（GET请求）不消耗令牌次数
- 当`remaining_calls`降至0时，API将拒绝后续请求，直到增购令牌次数

#### 4. 查询和更新令牌额度

可随时查询令牌剩余次数：
```bash
curl "http://localhost:3010/apiDatabase/auth/token/info?ws_id=workspace123"
```

当需要增加使用次数时：
```bash
curl -X POST http://localhost:3010/apiDatabase/auth/token/update \
  -H "Content-Type: application/json" \
  -d '{"ws_id":"workspace123","operation":"add","calls_value":500}'
```

### 使用令牌访问API

在访问其他API端点时，可以通过以下两种方式提供令牌:

1. 通过请求头 (推荐):
```
accessToken: your_access_token
```

2. 通过查询参数:
```
?access_token=your_access_token
```

### 令牌错误码

| 错误码 | 描述 |
|--------|------|
| 1401 | 未提供访问令牌 |
| 1402 | 无效的访问令牌 |
| 1403 | 令牌调用次数已用完 |
| 1050 | 工作区ID已存在关联的令牌 |
| 1051 | 未找到工作区ID关联的令牌 | 