# 标准 Docker 镜像 - 多阶段构建
# 支持多架构: linux/amd64, linux/arm64, linux/arm/v7, linux/arm/v6, linux/386, linux/mips64le, linux/mipsle, linux/riscv64
# 支持多版本: full, full_docker, standard, standard_docker, medium, lite, lite_waf

# ==================== 基础构建阶段 ====================
FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make gcc musl-dev upx ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制 go.mod（go.sum 由容器内 go mod download 生成）
COPY go.mod ./

# 下载依赖并生成 go.sum
RUN go mod download

# 复制源代码
COPY . .

# 设置构建参数
ARG VERSION=3.5.0
ARG BUILD_TIME
ARG GIT_COMMIT
ARG CGO_ENABLED=0

# 编译基础二进制 (所有版本共用)
RUN CGO_ENABLED=${CGO_ENABLED} go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o hnproxy \
    .

# 压缩二进制文件
RUN upx --best --lzma hnproxy || true


# ==================== 完整 Docker 版 (完整功能 + Docker 管理) ====================
FROM alpine:3.19 AS full_docker

# 安装运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    docker-cli \
    docker-compose \
    curl \
    wget \
    bash \
    shadow \
    su-exec

# 复制二进制和入口脚本
COPY --from=builder /build/hnproxy /hnproxy
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# 复制 CA 证书和时区
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# 创建非 root 用户
RUN addgroup -g 1000 hnproxy && adduser -u 1000 -G hnproxy -s /bin/sh -D hnproxy

# 设置工作目录
WORKDIR /app/conf

# 环境变量
ENV HNPROXY_CONFIG_DIR=/app/conf
ENV TZ=Asia/Shanghai

# 暴露端口
EXPOSE 16601 80 443

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/hnproxy", "-c", "/app/conf/hnproxy.conf", "-info"] || exit 1

# 入口点
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-c", "/app/conf/hnproxy.conf"]

LABEL edition="full_docker"
LABEL description="完整 Docker 版 - 完整功能包含 Docker 管理"


# ==================== 标准 Docker 版 (基础功能 + Docker 管理) ====================
FROM alpine:3.19 AS standard_docker

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    docker-cli \
    curl \
    wget \
    bash \
    shadow \
    su-exec

COPY --from=builder /build/hnproxy /hnproxy
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

RUN addgroup -g 1000 hnproxy && adduser -u 1000 -G hnproxy -s /bin/sh -D hnproxy

WORKDIR /app/conf
ENV HNPROXY_CONFIG_DIR=/app/conf
ENV TZ=Asia/Shanghai

EXPOSE 16601 80 443

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/hnproxy", "-c", "/app/conf/hnproxy.conf", "-info"] || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-c", "/app/conf/hnproxy.conf"]

LABEL edition="standard_docker"
LABEL description="标准 Docker 版 - 基础功能包含 Docker 管理"


# ==================== 轻量 WAF 版 (WAF + gRPC) ====================
FROM alpine:3.19 AS lite_waf

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    wget \
    bash \
    shadow \
    su-exec

COPY --from=builder /build/hnproxy /hnproxy
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

RUN addgroup -g 1000 hnproxy && adduser -u 1000 -G hnproxy -s /bin/sh -D hnproxy

WORKDIR /app/conf
ENV HNPROXY_CONFIG_DIR=/app/conf
ENV TZ=Asia/Shanghai

EXPOSE 16601 80 443

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/hnproxy", "-c", "/app/conf/hnproxy.conf", "-info"] || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-c", "/app/conf/hnproxy.conf"]

LABEL edition="lite_waf"
LABEL description="轻量 WAF 版 - 含 Coraza WAF 和 gRPC 反向代理"


# ==================== 完整版 (完整功能，无 Docker 管理) ====================
FROM alpine:3.19 AS full

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    wget \
    bash \
    shadow \
    su-exec

COPY --from=builder /build/hnproxy /hnproxy
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

RUN addgroup -g 1000 hnproxy && adduser -u 1000 -G hnproxy -s /bin/sh -D hnproxy

WORKDIR /app/conf
ENV HNPROXY_CONFIG_DIR=/app/conf
ENV TZ=Asia/Shanghai

EXPOSE 16601 80 443

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/hnproxy", "-c", "/app/conf/hnproxy.conf", "-info"] || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-c", "/app/conf/hnproxy.conf"]

LABEL edition="full"
LABEL description="完整版 - 完整功能，无 Docker 管理"


# ==================== 增强版 (中等功能集) ====================
FROM alpine:3.19 AS medium

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    wget \
    bash \
    shadow \
    su-exec

COPY --from=builder /build/hnproxy /hnproxy
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

RUN addgroup -g 1000 hnproxy && adduser -u 1000 -G hnproxy -s /bin/sh -D hnproxy

WORKDIR /app/conf
ENV HNPROXY_CONFIG_DIR=/app/conf
ENV TZ=Asia/Shanghai

EXPOSE 16601 80 443

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/hnproxy", "-c", "/app/conf/hnproxy.conf", "-info"] || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-c", "/app/conf/hnproxy.conf"]

LABEL edition="medium"
LABEL description="增强版 - 中等功能集"


# ==================== 标准版 (基础功能集) ====================
FROM alpine:3.19 AS standard

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    wget \
    bash \
    shadow \
    su-exec

COPY --from=builder /build/hnproxy /hnproxy
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

RUN addgroup -g 1000 hnproxy && adduser -u 1000 -G hnproxy -s /bin/sh -D hnproxy

WORKDIR /app/conf
ENV HNPROXY_CONFIG_DIR=/app/conf
ENV TZ=Asia/Shanghai

EXPOSE 16601 80 443

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/hnproxy", "-c", "/app/conf/hnproxy.conf", "-info"] || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-c", "/app/conf/hnproxy.conf"]

LABEL edition="standard"
LABEL description="标准版 - 基础功能集"


# ==================== 轻量版 (最精简) ====================
FROM alpine:3.19 AS lite

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    wget \
    bash \
    shadow \
    su-exec

COPY --from=builder /build/hnproxy /hnproxy
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

RUN addgroup -g 1000 hnproxy && adduser -u 1000 -G hnproxy -s /bin/sh -D hnproxy

WORKDIR /app/conf
ENV HNPROXY_CONFIG_DIR=/app/conf
ENV TZ=Asia/Shanghai

EXPOSE 16601 80 443

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/hnproxy", "-c", "/app/conf/hnproxy.conf", "-info"] || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["-c", "/app/conf/hnproxy.conf"]

LABEL edition="lite"
LABEL description="轻量版 - 最精简功能集"


# ==================== 默认目标 (完整 Docker 版) ====================
FROM full_docker

# 公共标签
LABEL maintainer="gdy666"
LABEL org.opencontainers.image.title="家庭网络反代"
LABEL org.opencontainers.image.description="个人和家庭网络工具，提供 DDNS、反向代理、证书管理、文件服务等功能"
LABEL org.opencontainers.image.version="3.5.0"
LABEL org.opencontainers.image.vendor="gdy666"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/gdy666/home-network-proxy"
LABEL org.opencontainers.image.documentation="https://lucky666.cn/docs/intro"