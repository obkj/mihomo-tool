#!/bin/sh

# Mihomo-Tool Unified Management Script
# Supports: Debian, Ubuntu, CentOS, Arch, OpenWrt, Alpine
# Architectures: amd64, arm64, armv7, 386, mips, mipsle, mips64, mips64le
# Usage: ./mihomo-tool.sh [install|uninstall]

set -e

# Configuration
REPO="obkj/mihomo-tool"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/mihomo-tool"
SERVICE_NAME="mihomo-tool"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    error "Please run as root"
fi

# Detect OS
OS_TYPE="linux"
OS_FOR_DOWNLOAD="linux" # Asset name for download is 'linux' for all distros
if [ -f /etc/openwrt_release ]; then
    OS_TYPE="openwrt"
    INSTALL_DIR="/usr/bin"
elif [ -f /etc/alpine-release ] || grep -q "Alpine" /etc/os-release 2>/dev/null; then
    OS_TYPE="alpine"
    # Alpine uses linux binaries with gcompat
fi

check_and_install_deps() {
    log "Checking and installing dependencies..."
    DEPS="curl tar ca-certificates"
    
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update
        apt-get install -y $DEPS
    elif command -v yum >/dev/null 2>&1; then
        yum install -y $DEPS
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y $DEPS
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm $DEPS
    elif command -v apk >/dev/null 2>&1; then
        apk add --no-cache $DEPS gcompat
    elif command -v opkg >/dev/null 2>&1; then
        opkg update
        opkg install curl tar ca-bundle
    else
        log "Manual dependency check required: please ensure $DEPS are installed."
    fi
}

do_uninstall() {
    log "Stopping and disabling service..."
    if [ "$OS_TYPE" == "openwrt" ]; then
        if [ -f /etc/init.d/mihomo-tool ]; then
            /etc/init.d/mihomo-tool stop
            /etc/init.d/mihomo-tool disable
            rm /etc/init.d/mihomo-tool
        fi
    elif [ "$OS_TYPE" == "alpine" ]; then
        if [ -f /etc/init.d/mihomo-tool ]; then
            rc-service mihomo-tool stop || true
            rc-update del mihomo-tool default || true
            rm /etc/init.d/mihomo-tool
        fi
    else
        if systemctl is-active --quiet mihomo-tool; then
            systemctl stop mihomo-tool
        fi
        if systemctl is-enabled --quiet mihomo-tool; then
            systemctl disable mihomo-tool
        fi
        rm -f /etc/systemd/system/mihomo-tool.service
        systemctl daemon-reload
    fi

    log "Removing binary and configuration..."
    rm -f "$INSTALL_DIR/mihomo-tool"
    rm -rf "$CONFIG_DIR"

    log "Mihomo-Tool has been uninstalled."
}

do_install() {
    # Install dependencies
    check_and_install_deps

    # Detect Architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64) GOARCH="amd64" ;;
        aarch64) GOARCH="arm64" ;;
        armv7*) GOARCH="arm" ;;
        i386|i686) GOARCH="386" ;;
        mips64) GOARCH="mips64" ;;
        mips64el) GOARCH="mips64le" ;;
        mips) GOARCH="mips" ;;
        mipsel) GOARCH="mipsle" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    log "Detected OS: $OS_TYPE, Architecture: $GOARCH"

    # Get latest version
    log "Fetching latest version from GitHub..."
    LATEST_TAG=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$LATEST_TAG" ]; then
        error "Failed to fetch latest version"
    fi
    log "Latest version: $LATEST_TAG"

    # Download URL
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/mihomo-tool-${OS_FOR_DOWNLOAD}-$GOARCH.tar.gz"

    if [ "$USE_PROXY" = "true" ]; then
        DOWNLOAD_URL="https://gh-proxy.org/$DOWNLOAD_URL"
        log "Using proxy: $DOWNLOAD_URL"
    fi

    log "Downloading from $DOWNLOAD_URL ..."
    TMP_DIR=$(mktemp -d)
    curl -L -o "$TMP_DIR/mihomo-tool.tar.gz" "$DOWNLOAD_URL"
    tar -xzf "$TMP_DIR/mihomo-tool.tar.gz" -C "$TMP_DIR"

    # Install binary
    mkdir -p "$CONFIG_DIR"
    BINARY_TEMP_DIR="$TMP_DIR/mihomo-tool-${OS_FOR_DOWNLOAD}-$GOARCH"
    cp "$BINARY_TEMP_DIR/mihomo-tool" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/mihomo-tool"

    # Copy essential files if missing
    if [ ! -f "$CONFIG_DIR/index.html" ]; then
        cp "$BINARY_TEMP_DIR/index.html" "$CONFIG_DIR/"
        cp -r "$BINARY_TEMP_DIR/css" "$CONFIG_DIR/"
        cp -r "$BINARY_TEMP_DIR/js" "$CONFIG_DIR/"
    fi

    log "Installed binary to $INSTALL_DIR/mihomo-tool"

    # Service Management
    if [ "$OS_TYPE" == "openwrt" ]; then
        log "Configuring procd service for OpenWrt..."
        cat <<EOF > /etc/init.d/mihomo-tool
#!/bin/sh /etc/rc.common

START=99
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command "$INSTALL_DIR/mihomo-tool"
    procd_set_param stderr 1
    procd_set_param stdout 1
    procd_set_param respawn
    procd_set_param chdir "$CONFIG_DIR"
    procd_close_instance
}
EOF
        chmod +x /etc/init.d/mihomo-tool
        /etc/init.d/mihomo-tool enable
        /etc/init.d/mihomo-tool start
    elif [ "$OS_TYPE" == "alpine" ]; then
        log "Configuring OpenRC service for Alpine..."
        cat <<EOF > /etc/init.d/mihomo-tool
#!/sbin/openrc-run

description="Mihomo-Tool Service"
command="$INSTALL_DIR/mihomo-tool"
command_background="yes"
directory="$CONFIG_DIR"
pidfile="/run/mihomo-tool.pid"

depend() {
    need net
    after firewall
}
EOF
        chmod +x /etc/init.d/mihomo-tool
        rc-update add mihomo-tool default
        rc-service mihomo-tool start
    else
        log "Configuring systemd service..."
        cat <<EOF > /etc/systemd/system/mihomo-tool.service
[Unit]
Description=Mihomo-Tool Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$CONFIG_DIR
ExecStart=$INSTALL_DIR/mihomo-tool
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload
        systemctl enable mihomo-tool
        systemctl start mihomo-tool
    fi

    log "Mihomo-Tool installed and started successfully!"
    log "Web UI available at http://<your-ip>:58888"

    # Cleanup
    rm -rf "$TMP_DIR"
}

# Subcommand handling
ACTION=${1:-install}

case $ACTION in
    install)
        do_install
        ;;
    proxy-install)
        export USE_PROXY="true"
        do_install
        ;;
    uninstall)
        do_uninstall
        ;;
    *)
        echo "Usage: $0 [install|proxy-install|uninstall]"
        exit 1
        ;;
esac
