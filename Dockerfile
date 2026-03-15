# ---- Final Stage ----
# 使用一个最小化的基础镜像
FROM alpine:latest

# 接收构建时传入的 TZ 参数并设置时区
# 默认为 Asia/Shanghai
ARG TZ=Asia/Shanghai
RUN apk add --no-cache tzdata && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone

# 接收构建时由 buildx 自动注入的目标平台信息
ARG TARGETPLATFORM

# 设置工作目录
WORKDIR /app

# 从构建上下文中复制应用程序二进制文件和协议文件到 /app 目录
COPY bin/${TARGETPLATFORM}/mihomo-tool .
COPY LICENSE .

# 暴露应用程序运行的端口
# 应用程序在 58888 端口上提供服务
EXPOSE 58888

# 创建一个专门用于存放配置和数据的目录，并将其声明为数据卷
VOLUME /data

# 将工作目录切换到数据目录
WORKDIR /data

# 设置容器的入口点
# 从 /app 目录执行主程序。这样，程序产生的所有文件（如配置文件）都会被保存在当前工作目录（/data）中
ENTRYPOINT ["/app/mihomo-tool"]