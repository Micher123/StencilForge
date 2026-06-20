#!/usr/bin/env bash
#
# deploy.sh — Сборка и развёртывание StencilForge
#
# Использование:
#   ./deploy.sh build      Сборка проекта (фронтенд + бэкенд)
#   ./deploy.sh start      Запуск сервера в фоне
#   ./deploy.sh stop       Остановка сервера
#   ./deploy.sh restart    Перезапуск сервера
#   ./deploy.sh logs       Просмотр логов
#   ./deploy.sh status     Статус сервера
#   ./deploy.sh clean      Очистка артефактов сборки
#   ./deploy.sh install    Установка зависимостей
#   ./deploy.sh dev        Запуск в dev-режиме (frontend + backend)
#
# Переменные окружения:
#   PORT — порт сервера (по умолчанию 8080)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

PORT="${PORT:-8080}"
PID_FILE="$SCRIPT_DIR/.server.pid"
LOG_FILE="$SCRIPT_DIR/server.log"
BACKEND_BIN="$SCRIPT_DIR/backend/stencilforge-server"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "${CYAN}[STEP]${NC}  $*"; }

# --------------------------------------------------
# Проверка инструментов
# --------------------------------------------------
check_tools() {
    local missing=0
    for tool in go node npm; do
        if ! command -v "$tool" &>/dev/null; then
            log_error "Не найден: $tool. Установите $tool и повторите."
            missing=1
        fi
    done
    if [[ $missing -ne 0 ]]; then
        exit 1
    fi
}

# --------------------------------------------------
# Сборка фронтенда
# --------------------------------------------------
build_frontend() {
    log_step "Сборка фронтенда..."
    cd "$SCRIPT_DIR/frontend"
    npm ci --silent 2>/dev/null || npm install --silent
    npx webpack --mode production
    log_info "Фронтенд собран."
    cd "$SCRIPT_DIR"
}

# --------------------------------------------------
# Сборка бэкенда
# --------------------------------------------------
build_backend() {
    log_step "Сборка бэкенда..."
    cd "$SCRIPT_DIR/backend"
    CGO_ENABLED=1 go build -o stencilforge-server .
    log_info "Бэкенд собран."
    cd "$SCRIPT_DIR"
}

# --------------------------------------------------
# Полная сборка
# --------------------------------------------------
cmd_build() {
    check_tools
    build_frontend
    build_backend
    log_info "Сборка завершена успешно."
}

# --------------------------------------------------
# Запуск сервера в фоне
# --------------------------------------------------
cmd_start() {
    if [[ -f "$PID_FILE" ]]; then
        local pid
        pid="$(cat "$PID_FILE")"
        if kill -0 "$pid" 2>/dev/null; then
            log_warn "Сервер уже запущен (PID $pid). Используйте restart для перезапуска."
            return 1
        else
            log_info "Удалён устаревший PID-файл."
            rm -f "$PID_FILE"
        fi
    fi

    if [[ ! -x "$BACKEND_BIN" ]]; then
        log_warn "Бинарник не найден. Запускаю сборку..."
        cmd_build
    fi

    log_step "Запуск сервера на порту $PORT..."
    PORT="$PORT" nohup "$BACKEND_BIN" >> "$LOG_FILE" 2>&1 &
    local pid=$!
    echo "$pid" > "$PID_FILE"
    sleep 1

    if kill -0 "$pid" 2>/dev/null; then
        log_info "Сервер запущен (PID $pid) на http://localhost:$PORT"
        log_info "Логи: tail -f $LOG_FILE"
    else
        log_error "Сервер не запустился. Проверьте $LOG_FILE"
    fi
}

# --------------------------------------------------
# Остановка сервера
# --------------------------------------------------
cmd_stop() {
    if [[ ! -f "$PID_FILE" ]]; then
        log_warn "PID-файл не найден. Сервер, вероятно, не запущен."
        # попробуем найти процесс по имени
        pkill -f stencilforge-server 2>/dev/null && log_info "Процесс stencilforge-server остановлен." || true
        return 0
    fi

    local pid
    pid="$(cat "$PID_FILE")"
    if kill -0 "$pid" 2>/dev/null; then
        log_step "Остановка сервера (PID $pid)..."
        kill "$pid" 2>/dev/null || true
        sleep 1
        # force kill если не остановился
        if kill -0 "$pid" 2>/dev/null; then
            log_warn "Принудительная остановка..."
            kill -9 "$pid" 2>/dev/null || true
        fi
        log_info "Сервер остановлен."
    else
        log_info "Процесс $pid не найден (уже остановлен)."
    fi
    rm -f "$PID_FILE"
}

