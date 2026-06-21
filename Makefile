.PHONY: all build build-frontend build-backend deploy dev clean run upgrade-to-ultima

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
	@fuser -k 8080/tcp 2>/dev/null || true
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

# === Администрирование ===

# Апгрейд пользователя до тарифа Ultima (32 слоя)
# Использование: make upgrade-to-ultima EMAIL=user@example.com
upgrade-to-ultima:
	@if [ -z "$(EMAIL)" ]; then \
		echo "Ошибка: укажите EMAIL пользователя"; \
		echo "Пример: make upgrade-to-ultima EMAIL=user@example.com"; \
		exit 1; \
	fi
	@DB_DIR=$${STENCILFORGE_DATA_DIR:-$$HOME/.stencilforge}; \
	DB_FILE="$$DB_DIR/stencilforge.db"; \
	if [ ! -f "$$DB_FILE" ]; then \
		echo "Ошибка: база данных не найдена: $$DB_FILE"; \
		echo "Запустите сервер хотя бы раз (make deploy) для создания БД."; \
		exit 1; \
	fi; \
	echo "Апгрейд пользователя $(EMAIL) до тарифа Ultima (32 слоя)..."; \
	sqlite3 "$$DB_FILE" "UPDATE users SET plan = 'ultima', max_layers = 32 WHERE email = '$(EMAIL)';"; \
	echo "Готово. Проверка:"; \
	sqlite3 -header -column "$$DB_FILE" "SELECT id, username, email, plan, max_layers FROM users WHERE email = '$(EMAIL)';"
