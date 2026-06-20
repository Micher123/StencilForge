# StencilForge

Сервис для создания трафаретных слоёв из изображений через веб-интерфейс.

Загрузите изображение, укажите количество слоёв — и получите набор чёрно-белых масок, готовых для печати, вырезания и послойного нанесения краски.

- Загрузка изображений в форматах PNG, JPEG, BMP, TIFF
- Автоматическая / ручная установка количества слоёв (2–16)
- Качественная кластеризация: k-means++ в цветовом пространстве CIELAB, 50 итераций
- Постобработка масок: медианный фильтр, морфологическое закрытие, фильтрация мелких компонент
- Сортировка слоёв от тёмного к светлому для правильного послойного наложения
- Увеличение слоя по клику на миниатюру (модальное окно)
- Скачивание как отдельных слоёв (PNG), так и всех разом (ZIP)
- **Аутентификация пользователей**: регистрация, вход, JWT-токены, bcrypt-хэширование паролей
- Светлая / тёмная тема веб-интерфейса

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

## Быстрый старт

### 1. Клонирование

```bash
git clone https://github.com/Micher123/StencilForge.git
cd StencilForge
```

### 2. Сборка и запуск одной командой

```bash
make run
```

Сервер будет доступен на **http://localhost:8080**.

### 3. Альтернативно — через bash-скрипт

```bash
chmod +x deploy.sh
./deploy.sh build   # Сборка
./deploy.sh start   # Запуск в фоне
./deploy.sh logs    # Просмотр логов
./deploy.sh stop    # Остановка
```

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

## Команды bash-скрипта `deploy.sh`

| Команда | Описание |
|---------|----------|
| `./deploy.sh build` | Сборка проекта |
| `./deploy.sh start` | Запуск сервера в фоне |
| `./deploy.sh stop` | Остановка сервера |
| `./deploy.sh restart` | Перезапуск сервера |
| `./deploy.sh logs` | Просмотр логов |
| `./deploy.sh status` | Статус сервера |
| `./deploy.sh clean` | Очистка артефактов |
| `./deploy.sh install` | Установка зависимостей |
| `./deploy.sh dev` | Dev-режим (frontend + backend) |

## Структура проекта

```
StencilForge/
├── backend/
│   ├── main.go                 # Точка входа, HTTP-сервер
│   ├── go.mod / go.sum
│   ├── handlers/
│   │   ├── handlers.go         # Обработчики API изображений (upload, layers, download)
│   │   └── auth_handlers.go    # Обработчики аутентификации (register, login, logout, me)
│   ├── processor/processor.go  # Обработка изображений, k-means, маски
│   ├── auth/auth.go            # JWT-токены, bcrypt-хэширование, middleware
│   └── db/db.go                # SQLite БД, модель пользователей
├── frontend/
│   ├── package.json
│   ├── tsconfig.json
│   ├── webpack.config.js
│   ├── public/index.html
│   └── src/
│       ├── index.tsx
│       ├── App.tsx
│       ├── components/
│       │   ├── AuthPage.tsx     # Страница регистрации / входа
│       │   ├── LayersPanel.tsx  # Отображение и zoom слоёв
│       │   ├── ThemeToggle.tsx  # Переключение темы
│       │   └── UploadZone.tsx   # Загрузка изображения
│       └── styles/global.css
├── Makefile
├── deploy.sh                    # Bash-скрипт сборки/развёртывания
├── README.md
└── .gitignore
```

## API

Все эндпоинты (кроме регистрации и входа) требуют заголовок:
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

## Алгоритм выделения слоёв

1. Конвертация RGB → CIELAB (перцептивно равномерное цветовое пространство)
2. Кластеризация k-means++ (50 итераций)
3. Постобработка бинарных масок:
   - Медианный фильтр 3×3 (удаление шума)
   - Морфологическое закрытие (заполнение дырок)
   - Фильтрация мелких компонент (< 0.1% площади)
4. Сортировка слоёв от тёмного к светлому (Rec. 601 luminance)

## Настройка порта

Переменная окружения `PORT` (по умолчанию 8080):

```bash
PORT=9090 make deploy
```

## Лицензия

MIT