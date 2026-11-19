# 家庭网络反代 Makefile
# 支持多平台交叉编译、Docker 构建、发布打包

VERSION ?= 3.5.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

# 默认目标
.PHONY: all build clean test docker release help

all: build

# ==================== 本地构建 ====================
build:
	@echo "构建 家庭网络反代 $(VERSION) ..."
	@CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o hnproxy .
	@echo "构建完成: ./hnproxy"

build-linux:
	@echo "构建 Linux AMD64 ..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o hnproxy_linux_amd64 .

build-linux-arm64:
	@echo "构建 Linux ARM64 ..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o hnproxy_linux_arm64 .

build-linux-armv7:
	@echo "构建 Linux ARMv7 ..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="$(LDFLAGS)" -o hnproxy_linux_armv7 .

build-linux-mips64le:
	@echo "构建 Linux MIPS64LE ..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags="$(LDFLAGS)" -o hnproxy_linux_mips64le .

build-linux-mipsle:
	@echo "构建 Linux MIPSLE ..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags="$(LDFLAGS)" -o hnproxy_linux_mipsle .

build-linux-riscv64:
	@echo "构建 Linux RISC-V64 ..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build -ldflags="$(LDFLAGS)" -o hnproxy_linux_riscv64 .

build-darwin:
	@echo "构建 macOS AMD64 ..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o hnproxy_darwin_amd64 .

build-darwin-arm64:
	@echo "构建 macOS ARM64 ..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o hnproxy_darwin_arm64 .

build-windows:
	@echo "构建 Windows AMD64 ..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o hnproxy_windows_amd64.exe .

build-windows-arm64:
	@echo "构建 Windows ARM64 ..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o hnproxy_windows_arm64.exe .

build-freebsd:
	@echo "构建 FreeBSD AMD64 ..."
	@CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o hnproxy_freebsd_amd64 .

# 所有平台构建
build-all: build-linux build-linux-arm64 build-linux-armv7 build-linux-mips64le build-linux-mipsle build-linux-riscv64 build-darwin build-darwin-arm64 build-windows build-windows-arm64 build-freebsd
	@echo "所有平台构建完成"

# ==================== 压缩打包 ====================
compress: build-all
	@echo "压缩打包..."
	@for f in hnproxy_*; do \
		if [[ "$$f" == *.exe ]]; then \
			zip "$${f%.exe}.zip" "$$f"; \
		else \
			tar -czf "$$f.tar.gz" "$$f"; \
		fi; \
	done
	@echo "打包完成"

# ==================== Docker 构建 ====================
docker-build:
	@echo "构建 Docker 镜像..."
	@docker build -t gdy666/home-network-proxy:$(VERSION) -t gdy666/home-network-proxy:latest .

docker-build-multiarch:
	@echo "构建多架构 Docker 镜像..."
	@docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6,linux/386,linux/mips64le,linux/mipsle,linux/riscv64 \
		-t gdy666/home-network-proxy:$(VERSION) -t gdy666/home-network-proxy:latest --push .

docker-build-full:
	@echo "构建完整 Docker 版..."
	@docker build -t gdy666/home-network-proxy:v3_full --target full .

docker-build-standard-docker:
	@echo "构建标准 Docker 版..."
	@docker build -t gdy666/home-network-proxy:v3_standard_docker --target standard_docker .

docker-build-lite-waf:
	@echo "构建轻量 WAF 版..."
	@docker build -t gdy666/home-network-proxy:v3_lite_waf --target lite_waf .

# ==================== 测试 ====================
test:
	@go test -v ./...

test-race:
	@go test -race -v ./...

test-cover:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

# ==================== 代码质量 ====================
fmt:
	@go fmt ./...

vet:
	@go vet ./...

lint:
	@golangci-lint run

tidy:
	@go mod tidy

# ==================== 运行 ====================
run: build
	@./hnproxy -c hnproxy.conf

run-dev:
	@go run -ldflags="$(LDFLAGS)" . -c hnproxy.conf

# ==================== 安装 ====================
install: build
	@sudo cp hnproxy /usr/local/bin/hnproxy
	@sudo mkdir -p /etc/hnproxy
	@[ -f /etc/hnproxy/hnproxy.conf ] || sudo cp config/hnproxy.conf.example /etc/hnproxy/hnproxy.conf
	@echo "安装完成"

# ==================== 发布 ====================
release: clean build-all compress
	@echo "发布包准备完成"
	@ls -lh hnproxy_*

# ==================== 清理 ====================
clean:
	@rm -f hnproxy hnproxy_* coverage.out
	@rm -rf dist
	@echo "清理完成"

# ==================== 依赖 ====================
deps:
	@go mod download
	@go mod verify

# ==================== 生成文档 ====================
docs:
	@echo "生成 API 文档..."
	@swag init -g main.go -o docs

# ==================== 帮助 ====================
help:
	@echo "家庭网络反代 Makefile 帮助"
	@echo ""
	@echo "构建目标:"
	@echo "  make build              - 构建当前平台"
	@echo "  make build-all          - 构建所有平台"
	@echo "  make build-linux        - 构建 Linux AMD64"
	@echo "  make build-linux-arm64  - 构建 Linux ARM64"
	@echo "  make build-darwin       - 构建 macOS AMD64"
	@echo "  make build-darwin-arm64 - 构建 macOS ARM64"
	@echo "  make build-windows      - 构建 Windows AMD64"
	@echo "  make build-freebsd      - 构建 FreeBSD AMD64"
	@echo ""
	@echo "Docker 目标:"
	@echo "  make docker-build       - 构建单架构镜像"
	@echo "  make docker-build-multiarch - 构建多架构镜像并推送"
	@echo ""
	@echo "其他目标:"
	@echo "  make test               - 运行测试"
	@echo "  make fmt                - 格式化代码"
	@echo "  make lint               - 代码检查"
	@echo "  make clean              - 清理构建产物"
	@echo "  make release            - 完整发布构建"
	@echo "  make install            - 安装到系统"
	@echo "  make run                - 运行程序"