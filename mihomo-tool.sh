#!/bin/sh

# Mihomo-Tool 全系通用精简版
set -e

REPO="obkj/mihomo-tool"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/mihomo-tool"
GH_PROXY="https://gh-proxy.org/"

log() { echo -e "\033[0;34m[INFO]\033[0m $1"; }
error() { echo -e "\033[0;31m[ERROR]\033[0m $1"; exit 1; }

[ "$(id -u)" -ne 0 ] && error "请以 root 权限运行"

# 1. 环境适配：目录与系统类型
OS_TYPE="linux"
if [ -f /etc/openwrt_release ]; then
    OS_TYPE="openwrt"; INSTALL_DIR="/usr/bin"
elif [ -f /etc/alpine-release ]; then
    OS_TYPE="alpine"
fi

# 2. 智能区域检测 (ip.sb)
COUNTRY=$(curl -s --connect-timeout 5 https://api.ip.sb/geoip | grep -o '"country_code":"[^"]*"' | cut -d'"' -f4 || echo "Unknown")
USE_PROXY=false
[ "$COUNTRY" = "CN" ] && USE_PROXY=true && log "检测到境内 IP，启用加速代理..."

# 3. 安装依赖
log "安装依赖..."
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

LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
[ -z "$LATEST_TAG" ] && error "无法获取版本"

# 5. 流式下载并安装
URL="https://github.com/$REPO/releases/download/$LATEST_TAG/mihomo-tool-linux-$GOARCH.tar.gz"
[ "$USE_PROXY" = true ] && URL="${GH_PROXY}${URL}"

log "正在安装 $LATEST_TAG ($GOARCH)..."
TMP_DIR=$(mktemp -d)
curl -sSL "$URL" | tar -xz -C "$TMP_DIR"

mkdir -p "$CONFIG_DIR"
BIN_SOURCE="$TMP_DIR/mihomo-tool-linux-$GOARCH"
cp "$BIN_SOURCE/mihomo-tool" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/mihomo-tool"
[ ! -f "$CONFIG_DIR/index.html" ] && cp -r "$BIN_SOURCE"/* "$CONFIG_DIR/" || true

# 6. 服务配置（适配 Systemd / OpenRC / Procd）
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
command="$INSTALL_DIR/mihomo-tool"
command_background="yes"
directory="$CONFIG_DIR"
pidfile="/run/\${RC_SVCNAME}.pid"
depend() { need net; }
EOF
    chmod +x /etc/init.d/mihomo-tool
    rc-update add mihomo-tool default && rc-service mihomo-tool restart
else
    # 适配通用 Linux (Systemd)
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
log "Web UI 地址: http://$(ip route get 1 2>/dev/null | awk '{print $7;exit}' || echo "localhost"):58888"
rm -rf "$TMP_DIR"