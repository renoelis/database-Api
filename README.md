# 数据库API服务

这是一个高性能的数据库中间件服务，使用Go语言和Gin框架实现。服务提供统一的API接口，用于连接和操作PostgreSQL和MongoDB数据库，支持令牌认证、连接池管理和并发控制。

## 功能特性

- **统一数据库操作接口**
  - PostgreSQL: 执行SQL查询，支持参数化查询
  - MongoDB: 支持常见操作(find, insert, update, delete, aggregate等)
  
- **高性能**
  - 高效的连接池管理，连接复用
  - 数据库连接池预热
  - 自动清理空闲连接
  - 数据格式优化处理
  
- **令牌认证系统**
  - 支持不同类型的令牌(无限制/有限制)
  - 支持不同插件类型的令牌(postgresql/mongodb/all)
  - 令牌使用日志记录
  - 定期清理过期日志

- **安全与控制机制**
  - 并发控制和限流
  - 数据库连接超时控制
  - 优雅关闭，确保资源释放

## 技术架构

```
数据库API服务
├─ cmd/                      # 入口程序
│  └─ main.go                # 主程序入口
├─ config/                   # 配置管理
│  └─ config.go              # 配置加载和获取
├─ controller/               # 控制器层
│  ├─ auth.go                # 认证相关控制器
│  ├─ mongodb.go             # MongoDB操作控制器
│  └─ postgresql.go          # PostgreSQL操作控制器
├─ database/                 # 数据库层
│  ├─ auth_operations.go     # 认证数据库操作
│  └─ config.go              # 数据库配置
├─ middleware/               # 中间件
│  ├─ auth.go                # 认证中间件
│  └─ concurrency.go         # 并发控制中间件
├─ model/                    # 数据模型
│  └─ auth.go                # 令牌模型
├─ router/                   # 路由配置
│  └─ router.go              # 路由设置
├─ utils/                    # 工具类
│  ├─ pool.go                # 连接池管理
│  └─ response.go            # 响应格式化
├─ .dockerignore             # Docker忽略文件
├─ Dockerfile                # Docker构建文件
├─ docker-compose-database-api-public-go.yml # Docker Compose配置
├─ go.mod                    # Go模块定义
└─ README.md                 # 项目说明文档
```

## 环境变量配置

服务可以通过以下环境变量进行配置：

| 环境变量 | 描述 | 默认值 |
|---------|------|-------|
| PORT | 服务监听端口 | 3010 |
| MAX_CONCURRENT_REQUESTS | 最大并发请求数 | 200 |
| POSTGRESQL_MAX_CONCURRENT | PostgreSQL最大并发请求数 | 100 |
| MONGODB_MAX_CONCURRENT | MongoDB最大并发请求数 | 100 |
| DEBUG_MODE | 调试模式，设置为"true"或"1"启用 | false |

### 认证数据库配置

认证系统使用PostgreSQL数据库，可以通过以下环境变量配置：

| 环境变量 | 描述 | 默认值 |
|---------|------|-------|
| AUTH_DB_HOST | 认证数据库主机地址 | XXXXX |
| AUTH_DB_PORT | 认证数据库端口 | 5432 |
| AUTH_DB_NAME | 认证数据库名称 | XXXXX |
| AUTH_DB_USER | 认证数据库用户名 | XXXXX |
| AUTH_DB_PASSWORD | 认证数据库密码 | XXXXX |
| AUTH_DB_SSLMODE | 认证数据库SSL模式 | disable |

## 数据库表结构

认证系统使用以下数据库表：

1. **db_api_tokens** - 存储API访问令牌
   - `token_id`: 令牌ID (主键)
   - `access_token`: 访问令牌 (唯一)
   - `email`: 用户邮箱
   - `ws_id`: 工作区ID
   - `token_type`: 令牌类型 (limited/unlimited)
   - `plugin_type`: 插件类型 (postgresql/mongodb/all)
   - `remaining_calls`: 剩余调用次数
   - `total_calls`: 总调用次数
   - `is_active`: 是否激活
   - `created_at`: 创建时间
   - `updated_at`: 更新时间
   - 注：`ws_id`和`plugin_type`组合唯一约束

