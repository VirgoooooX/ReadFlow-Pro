# 构建阶段 - 使用官方 Golang 镜像
FROM golang:1.21-bookworm AS builder

# 设置构建参数以减少层大小
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libvips-dev \
    ca-certificates \
    xz-utils \
    wget \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# 下载并安装 UPX
RUN wget -q https://github.com/upx/upx/releases/download/v4.2.1/upx-4.2.1-amd64_linux.tar.xz \
    && tar -xf upx-4.2.1-amd64_linux.tar.xz \
    && mv upx-4.2.1-amd64_linux/upx /usr/local/bin/ \
    && rm -rf upx-4.2.1-amd64_linux*

WORKDIR /app

# 只复制必要的文件
COPY go.mod ./

# 先执行 go mod tidy 然后下载依赖
RUN GOSUMDB=off go mod tidy

# 复制源代码
COPY . .

# 构建应用
# 针对本地构建架构编译（不强制 amd64），这样可以在任何架构上构建
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -trimpath \
    -a \
    -mod=mod \
    -installsuffix cgo \
    -o server ./cmd/server

# 使用 UPX 压缩二进制文件（减小 50-70%）
RUN upx --best --lzma /app/server || true

# 验证二进制文件大小
RUN ls -lh /app/server

# 运行阶段 - 使用轻量级镜像
FROM debian:bookworm-slim

# 安装最小化运行时依赖
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    libvips42 \
    wget \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* /var/cache/apt/*

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/server .

# 复制静态文件
COPY internal/api/admin.html ./internal/api/admin.html

# 创建数据目录
RUN mkdir -p /app/data /app/static/images

# 暴露端口
EXPOSE 8080

# 健康检查（使用 wget GET 请求）
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null http://localhost:8080/health || exit 1

# 运行服务
CMD ["./server"]
