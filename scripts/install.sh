#!/bin/bash
# 家庭网络反代 自动安装脚本 v3
# 支持：Linux (systemd/init.d), OpenWrt, macOS (launchd), FreeBSD
# 支持架构：x86_64, arm64, armv7, armv6, i386, mips64le, mipsle, riscv64

set -euo pipefail

# ==================== 配置变量 ====================
RELEASE_URL="${RELEASE_URL:-https://github.com/mckk89/home-network-proxy/releases/download}"
VERSION="${VERSION:-latest}"
PRODUCT="${PRODUCT:-full}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/hnproxy}"
SERVICE_NAME="hnproxy"
BINARY_NAME="hnproxy"
GITHUB_REPO="mckk89/home-network-proxy"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ==================== 工具函数 ====================
log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

die() { log_error "$*"; exit 1; }

has_cmd() { command -v "$1" >/dev/null 2>&1; }

is_root() { [ "$(id -u)" -eq 0 ]; }

detect_os() {
    case "$(uname -s)" in
        Linux) OS="linux" ;;
        Darwin) OS="darwin" ;;
        FreeBSD) OS="freebsd" ;;
        *) die "不支持的操作系统: $(uname -s)" ;;
    esac
}

detect_arch() {
    local machine=$(uname -m)
    case "$machine" in
        x86_64|amd64) ARCH="x86_64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7*|armv7l) ARCH="armv7" ;;
        armv6*) ARCH="armv6" ;;
        i386|i686) ARCH="i386" ;;
        mips64le) ARCH="mips64le" ;;
        mipsle|mips) ARCH="mipsle" ;;
        riscv64) ARCH="riscv64" ;;
        *) die "不支持的架构: $machine" ;;
    esac
}

detect_init_system() {
    if [ "$OS" = "linux" ]; then
        if [ -d /run/systemd/system ] && has_cmd systemctl; then
            INIT_SYSTEM="systemd"
        elif [ -f /etc/init.d/hnproxy ] || [ -d /etc/init.d ] && has_cmd service; then
            INIT_SYSTEM="initd"
        elif [ -f /etc/rc.common ] && has_cmd /etc/init.d; then
            INIT_SYSTEM="openwrt"
        else
            INIT_SYSTEM="unknown"
        fi
    elif [ "$OS" = "darwin" ]; then
        INIT_SYSTEM="launchd"
    elif [ "$OS" = "freebsd" ]; then
        INIT_SYSTEM="freebsd"
    else
        INIT_SYSTEM="unknown"
    fi
}

# ==================== 下载函数 ====================
download_file() {
    local url="$1"
    local output="$2"
    log_info "下载: $url"
    
    if has_cmd curl; then
        curl -fL -o "$output" "$url" || return 1
    elif has_cmd wget; then
        wget -O "$output" "$url" || return 1
    elif has_cmd fetch; then
        fetch -o "$output" "$url" || return 1
    else
        die "需要 curl、wget 或 fetch 来下载文件"
    fi
}

get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        log_info "获取最新版本信息..."
        local api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
        local version=""
        
        if has_cmd curl; then
            version=$(curl -fsSL "$api_url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\([^"]*\)".*/\1/')
        elif has_cmd wget; then
            version=$(wget -qO- "$api_url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\([^"]*\)".*/\1/')
        fi
        
        if [ -n "$version" ]; then
            VERSION="$version"
            log_info "最新版本: $VERSION"
        else
            log_warn "无法获取最新版本，使用默认版本"
            VERSION="3.5.0"
        fi
    fi
}

download_binary() {
    local tmpdir=$(mktemp -d)
    local archive="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
    local url="${RELEASE_URL}/v${VERSION}/${archive}"
    
    log_info "下载 ${PRODUCT} ${VERSION} (${OS}/${ARCH})..."
    
    # 尝试从 GitHub Releases 下载
    local github_url="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${archive}"
    
    if download_file "$github_url" "${tmpdir}/${archive}"; then
        log_success "从 GitHub 下载成功"
    elif download_file "$url" "${tmpdir}/${archive}"; then
        log_success "从官方镜像下载成功"
    else
        # 尝试不带 v 前缀
        local url2="${RELEASE_URL}/${VERSION}/${archive}"
        local github_url2="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${archive}"
        
        if download_file "$github_url2" "${tmpdir}/${archive}"; then
            log_success "从 GitHub 下载成功"
        elif download_file "$url2" "${tmpdir}/${archive}"; then
            log_success "从官方镜像下载成功"
        else
            die "下载失败，请检查版本号和网络连接"
        fi
    fi
    
    # 解压
    tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"
    
    # 查找二进制文件
    local binary_path=$(find "$tmpdir" -name "$BINARY_NAME" -type f | head -1)
    if [ -z "$binary_path" ]; then
        die "解压后未找到二进制文件"
    fi
    
    echo "$binary_path"
    rm -rf "$tmpdir"
}

