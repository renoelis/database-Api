FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制所有源代码
COPY . .

# 确保依赖正确
RUN go mod tidy && go mod download && go mod verify

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o database-api-service ./cmd/main.go

# 构建最终镜像
FROM alpine:latest

# 安装 ca-certificates
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 从构建阶段复制编译好的应用
COPY --from=builder /app/database-api-service .

# 暴露端口
EXPOSE 3010

# 设置非敏感环境变量默认值
ENV PORT=3010 \
    MAX_CONCURRENT_REQUESTS=200 \
    POSTGRESQL_MAX_CONCURRENT=100 \
    MONGODB_MAX_CONCURRENT=100

# 启动应用
CMD ["./database-api-service"]