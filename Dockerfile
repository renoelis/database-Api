FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装必要的工具和依赖
RUN apk add --no-cache gcc musl-dev git

# 复制go.mod和go.sum
COPY go.mod ./
COPY go.sum ./

# 下载依赖
RUN go mod download

# 复制代码
COPY . .

# 构建应用
RUN go build -o main ./cmd/main.go

# 使用轻量级镜像运行应用
FROM alpine:latest

WORKDIR /root/

# 安装ca-certificates，用于HTTPS连接
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 从builder阶段复制编译好的二进制文件
COPY --from=builder /app/main .

# 创建配置目录
RUN mkdir -p /root/config

# 暴露端口
EXPOSE 3011

# 运行应用
CMD ["./main"] 