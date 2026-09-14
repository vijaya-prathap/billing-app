SHELL := /bin/bash

BACKEND_DIR  := backend
FRONTEND_DIR := frontend
COMPOSE      := docker compose
NAMESPACE    := billing
HELM_RELEASE := billing-app
HELM_CHART   := helm/billing-app
IMAGE_TAG    ?= latest

# Runs the mysql client inside the compose container using that container's own credentials.
MYSQL_CLIENT = $(COMPOSE) exec -T mysql sh -c 'mysql -u"$$MYSQL_USER" -p"$$MYSQL_PASSWORD" "$$MYSQL_DATABASE"'

.DEFAULT_GOAL := help

.PHONY: help run build test vet fmt tidy \
	frontend-install frontend-dev frontend-build \
	up down logs db-up migrate seed db-shell db-reset docker-build \
	k8s-apply helm-lint helm-template helm-deploy

help: ## List available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## ---- Backend ----

run: ## Run the API locally with variables from .env
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(BACKEND_DIR) && go run ./cmd/api

build: ## Build the API binary into backend/bin
	cd $(BACKEND_DIR) && CGO_ENABLED=0 go build -trimpath -o bin/billing-api ./cmd/api

test: ## Run backend tests with the race detector
	cd $(BACKEND_DIR) && go test -race -count=1 ./...

vet: ## Run go vet
	cd $(BACKEND_DIR) && go vet ./...

fmt: ## Format Go code
	cd $(BACKEND_DIR) && gofmt -w .

tidy: ## Tidy Go modules
	cd $(BACKEND_DIR) && go mod tidy

## ---- Frontend ----

frontend-install: ## Install frontend dependencies
	cd $(FRONTEND_DIR) && npm ci

frontend-dev: ## Start the Vite dev server (proxies /api to localhost:8080)
	cd $(FRONTEND_DIR) && npm run dev

frontend-build: ## Type-check and build the frontend
	cd $(FRONTEND_DIR) && npm run build

## ---- Docker / database ----

up: ## Build and start mysql, api, and frontend
	$(COMPOSE) up -d --build

down: ## Stop all services (data volume is kept)
	$(COMPOSE) down

logs: ## Tail API logs
	$(COMPOSE) logs -f api

db-up: ## Start only MySQL
	$(COMPOSE) up -d --wait mysql

migrate: ## Apply all migrations to the compose MySQL (idempotent)
	@for f in $(BACKEND_DIR)/migrations/*.sql; do \
		echo "applying $$f"; \
		$(MYSQL_CLIENT) < "$$f" || exit 1; \
	done

seed: ## Load development seed data (idempotent)
	$(MYSQL_CLIENT) < database/seed/seed.sql

db-shell: ## Open a MySQL shell
	$(COMPOSE) exec mysql sh -c 'mysql -u"$$MYSQL_USER" -p"$$MYSQL_PASSWORD" "$$MYSQL_DATABASE"'

db-reset: ## DESTRUCTIVE: delete the MySQL volume and re-initialize from migrations + seed
	@read -r -p "This permanently deletes all local MySQL data. Type 'yes' to continue: " answer; \
	[ "$$answer" = "yes" ] || { echo "aborted"; exit 1; }
	$(COMPOSE) down -v
	$(COMPOSE) up -d --wait mysql

docker-build: ## Build API and frontend images
	docker build -t billing-api:$(IMAGE_TAG) $(BACKEND_DIR)
	docker build -t billing-frontend:$(IMAGE_TAG) $(FRONTEND_DIR)

## ---- Kubernetes ----

k8s-apply: ## Apply raw manifests (skips secret.yaml, which holds placeholders)
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/configmap.yaml -f k8s/deployment.yaml -f k8s/service.yaml

helm-lint: ## Lint the Helm chart
	helm lint $(HELM_CHART)

helm-template: ## Render the Helm chart locally
	helm template $(HELM_RELEASE) $(HELM_CHART) --namespace $(NAMESPACE)

helm-deploy: ## Install/upgrade the Helm release (IMAGE_TAG=sha-<commit>)
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(NAMESPACE) --create-namespace \
		--set image.tag=$(IMAGE_TAG) --atomic --wait --timeout 5m