2. **db_token_usage_logs** - 记录令牌使用日志
   - `log_id`: 日志ID (主键)
   - `token_id`: 令牌ID (外键)
   - `ws_id`: 工作区ID
   - `operation_type`: 操作类型
   - `target_database`: 目标数据库
   - `target_collection`: 目标集合
   - `created_at`: 创建时间
   - `status`: 状态
   - `request_details`: 请求详情

## 性能优化

### 连接池管理

服务使用高效的连接池管理策略，包括：

1. **连接池预热** - 服务启动时预先初始化连接池，减少首次请求的响应时间
2. **连接复用** - 相同连接参数的请求复用已建立的连接，避免重复建立连接开销
3. **自动清理** - 定期清理长时间未使用的连接，释放资源
4. **连接池参数优化** - 针对PostgreSQL和MongoDB优化的连接池配置参数，如：
   - `MaxConns`：最大连接数 (20)
   - `MinConns`：最小连接数 (1)
   - `MaxConnLifetime`：连接最大生命周期 (30分钟)
   - `MaxConnIdleTime`：连接最大空闲时间 (5分钟)

### MongoDB数据处理

MongoDB查询结果中的ObjectID字段(`_id`)会自动转换为简洁的字符串格式：
- 优化前: `"_id": "ObjectID(\"681f80883ca8859d7c2e95e2\")"`
- 优化后: `"_id": "681f80883ca8859d7c2e95e2"`

## API接口

### 数据库操作接口

#### PostgreSQL查询

```
POST /apiDatabase/postgresql
Content-Type: application/json
accessToken: <您的访问令牌>

{
  "host": "数据库主机",
  "port": 5432,
  "database": "数据库名称",
  "user": "用户名",
  "password": "密码",
  "sslmode": "disable",
  "query": "SELECT * FROM your_table WHERE column = $1",
  "parameters": ["参数值"]
}
```

响应示例：

```json
{
  "code": 0,
  "message": "查询成功",
  "data": [
    {"id": 1, "name": "示例1"},
    {"id": 2, "name": "示例2"}
  ]
}
```

#### MongoDB操作

```
POST /apiDatabase/mongodb
Content-Type: application/json
accessToken: <您的访问令牌>

{
  "connection": {
    "host": "数据库主机",
    "port": 27017,
    "database": "数据库名称",
    "username": "用户名",
    "password": "密码",
    "auth_source": "admin"
  },
  "collection": "集合名称",
  "operation": "find",
  "filter": {"status": "active"},
  "projection": {"_id": 1, "name": 1},
  "sort": ["name", "-createTime"],
  "limit": 10,
  "skip": 0
}
```

响应示例：

```json
{
  "code": 0,
  "message": "操作成功",
  "data": [
    {"_id": "62f5d8c3e32b835b8236c321", "name": "示例1"},
    {"_id": "62f5d8c9e32b835b8236c322", "name": "示例2"}
  ]
}
```

MongoDB支持的操作包括：
- `find` - 查询多条记录
- `findone` - 查询单条记录
- `insert` - 插入单条记录
- `insertmany` - 批量插入记录
- `update` - 更新单条记录
- `updatemany` - 批量更新记录
- `delete` - 删除单条记录
- `deletemany` - 批量删除记录
- `aggregate` - 聚合操作
- `count` - 计数操作

### 认证相关接口

#### 创建令牌

```
POST /apiDatabase/auth/token
Content-Type: application/json

{
  "email": "user@example.com",
  "ws_id": "工作区ID",
  "token_type": "limited",
  "plugin_type": "postgresql", // 可选值: postgresql, mongodb, all (默认为all)
  "total_calls": 1000
}
```

响应示例：