# ==================== 安装函数 ====================
install_binary() {
    local src="$1"
    log_info "安装二进制文件到 $INSTALL_DIR/$BINARY_NAME"
    
    mkdir -p "$INSTALL_DIR"
    cp "$src" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    # 创建配置目录
    mkdir -p "$CONFIG_DIR"
    
    log_success "二进制文件安装完成"
}

create_config() {
    local config_file="$CONFIG_DIR/hnproxy.conf"
    
    if [ ! -f "$config_file" ]; then
        log_info "创建默认配置文件: $config_file"
        cat > "$config_file" <<EOF
# 家庭网络反代 配置文件
# 详细配置请参考: https://lucky666.cn/docs/intro

web:
  port: 16601
  username: "666"
  password: "666"
  safe_url: ""
  enable_https: false
  cert_file: ""
  key_file: ""
  disable_2fa: false

ddns:
  enabled: false
  providers: {}
  interval: 300
  ipv4_enabled: true
  ipv6_enabled: true

reverse_proxy:
  enabled: false
  services: []

socket_proxy:
  enabled: false
  services: []

modules:
  enabled: []
  disabled: []

storage:
  path: "$CONFIG_DIR"

security:
  ip_whitelist: []
  ip_blacklist: []
  enable_waf: false
  waf_mode: "detection"
EOF
        log_success "默认配置文件已创建"
    else
        log_info "配置文件已存在，跳过创建"
    fi
}

install_systemd() {
    log_info "安装 systemd 服务..."
    
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=家庭网络反代 - Personal and Home Network Tool
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=${INSTALL_DIR}/${BINARY_NAME} -c ${CONFIG_DIR}/hnproxy.conf
Restart=always
RestartSec=5
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${CONFIG_DIR}

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    log_success "systemd 服务已安装并设置开机自启"
}

install_initd() {
    log_info "安装 init.d 服务..."
    
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
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
EOF

    chmod +x "/etc/init.d/${SERVICE_NAME}"
    /etc/init.d/"$SERVICE_NAME" enable
    log_success "init.d 服务已安装并设置开机自启"
}

install_launchd() {
    log_info "安装 launchd 服务..."
    
    local plist="/Library/LaunchDaemons/com.hnproxy.plist"
    
    cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.hnproxy</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
        <string>-c</string>
        <string>${CONFIG_DIR}/hnproxy.conf</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/hnproxy.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/hnproxy.error.log</string>
</dict>
</plist>
EOF

    launchctl load "$plist"
    launchctl enable "system/com.hnproxy"
    log_success "launchd 服务已安装并设置开机自启"
}

install_freebsd() {
    log_info "安装 FreeBSD rc.d 服务..."
    
    cat > "/usr/local/etc/rc.d/hnproxy" <<'EOF'
#!/bin/sh

. /etc/rc.subr

name="hnproxy"
rcvar="hnproxy_enable"
command="/usr/local/bin/hnproxy"
command_args="-c /etc/hnproxy/hnproxy.conf"
pidfile="/var/run/hnproxy.pid"
required_dirs="/etc/hnproxy"

load_rc_config $name
: ${hnproxy_enable:="NO"}

run_rc_command "$1"
EOF

    chmod +x "/usr/local/etc/rc.d/hnproxy"
    sysrc "hnproxy_enable=YES"
    log_success "FreeBSD 服务已安装并设置开机自启"
}

