# Mihomo Relay Manager

一个用于管理 Mihomo 内核中继（Relay）配置的 Web UI 工具。它允许用户通过订阅或手动方式配置前置代理，并指定一个落地代理，动态生成 Mihomo 的配置文件。

## ✨ 功能特性

- **多种前置代理源**:
  - 支持通过订阅链接定时更新前置代理节点。
  - 支持手动填入单个前置代理节点。
- **节点自动测试**: 自动测试订阅中的节点，并选用延迟最低的节点作为前置代理。
- **配置管理**:
  - 在 Web UI 中实时预览生成的 Mihomo 配置文件。
  - 一键将配置应用到后端服务。
  - 支持复制、下载配置文件。
- **服务与状态监控**:
  - 在 UI 中查看后端 Mihomo 内核的运行状态。
  - 实时查看后端服务的日志输出。
  - 提供重启、安装内核等便捷操作。
- **多语言支持**: 界面支持中英文切换。
- **Docker 化部署**: 提供开箱即用的 Docker 镜像，简化部署流程。

## 🚀 快速开始

推荐使用 Docker 进行部署，这是最简单快捷的方式。

### 使用 Docker (推荐)

1.  **拉取镜像**

    从 GitHub Container Registry (ghcr.io) 拉取最新的 Docker 镜像。

    ```bash
    docker pull ghcr.io/obkj/mihomo-tool:latest
    ```

2.  **运行容器**

    执行以下命令来启动容器。

    ```bash
    docker run -d \
        -p 58888:58888 \
        -v /path/to/your/data:/data \
        --name mihomo-manager \
        ghcr.io/obkj/mihomo-tool:latest
    ```

    参数说明:
    - `-d`: 后台运行容器。
    - `-p 58888:58888`: 将主机的 `58888` 端口映射到容器的 `58888` 端口。这是访问 Web UI 所必需的。
    - `-v /path/to/your/data:/data`: **（重要）** 将主机上的一个目录（例如 `/opt/mihomo-data`）挂载到容器的 `/data` 目录。所有由程序生成的配置文件、Mihomo 内核以及日志都会保存在这里，从而实现数据的持久化，即使容器被删除或更新，配置也不会丢失。
    - `--name mihomo-manager`: 为容器指定一个友好的名称。

3.  **访问 Web UI**

    打开浏览器，访问 `http://<your-server-ip>:58888` 即可开始配置。

### 从源码构建

如果您希望自行构建，可以按照以下步骤操作：

1.  克隆本仓库。
2.  确保您已安装 Go (版本 1.21+)。
3.  在项目根目录下运行构建命令：
    ```bash
    go build -ldflags="-s -w" -o mihomo-tool main.go
    ```
4.  运行程序：
    ```bash
    ./mihomo-tool
    ```
    程序会托管 `index.html` 及相关静态资源，并启动后端的 API 服务。

## 📄 许可证

本项目采用 MIT 许可证。