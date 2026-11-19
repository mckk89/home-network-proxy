#!/bin/bash
# 家庭网络反代 多平台构建脚本
# 用法: ./scripts/build.sh [version]

set -euo pipefail

VERSION="${1:-3.5.0}"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

OUTPUT_DIR="dist/${VERSION}"
mkdir -p "${OUTPUT_DIR}"

echo "========================================"
echo "  家庭网络反代 v${VERSION} 多平台构建"
echo "========================================"
echo "版本: ${VERSION}"
echo "构建时间: ${BUILD_TIME}"
echo "Git Commit: ${GIT_COMMIT}"
echo "输出目录: ${OUTPUT_DIR}"
echo "========================================"

# 构建函数
build() {
    local os=$1
    local arch=$2
    local arm=$3
    local output=$4
    
    echo "构建 ${os}/${arch}${arm:+/${arm}} ..."
    
    local env="CGO_ENABLED=0 GOOS=${os} GOARCH=${arch}"
    [ -n "$arm" ] && env="${env} GOARM=${arm#v}"
    
    if eval "${env} go build -ldflags=\"${LDFLAGS}\" -o ${OUTPUT_DIR}/${output} ."; then
        echo "  ✓ 成功: ${output}"
    else
        echo "  ✗ 失败: ${output}"
        return 1
    fi
}

# Linux 构建
echo ""
echo "=== Linux 平台 ==="
build linux amd64 "" "hnproxy_${VERSION}_linux_x86_64"
build linux arm64 "" "hnproxy_${VERSION}_linux_arm64"
build linux arm v7 "hnproxy_${VERSION}_linux_armv7"
build linux arm v6 "hnproxy_${VERSION}_linux_armv6"
build linux 386 "" "hnproxy_${VERSION}_linux_i386"
build linux mips64le "" "hnproxy_${VERSION}_linux_mips64le"
build linux mipsle "" "hnproxy_${VERSION}_linux_mipsle"
build linux riscv64 "" "hnproxy_${VERSION}_linux_riscv64"

# macOS 构建
echo ""
echo "=== macOS 平台 ==="
build darwin amd64 "" "hnproxy_${VERSION}_darwin_x86_64"
build darwin arm64 "" "hnproxy_${VERSION}_darwin_arm64"

# Windows 构建
echo ""
echo "=== Windows 平台 ==="
build windows amd64 "" "hnproxy_${VERSION}_windows_x86_64.exe"
build windows arm64 "" "hnproxy_${VERSION}_windows_arm64.exe"

# FreeBSD 构建
echo ""
echo "=== FreeBSD 平台 ==="
build freebsd amd64 "" "hnproxy_${VERSION}_freebsd_x86_64"

# 压缩打包
echo ""
echo "=== 压缩打包 ==="
cd "${OUTPUT_DIR}"
for f in hnproxy_*; do
    case "$f" in
        *.exe)
            zip "${f%.exe}.zip" "$f" && echo "  ✓ ${f%.exe}.zip"
            ;;
        *.tar.gz|*.zip)
            ;;
        *)
            # tar.gz 内统一放置名为 hnproxy 的可执行文件，便于安装脚本识别
            stage=$(mktemp -d)
            cp "$f" "${stage}/hnproxy"
            tar -czf "${f}.tar.gz" -C "${stage}" hnproxy && echo "  ✓ ${f}.tar.gz"
            rm -rf "${stage}"
            ;;
    esac
done

# 生成校验和
echo ""
echo "=== 生成校验和 ==="
sha256sum hnproxy_* > SHA256SUMS 2>/dev/null || shasum -a 256 hnproxy_* > SHA256SUMS
echo "  ✓ SHA256SUMS"

cd - > /dev/null

echo ""
echo "========================================"
echo "  构建完成！"
echo "========================================"
echo "输出目录: ${OUTPUT_DIR}"
ls -lh "${OUTPUT_DIR}/"
echo ""
echo "校验和:"
cat "${OUTPUT_DIR}/SHA256SUMS"