install_openwrt() {
    log_info "检测到 OpenWrt 系统"
    log_info "本仓库暂未随 Release 发布 IPK；可继续使用 tar 二进制 + procd 服务安装"
    log_info "如需 IPK，请用 scripts/openwrt/ 下的 Makefile 在 OpenWrt SDK 中自行编译"
    
    # 仍然尝试安装二进制文件
    install_binary "$1"
    create_config
    install_initd
}

# ==================== 服务管理 ====================
start_service() {
    log_info "启动 $SERVICE_NAME 服务..."
    case "$INIT_SYSTEM" in
        systemd) systemctl start "$SERVICE_NAME" ;;
        initd) /etc/init.d/"$SERVICE_NAME" start ;;
        launchd) launchctl start "system/com.hnproxy" ;;
        freebsd) service hnproxy start ;;
        openwrt) /etc/init.d/"$SERVICE_NAME" start ;;
        *) log_warn "未知的初始化系统，请手动启动: $INSTALL_DIR/$BINARY_NAME -c $CONFIG_DIR/hnproxy.conf" ;;
    esac
    
    sleep 2
    if check_service; then
        log_success "服务启动成功"
    else
        log_error "服务启动失败，请检查日志"
        return 1
    fi
}

check_service() {
    case "$INIT_SYSTEM" in
        systemd) systemctl is-active --quiet "$SERVICE_NAME" ;;
        initd) /etc/init.d/"$SERVICE_NAME" status >/dev/null 2>&1 ;;
        launchd) launchctl print "system/com.hnproxy" >/dev/null 2>&1 ;;
        freebsd) service hnproxy status >/dev/null 2>&1 ;;
        openwrt) /etc/init.d/"$SERVICE_NAME" status >/dev/null 2>&1 ;;
        *) pgrep -f "$BINARY_NAME.*-c.*$CONFIG_DIR" >/dev/null ;;
    esac
}

stop_service() {
    log_info "停止 $SERVICE_NAME 服务..."
    case "$INIT_SYSTEM" in
        systemd) systemctl stop "$SERVICE_NAME" ;;
        initd) /etc/init.d/"$SERVICE_NAME" stop ;;
        launchd) launchctl stop "system/com.hnproxy" ;;
        freebsd) service hnproxy stop ;;
        openwrt) /etc/init.d/"$SERVICE_NAME" stop ;;
        *) pkill -f "$BINARY_NAME.*-c.*$CONFIG_DIR" ;;
    esac
}

restart_service() {
    stop_service
    sleep 1
    start_service
}

# ==================== 卸载函数 ====================
uninstall() {
    log_info "卸载 家庭网络反代..."
    
    stop_service
    
    case "$INIT_SYSTEM" in
        systemd)
            systemctl disable "$SERVICE_NAME" 2>/dev/null || true
            rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
            systemctl daemon-reload
            ;;
        initd)
            /etc/init.d/"$SERVICE_NAME" disable 2>/dev/null || true
            rm -f "/etc/init.d/${SERVICE_NAME}"
            ;;
        launchd)
            launchctl disable "system/com.hnproxy" 2>/dev/null || true
            launchctl unload "/Library/LaunchDaemons/com.hnproxy.plist" 2>/dev/null || true
            rm -f "/Library/LaunchDaemons/com.hnproxy.plist"
            ;;
        freebsd)
            sysrc -x hnproxy_enable 2>/dev/null || true
            rm -f "/usr/local/etc/rc.d/hnproxy"
            ;;
        openwrt)
            /etc/init.d/"$SERVICE_NAME" disable 2>/dev/null || true
            rm -f "/etc/init.d/${SERVICE_NAME}"
            ;;
    esac
    
    rm -f "$INSTALL_DIR/$BINARY_NAME"
    log_success "程序文件已删除，配置目录 ($CONFIG_DIR) 已保留"
}

# ==================== 更新函数 ====================
update_binary() {
    log_info "更新 家庭网络反代 核心程序..."
    
    local was_running=false
    if check_service; then
        was_running=true
        stop_service
    fi
    
    local binary_path=$(download_binary)
    install_binary "$binary_path"
    
    if [ "$was_running" = true ]; then
        start_service
    fi
    
    log_success "核心程序更新完成"
}

