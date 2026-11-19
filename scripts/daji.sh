#!/bin/bash
# 家庭网络反代 半自动离线安装脚本
# 适用于无法访问外网的环境
# 使用方法：
# 1. 将 hnproxy_<版本>_<系统>_<架构>.tar.gz 上传到 /tmp 目录
# 2. 运行 chmod +x daji.sh && sh daji.sh
# 3. 选择对应的架构进行安装

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

die() { log_error "$*"; exit 1; }

BINARY_NAME="hnproxy"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/hnproxy"
SERVICE_NAME="hnproxy"

detect_arch() {
    local machine=$(uname -m)
    case "$machine" in
        x86_64|amd64) echo "x86_64" ;;
        aarch64|arm64) echo "arm64" ;;
        armv7*|armv7l) echo "armv7" ;;
        armv6*) echo "armv6" ;;
        i386|i686) echo "i386" ;;
        mips64le) echo "mips64le" ;;
        mipsle|mips) echo "mipsle" ;;
        riscv64) echo "riscv64" ;;
        *) die "不支持的架构: $machine" ;;
    esac
}

detect_os() {
    case "$(uname -s)" in
        Linux) echo "linux" ;;
        Darwin) echo "darwin" ;;
        FreeBSD) echo "freebsd" ;;
        *) die "不支持的操作系统: $(uname -s)" ;;
    esac
}

list_packages() {
    log_info "当前 /tmp 目录下的家庭网络反代安装包："
    local packages=$(ls -1 /tmp/hnproxy_*.tar.gz 2>/dev/null | head -20)
    if [ -z "$packages" ]; then
        log_warn "未找到任何安装包"
        log_info "请将 hnproxy_<版本>_<系统>_<架构>.tar.gz 上传到 /tmp 目录"
        return 1
    fi
    echo "$packages" | nl -w2 -s'. '
}

install_package() {
    local package="$1"
    log_info "安装: $package"
    
    local tmpdir=$(mktemp -d)
    tar -xzf "$package" -C "$tmpdir"
    
    local binary=$(find "$tmpdir" -name "$BINARY_NAME" -type f | head -1)
    if [ -z "$binary" ]; then
        rm -rf "$tmpdir"
        die "包中未找到 $BINARY_NAME 二进制文件"
    fi
    
    # 安装二进制
    mkdir -p "$INSTALL_DIR"
    cp "$binary" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    # 创建配置目录
    mkdir -p "$CONFIG_DIR"
    
    # 创建默认配置
    local config_file="$CONFIG_DIR/hnproxy.conf"
    if [ ! -f "$config_file" ]; then
        cat > "$config_file" <<EOF
web:
  port: 16601
  username: "666"
  password: "666"
  safe_url: ""
  enable_https: false

ddns:
  enabled: false
  providers: {}
  interval: 300

reverse_proxy:
  enabled: false
  services: []

socket_proxy:
  enabled: false
  services: []

storage:
  path: "$CONFIG_DIR"

security:
  ip_whitelist: []
  ip_blacklist: []
  enable_waf: false
EOF
    fi
    
    # 安装服务
    install_service
    
    # 启动服务
    start_service
    
    rm -rf "$tmpdir"
    log_success "安装完成！"
    show_info
}

install_service() {
    local os=$(detect_os)
    
    if [ "$os" = "linux" ]; then
        if [ -d /run/systemd/system ] && command -v systemctl >/dev/null; then
            cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=家庭网络反代
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} -c ${CONFIG_DIR}/hnproxy.conf
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
            systemctl daemon-reload
            systemctl enable "$SERVICE_NAME"
        elif [ -f /etc/rc.common ]; then
            # OpenWrt
            cat > "/etc/init.d/${SERVICE_NAME}" <<'EOF'
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1
PROG=/usr/local/bin/hnproxy
CONFIG=/etc/hnproxy/hnproxy.conf
start_service() {
    procd_open_instance
    procd_set_param command "$PROG" "-c" "$CONFIG"
    procd_set_param respawn 5 3 0
    procd_close_instance
}
EOF
            chmod +x "/etc/init.d/${SERVICE_NAME}"
            /etc/init.d/"$SERVICE_NAME" enable
        else
            # 通用 init.d
            cat > "/etc/init.d/${SERVICE_NAME}" <<EOF
