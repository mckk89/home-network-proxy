#!/bin/sh
# 标准 Docker 入口脚本
# 支持配置文件自动生成、权限修复、服务启动

set -e

# 默认配置
CONFIG_DIR="${HNPROXY_CONFIG_DIR:-/app/conf}"
CONFIG_FILE="${CONFIG_DIR}/hnproxy.conf"
BINARY="/hnproxy"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# 显示启动横幅
show_banner() {
    cat <<'EOF'
  _   _ _   _ ____  ____   _____  _____   __
 | | | | \ | |  _ \|  _ \ / _ \ \/ / __|  \ \
 | |_| |  \| | |_) | |_) | | | \  /| _|    | |
 |  _  | |\  |  __/|  _ <| |_| /  \| |___  | |
 |_| |_|_| \_|_|   |_| \_\\___/_/\_\_____| | |
                                            
   家庭网络反代 / Home Network Proxy
EOF
    echo "版本: $(${BINARY} -version 2>/dev/null | head -1 || echo 'unknown')"
    echo "配置目录: ${CONFIG_DIR}"
    echo "配置文件: ${CONFIG_FILE}"
    echo ""
}

# 修复权限
fix_permissions() {
    log_info "检查目录权限..."
    
    # 确保配置目录存在
    mkdir -p "${CONFIG_DIR}"
    
    # 如果是 root 运行，尝试修复权限
    if [ "$(id -u)" = "0" ]; then
        # 如果指定了 PUID/PGID，修改文件所有者
        if [ -n "${PUID}" ] && [ -n "${PGID}" ]; then
            log_info "设置用户/组: ${PUID}:${PGID}"
            chown -R "${PUID}:${PGID}" "${CONFIG_DIR}" 2>/dev/null || true
        fi
    fi
}

# 生成默认配置
generate_config() {
    if [ ! -f "${CONFIG_FILE}" ]; then
        log_info "生成默认配置文件..."
        
        cat > "${CONFIG_FILE}" <<EOF
# 家庭网络反代 配置文件 - 自动生成于 $(date)
# 完整配置参考: https://lucky666.cn/docs/intro

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
  path: "${CONFIG_DIR}"

security:
  ip_whitelist: []
  ip_blacklist: []
  enable_waf: false
  waf_mode: "detection"
EOF
        log_success "默认配置文件已创建: ${CONFIG_FILE}"
        log_warn "请立即登录 Web 界面修改默认用户名和密码 (666/666)！"
        log_warn "建议设置安全入口 (safe_url) 并启用 HTTPS"
    else
        log_info "配置文件已存在，跳过生成"
    fi
}

# 检查配置
check_config() {
    log_info "检查配置文件..."
    
    if [ ! -f "${CONFIG_FILE}" ]; then
        log_error "配置文件不存在: ${CONFIG_FILE}"
        exit 1
    fi
    
    # 验证配置文件语法 (简单检查)
    if ! grep -q "web:" "${CONFIG_FILE}"; then
        log_warn "配置文件可能不完整，建议检查"
    fi
    
    log_success "配置文件检查通过"
}

# 处理信号
handle_signal() {
    log_info "收到关闭信号，正在停止..."
    kill -TERM "${child_pid}" 2>/dev/null || true
    wait "${child_pid}" 2>/dev/null || true
    log_success "服务已停止"
    exit 0
}

# 主函数
main() {
    show_banner
    fix_permissions
    generate_config
    check_config
    
    # 设置信号处理
    trap handle_signal TERM INT
    
    # 构建启动参数
    ARGS="-c ${CONFIG_FILE}"
    
    # 添加额外参数
    [ -n "${HNPROXY_ARGS}" ] && ARGS="${ARGS} ${HNPROXY_ARGS}"
    
    log_info "启动 家庭网络反代: ${BINARY} ${ARGS}"
    echo ""
    
    # 启动主程序
    exec ${BINARY} ${ARGS} &
    child_pid=$!
    
    # 等待子进程
    wait "${child_pid}"
}

# 运行主函数
main "$@"