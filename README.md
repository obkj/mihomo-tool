# mihomo-tool

一个 mihomo 辅助工具。

## 部署说明

您可以选择以下任意一种方式进行部署。

### 方式一：使用 Docker (推荐)

这是最推荐的部署方式，简单快捷。

1.  **拉取 Docker 镜像**

    我们为每个版本都构建了支持 `linux/amd64`, `linux/arm64`, `linux/arm/v7` 平台的 Docker 镜像，并推送到了 GitHub Container Registry。

    请将下方命令中的 obkj 替换为仓库所有者的 GitHub 用户名或组织名。

    *   拉取最新版镜像：
        ```bash
        docker pull ghcr.io/obkj/mihomo-tool:latest
        ```

    *   拉取指定版本镜像 (例如 `v202603141233`):
        ```bash
        docker pull ghcr.io/obkj/mihomo-tool:v202603141233
        ```

2.  **运行容器**

    执行以下命令启动容器。默认情况下，这会将容器的 `8080` 端口映射到主机的 `8080` 端口。

    ```bash
    docker run -d -p 8080:8080 --name mihomo-tool --restart always ghcr.io/obkj/mihomo-tool:latest
    ```
    > **注意**：请确保将 obkj 替换为正确的用户名或组织名。如果需要，您可以自行更改主机端口映射。

### 方式二：使用预编译的二进制文件

如果您不希望使用 Docker，也可以直接从项目的 Releases 页面 下载我们为您预编译好的程序。

我们为 Windows, Linux, macOS, FreeBSD 等主流操作系统和多种 CPU 架构 (如 `amd64`, `arm64`) 都提供了二进制包。

1.  前往 Releases 页面。
2.  找到最新的版本，下载符合您系统和架构的压缩包 (`.zip` 或 `.tar.gz`)。
3.  解压后即可直接运行其中的可执行文件。

## 从源码构建

1.  安装 Go 环境 (需要 `1.21` 或更高版本)。
2.  克隆此仓库到本地。
3.  在项目根目录下，执行构建命令：
    ```bash
    go build -ldflags="-s -w" -o "mihomo-tool" main.go
    ```