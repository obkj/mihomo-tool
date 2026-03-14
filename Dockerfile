# ---- Builder Stage ----
# 使用官方的 Golang 镜像作为构建器
FROM golang:1.21-alpine AS builder

# 设置容器内的工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 文件以下载依赖
# 这一步可以利用 Docker 的层缓存机制
COPY go.mod go.sum ./
RUN go mod download

# 复制其余的源代码
COPY . .

# 构建 Go 应用
# - CGO_ENABLED=0: 禁用 CGO 以创建静态二进制文件
# - ldflags="-s -w": 去除调试信息，减小二进制文件大小
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /mihomo-tool main.go

# ---- Final Stage ----
# 使用一个最小化的基础镜像
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 从构建器阶段复制编译好的二进制文件
COPY --from=builder /mihomo-tool .

# 复制静态资源
COPY index.html .
COPY css/ ./css/
COPY js/ ./js/

# 暴露应用程序运行的端口
# 注意：请将 58888 替换为您的应用实际使用的端口
EXPOSE 58888

# 设置容器的入口点
ENTRYPOINT ["./mihomo-tool"]