```json
{
  "code": 0,
  "message": "令牌创建成功",
  "data": {
    "token_id": 1,
    "access_token": "tvKljrkP6HgH7on67wTYnxPJ7ju4QyBHUzuXWc1YGMW0inPO",
    "ws_id": "工作区ID",
    "token_type": "limited",
    "plugin_type": "postgresql",
    "remaining_calls": 1000,
    "total_calls": 1000,
    "used_calls": 0
  }
}
```

#### 更新令牌调用次数

```
POST /apiDatabase/auth/token/update
Content-Type: application/json

{
  "access_token": "tvKljrkP6HgH7on67wTYnxPJ7ju4QyBHUzuXWc1YGMW0inPO",
  "operation": "add",  // 可选值: add, set, unlimited
  "calls_value": 100   // add和set操作时需要提供
}
```

响应示例：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "token_id": 1,
    "ws_id": "工作区ID",
    "token_type": "limited",
    "plugin_type": "postgresql",
    "remaining_calls": 1100,
    "total_calls": 1100,
    "used_calls": 0
  }
}
```

> **注意**: 
> - `add` 操作会同时增加 `total_calls` 和 `remaining_calls`
> - `set` 操作会将 `total_calls` 和 `remaining_calls` 同时设置为指定值
> - `unlimited` 操作会将令牌转换为无限制类型，移除调用次数限制

#### 查询令牌信息

```
GET /apiDatabase/auth/token/info?ws_id=工作区ID&plugin_type=postgresql
```

> 注意：如果不提供plugin_type参数，将返回该工作区的所有令牌列表

响应示例（指定plugin_type时）：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "token_id": 1,
    "access_token": "tvKljrkP6HgH7on67wTYnxPJ7ju4QyBHUzuXWc1YGMW0inPO",
    "ws_id": "工作区ID",
    "email": "user@example.com",
    "token_type": "limited",
    "plugin_type": "postgresql",
    "remaining_calls": 600,
    "total_calls": 1000,
    "used_calls": 400,
    "is_active": true,
    "created_at": "2023-05-28T10:00:00Z",
    "updated_at": "2023-05-28T10:15:00Z"
  }
}
```

响应示例（不指定plugin_type时）：

```json
{
  "code": 0,
  "message": "成功",
  "data": [
    {
      "token_id": 1,
      "access_token": "tvKljrkP6HgH7on67wTYnxPJ7ju4QyBHUzuXWc1YGMW0inPO",
      "ws_id": "工作区ID",
      "email": "user@example.com",
      "token_type": "limited",
      "plugin_type": "postgresql",
      "remaining_calls": 600,
      "total_calls": 1000,
      "used_calls": 400,
      "is_active": true,
      "created_at": "2023-05-28T10:00:00Z",
      "updated_at": "2023-05-28T10:15:00Z"
    },
    {
      "token_id": 2,
      "access_token": "a8dFgH7jK9L0mN1pQ2rS3tU4vW5xY6zAb2CdE3FgH4iJ5k",
      "ws_id": "工作区ID",
      "email": "user@example.com",
      "token_type": "unlimited",
      "plugin_type": "mongodb",
      "is_active": true,
      "created_at": "2023-05-28T11:00:00Z",
      "updated_at": "2023-05-28T11:00:00Z"
    }
  ]
}
```

### 系统控制接口

> **重要说明**: 系统控制接口仅允许"ws_id": "renoelis"的令牌进行调用，其他令牌无权访问这些接口。

#### 查询系统配置

```
GET /apiDatabase/system/config
accessToken: <renoelis管理员令牌>
```

响应示例：

```json
{
  "code": 0,
  "message": "系统配置查询成功",
  "data": {
    "max_concurrent_requests": 200,
    "postgresql_max_concurrent": 100,
    "mongodb_max_concurrent": 100,
    "debug_mode": false
  }
}
```

#### 禁用并发限制

```
GET /apiDatabase/system/concurrency/disable
accessToken: <renoelis管理员令牌>
```

#### 启用并发限制

```
GET /apiDatabase/system/concurrency/enable
accessToken: <renoelis管理员令牌>
```

## 构建与运行

### 环境变量配置

