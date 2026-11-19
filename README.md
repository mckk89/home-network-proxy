# 家庭网络反代 (Home Network Proxy)

> 一个界面管理你的家庭/个人网络：动态域名 (DDNS)、反向代理、端口转发、证书管理、文件服务、Web 终端、Docker 管理、WAF、计划任务、网络唤醒……

![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)
![Go 1.21+](https://img.shields.io/badge/go-1.21%2B-00ADD8?logo=go)
![Docker](https://img.shields.io/badge/docker-multi--arch-blue)

**中文** | [English](README.md)

---

## 目录

- [项目介绍](#-项目介绍)
- [功能特性](#-功能特性)
- [项目结构](#-项目结构)
- [安装部署](#-安装部署)
  - [方式一：Docker 容器部署（推荐）](#方式一docker-容器部署推荐)
  - [方式二：二进制安装（Linux / macOS / FreeBSD）](#方式二二进制安装linux--macos--freebsd)
  - [方式三：OpenWrt 部署](#方式三openwrt-部署)
  - [方式四：Windows 服务安装](#方式四windows-服务安装)
  - [方式五：从源码构建](#方式五从源码构建)
- [快速开始](#-快速开始)
- [配置说明](#-配置说明)
- [安全建议](#-安全建议)

---

## 📖 项目介绍

**家庭网络反代（Home Network Proxy，简称 hnproxy）** 是一款面向个人和家庭网络的可视化网络管理工具。

它最初为在路由器上实现「公网 IPv6 → 内网 IPv4 端口转发」而开发，如今也可运行在 NAS、Linux 服务器、macOS、Windows、OpenWrt 路由器上。你可以从一个 Web 界面统一管理常用的网络服务，而不必为每个任务单独维护一套工具：

- 域名解析（DDNS）
- Web 反向代理 / 端口转发
- SSL 证书申请与续期
- 文件服务（WebDAV / FTP / SMB）
- 内网穿透（STUN / Cloudflare Tunnel / FRP）
- 设备自动化（网络唤醒 / 计划任务 / Web 终端 / Docker 管理）
- 安全防护（WAF / IP 黑白名单 / 认证）

### 核心特性

| 场景 | 功能 |
| --- | --- |
| 域名与服务访问 | 动态域名（DDNS，支持阿里云/DNSPod/Cloudflare/DuckDNS/No-IP 等 7 家）、Web 反向代理（含 WebSocket）、TCP/UDP 端口转发、SSL/TLS 证书申请与续期 |
| 内网穿透 | STUN、Cloudflared Tunnel、FRP 相关能力（按构建版本启用） |
| 文件与存储 | 存储管理、WebDAV、FTP、SMB 服务，以及 DLNA / FileBrowser / Rclone（按构建版本） |
| 设备与自动化 | 网络唤醒（WOL）、计划任务（cron/间隔/单次）、Docker 容器管理、Web 终端 |
| 安全与认证 | IP 白/黑名单、WAF（检测/防护双模式）、第三方认证、Passkey |
| 日常管理 | Web 管理界面（18 个页面）、日志中心、配置备份与恢复 |

### 版本变体

项目提供 7 种构建变体，功能范围不同（类似原 Lucky 的 XiaoJV/Lucky/万吉体系）：

| 变体 | 标签 | 说明 |
| --- | --- | --- |
| 轻量版 | `lite` | 最精简：核心网络功能 |
| 轻量 WAF 版 | `lite_waf` | 轻量版 + Coraza WAF + gRPC 代理 |
| 标准版 | `standard` | 基础功能集（+ 存储/FTP/WebDAV/WOL/终端） |
| 标准 Docker 版 | `standard_docker` | 标准版 + Docker 容器管理 |
| 增强版 | `medium` | 中等功能集（+ DLNA/CF Tunnel/FRP/SMB） |
| 完整版 | `full` | 完整功能（+ FileBrowser/Rclone） |
| 完整 Docker 版 | `full_docker`（默认镜像） | 完整功能 + Docker 容器管理 |

> 各变体之间的**配置文件通用**，可以自由切换；切换后实际可用功能以目标构建包含的模块为准。

### 支持平台

- **二进制**：Linux (x86_64 / arm64 / armv7 / armv6 / i386 / mips64le / mipsle / riscv64)、macOS (x86_64 / arm64)、Windows (x86_64 / arm64)、FreeBSD (x86_64)
- **容器**：多架构 Docker 镜像（Alpine，约 12MB 未压缩 / 4MB 压缩）
- **路由器**：OpenWrt IPK 包、一键脚本安装

---

## ⚙️ 项目结构

```
home-network-proxy/
├── main.go                     # 程序入口（参数解析、信号处理、优雅关闭）
├── go.mod / go.sum             # Go 模块定义
├── Dockerfile                  # 多阶段多架构 Docker 构建（7 个变体 target）
├── docker-compose.yml          # Compose 部署模板（全部 7 个变体）
├── docker-entrypoint.sh        # 容器入口脚本（自动建配置、修权限、处理信号）
├── Makefile                    # 本地构建/测试/发布命令
├── LICENSE                     # MIT 许可证
├── config/
│   └── hnproxy.conf.example    # 配置文件示例（含全部可配置项注释）
├── internal/
│   ├── config/                 # YAML/JSON 配置加载、默认值、保存
│   ├── ddns/                   # DDNS：7 家服务商、公网 IP 探测、变更检测
│   ├── reverseproxy/           # 反向代理：HTTP/HTTPS/WebSocket、域名+路径匹配
│   ├── socketproxy/            # TCP/UDP 端口转发
│   ├── webdav/                 # WebDAV：PROPFIND/PUT/DELETE/MKCOL/COPY/MOVE
│   ├── ftp/                    # FTP 服务（基础协议框架）
│   ├── ssl/                    # SSL 证书管理（自签名 + ACME 接口）
│   ├── tasks/                  # 计划任务（cron / 间隔 / 单次）
│   ├── terminal/               # Web 终端（WebSocket + shell）
│   ├── docker/                 # Docker 容器管理 API
│   ├── waf/                    # WAF：IP 黑白名单 + 检测/防护模式
│   ├── wol/                    # 网络唤醒（Wake-on-LAN 魔术包）
│   ├── module/                 # 模块注册系统（26 个可开关模块）
│   └── web/
│       ├── server.go           # Web 管理界面（Gin，18 个页面 + REST API）
│       ├── templates/          # 内嵌 HTML 模板
│       └── static/             # 内嵌静态资源
├── scripts/
│   ├── install.sh              # Linux/macOS/FreeBSD 自动安装脚本（服务管理）
│   ├── daji.sh                 # 离线安装脚本
│   ├── build.sh                # 多平台交叉编译 + 打包 + SHA256
│   ├── openwrt/                # OpenWrt IPK 包构建（Makefile + init + UCI）
│   └── windows/                # Windows 服务安装/卸载 PowerShell 脚本
└── dist/                       # 构建产物（二进制 + tar.gz + SHA256SUMS）
```

---

## 🚀 安装部署

> **部署前请先选择构建变体**（见 [版本变体](#-版本变体)）。下文命令以默认镜像（完整 Docker 版）为例，其他变体替换镜像标签即可。
>
> ⚠️ **镜像说明**：仓库只提供源码，**不包含已发布的 Docker 镜像**。下文统一以 `mckk89/hnproxy` 作为示例镜像名，请先按 [5.3 节](#53-多架构-docker-镜像构建) 自行构建并推送到你的镜像仓库，或把命令中的镜像名替换为你自己的镜像。

### 方式一：Docker 容器部署（推荐）

#### 1.1 直接运行容器（Linux host 网络模式，推荐）

```bash
# 创建配置目录（宿主机持久化配置，删除容器后配置不丢失）
mkdir -p /root/hnproxyconf

# 启动容器
docker run -d \
  --name hnproxy \
  --restart=always \
  --net=host \
  -v /root/hnproxyconf:/app/conf \
  -v /var/run/docker.sock:/var/run/docker.sock \   # 仅完整 Docker 版需要（Docker 管理功能）
  mckk89/hnproxy:v3

# 查看日志
docker logs -f hnproxy

# 验证
curl -s http://127.0.0.1:16601/health    # 期望返回 {"status":"ok"}
```

> `--net=host` 仅 Linux 支持。这样容器直接使用宿主机网络栈，**支持 IPv6**，且端口 80/443 等无需映射，性能最佳。
> **Windows / macOS** 的 Docker Desktop 不支持 host 网络模式，请改用 1.2 的端口映射方式。

#### 1.2 端口映射模式（Windows / macOS / 通用）

```bash
docker run -d \
  --name hnproxy \
  --restart=always \
  -p 16601:16601 \
  -p 80:80 \
  -p 443:443 \
  -v /path/to/hnproxyconf:/app/conf \
  mckk89/hnproxy:v3_standard_docker
```

⚠️ 端口映射模式下 IPv6 不可用；如端口访问异常请优先改回 host 模式。

#### 1.3 Docker Compose 部署（推荐长期使用）

```bash
cd /opt && mkdir hnproxy && cd hnproxy
```

创建 `docker-compose.yml`（只保留需要的那一个 service）：

```yaml
services:
  hnproxy:
    image: mckk89/hnproxy:v3   # 完整 Docker 版（默认）
    # 其他变体标签：
    #   v3_standard_docker  标准 Docker 版
    #   v3_lite_waf         轻量 WAF 版
    #   v3_full             完整版（无 Docker 管理）
    #   v3_medium           增强版
    #   v3_standard         标准版
    #   v3_lite             轻量版
    container_name: hnproxy
    network_mode: host                     # Windows/macOS 注释掉本行并放开下方 ports
    # ports:
    #   - "16601:16601"
    #   - "80:80"
    #   - "443:443"
    volumes:
      - ./data:/app/conf                   # 配置持久化
      - /var/run/docker.sock:/var/run/docker.sock   # 仅 Docker 管理变体需要
    restart: always
    environment:
      - TZ=Asia/Shanghai
```

```bash
mkdir -p data
docker compose up -d
docker compose logs -f
```

#### 1.4 容器内路径说明

| 容器内路径 | 说明 |
| --- | --- |
| `/lucky` | 程序二进制 |
| `/app/conf` | 配置目录（挂载到宿主机） |
| `/app/conf/hnproxy.conf` | 主配置文件（首次启动自动生成） |
| `/app/conf/secrets.yaml` | DDNS 等敏感凭据（权限 0600） |
| `/app/conf/certs/` | SSL 证书存放目录 |

**常用运维命令：**

```bash
docker exec -it hnproxy sh                          # 进入容器
docker restart hnproxy                               # 重启
docker stop hnproxy && docker rm hnproxy            # 停止并删除（配置保留在 data/）
docker compose pull && docker compose up -d          # 升级（先备份 ./data 目录）
```

---

### 方式二：二进制安装（Linux / macOS / FreeBSD）

#### 2.1 一键自动安装脚本

脚本自动完成：下载对应架构二进制 → 安装到 `/usr/local/bin` → 生成配置文件 → 注册系统服务并开机自启 → 启动。

```bash
# 在线一键安装（从本仓库 main 分支拉取脚本，安装包从本仓库 Releases 下载）
VERSION=3.5.0 sh -c "$(curl -fsSL https://raw.githubusercontent.com/mckk89/home-network-proxy/main/scripts/install.sh)"

# 或先下载脚本再执行（便于查看内容）
curl -fsSL -o install.sh https://raw.githubusercontent.com/mckk89/home-network-proxy/main/scripts/install.sh
sudo sh install.sh --version 3.5.0 --product full

# 指定产品变体（默认 full）
sudo sh install.sh --version 3.5.0 --product standard

# 预演（不实际安装）
sh install.sh --dry-run
```

脚本选项：

| 选项 | 说明 |
| --- | --- |
| `--version 3.5.0` | 指定版本 |
| `--product standard` | 指定变体：full/standard/medium/lite/lite_waf/full_docker/standard_docker |
| `--dir /opt/hnproxy` | 自定义安装目录 |
| `--arch arm64` | 手动指定架构（自动检测失败时） |
| `--no-start` | 安装并注册服务但不启动 |
| `--update` | 仅更新核心二进制（保留服务与配置） |
| `--uninstall` | 卸载（保留配置） |

各系统服务注册方式（脚本自动选择）：

| 系统 | 服务方式 |
| --- | --- |
| Linux + systemd | `/etc/systemd/system/hnproxy.service`（含安全加固参数） |
| Linux + init.d / OpenWrt | `/etc/init.d/hnproxy`（procd 脚本） |
| macOS | `/Library/LaunchDaemons/com.hnproxy.plist` |
| FreeBSD | `/usr/local/etc/rc.d/hnproxy` |

#### 2.2 手动安装（适合离线/定制环境）

```bash
# 1. 下载对应架构的 tar.gz（GitHub Releases 或 dist 目录）
#    命名规则: hnproxy_<版本>_<系统>_<架构>.tar.gz
#    架构对照: x86_64/arm64/armv7/armv6/i386/mips64le/mipsle/riscv64

# 2. 解压并安装
tar -xzf hnproxy_3.5.0_linux_arm64.tar.gz
sudo install -m 755 hnproxy /usr/local/bin/hnproxy

# 3. 创建配置
sudo mkdir -p /etc/hnproxy
sudo cp config/hnproxy.conf.example /etc/hnproxy/hnproxy.conf

# 4. 手动运行（前台）
sudo /usr/local/bin/hnproxy -c /etc/hnproxy/hnproxy.conf

# 5. 注册开机自启（systemd 示例）
sudo tee /etc/systemd/system/hnproxy.service > /dev/null <<'EOF'
[Unit]
Description=Home Network Proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/hnproxy -c /etc/hnproxy/hnproxy.conf
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now hnproxy
```

#### 2.3 命令行参数

| 参数 | 说明 |
| --- | --- |
| `-c /path/hnproxy.conf` | 指定配置文件（不存在则自动创建） |
| `-cd /path` | 指定配置目录（2.x 风格） |
| `-version` | 显示版本号 |
| `-info` | 显示构建信息 |
| `-r CancelSafeURL` | 取消安全入口 |
| `-r Disable2FA` | 禁用双因子认证 |
| `-r ResetUser` | 重置账号密码为 666/666 |
| `-r Restart` | 重启程序 |
| `-r SetHttpAdminPort` / `-r SetHttpsAdminPort` | 修改管理端口 |
| `-r Unlock` | 解锁登录限制 |

#### 2.4 离线安装（无法访问外网）

```bash
# 1. 在有网机器下载对应架构 tar.gz，拷贝到目标机 /tmp
# 2. 执行离线脚本
chmod +x daji.sh
sh daji.sh            # 自动识别 CPU 架构，从 /tmp 读取包
```

---

### 方式三：OpenWrt 部署

#### 3.1 IPK 包安装（需自行构建）

> 本仓库**暂未随 Release 发布 IPK 包**。如需 IPK，请使用 `scripts/openwrt/` 下的 Makefile 在 OpenWrt SDK 中自行编译；也可直接用 3.2 的脚本安装 tar 二进制。

编译参考：

```bash
# 在 OpenWrt SDK 目录中
cp -r scripts/openwrt package/hnproxy
make package/hnproxy/compile V=s
# 产物在 bin/packages/<架构>/base/ 下
```

安装（编译出包后）：

```bash
opkg update && opkg install curl
opkg install hnproxy_<版本>_<架构>.ipk
opkg install luci-app-hnproxy_<版本>_all.ipk   # 如构建了 LuCI 插件
opkg install luci-i18n-hnproxy-zh-cn_<版本>_all.ipk
/etc/init.d/hnproxy enable && /etc/init.d/hnproxy start
```

> ⚠️ 安装前请确认已卸载所有第三方 hnproxy IPK 包，并删除残留的 `/etc/config/hnproxy` 和 `/etc/init.d/hnproxy`。

#### 3.2 一键脚本安装（推荐，可直接用）

```bash
# 先安装 curl（部分精简固件没有）
opkg update && opkg install curl

# 安装（脚本自动识别 tar 包架构；OpenWrt 无 IPK 时回退 tar + procd 服务）
VERSION=3.5.0 sh -c "$(curl -fsSL https://raw.githubusercontent.com/mckk89/home-network-proxy/main/scripts/install.sh)"
```

---

### 方式四：Windows 服务安装

```powershell
# 1. 下载 hnproxy_3.5.0_windows_x86_64.zip（管理员 PowerShell）
# 2. 解压到 C:\Program Files\HNProxy
Expand-Archive .\hnproxy_3.5.0_windows_x86_64.zip -DestinationPath "C:\Program Files\HNProxy"

# 3. 安装为 Windows 服务（开机自启 + 失败自动重启）
cd "C:\Program Files\HNProxy"
.\install_service.ps1

# 常用命令
net start HNProxy        # 启动
net stop HNProxy         # 停止
sc query HNProxy         # 状态

# 4. 卸载
.\uninstall_service.ps1
```

> 服务模式下托盘不显示图标，卸载请通过脚本或后台「设置」页面完成。安装服务需要**管理员权限**。
> 直接双击 `hnproxy.exe` 也可运行（配置文件放在同目录 `hnproxy.conf`）。

---

### 方式五：从源码构建

#### 5.1 前置要求

- Go 1.21+
- 各平台交叉编译零额外依赖（CGO_ENABLED=0）

#### 5.2 构建

```bash
git clone https://github.com/mckk89/home-network-proxy.git
cd home-network-proxy

# 当前平台
go build -ldflags="-s -w -X main.Version=3.5.0" -o hnproxy .

# 全部平台 + 打包 + SHA256（产物输出到 dist/<版本>/）
./scripts/build.sh 3.5.0
```

#### 5.3 多架构 Docker 镜像构建

```bash
# 需要 docker buildx
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6,linux/386,linux/mips64le,linux/mipsle,linux/riscv64 \
  -t mckk89/hnproxy:v3 \
  --push .

# 构建单个变体
docker build -t mckk89/hnproxy:v3_standard_docker --target standard_docker .
```

#### 5.4 测试

```bash
go test -timeout 60s ./...
go vet ./...
```

---

## 🏁 快速开始

1. **启动**：按上述任一方式部署，默认管理地址 `http://设备IP:16601`
2. **登录**：默认账号 `666`，密码 `666`
3. **安全加固（必做）**：
   - 立即修改默认用户名/密码（设置页面）
   - 设置**安全入口**（safe_url），访问地址变为 `http://IP:16601/你的入口/`
   - 外网暴露时启用 HTTPS
4. **配置服务**（后台「快速上手」或直接编辑配置）：
   - DDNS：绑定域名 + 选择服务商，在 `secrets.yaml` 填凭据
   - 反向代理：`域名|目标地址|SSL|WebSocket` 规则
   - 端口转发：`名称|tcp/udp|监听地址|目标地址`
5. **备份**：设置页底部定期备份配置（升级前务必单独保留一份）

---

## ⚙️ 配置说明

配置文件默认位置：`/etc/hnproxy/hnproxy.conf`（Docker 中为 `/app/conf/hnproxy.conf`）

```yaml
web:
  port: 16601                     # 管理界面端口
  username: "admin"
  password: "your_secure_password"
  safe_url: "mysecret"            # 安全入口，访问 /mysecret/
  enable_https: false
  cert_file: ""
  key_file: ""
  disable_2fa: false

ddns:
  enabled: true
  providers:                      # 域名 -> 服务商
    "example.com": "alidns"
    "sub.example.com": "cloudflare"
  interval: 300                   # 检查间隔（秒）
  ipv4_enabled: true
  ipv6_enabled: true

reverse_proxy:
  enabled: true
  services:
    # 格式: 名称|域名|目标|启用|SSL|证书|私钥|WebSocket|超时秒|重试|路径前缀
    - "web|example.com|http://localhost:8080|true|false|||false|30|3|"
    - "api|api.example.com|http://localhost:3000|true|true|cert.pem|key.pem|true|30|3|/api"

socket_proxy:
  enabled: true
  services:
    # 格式: 名称|协议|监听地址|目标地址|启用
    - "ssh|tcp|0.0.0.0:2222|192.168.1.100:22|true"
    - "dns|udp|0.0.0.0:5333|8.8.8.8:53|true"

modules:
  enabled: ["ddns", "reverse_proxy", "socket_proxy", "ssl", "tasks", "auth", "logs", "webui"]
  disabled: []

storage:
  path: "/app/conf"               # 配置/数据根目录

security:
  ip_whitelist: ["192.168.1.0/24"]
  ip_blacklist: []
  enable_waf: true
  waf_mode: "prevention"          # detection=仅记录, prevention=拦截

# ==================== DDNS 敏感凭据（存 secrets.yaml，权限 0600）====================
# alidns:
#   access_key_id: "LTAI..."
#   access_key_secret: "..."
# dnspod:
#   api_key: "ID,Token"
# cloudflare:
#   api_token: "..."
#   zone_id: "..."
# callback:
#   url: "https://api.example.com/update?domain={domain}&ip={ip}"
# he:
#   account_id: "..."
# noip:
#   username: "..."
#   password: "..."
# duckdns:
#   token: "..."
```

**内网 IP 识别**：在配置目录放置 `localips` 文件（每行一个 IP/网段）可自定义内网范围；文件非空时忽略默认内置网段（10/8、172.16/12、169.254/16、192.168/16）。

---

## 🔒 安全建议

- **立即修改默认账号密码**，并设置复杂的安全入口
- 隐藏入口**不能替代认证和防火墙**，外网访问务必用 HTTPS
- Docker 管理 / Web 终端 / 文件服务请严格限制访问 IP（白名单）
- 不要把 `/var/run/docker.sock` 挂载给不可信容器
- 配置和备份按**明文敏感信息**保管，泄露后立即更换所有密码/令牌
- 升级、迁移、降级之前**单独备份一份**当前版本配置

---

## 📄 许可证

[MIT](LICENSE)

## 🔗 相关

- 构建产物目录：`dist/<版本>/`（12 个平台二进制 + tar.gz + SHA256SUMS）
- 反馈问题：GitHub Issues（请附版本、环境、现象、排查步骤，移除敏感信息）
