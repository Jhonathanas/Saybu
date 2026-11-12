SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

GO      ?= go
COMPOSE ?= docker compose
BUF     ?= buf
SERVICES := gateway auth catalog shipping order payment notification

.PHONY: help up down logs ps restart proto migrate seed dev dev-frontend test test-integration lint fmt build clean

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

proto: ## Generate kode dari protobuf
	cd backend/proto && $(BUF) lint && $(BUF) generate
	@echo "generated -> backend/gen"

migrate: ## Jalankan migrasi semua database (M1+)
	@echo "Belum ada migrasi. Ditambahkan pada M1."

seed: ## Isi data contoh (M2+)
	@echo "Belum ada seed. Ditambahkan pada M2."

dev: ## Jalankan semua service Go
	bash infra/scripts/dev.sh

dev-frontend: ## Jalankan frontend (M1+)
	@test -f frontend/package.json || { echo "frontend belum ada (M1)"; exit 1; }
	cd frontend && npm run dev

test: ## Unit test
	cd backend && $(GO) test ./...

test-integration: ## Test integrasi (butuh Kafka berjalan)
	cd backend && KAFKA_BROKERS=$${KAFKA_BROKERS:-localhost:29092} $(GO) test -tags=integration ./pkg/kafkax/...

lint: ## Vet + golangci-lint bila tersedia
	cd backend && $(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then cd backend && golangci-lint run; else echo "golangci-lint tidak terpasang, dilewati"; fi

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
