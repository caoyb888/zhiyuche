# 在远程编译机（Linux）上使用。Windows 本机只写代码。
SHELL := /bin/bash
export PATH := $(PATH):/usr/local/go/bin:$(HOME)/go/bin

API_DIR := apps/api
WEB_DIR := apps/web
COMPOSE := docker compose -f deploy/docker-compose.yml --env-file deploy/.env
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/caoyb888/zhiyuche/apps/api/internal/server.Version=$(VERSION)

.PHONY: help api-tidy api-build api-test api-vet api-run api-migrate web-install web-build web-check \
        infra-up infra-down infra-logs infra-ps env check

help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n",$$1,$$2}'

env: ## 生成缺省的 .env 文件（已存在则跳过；无 .env.example 的目录忽略）
	@for d in deploy $(API_DIR) $(WEB_DIR); do \
	  if [ -f $$d/.env.example ] && [ ! -f $$d/.env ]; then cp $$d/.env.example $$d/.env && echo "created $$d/.env"; fi; \
	done

# ---- backend ----
api-tidy: ## go mod tidy
	cd $(API_DIR) && go mod tidy

api-vet: ## go vet
	cd $(API_DIR) && go vet ./...

api-test: ## go test
	cd $(API_DIR) && go test -race -count=1 ./...

api-build: ## 编译所有服务到 bin/
	@mkdir -p bin
	cd $(API_DIR) && for c in api iot ocpp worker simulator; do \
	  go build -trimpath -ldflags "$(LDFLAGS)" -o ../../bin/zhiyuche-$$c ./cmd/$$c || exit 1; done
	@ls -la bin/

api-run: ## 前台运行 api（读取 apps/api/.env）
	cd $(API_DIR) && go run -ldflags "$(LDFLAGS)" ./cmd/api

api-migrate: ## 执行迁移：make api-migrate CMD=status
	cd $(API_DIR) && go run ./cmd/api migrate $(or $(CMD),up)

# ---- frontend ----
# npm workspaces：依赖统一装在仓库根，lockfile 只有根目录一份
web-install: ## npm install（仓库根，workspaces）
	npm install --no-audit --no-fund

web-check: ## typecheck + lint
	npm run typecheck -w $(WEB_DIR) && npm run lint -w $(WEB_DIR)

web-build: ## 生产构建
	npm run build -w $(WEB_DIR)

# ---- infra ----
infra-up: env ## 启动 PG/Redis/EMQX/NATS/MinIO
	$(COMPOSE) up -d
	$(COMPOSE) ps

infra-down: ## 停止基础设施（保留数据卷）
	$(COMPOSE) down

infra-logs: ## 查看日志
	$(COMPOSE) logs -f --tail=100

infra-ps: ## 容器状态
	$(COMPOSE) ps

check: api-vet api-test web-check ## CI 同款检查
