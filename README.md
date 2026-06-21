# StencilForge

Сервис для создания трафаретных слоёв из изображений через веб-интерфейс.

Загрузите изображение, укажите количество слоёв — и получите набор чёрно-белых масок, готовых для печати, вырезания и послойного нанесения краски.

## Возможности

- Загрузка изображений в форматах PNG, JPEG, BMP, TIFF
- Автоматическая / ручная установка количества слоёв (2–32 в зависимости от тарифа)
- Качественная кластеризация: k-means++ в цветовом пространстве CIELAB, 50 итераций
- Постобработка масок: медианный фильтр, морфологическое закрытие, фильтрация мелких компонент
- Сортировка слоёв от тёмного к светлому для правильного послойного наложения
- Увеличение слоя по клику на миниатюру (модальное окно)
- Скачивание как отдельных слоёв (PNG), так и всех разом (ZIP)
- **Аутентификация пользователей**: регистрация, вход, JWT-токены, bcrypt-хэширование паролей
- **Тарифные планы**: Free (4 слоя), Pro (16 слоёв), Ultima (32 слоя)
- **Платёжная система**: интеграция с ЮKassa
- Светлая / тёмная тема веб-интерфейса
- **Кластерная архитектура**: горизонтальное масштабирование на множество нод

---

## Требования

- **Go** ≥ 1.21 (с поддержкой CGO для SQLite)
- **Node.js** ≥ 18, **npm** ≥ 9
- **GCC** (для компиляции SQLite через CGO)

Проверка:
```bash
go version
node --version
npm --version
gcc --version
```

---

## Быстрый старт

### 1. Клонирование

```bash
git clone https://github.com/Micher123/StencilForge.git
cd StencilForge
```

### 2. Настройка окружения

```bash
cp .env.exemple .env
# Отредактируйте .env — задайте JWT_SECRET, параметры ЮKassa (опционально)
```

### 3. Сборка и запуск одной командой

```bash
make run
```

Сервер будет доступен на **http://localhost:8080**.

### 4. Альтернативно — через bash-скрипт

```bash
chmod +x deploy.sh
./deploy.sh build   # Сборка
./deploy.sh start   # Запуск в фоне
./deploy.sh logs    # Просмотр логов
./deploy.sh stop    # Остановка
```

---

## Команды Makefile

