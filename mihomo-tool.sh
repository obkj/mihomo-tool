#!/bin/sh

# Mihomo-Tool 精简版 - 自动检测国内 IP 并使用代理
# 支持: Debian, Ubuntu, CentOS, Arch, OpenWrt, Alpine

set -e

# 配置信息
REPO="obkj/mihomo-tool"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/mihomo-tool"
GH_PROXY="https://gh-proxy.org/"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# 环境检查
if [ "$(id -u)" -ne 0 ]; then error "请以 root 权限运行"; fi

# 目录适配
if [ -f /etc/openwrt_release ]; then
    INSTALL_DIR="/usr/bin"
fi

# 核心功能：检测是否需要代理
check_proxy_needed() {
    log "正在检测网络区域..."
    # 访问 ip.sb 获取地理位置，只匹配 country_code
    COUNTRY=$(curl -s https://api.ip.sb/geoip | grep -o '"country_code":"[^"]*"' | cut -d'"' -f4)
    
    if [ "$COUNTRY" = "CN" ]; then
        log "检测到境内 IP ($COUNTRY)，将使用 GitHub 加速代理..."
        USE_PROXY=true
    else
        log "检测到境外 IP ($COUNTRY)，将直接连接 GitHub..."
        USE_PROXY=false
    fi
}

# 依赖安装
install_deps() {
    log "检查依赖..."
    PKGS="curl tar ca-certificates"
    if command -v apt-get >/dev/null; then apt-get update && apt-get install -y $PKGS
    elif command -v yum >/dev/null; then yum install -y $PKGS
    elif command -v apk >/dev/null; then apk add --no-cache $PKGS gcompat
    elif command -v opkg >/dev/null; then opkg update && opkg install curl tar ca-bundle
    fi
}

do_install() {
    install_deps
    check_proxy_needed

    # 架构识别
    ARCH=$(uname -m)
    case $ARCH in
        x86_64) GOARCH="amd64" ;;
        aarch64) GOARCH="arm64" ;;
        armv7*) GOARCH="arm" ;;
        *) error "不支持的架构: $ARCH" ;;
    esac

    # 直接从 GitHub API 获取最新版本
    log "获取最新版本号..."
    LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    [ -z "$LATEST_TAG" ] && error "无法获取版本号"

    # 构造下载链接
    URL="https://github.com/$REPO/releases/download/$LATEST_TAG/mihomo-tool-linux-$GOARCH.tar.gz"
    [ "$USE_PROXY" = true ] && URL="${GH_PROXY}${URL}"

    log "正在下载: $URL"
    TMP_DIR=$(mktemp -d)
    curl -L "$URL" | tar -xz -C "$TMP_DIR"

    # 文件分发
    mkdir -p "$CONFIG_DIR"
    BIN_SOURCE="$TMP_DIR/mihomo-tool-linux-$GOARCH"
    cp "$BIN_SOURCE/mihomo-tool" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/mihomo-tool"
    
    # 初始化静态文件
    [ ! -f "$CONFIG_DIR/index.html" ] && cp -r "$BIN_SOURCE"/* "$CONFIG_DIR/" || true

    # 配置服务
    setup_service
    
    log "安装完成！"
    log "Web 界面: http://$(ip route get 1 2>/dev/null | awk '{print $7;exit}' || echo "localhost"):58888"
    rm -rf "$TMP_DIR"
}

setup_service() {
    # 针对不同系统配置服务
    if [ -f /etc/init.d/rc.common ]; then # OpenWrt
        cat <<EOF > /etc/init.d/mihomo-tool
#!/bin/sh /etc/rc.common
START=99
USE_PROCD=1
start_service() {
    procd_open_instance
    procd_set_param command "$INSTALL_DIR/mihomo-tool"
    procd_set_param chdir "$CONFIG_DIR"
    procd_set_param respawn
    procd_close_instance
}
EOF
        chmod +x /etc/init.d/mihomo-tool
        /etc/init.d/mihomo-tool enable && /etc/init.d/mihomo-tool start
    elif command -v systemctl >/dev/null; then # Systemd
        cat <<EOF > /etc/systemd/system/mihomo-tool.service
[Unit]
Description=Mihomo-Tool
After=network.target
[Service]
WorkingDirectory=$CONFIG_DIR
ExecStart=$INSTALL_DIR/mihomo-tool
Restart=always
[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload && systemctl enable mihomo-tool --now
    fi
}

# 运行逻辑
ACTION=${1:-install}
if [ "$ACTION" = "uninstall" ]; then
    log "正在卸载..."
    systemctl stop mihomo-tool 2>/dev/null || /etc/init.d/mihomo-tool stop 2>/dev/null || true
    rm -f "$INSTALL_DIR/mihomo-tool" /etc/systemd/system/mihomo-tool.service /etc/init.d/mihomo-tool
    log "卸载成功"
else
    do_install
fi