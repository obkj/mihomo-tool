#!/bin/sh

# Mihomo-Tool Unified Management Script (Smart Proxy Version)
# Supports: Debian, Ubuntu, CentOS, Arch, OpenWrt, Alpine
# Usage: ./mihomo-tool.sh [install|uninstall]

set -e

# Configuration
REPO="obkj/mihomo-tool"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/mihomo-tool"
SERVICE_NAME="mihomo-tool"
GH_PROXY="https://gh-proxy.org/"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    error "Please run as root"
fi

# Detect OS
OS_TYPE="linux"
if [ -f /etc/openwrt_release ]; then
    OS_TYPE="openwrt"
    INSTALL_DIR="/usr/bin"
elif [ -f /etc/alpine-release ] || grep -q "Alpine" /etc/os-release 2>/dev/null; then
    OS_TYPE="alpine"
fi

# 智能检测是否需要代理 (通过检测 Cloudflare 连通性)
check_proxy_needed() {
    log "Checking network environment..."
    # 如果 3 秒内无法连接 Google/Cloudflare，则认为在境内，需要代理
    if ! curl -Is --connect-timeout 3 https://www.google.com > /dev/null 2>&1; then
        log "International network detected as slow, enabling GitHub Proxy..."
        USE_PROXY=true
    else
        log "Direct connection to GitHub is available."
        USE_PROXY=false
    fi
}

check_and_install_deps() {
    log "Checking dependencies..."
    DEPS="curl tar ca-certificates"
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update && apt-get install -y $DEPS
    elif command -v yum >/dev/null 2>&1; then
        yum install -y $DEPS
    elif command -v apk >/dev/null 2>&1; then
        apk add --no-cache $DEPS gcompat
    elif command -v opkg >/dev/null 2>&1; then
        opkg update && opkg install curl tar ca-bundle
    fi
}

do_install() {
    check_and_install_deps
    check_proxy_needed

    # 架构检测
    ARCH=$(uname -m)
    case $ARCH in
        x86_64) GOARCH="amd64" ;;
        aarch64) GOARCH="arm64" ;;
        armv7*) GOARCH="arm" ;;
        i386|i686) GOARCH="386" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    # 获取最新版本 (直连 GitHub API)
    log "Fetching latest version..."
    LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    [ -z "$LATEST_TAG" ] && error "Failed to fetch version from GitHub API."

    # 拼接下载地址
    BASE_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/mihomo-tool-linux-$GOARCH.tar.gz"
    if [ "$USE_PROXY" = true ]; then
        DOWNLOAD_URL="${GH_PROXY}${BASE_URL}"
    else
        DOWNLOAD_URL="${BASE_URL}"
    fi

    log "Downloading from: $DOWNLOAD_URL"
    TMP_DIR=$(mktemp -d)
    curl -L "$DOWNLOAD_URL" | tar -xz -C "$TMP_DIR"

    # 安装文件
    mkdir -p "$CONFIG_DIR"
    BIN_DIR="$TMP_DIR/mihomo-tool-linux-$GOARCH"
    cp "$BIN_DIR/mihomo-tool" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/mihomo-tool"
    
    # 初始化配置/网页文件 (如果不存在)
    [ ! -f "$CONFIG_DIR/index.html" ] && cp -r "$BIN_DIR"/* "$CONFIG_DIR/" || true

    # 写入服务配置 (此处根据 OS_TYPE 自动选择 systemd/init.d/openrc)
    setup_service

    log "Installation successful! Access WebUI at http://$(ip route get 1 | awk '{print $7;exit}'):58888"
    rm -rf "$TMP_DIR"
}

setup_service() {
    if [ "$OS_TYPE" = "openwrt" ]; then
        cat <<EOF > /etc/init.d/mihomo-tool
#!/bin/sh /etc/rc.common
START=99
USE_PROCD=1
start_service() {
    procd_open_instance
    procd_set_param command "$INSTALL_DIR/mihomo-tool"
    procd_set_param respawn
    procd_set_param chdir "$CONFIG_DIR"
    procd_close_instance
}
EOF
        chmod +x /etc/init.d/mihomo-tool
        /etc/init.d/mihomo-tool enable && /etc/init.d/mihomo-tool start
    elif [ "$OS_TYPE" = "alpine" ]; then
        cat <<EOF > /etc/init.d/mihomo-tool
#!/sbin/openrc-run
command="$INSTALL_DIR/mihomo-tool"
command_background="yes"
directory="$CONFIG_DIR"
pidfile="/run/mihomo-tool.pid"
EOF
        chmod +x /etc/init.d/mihomo-tool
        rc-update add mihomo-tool default && rc-service mihomo-tool start
    else
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

do_uninstall() {
    log "Uninstalling..."
    systemctl stop mihomo-tool 2>/dev/null || rc-service mihomo-tool stop 2>/dev/null || /etc/init.d/mihomo-tool stop 2>/dev/null || true
    rm -f "$INSTALL_DIR/mihomo-tool" /etc/systemd/system/mihomo-tool.service /etc/init.d/mihomo-tool
    log "Uninstalled."
}

ACTION=${1:-install}
case $ACTION in
    install) do_install ;;
    uninstall) do_uninstall ;;
    *) echo "Usage: $0 [install|uninstall]"; exit 1 ;;
esac