# 数据库API服务 (Go版本)

这是一个使用Go语言实现的数据库API服务，提供对PostgreSQL和MongoDB数据库的操作接口。

## 功能特性

- 支持PostgreSQL数据库操作
- 支持MongoDB数据库操作
- API访问令牌认证
- 请求并发限制
- 连接池管理
- 统一的错误处理
- Docker部署支持

## 项目结构

```
database-api-go
├── cmd
│   └── main.go               # 应用入口
├── config
│   └── config.go             # 配置加载
├── controllers
│   ├── basic.go              # 基础控制器
│   ├── mongodb.go            # MongoDB控制器
│   └── postgresql.go         # PostgreSQL控制器
├── middleware
│   ├── auth.go               # 认证中间件
│   ├── concurrency.go        # 并发控制中间件
│   ├── cors.go               # CORS中间件
│   ├── logger.go             # 日志中间件
│   └── recovery.go           # 错误恢复中间件
├── models
│   ├── mongodb.go            # MongoDB数据模型
│   └── postgresql.go         # PostgreSQL数据模型
├── routers
│   └── router.go             # 路由注册
├── services
│   ├── mongodb_pool.go       # MongoDB连接池
│   └── postgresql_pool.go    # PostgreSQL连接池
├── utils
│   └── response.go           # 响应工具
├── .dockerignore
├── Dockerfile
├── docker-compose-database-api-go.yml
└── go.mod
```

## 环境变量配置

服务可通过以下环境变量进行配置：

- `PORT`: 服务器监听端口，默认为3011
- `MAX_CONCURRENT_REQUESTS`: 最大并发请求数，默认为200
- `POSTGRESQL_MAX_CONCURRENT`: PostgreSQL最大并发请求数，默认为100
- `MONGODB_MAX_CONCURRENT`: MongoDB最大并发请求数，默认为100

## API接口文档

### 认证

所有API请求需要在请求头中包含`accessToken`字段进行认证。可以通过以下接口获取当前有效的访问令牌：

```
GET /apiDatabase/token
```

### PostgreSQL接口

```
POST /apiDatabase/postgresql
```

请求体示例：

```json
{
  "connection": {
    "host": "localhost",
    "port": 5432,
    "database": "postgres",
    "user": "postgres",
    "password": "password",
    "sslmode": "prefer"
  },
  "sql": "SELECT * FROM users WHERE id = $1",
  "parameters": [1]
}
```

### MongoDB接口

```
POST /apiDatabase/mongodb
```

请求体示例：

```json
{
  "connection": {
    "host": "localhost",
    "port": 27017,
    "database": "test",
    "username": "admin",
    "password": "password",
    "auth_source": "admin"
  },
  "collection": "users",
  "operation": "find",
  "filter": {
    "age": {"$gt": 18}
  },
  "projection": {
    "name": 1,
    "age": 1,
    "_id": 0
  }
}
```

## 构建与运行

### 本地运行

1. 构建应用：

```bash
go build -o main ./cmd/main.go
```

2. 运行应用：

```bash
./main
```

### Docker构建与运行

1. 构建Docker镜像：

```bash
docker build -t database-api-go .
```

2. 使用docker-compose运行：

```bash
docker-compose -f docker-compose-database-api-go.yml up -d
```

### 服务器部署

1. 本地构建Docker镜像并保存为tar包：

```bash
docker build -t database-api-go .
docker save -o database-api-go.tar database-api-go:latest
```

2. 将tar包上传到服务器

3. 在服务器上加载镜像并运行：

```bash
docker load -i database-api-go.tar
docker-compose -f docker-compose-database-api-go.yml up -d
``` 