# --------------------------------------------------
# Перезапуск
# --------------------------------------------------
cmd_restart() {
    cmd_stop
    sleep 1
    cmd_start
}

# --------------------------------------------------
# Логи
# --------------------------------------------------
cmd_logs() {
    if [[ -f "$LOG_FILE" ]]; then
        tail -f "$LOG_FILE"
    else
        log_warn "Файл логов не найден: $LOG_FILE"
    fi
}

# --------------------------------------------------
# Статус
# --------------------------------------------------
cmd_status() {
    if [[ -f "$PID_FILE" ]]; then
        local pid
        pid="$(cat "$PID_FILE")"
        if kill -0 "$pid" 2>/dev/null; then
            log_info "Сервер запущен (PID $pid) на http://localhost:$PORT"
        else
            log_warn "PID-файл есть, но процесс $pid не найден."
            rm -f "$PID_FILE"
        fi
    else
        log_info "Сервер не запущен."
    fi
}

# --------------------------------------------------
# Очистка
# --------------------------------------------------
cmd_clean() {
    log_step "Очистка артефактов..."
    rm -f "$BACKEND_BIN"
    rm -f "$PID_FILE"
    rm -f "$LOG_FILE"
    rm -rf "$SCRIPT_DIR/frontend/public/dist"
    rm -rf "$SCRIPT_DIR/frontend/node_modules"
    log_info "Очистка завершена."
}

# --------------------------------------------------
# Установка зависимостей
# --------------------------------------------------
cmd_install() {
    check_tools
    log_step "Установка зависимостей фронтенда..."
    cd "$SCRIPT_DIR/frontend"
    npm install
    cd "$SCRIPT_DIR"
    log_step "Загрузка зависимостей бэкенда..."
    cd "$SCRIPT_DIR/backend"
    go mod download
    cd "$SCRIPT_DIR"
    log_info "Зависимости установлены."
}

# --------------------------------------------------
# Dev-режим
# --------------------------------------------------
cmd_dev() {
    check_tools
    log_info "Запуск в dev-режиме..."
    log_info "Бэкенд: http://localhost:$PORT"
    log_info "Фронтенд (dev): http://localhost:3000"
    trap 'kill 0' EXIT
    (cd "$SCRIPT_DIR/backend" && PORT="$PORT" CGO_ENABLED=1 go run .) &
    (cd "$SCRIPT_DIR/frontend" && npm start) &
    wait
}

# --------------------------------------------------
# Точка входа
# --------------------------------------------------
usage() {
    echo "Использование: $0 {build|start|stop|restart|logs|status|clean|install|dev}"
    echo ""
    echo "  build      Сборка проекта (фронтенд + бэкенд)"
    echo "  start      Запуск сервера в фоне"
    echo "  stop       Остановка сервера"
    echo "  restart    Перезапуск сервера"
    echo "  logs       Просмотр логов (tail -f)"
    echo "  status     Статус сервера"
    echo "  clean      Очистка артефактов сборки"
    echo "  install    Установка зависимостей"
    echo "  dev        Запуск в dev-режиме (frontend + backend)"
    echo ""
    echo "Переменные окружения:"
    echo "  PORT       Порт сервера (по умолчанию 8080)"
    exit 0
}

if [[ $# -eq 0 ]]; then
    usage
fi

case "$1" in
    build)    cmd_build ;;
    start)    cmd_start ;;
    stop)     cmd_stop ;;
    restart)  cmd_restart ;;
    logs)     cmd_logs ;;
    status)   cmd_status ;;
    clean)    cmd_clean ;;
    install)  cmd_install ;;
    dev)      cmd_dev ;;
    -h|--help|help) usage ;;
    *)
        log_error "Неизвестная команда: $1"
        usage
        ;;
esac