#!/bin/sh
### BEGIN INIT INFO
Provides:          hnproxy
Required-Start:    \$network \$syslog
Required-Stop:     \$network \$syslog
Default-Start:     2 3 4 5
Default-Stop:      0 1 6
Short-Description: 家庭网络反代 network tool
### END INIT INFO

NAME=hnproxy
DAEMON=${INSTALL_DIR}/${BINARY_NAME}
CONFIG=${CONFIG_DIR}/hnproxy.conf
PIDFILE=/var/run/hnproxy.pid

case "\$1" in
    start)
        echo "Starting \$NAME..."
        start-stop-daemon --start --background --make-pidfile --pidfile \$PIDFILE --exec \$DAEMON -- -c \$CONFIG
        ;;
    stop)
        echo "Stopping \$NAME..."
        start-stop-daemon --stop --pidfile \$PIDFILE
        ;;
    restart)
        \$0 stop
        sleep 1
        \$0 start
        ;;
    *)
        echo "Usage: \$0 {start|stop|restart}"
        exit 1
        ;;
esac
EOF
            chmod +x "/etc/init.d/${SERVICE_NAME}"
            update-rc.d "$SERVICE_NAME" defaults 2>/dev/null || true
        fi
    elif [ "$os" = "darwin" ]; then
        cat > "/Library/LaunchDaemons/com.hnproxy.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.hnproxy</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
        <string>-c</string>
        <string>${CONFIG_DIR}/hnproxy.conf</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
</dict>
</plist>
EOF
        launchctl load "/Library/LaunchDaemons/com.hnproxy.plist"
        launchctl enable "system/com.hnproxy"
    fi
}

start_service() {
    local os=$(detect_os)
    if [ "$os" = "linux" ]; then
        if [ -d /run/systemd/system ]; then
            systemctl start "$SERVICE_NAME"
        else
            /etc/init.d/"$SERVICE_NAME" start
        fi
    elif [ "$os" = "darwin" ]; then
        launchctl start "system/com.hnproxy"
    fi
    sleep 2
}

show_info() {
    local ip=$(hostname -I 2>/dev/null | awk '{print $1}' || ifconfig | grep 'inet ' | grep -v '127.0.0.1' | head -1 | awk '{print $2}')
    echo
    echo "========================================"
    echo "  家庭网络反代安装完成"
    echo "========================================"
    echo "Web 界面: http://${ip:-<你的IP>}:16601"
    echo "默认账号: 666"
    echo "默认密码: 666"
    echo "配置文件: $CONFIG_DIR/hnproxy.conf"
    echo "========================================"
    echo
    echo "重要提醒："
    echo "1. 请立即登录修改默认密码"
    echo "2. 建议设置安全入口"
    echo "3. 定期备份配置文件"
    echo
}

main() {
    echo "========================================"
    echo "  家庭网络反代离线安装脚本"
    echo "========================================"
    echo
    
    local arch=$(detect_arch)
    local os=$(detect_os)
    log_info "检测到系统: $os/$arch"
    
    if [ $# -eq 1 ] && [ "$1" = "list" ]; then
        list_packages
        exit 0
    fi
    
    local packages=($(ls -1 /tmp/hnproxy_*.tar.gz 2>/dev/null))
    if [ ${#packages[@]} -eq 0 ]; then
        log_error "未找到安装包"
        log_info "请将 hnproxy_<版本>_${os}_${arch}.tar.gz 上传到 /tmp 目录"
        exit 1
    fi
    
    if [ ${#packages[@]} -eq 1 ]; then
        install_package "${packages[0]}"
    else
        echo "发现多个安装包，请选择："
        for i in "${!packages[@]}"; do
            echo "  $((i+1)). ${packages[i]}"
        done
        echo -n "请输入编号: "
        read -r choice
        if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le ${#packages[@]} ]; then
            install_package "${packages[$((choice-1))]}"
        else
            die "无效选择"
        fi
    fi
}

main "$@"