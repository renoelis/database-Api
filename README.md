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