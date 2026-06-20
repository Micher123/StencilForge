.PHONY: all build build-frontend build-backend deploy dev clean run

# === Основные цели ===

all: build

# Полная сборка (фронтенд + бэкенд)
build: build-frontend build-backend
	@echo "=== Сборка завершена ==="

# Сборка фронтенда (production)
build-frontend:
	@echo "=== Сборка фронтенда ==="
	cd frontend && npm ci --silent 2>/dev/null || npm install --silent
	cd frontend && npx webpack --mode production
	@echo "=== Фронтенд собран ==="

# Сборка бэкенда
build-backend:
	@echo "=== Сборка бэкенда ==="
	cd backend && CGO_ENABLED=1 go build -o stencilforge-server .
	@echo "=== Бэкенд собран ==="

# Очистка артефактов сборки
clean:
	@echo "=== Очистка ==="
	rm -f backend/stencilforge-server
	rm -rf frontend/public/dist
	rm -rf frontend/node_modules
	@echo "=== Очистка завершена ==="

# === Развертывание ===

# Запуск production-сервера (предварительно собрать через make build)
deploy:
	@echo "=== Запуск StencilForge ==="
	cd backend && ./stencilforge-server

# Запуск в фоне с PID-файлом
deploy-background:
	@echo "=== Запуск StencilForge в фоне ==="
	cd backend && nohup ./stencilforge-server > ../server.log 2>&1 & echo $$! > ../.server.pid
	@echo "Сервер запущен. PID: $$(cat .server.pid)"
	@echo "Логи: server.log"
	@echo "Остановить: make stop"

# Остановка фонового сервера
stop:
	@if [ -f .server.pid ]; then \
		PID=$$(cat .server.pid); \
		echo "Остановка сервера (PID $$PID)..."; \
		kill $$PID 2>/dev/null || true; \
		rm -f .server.pid; \
		echo "Сервер остановлен."; \
	else \
		echo "Сервер не запущен (нет .server.pid)"; \
	fi

# === Разработка ===

# Запуск бэкенда в режиме разработки (go run)
dev-backend:
	cd backend && CGO_ENABLED=1 go run .

# Запуск фронтенда в режиме разработки (webpack dev-server с hot-reload)
dev-frontend:
	cd frontend && npm start

# Полный dev-режим (бэкенд + фронтенд параллельно)
dev:
	@echo "=== Запуск в dev-режиме ==="
	@echo "Бэкенд: http://localhost:8080"
	@echo "Фронтенд (dev): http://localhost:3000"
	@trap 'kill 0' EXIT; \
	(cd backend && CGO_ENABLED=1 go run .) & \
	(cd frontend && npm start) & \
	wait

# Быстрый запуск (сборка и запуск одной командой)
run: build deploy