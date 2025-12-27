.PHONY: help build run test clean docker-build docker-up docker-down docker-logs

help: ## 显示帮助信息
	@echo "ReadFlow Gateway - Makefile Commands"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## 构建应用
	CGO_ENABLED=1 go build -o server ./cmd/server

run: ## 运行应用
	go run ./cmd/server/main.go

test: ## 运行测试
	go test -v ./...

clean: ## 清理构建文件
	rm -f server
	rm -rf data/*.db*
	rm -rf static/images/*

docker-build: ## 构建 Docker 镜像
	docker compose build

docker-up: ## 启动 Docker 容器
	docker compose up -d

docker-down: ## 停止 Docker 容器
	docker compose down

docker-logs: ## 查看 Docker 日志
	docker compose logs -f

docker-restart: docker-down docker-up ## 重启 Docker 容器

deps: ## 安装依赖
	go mod download
	go mod tidy

fmt: ## 格式化代码
	go fmt ./...

lint: ## 代码检查
	golangci-lint run

dev: ## 开发模式运行（带热重载）
	air
