# 构建阶段 - 使用官方 Golang 镜像
FROM golang:1.23-bookworm AS builder

# 设置 GOPROXY 以加速构建（针对中国大陆地区）
ENV GOPROXY=https://goproxy.cn,direct

# 安装构建依赖
# libvips-dev: 图片处理依赖
# pkg-config: 帮助 cgo 找到库
# upx: 二进制压缩工具
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libvips-dev \
    pkg-config \
    ca-certificates \
    xz-utils \
    wget \
    && rm -rf /var/lib/apt/lists/*

# 下载并安装 UPX (可选，用于压缩二进制文件)
RUN wget -q https://github.com/upx/upx/releases/download/v4.2.1/upx-4.2.1-amd64_linux.tar.xz \
    && tar -xf upx-4.2.1-amd64_linux.tar.xz \
    && mv upx-4.2.1-amd64_linux/upx /usr/local/bin/ \
    && rm -rf upx-4.2.1-amd64_linux*

WORKDIR /app

# 复制依赖文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
# CGO_ENABLED=1 是必须的，因为用到了 govips (libvips 绑定)
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o server ./cmd/server

# 使用 UPX 压缩二进制文件
RUN upx --best --lzma /app/server || true

# 运行阶段 - 使用轻量级镜像
FROM debian:bookworm-slim

# 安装运行时依赖
# libvips42: 运行时图片处理库
# ca-certificates: HTTPS 支持
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    libvips42 \
    wget \
    curl \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* /var/cache/apt/*

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/server .

# 复制必要的静态资源
# 注意：如果有其他静态资源目录（如 templates），也需要在此复制
COPY internal/api/admin.html ./internal/api/admin.html

# 创建数据和静态文件目录
RUN mkdir -p /app/data /app/static/images

# 创建非 root 用户以提高安全性
RUN useradd -m -u 1000 -s /bin/bash appuser && \
    chown -R appuser:appuser /app

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# 运行服务
CMD ["./server"]
