#!/bin/sh

# Mihomo-Tool 全系通用精简版 - 先下载后解压版
set -e

REPO="obkj/mihomo-tool"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/mihomo-tool"
GH_PROXY="https://gh-proxy.org/"

log() { echo -e "\033[0;34m[INFO]\033[0m $1"; }
error() { echo -e "\033[0;31m[ERROR]\033[0m $1"; exit 1; }

[ "$(id -u)" -ne 0 ] && error "请以 root 权限运行"

# 1. 环境与系统类型识别
OS_TYPE="linux"
if [ -f /etc/openwrt_release ]; then
    OS_TYPE="openwrt"; INSTALL_DIR="/usr/bin"
elif [ -f /etc/alpine-release ]; then
    OS_TYPE="alpine"
fi

# 2. 智能区域检测 (ip.sb)
# 使用简洁的 API 检查，若为 CN 则自动加代理
COUNTRY=$(curl -s --connect-timeout 5 https://api.ip.sb/geoip | grep -o '"country_code":"[^"]*"' | cut -d'"' -f4 || echo "Unknown")
USE_PROXY=false
[ "$COUNTRY" = "CN" ] && USE_PROXY=true && log "检测到境内 IP ($COUNTRY)，启用 GitHub 加速代理..."

# 3. 安装依赖
log "检查并安装必要工具..."
if command -v apt-get >/dev/null; then apt-get update && apt-get install -y curl tar ca-certificates
elif command -v yum >/dev/null; then yum install -y curl tar ca-certificates
elif command -v dnf >/dev/null; then dnf install -y curl tar ca-certificates
elif command -v apk >/dev/null; then apk add --no-cache curl tar ca-certificates gcompat
elif command -v opkg >/dev/null; then opkg update && opkg install curl tar ca-bundle
fi

# 4. 获取架构与最新版本 (GitHub API)
ARCH=$(uname -m)
case $ARCH in
    x86_64) GOARCH="amd64" ;;
    aarch64) GOARCH="arm64" ;;
    armv7*) GOARCH="arm" ;;
    *) error "不支持的架构: $ARCH" ;;
esac

log "正在获取最新版本信息..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
[ -z "$LATEST_TAG" ] && error "无法获取 GitHub 版本，请检查网络"

# 5. 下载到临时文件
URL="https://github.com/$REPO/releases/download/$LATEST_TAG/mihomo-tool-linux-$GOARCH.tar.gz"
[ "$USE_PROXY" = true ] && URL="${GH_PROXY}${URL}"

TMP_DIR=$(mktemp -d)
PKG_FILE="$TMP_DIR/mihomo.tar.gz"

log "开始下载: $URL"
curl -L -o "$PKG_FILE" "$URL"

# 6. 解压并安装
log "下载完成，正在解压安装..."
tar -xzf "$PKG_FILE" -C "$TMP_DIR"

mkdir -p "$CONFIG_DIR"
BIN_SOURCE="$TMP_DIR/mihomo-tool-linux-$GOARCH"
cp "$BIN_SOURCE/mihomo-tool" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/mihomo-tool"

# 如果配置目录没有静态页面，则拷贝
[ ! -f "$CONFIG_DIR/index.html" ] && cp -r "$BIN_SOURCE"/* "$CONFIG_DIR/" || true

# 7. 服务配置
if [ "$OS_TYPE" = "openwrt" ]; then
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
    /etc/init.d/mihomo-tool enable && /etc/init.d/mihomo-tool restart
elif [ "$OS_TYPE" = "alpine" ]; then
    cat <<EOF > /etc/init.d/mihomo-tool
#!/sbin/openrc-run
description="mihomo-tool service"
command="$INSTALL_DIR/mihomo-tool"
command_background="yes"
directory="$CONFIG_DIR"
pidfile="/run/\${RC_SVCNAME}.pid"
depend() { need net; }
EOF
    chmod +x /etc/init.d/mihomo-tool
    rc-update add mihomo-tool default && rc-service mihomo-tool restart
else
    cat <<EOF > /etc/systemd/system/mihomo-tool.service
[Unit]
Description=Mihomo-Tool
After=network.target

[Service]
Type=simple
WorkingDirectory=$CONFIG_DIR
ExecStart=$INSTALL_DIR/mihomo-tool
Restart=always

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable mihomo-tool --now
fi

log "安装成功！"
# 获取本机 IP 并输出
LOCAL_IP=$(ip route get 1 2>/dev/null | awk '{print $7;exit}' || echo "localhost")
log "Web UI 地址: http://$LOCAL_IP:58888"

# 8. 清理
rm -rf "$TMP_DIR"