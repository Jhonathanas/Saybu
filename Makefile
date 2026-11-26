SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

GO      ?= go
COMPOSE ?= docker compose
BUF     ?= buf
SERVICES := gateway auth catalog shipping order payment notification

.PHONY: help up down logs ps restart proto migrate migrate-down seed dev dev-frontend test test-frontend test-integration lint lint-frontend fmt build clean observability observability-down load-test backup restore

help: ## Tampilkan daftar target
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

up: ## Jalankan Postgres, Kafka, Kafka UI
	$(COMPOSE) up -d
	@echo "Kafka UI: http://localhost:8090"

down: ## Hentikan semua container
	$(COMPOSE) down

logs: ## Ikuti log container
	$(COMPOSE) logs -f --tail=100

ps: ## Status container
	$(COMPOSE) ps

restart: ## Restart container infra
	$(COMPOSE) restart

observability: ## Jalankan Prometheus + Grafana (profil observability)
	$(COMPOSE) --profile observability up -d prometheus grafana
	@echo "Prometheus: http://localhost:9090"
	@echo "Grafana:    http://localhost:3000 ($${GRAFANA_USER:-admin}/$${GRAFANA_PASSWORD:-admin})"

observability-down: ## Hentikan Prometheus + Grafana
	$(COMPOSE) --profile observability down prometheus grafana

load-test: ## Load test k6 (100 VU katalog + 10 checkout/detik) via Docker
	docker run --rm -i --add-host=host.docker.internal:host-gateway \
		-e API_BASE=$${API_BASE:-http://host.docker.internal:8081} \
		-e DURATION=$${DURATION:-30s} \
		grafana/k6 run - < infra/load/k6-saybu.js

backup: ## Backup semua database ke backups/<timestamp>
	bash infra/scripts/backup.sh

restore: ## Restore database dari backup (pakai DIR=backups/<timestamp>)
	@test -n "$(DIR)" || { echo "pakai: make restore DIR=backups/<timestamp>"; exit 1; }
	bash infra/scripts/restore.sh "$(DIR)"

proto: ## Generate kode dari protobuf
	cd backend/proto && $(BUF) lint && $(BUF) generate
	@echo "generated -> backend/gen"

migrate: ## Jalankan migrasi semua database (goose up)
	bash infra/scripts/migrate.sh up

migrate-down: ## Rollback migrasi semua database (goose down)
	bash infra/scripts/migrate.sh down

seed: ## Isi data contoh: 2 kategori, 12 produk, 1 admin (idempoten)
	cd backend && $(GO) run ./cmd/seed

dev: ## Jalankan semua service Go
	bash infra/scripts/dev.sh

dev-frontend: ## Jalankan frontend (M1+)
	@test -f frontend/package.json || { echo "frontend belum ada (M1)"; exit 1; }
	cd frontend && npm run dev

test: ## Unit test Go
	cd backend && $(GO) test ./...

test-frontend: ## Test frontend (Vitest)
	cd frontend && npm run test

test-integration: ## Test integrasi (butuh Postgres + Kafka berjalan)
	cd backend && set -a && . ../.env && set +a && KAFKA_BROKERS=$${KAFKA_BROKERS:-localhost:29092} $(GO) test -tags=integration -count=1 ./...

lint: ## Vet + golangci-lint bila tersedia
	cd backend && $(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then cd backend && golangci-lint run; else echo "golangci-lint tidak terpasang, dilewati"; fi

lint-frontend: ## Lint + typecheck frontend
	cd frontend && npm run lint && npm run typecheck

audit: ## Scan kerentanan Go + npm
	@command -v govulncheck >/dev/null 2>&1 || { echo "govulncheck belum terpasang: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; }
	cd backend && govulncheck ./...
	cd frontend && npm audit --audit-level=high

fmt: ## Format kode Go
	cd backend && gofmt -w .

build: ## Build semua binary ke backend/bin
	cd backend && mkdir -p bin && for svc in $(SERVICES); do \
		echo "building $$svc"; $(GO) build -o bin/$$svc ./services/$$svc/cmd || exit 1; \
	done
	@ls -lh backend/bin

clean: ## Hapus binary & data container
	rm -rf backend/bin
	$(COMPOSE) down -v
