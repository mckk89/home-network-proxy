# 家庭网络反代 v3.5.0 — 发布说明

> 一个 Web 界面管理家庭/个人网络：DDNS、反向代理、端口转发、SSL 证书、WebDAV/FTP、WAF、计划任务、Web 终端、Docker 管理。

## ✨ 本次发布

- 完整 Web 管理界面（18 个页面）
- 7 家 DDNS 服务商（阿里云 / DNSPod / Cloudflare / 回调 / HE / No-IP / DuckDNS）
- HTTP/HTTPS/WebSocket 反向代理，域名 + 路径匹配
- TCP/UDP 端口转发
- SSL 证书管理（自签名 + ACME 接口）
- WebDAV（PROPFIND/PUT/DELETE/MKCOL/COPY/MOVE）
- FTP 基础服务
- WAF（IP 黑白名单，检测/防护双模式）
- 计划任务（cron / 间隔 / 单次）
- Web 终端（WebSocket）
- Docker 容器管理
- Wake-on-LAN 网络唤醒
- 26 个可开关模块

## 📦 构建变体

| 变体 | Docker 标签 | 说明 |
| --- | --- | --- |
| 完整 Docker | `v3` | 完整功能 + Docker 管理（默认） |
| 标准 Docker | `v3_standard_docker` | 标准功能 + Docker 管理 |
| 轻量 WAF | `v3_lite_waf` | 轻量 + WAF + gRPC |
| 完整 | `v3_full` | 完整功能，无 Docker 管理 |
| 增强 | `v3_medium` | 中等功能集 |
| 标准 | `v3_standard` | 基础功能集 |
| 轻量 | `v3_lite` | 最精简 |

## ⬇️ 安装包

### 二进制（按平台选择）

| 文件 | 平台 |
| --- | --- |
| `hnproxy_3.5.0_linux_x86_64.tar.gz` | Linux 64 位（x86_64/amd64） |
| `hnproxy_3.5.0_linux_arm64.tar.gz` | Linux ARM 64 位（树莓派 4/5、NAS ARM、手机） |
| `hnproxy_3.5.0_linux_armv7.tar.gz` | Linux ARM 32 位 |
| `hnproxy_3.5.0_linux_armv6.tar.gz` | Linux ARMv6（老设备） |
| `hnproxy_3.5.0_linux_i386.tar.gz` | Linux 32 位 |
| `hnproxy_3.5.0_linux_mips64le.tar.gz` | Linux MIPS64LE |
| `hnproxy_3.5.0_linux_mipsle.tar.gz` | Linux MIPS32LE |
| `hnproxy_3.5.0_linux_riscv64.tar.gz` | Linux RISC-V 64 |
| `hnproxy_3.5.0_darwin_x86_64.tar.gz` | macOS Intel |
| `hnproxy_3.5.0_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `hnproxy_3.5.0_windows_x86_64.zip` | Windows 64 位 |
| `hnproxy_3.5.0_windows_arm64.zip` | Windows ARM64 |

> 校验：`sha256sum -c SHA256SUMS` 或单独比对 `SHA256SUMS` 中的指纹。

## 🚀 快速部署

### Docker（推荐）

```bash
mkdir -p /root/hnproxyconf
docker run -d \
  --name hnproxy \
  --restart=always \
  --net=host \
  -v /root/hnproxyconf:/app/conf \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gdy666/home-network-proxy:v3
```

访问 `http://设备IP:16601`，默认账号 `666` / 密码 `666`（**登录后立即修改**）。

### 二进制一键安装（Linux/macOS/FreeBSD）

```bash
curl -fsSL http://release.66666.host/install.sh | sh
```

### Windows（管理员 PowerShell）

```powershell
Expand-Archive .\hnproxy_3.5.0_windows_x86_64.zip -DestinationPath "C:\Program Files\HNProxy"
cd "C:\Program Files\HNProxy"
.\install_service.ps1
```

### OpenWrt

```bash
opkg update && opkg install curl
sh -c "$(curl -fsSL http://release.66666.host/install.sh)"
```

## 📋 升级

```bash
# 升级前先备份配置目录
docker compose pull && docker compose up -d

# 二进制：重新执行安装脚本（--update 保留服务与配置）
sh install.sh --update
```

## 📝 完整文档

详见仓库 [README.md](../tree/main/README.md)：项目介绍、5 种部署方式、配置说明、安全建议。

## 🐛 已知限制

- FTP 为简化协议框架，生产建议使用成熟 FTP 服务端
- WAF 当前为 IP 黑白名单层，Coraza/OWASP CRS 规则引擎接口已预留
- ACME 自动续期已留接口，当前默认自签名
- 个别重型模块（FRP / DLNA / SMB / FileBrowser / Rclone）以模块占位，后续版本逐个补齐

## 📄 许可证

MIT