本服务支持两种环境变量配置方式：

1. **通过docker-compose文件配置**
   
   环境变量已经在`docker-compose-database-api-public-go.yml`文件中配置，您可以直接修改该文件中的values：
   
   ```yaml
   environment:
     - PORT=3010
     - MAX_CONCURRENT_REQUESTS=200
     - POSTGRESQL_MAX_CONCURRENT=100
     - MONGODB_MAX_CONCURRENT=100
     - AUTH_DB_HOST=XXXX
     - AUTH_DB_PORT=5432
     - AUTH_DB_USER=XXXXX
     - AUTH_DB_PASSWORD=XXXXX
     - AUTH_DB_NAME=XXXX
   ```

2. **通过.env文件配置** (可选)

   如果您想使用.env文件，可创建一个.env文件包含以下内容：
   ```
   # 服务配置
   PORT=3010
   MAX_CONCURRENT_REQUESTS=200
   POSTGRESQL_MAX_CONCURRENT=100
   MONGODB_MAX_CONCURRENT=100
   
   # 认证数据库配置
   AUTH_DB_HOST=XXXX
   AUTH_DB_PORT=5432
   AUTH_DB_NAME=XXXX
   AUTH_DB_USER=XXXXx
   AUTH_DB_PASSWORD=XXXXX
   AUTH_DB_SSLMODE=disable
   ```
   
   然后修改`docker-compose-database-api-public-go.yml`启用env文件：
   ```yaml
   services:
     database-api-public-go:
       # ... 其他配置 ...
       env_file:
         - .env
       # 删除或注释掉原有的environment部分
   ```

### 本地运行

```bash
# 安装依赖
go mod tidy

# 运行服务
go run cmd/main.go
```

### 使用Docker运行

```bash
# 构建Docker镜像
docker build -t database-api-public-go .

# 方法1：使用.env文件
docker run --env-file .env -p 3010:3010 database-api-public-go

# 方法2：使用单独的环境变量参数
docker run -p 3010:3010 \
  -e AUTH_DB_HOST=XXXX \
  -e AUTH_DB_PORT=5432 \
  -e AUTH_DB_USER=XXXX \
  -e AUTH_DB_PASSWORD=XXXXX \
  -e AUTH_DB_NAME=XXXX \
  database-api-public-go
```

### 使用Docker Compose运行

```bash
# 启动服务
docker-compose -f docker-compose-database-api-public-go.yml up -d
```

## 性能监控

服务日志会记录每个请求的处理时间明细，包括：
- 数据库连接时间
- SQL执行时间
- 结果处理时间
- 总响应时间

日志示例：
```
[2023-05-28 10:15:20] [总耗时=85.289ms] 数据库连接=0.016ms SQL执行=60.421ms 结果处理=23.112ms 行数=50
```

## 问题排查

### 常见问题

1. **请求被拒绝（429 Too Many Requests）**
   - 原因：超过并发限制
   - 解决方法：
     - 检查环境变量设置的并发限制值是否合理
     - 临时禁用并发限制：`GET /apiDatabase/system/concurrency/disable`（需要renoelis管理员令牌）
     - 优化客户端请求频率

2. **认证失败（401 Unauthorized 或 403 Forbidden）**
   - 原因：令牌无效、已过期或权限不足
   - 解决方法：
     - 检查令牌是否正确
     - 对于limited类型令牌，检查可用调用次数
     - 检查令牌的plugin_type是否有权限访问目标API（postgresql/mongodb）
     - 申请新的令牌

3. **数据库连接失败**
   - 原因：数据库配置错误或数据库服务不可用
   - 解决方法：
     - 检查连接参数是否正确
     - 确认数据库服务是否正常运行
     - 检查网络连接和防火墙设置

### 调试方法

1. 启用调试模式：设置环境变量`DEBUG_MODE=true`
2. 检查日志输出，特别关注连接时间和SQL执行时间
3. 使用系统配置接口查看当前系统状态：`GET /apiDatabase/system/config`（需要renoelis管理员令牌）