| Команда | Описание |
|---------|----------|
| `make build` | Полная сборка (фронтенд + бэкенд) |
| `make build-frontend` | Сборка только фронтенда (webpack production) |
| `make build-backend` | Сборка только бэкенда (`go build`) |
| `make run` | Сборка + запуск |
| `make deploy` | Запуск production-сервера |
| `make deploy-background` | Запуск в фоне с PID-файлом и логами |
| `make stop` | Остановка фонового сервера |
| `make dev` | Запуск в dev-режиме (фронтенд hot-reload + бэкенд) |
| `make dev-frontend` | Только фронтенд dev-server (порт 3000) |
| `make dev-backend` | Только бэкенд (`go run`) |
| `make clean` | Удаление артефактов сборки |
| `make cluster-main` | Запуск главной ноды кластера (dev) |
| `make cluster-worker` | Запуск worker-ноды кластера (dev) |
| `make cluster-dev` | Запуск локального кластера (main + 2 worker'а) |
| `make upgrade-to-ultima EMAIL=user@example.com` | Апгрейд пользователя до Ultima (32 слоя) |

### Примеры использования Makefile

```bash
# Запуск на нестандартном порту
PORT=9090 make deploy

# Запуск в dev-режиме (фронтенд на :3000, бэкенд на :8080)
make dev

# Апгрейд тарифа
make upgrade-to-ultima EMAIL=user@example.com

# Локальный кластер
make cluster-dev

# Отдельная worker-нода с параметрами
make cluster-worker PORT=8081 NODE_ID=worker-01 MAIN_URL=http://127.0.0.1:8080
```

---

## Команды bash-скрипта `deploy.sh`

| Команда | Описание |
|---------|----------|
| `./deploy.sh build` | Сборка проекта (фронтенд + бэкенд) |
| `./deploy.sh start` | Запуск сервера в фоне |
| `./deploy.sh stop` | Остановка сервера |
| `./deploy.sh restart` | Перезапуск сервера |
| `./deploy.sh logs` | Просмотр логов (`tail -f`) |
| `./deploy.sh status` | Статус сервера |
| `./deploy.sh clean` | Очистка артефактов сборки |
| `./deploy.sh install` | Установка зависимостей (npm + go mod download) |
| `./deploy.sh dev` | Dev-режим (frontend hot-reload + backend) |
| `./deploy.sh cluster-main` | Запуск главной ноды кластера |
| `./deploy.sh cluster-worker` | Запуск worker-ноды кластера |
| `./deploy.sh cluster-dev` | Локальный кластер из 3 нод (main + 2 worker'а) |

### Примеры использования `deploy.sh`

```bash
# Production-запуск
./deploy.sh build
./deploy.sh start
./deploy.sh logs
./deploy.sh status
./deploy.sh stop

# Dev-режим (фронтенд + бэкенд параллельно)
./deploy.sh dev

# Запуск главной ноды кластера на порту 8080
./deploy.sh cluster-main

# Запуск главной ноды на другом порту
PORT=9090 ./deploy.sh cluster-main

# Запуск worker-ноды, подключающейся к главной на 8080
STENCILFORGE_NODE_ID=worker-01 PORT=8081 ./deploy.sh cluster-worker

# Worker, подключающийся к удалённой главной ноде
STENCILFORGE_MAIN_URL=http://192.168.1.100:8080 PORT=8081 ./deploy.sh cluster-worker

# Локальный кластер со своим секретом и 8 worker'ами (максимум)
STENCILFORGE_CLUSTER_SECRET=my-production-secret STENCILFORGE_MAX_WORKERS=8 ./deploy.sh cluster-dev

# Запуск с .env файлом
source .env && ./deploy.sh cluster-main
```

---

## Кластерная архитектура

StencilForge поддерживает горизонтальное масштабирование через кластерную архитектуру. Кластер состоит из **одной главной ноды (Main)** и множества **вычислительных нод (Worker)**.

```
                       Интернет
                          │
                          ▼
                 ┌─────────────────┐
                 │   Load Balancer  │  (nginx / HAProxy / Cloud LB)
                 │   (опционально)  │
                 └────────┬────────┘
                          │
                          ▼
                 ┌─────────────────┐
                 │   MAIN NODE     │
                 │   :8080         │
                 │                 │
                 │ • Auth (JWT)    │
                 │ • SQLite (БД)   │
                 │ • Payments      │
                 │ • Cluster Orch. │
                 │ • Proxy →       │
                 └───┬───┬───┬─────┘
                     │   │   │
            ┌────────┘   │   └────────┐
            ▼            ▼            ▼
     ┌──────────┐ ┌──────────┐ ┌──────────┐
     │ WORKER 1 │ │ WORKER 2 │ │ WORKER N │
     │ :8081    │ │ :8082    │ │ :808N    │
     │          │ │          │ │          │
     │ Process. │ │ Process. │ │ Process. │
     │ K-Means  │ │ K-Means  │ │ K-Means  │
     └──────────┘ └──────────┘ └──────────┘
```

### Роли нод

| Компонент | Назначение |
|-----------|------------|
| **Main Node** | Аутентификация (JWT), база данных (SQLite), платёжная система, оркестрация worker'ов, проксирование stencil-запросов, раздача фронтенда |
| **Worker Node** | Только обработка изображений (processor: k-means, маски, слои). Нет БД, нет авторизации, нет фронтенда |

### Переменные окружения кластера

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `STENCILFORGE_CLUSTER_MODE` | `main` | Режим ноды: `main` или `worker` |
| `STENCILFORGE_NODE_ID` | авто-генерация | Уникальный идентификатор ноды |
| `STENCILFORGE_ADVERTISE_URL` | `http://127.0.0.1:<PORT>` | URL, по которому другие ноды могут связаться с этой |
| `STENCILFORGE_MAIN_URL` | `http://127.0.0.1:8080` | URL главной ноды (только для worker) |
| `STENCILFORGE_MAX_WORKERS` | `4` | Максимальное количество worker-нод (для main) |
| `STENCILFORGE_CLUSTER_SECRET` | `dev-secret` | Секретный ключ для межнодовой аутентификации |

### Протокол взаимодействия

Worker-ноды регистрируются на главной ноде (`POST /api/cluster/register`) и периодически отправляют heartbeat (`POST /api/cluster/heartbeat`). Главная нода ведёт реестр активных worker'ов и распределяет задания по стратегии round-robin.

При поступлении запроса на обработку изображения (`/api/upload`, `/api/layers`):
1. Главная нода принимает запрос от пользователя
2. Задание сериализуется и отправляется на свободный worker (`POST /api/cluster/job`)
3. Worker выполняет обработку и возвращает результат
4. Главная нода возвращает результат пользователю

Если все worker'ы заняты, задание обрабатывается локально на главной ноде (fallback).

### Запуск кластера на одной машине (для разработки)

```bash
# Способ 1: одной командой
make cluster-dev

# Способ 2: через bash-скрипт
./deploy.sh cluster-dev

# Способ 3: вручную в трёх терминалах
# Терминал 1 — Главная нода:
PORT=8080 STENCILFORGE_CLUSTER_MODE=main STENCILFORGE_NODE_ID=main-01 \
  STENCILFORGE_ADVERTISE_URL=http://127.0.0.1:8080 STENCILFORGE_CLUSTER_SECRET=dev-secret \
  cd backend && go run .

# Терминал 2 — Worker 1:
PORT=8081 STENCILFORGE_CLUSTER_MODE=worker STENCILFORGE_NODE_ID=worker-01 \
  STENCILFORGE_ADVERTISE_URL=http://127.0.0.1:8081 STENCILFORGE_MAIN_URL=http://127.0.0.1:8080 \
  STENCILFORGE_CLUSTER_SECRET=dev-secret \
  cd backend && go run .

# Терминал 3 — Worker 2:
PORT=8082 STENCILFORGE_CLUSTER_MODE=worker STENCILFORGE_NODE_ID=worker-02 \
  STENCILFORGE_ADVERTISE_URL=http://127.0.0.1:8082 STENCILFORGE_MAIN_URL=http://127.0.0.1:8080 \
  STENCILFORGE_CLUSTER_SECRET=dev-secret \
  cd backend && go run .
```

---

## Объединение нескольких кластеров в единую систему (Multi-Cluster)

Для production-среды с высокой нагрузкой можно развернуть **несколько независимых кластеров**, объединённых через общую инфраструктуру.

### Схема Multi-Cluster

```
                    Глобальный Load Balancer
                    (Geo-DNS / Anycast)
                            │
            ┌───────────────┼───────────────┐
            ▼               ▼               ▼
    ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
    │  Кластер A   │ │  Кластер B   │ │  Кластер C   │
    │  (Европа)    │ │  (Азия)      │ │  (Америка)   │
    │              │ │              │ │              │
    │ Main :8080   │ │ Main :8080   │ │ Main :8080   │
    │  ├─ W1 :8081 │ │  ├─ W1 :8081 │ │  ├─ W1 :8081 │
    │  ├─ W2 :8082 │ │  ├─ W2 :8082 │ │  ├─ W2 :8082 │
    │  └─ W3 :8083 │ │  └─ W3 :8083 │ │  └─ W3 :8083 │
    └──────────────┘ └──────────────┘ └──────────────┘
            │               │               │
            └───────────────┼───────────────┘
                            │
                    ┌──────────────┐
                    │   Shared DB   │
                    │ (PostgreSQL)  │
                    └──────────────┘
```

### Шаги по объединению кластеров

#### 1. Общая база данных

Замените локальный SQLite на общий PostgreSQL, чтобы все кластеры работали с единой пользовательской базой, тарифами и платёжной информацией.

```bash
# .env на каждой главной ноде
DATABASE_URL=postgres://user:password@shared-db.internal:5432/stencilforge
```

#### 2. Общий JWT-секрет

Все главные ноды должны использовать одинаковый `JWT_SECRET`, чтобы токен, выданный одним кластером, принимался другим.

```bash
# .env — одинаковый на ВСЕХ главных нодах
JWT_SECRET=super-secret-shared-key-here
```

#### 3. Гео-балансировка

Настройте Geo-DNS или Anycast для маршрутизации пользователей к ближайшему кластеру:

```dns
; Пример DNS-конфигурации (Route53 / Cloud DNS)
stencilforge.com.   A    1.2.3.4    (Кластер A — Европа, latency-based)
stencilforge.com.   A    5.6.7.8    (Кластер B — Азия, latency-based)
stencilforge.com.   A    9.10.11.12 (Кластер C — Америка, latency-based)
```

Или через nginx reverse-proxy с geo-модулем:
```nginx
# /etc/nginx/nginx.conf
geo $cluster {
    default    cluster-a.internal;
    10.0.0.0/8 cluster-b.internal;
    172.16.0.0/12 cluster-c.internal;
}

server {
    listen 80;
    server_name stencilforge.com;
    location / {
        proxy_pass http://$cluster:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

#### 4. Общая файловая система для загруженных изображений

Используйте общее хранилище (NFS, S3, MinIO) для загруженных изображений, чтобы worker любого кластера мог обработать запрос, даже если изображение было загружено через другой кластер.

```bash
# Монтирование общего NFS
sudo mount -t nfs shared-storage.internal:/exports/stencilforge /var/lib/stencilforge

# .env
STENCILFORGE_DATA_DIR=/var/lib/stencilforge
```

Или через S3-совместимое хранилище (адаптировав код для загрузки/выгрузки изображений через S3 API).

#### 5. Очередь заданий (опционально)

Для больших инсталляций можно использовать внешнюю очередь сообщений (Redis, RabbitMQ) для координации заданий между кластерами, чтобы worker одного кластера мог обрабатывать задания из очереди другого при неравномерной нагрузке.

```bash
# .env (при добавлении поддержки очереди)
QUEUE_BACKEND=redis
REDIS_URL=redis://shared-redis.internal:6379/0
```

#### 6. Пример production-запуска Multi-Cluster

```bash
# === Сервер 1: Кластер A (Европа) ===
# .env
PORT=8080
STENCILFORGE_CLUSTER_MODE=main
STENCILFORGE_NODE_ID=main-eu-01
STENCILFORGE_ADVERTISE_URL=http://10.0.1.10:8080
STENCILFORGE_MAX_WORKERS=8
STENCILFORGE_CLUSTER_SECRET=production-secret-eu
JWT_SECRET=global-jwt-secret
DATABASE_URL=postgres://stencilforge:password@global-db.internal:5432/stencilforge
STENCILFORGE_DATA_DIR=/mnt/shared-storage/stencilforge

./deploy.sh cluster-main

# Worker'ы в том же дата-центре:
STENCILFORGE_NODE_ID=worker-eu-01 STENCILFORGE_MAIN_URL=http://10.0.1.10:8080 \
  STENCILFORGE_CLUSTER_MODE=worker STENCILFORGE_ADVERTISE_URL=http://10.0.1.11:8081 \
  PORT=8081 STENCILFORGE_CLUSTER_SECRET=production-secret-eu \
  ./deploy.sh cluster-worker

# === Сервер 2: Кластер B (Азия) ===
# Аналогичный .env, но другой NODE_ID и ADVERTISE_URL
STENCILFORGE_NODE_ID=main-as-01
STENCILFORGE_ADVERTISE_URL=http://10.0.2.10:8080
STENCILFORGE_CLUSTER_SECRET=production-secret-as
# JWT_SECRET и DATABASE_URL — те же, что и в Кластере A
```

### Важные замечания

- **JWT_SECRET** должен быть одинаковым на всех главных нодах всех кластеров
- **CLUSTER_SECRET** может быть разным для каждого изолированного кластера (worker'ы общаются только со своим main)
- Worker'ы разных кластеров **не общаются** друг с другом
- Каждый кластер — независимая единица со своей оркестрацией
- При использовании общей БД (PostgreSQL) замените драйвер SQLite на `pgx` или `pq` в `backend/db/db.go`

---

## Структура проекта

```
StencilForge/
├── backend/
│   ├── main.go                     # Точка входа, HTTP-сервер, инициализация кластера
│   ├── go.mod / go.sum
│   ├── auth/
│   │   └── auth.go                 # JWT-токены, bcrypt-хэширование, middleware
│   ├── cluster/
│   │   ├── config.go               # Конфигурация кластера из переменных окружения
│   │   ├── node.go                 # Узел кластера, HTTP-клиент, очереди, health-check
│   │   └── handlers.go             # HTTP-хендлеры кластерного взаимодействия
│   ├── db/
│   │   └── db.go                   # SQLite БД, модель пользователей
│   ├── handlers/
│   │   ├── handlers.go             # Обработчики API изображений (upload, layers, download)
│   │   ├── auth_handlers.go        # Обработчики аутентификации (register, login, logout, me)
│   │   ├── payment_handlers.go     # Платёжные обработчики (ЮKassa)
│   │   ├── cluster_proxy.go        # Проксирование запросов с main на worker-ноды
│   │   └── worker_jobs.go          # Обработчики заданий на worker-нодах
│   ├── payment/
│   │   └── payment.go              # Интеграция с ЮKassa API
│   └── processor/
│       └── processor.go            # Обработка изображений, k-means, маски
├── frontend/
│   ├── package.json
│   ├── tsconfig.json
│   ├── webpack.config.js
│   ├── public/
│   │   └── index.html
│   └── src/
│       ├── index.tsx
│       ├── App.tsx
│       ├── components/
│       │   ├── AuthPage.tsx        # Страница регистрации / входа
│       │   ├── LayersPanel.tsx     # Отображение и zoom слоёв
│       │   ├── ProfilePage.tsx     # Профиль пользователя, тарифы
│       │   ├── ThemeToggle.tsx     # Переключение темы
│       │   ├── UploadZone.tsx      # Загрузка изображения
│       │   └── WelcomePage.tsx     # Приветственная страница (тарифные планы)
│       ├── styles/
│       │   └── global.css
│       └── utils/
│           └── errors.ts           # Обработка ошибок API
├── Makefile
├── deploy.sh                        # Bash-скрипт сборки/развёртывания
├── .env.exemple                     # Пример конфигурации
├── .gitignore
└── README.md
```

---

## API

Все эндпоинты (кроме регистрации, входа и вебхуков) требуют заголовок:
```
Authorization: Bearer <JWT-токен>
```

### Аутентификация

#### `POST /api/register`
Регистрация нового пользователя.

**Request:**
```json
{
  "username": "user",
  "email": "user@example.com",
  "password": "secure_password",
  "newsletter_opt_in": false
}
```

**Response:**
```json
{
  "ok": true,
  "token": "eyJhbGciOiJI...",
  "user": {
    "id": 1,
    "username": "user",
    "email": "user@example.com",
    "plan": "free",
    "newsletter_opt_in": false
  }
}
```

#### `POST /api/login`
Вход в систему.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "secure_password"
}
```

**Response:**
```json
{
  "ok": true,
  "token": "eyJhbGciOiJI...",
  "user": { ... }
}
```

#### `GET /api/me`
Получение информации о текущем пользователе (требует токен).

#### `POST /api/logout`
Выход (инвалидация токена на стороне клиента).

### Тарифы

#### `GET /api/plans`
Получение списка доступных тарифных планов.

**Response:**
```json
{
  "plans": [
    { "id": "free", "name": "Free", "max_layers": 4, "price": 0 },
    { "id": "pro", "name": "Pro", "max_layers": 16, "price": 500 },
    { "id": "ultima", "name": "Ultima", "max_layers": 32, "price": 1500 }
  ]
}
```

### Платежи (ЮKassa)

#### `POST /api/create-payment`
Создание платежа для апгрейда тарифа. Требует авторизацию.

**Request:**
```json
{
  "plan": "pro"
}
```

**Response:**
```json
{
  "ok": true,
  "payment_url": "https://yoomoney.ru/checkout/...",
  "payment_id": "..."
}
```

#### `POST /api/check-payment`
Проверка статуса платежа. Требует авторизацию.

**Request:**
```json
{
  "payment_id": "..."
}
```

#### `POST /api/payment-webhook`
Вебхук ЮKassa для автоматического подтверждения платежей (публичный).

### Работа с изображениями

#### `POST /api/upload`
Загрузка изображения. Требует авторизацию.

**Request:** `multipart/form-data`, поле `image`

**Response:**
```json
{
  "session_id": "photo.png_12345",
  "width": 800,
  "height": 600
}
```

#### `POST /api/layers`
Генерация трафаретных слоёв. Требует авторизацию.

**Request:**
```json
{
  "session_id": "photo.png_12345",
  "num_layers": 4,
  "auto_layers": false
}
```

**Response:**
```json
{
  "session_id": "photo.png_12345",
  "layers": [
    {
      "index": 0,
      "download_url": "/api/download-all?session=...&layer=0",
      "data_url": "data:image/png;base64,..."
    }
  ]
}
```

#### `GET /api/download-all?session=...&layer=N`
Скачивание отдельного слоя как PNG. Требует авторизацию.

#### `POST /api/download-all`
Скачивание всех слоёв в ZIP-архиве. Требует авторизацию.

**Request:**
```json
{
  "session_id": "photo.png_12345"
}
```

### Кластерные эндпоинты (внутренние, защищены ClusterSecret)

| Метод | Эндпоинт | Описание |
|-------|----------|----------|
| POST | `/api/cluster/register` | Регистрация worker'а на главной ноде |
| POST | `/api/cluster/heartbeat` | Heartbeat от worker'а |
| POST | `/api/cluster/ping` | Проверка доступности ноды |
| POST | `/api/cluster/job` | Отправка задания на worker (только main → worker) |

---

## Алгоритм выделения слоёв

1. Конвертация RGB → CIELAB (перцептивно равномерное цветовое пространство)
2. Кластеризация k-means++ (50 итераций)
3. Постобработка бинарных масок:
   - Медианный фильтр 3×3 (удаление шума)
   - Морфологическое закрытие (заполнение дырок)
   - Фильтрация мелких компонент (< 0.1% площади)
4. Сортировка слоёв от тёмного к светлому (Rec. 601 luminance)

---

## Настройка порта

Переменная окружения `PORT` (по умолчанию 8080):

```bash
PORT=9090 make deploy
# или
PORT=9090 ./deploy.sh start
```

---

## Переменные окружения

Полный список переменных (см. `.env.exemple`):

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `PORT` | `8080` | Порт HTTP-сервера |
| `JWT_SECRET` | — | Секретный ключ для JWT-токенов (обязательно сменить) |
| `STENCILFORGE_DATA_DIR` | `~/.stencilforge` | Путь к директории с данными (SQLite БД, загруженные изображения) |
| `YOOKASSA_SHOP_ID` | — | Идентификатор магазина ЮKassa |
| `YOOKASSA_SECRET_KEY` | — | Секретный ключ ЮKassa |
| `YOOKASSA_RETURN_URL` | — | URL возврата после оплаты |
| `STENCILFORGE_CLUSTER_MODE` | `main` | Режим ноды: `main` или `worker` |
| `STENCILFORGE_NODE_ID` | авто | Уникальный ID ноды |
| `STENCILFORGE_ADVERTISE_URL` | `http://127.0.0.1:<PORT>` | URL этой ноды |
| `STENCILFORGE_MAIN_URL` | `http://127.0.0.1:8080` | URL главной ноды (для worker) |
| `STENCILFORGE_MAX_WORKERS` | `4` | Макс. количество worker'ов (для main) |
| `STENCILFORGE_CLUSTER_SECRET` | `dev-secret` | Секретный ключ межнодового обмена |

---

## Лицензия

MIT