# ==================== 显示信息 ====================
show_info() {
    echo
    echo "========================================"
    echo "  家庭网络反代 安装信息"
    echo "========================================"
    echo "版本: $VERSION"
    echo "产品: $PRODUCT"
    echo "系统: $OS ($ARCH)"
    echo "初始化系统: $INIT_SYSTEM"
    echo "安装目录: $INSTALL_DIR"
    echo "配置目录: $CONFIG_DIR"
    echo "二进制文件: $INSTALL_DIR/$BINARY_NAME"
    echo "配置文件: $CONFIG_DIR/hnproxy.conf"
    echo "服务名称: $SERVICE_NAME"
    echo "Web 界面: http://$(hostname -I | awk '{print $1}'):16601"
    echo "默认账号: 666"
    echo "默认密码: 666"
    echo "========================================"
    echo
    echo "常用命令:"
    echo "  启动:   systemctl start hnproxy   (或 service hnproxy start)"
    echo "  停止:   systemctl stop hnproxy"
    echo "  重启:   systemctl restart hnproxy"
    echo "  状态:   systemctl status hnproxy"
    echo "  日志:   journalctl -u hnproxy -f"
    echo "  卸载:   $0 --uninstall"
    echo "  更新:   $0 --update"
    echo
}

# ==================== 主函数 ====================
main() {
    # 解析参数
    local action="install"
    local show_help=false
    local dry_run=false
    
    while [ $# -gt 0 ]; do
        case "$1" in
            --uninstall) action="uninstall" ;;
            --update) action="update" ;;
            --version) VERSION="$2"; shift ;;
            --product) PRODUCT="$2"; shift ;;
            --dir) INSTALL_DIR="$2"; shift ;;
            --config-dir) CONFIG_DIR="$2"; shift ;;
            --release-url) RELEASE_URL="$2"; shift ;;
            --dry-run) dry_run=true ;;
            --help|-h) show_help=true ;;
            *) die "未知参数: $1" ;;
        esac
        shift
    done
    
    if [ "$show_help" = true ]; then
        cat <<EOF
家庭网络反代 自动安装脚本 v3

用法: $0 [选项]

选项:
  --version VERSION     指定版本 (默认: latest)
  --product PRODUCT     产品类型: full, standard, medium, lite, full_docker, standard_docker, lite_waf
  --dir DIR             安装目录 (默认: /usr/local/bin)
  --config-dir DIR      配置目录 (默认: /etc/hnproxy)
  --release-url URL     发布站点 URL (默认: https://github.com/mckk89/home-network-proxy/releases/download)
  --update              仅更新核心程序
  --uninstall           卸载程序 (保留配置)
  --dry-run             仅检查，不实际安装
  --help, -h            显示帮助

环境变量:
  RELEASE_URL, VERSION, PRODUCT, INSTALL_DIR, CONFIG_DIR

示例:
  # 自动安装最新版完整版
  curl -fsSL https://raw.githubusercontent.com/mckk89/home-network-proxy/main/scripts/install.sh | sh

  # 指定版本安装
  $0 --version 3.5.0 --product full

  # 仅更新核心
  $0 --update

  # 卸载
  $0 --uninstall
EOF
        exit 0
    fi
    
    # 检查 root 权限
    if [ "$action" != "update" ] && [ "$OS" = "linux" ] && ! is_root; then
        die "安装需要 root 权限，请使用 sudo"
    fi
    
    # 检测系统
    detect_os
    detect_arch
    detect_init_system
    get_latest_version
    
    log_info "系统: $OS/$ARCH, 初始化系统: $INIT_SYSTEM"
    log_info "版本: $VERSION, 产品: $PRODUCT"
    
    if [ "$dry_run" = true ]; then
        log_info "干运行模式，不执行实际安装"
        show_info
        exit 0
    fi
    
    case "$action" in
        install)
            local binary_path=$(download_binary)
            install_binary "$binary_path"
            create_config
            
            case "$INIT_SYSTEM" in
                systemd) install_systemd ;;
                initd) install_initd ;;
                launchd) install_launchd ;;
                freebsd) install_freebsd ;;
                openwrt) install_openwrt "$binary_path" ;;
                *) log_warn "未知初始化系统，仅安装二进制文件" ;;
            esac
            
            start_service
            show_info
            ;;
        update)
            update_binary
            ;;
        uninstall)
            uninstall
            ;;
    esac
}

# 运行主函数
main "$@"