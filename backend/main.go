package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"stencilforge/auth"
	"stencilforge/cluster"
	"stencilforge/db"
	"stencilforge/handlers"
)

func main() {
	// Загрузка .env (если есть)
	loadEnv()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Загрузка конфигурации кластера
	clusterCfg := cluster.LoadConfig()

	// Инициализация узла кластера
	node := cluster.NewNode(clusterCfg)

	mux := http.NewServeMux()

	// Кластерные хендлеры
	ch := &cluster.ClusterHandlers{Node: node}

	// Главная нода: БД, авторизация, проксирование stencil-запросов
	if clusterCfg.IsMain() {
		// Инициализация БД
		dataDir := os.Getenv("STENCILFORGE_DATA_DIR")
		if dataDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				home = "."
			}
			dataDir = filepath.Join(home, ".stencilforge")
		}
		if err := db.Init(dataDir); err != nil {
			log.Fatalf("db init: %v", err)
		}
		defer db.Close()

		// Auth endpoints (публичные)
		mux.HandleFunc("/api/register", handlers.CORS(handlers.RegisterHandler))
		mux.HandleFunc("/api/login", handlers.CORS(handlers.LoginHandler))
		mux.HandleFunc("/api/logout", handlers.CORS(handlers.LogoutHandler))
		mux.HandleFunc("/api/me", handlers.CORS(auth.AuthMiddleware(handlers.MeHandler)))

		// Plans — публичный
		mux.HandleFunc("/api/plans", handlers.CORS(handlers.PlansHandler))

		// Payment endpoints (защищённые)
		mux.HandleFunc("/api/create-payment", handlers.CORS(auth.AuthMiddleware(handlers.CreatePaymentHandler)))
		mux.HandleFunc("/api/check-payment", handlers.CORS(auth.AuthMiddleware(handlers.CheckPaymentHandler)))

		// Webhook — публичный (от ЮKassa)
		mux.HandleFunc("/api/payment-webhook", handlers.CORS(handlers.PaymentWebhookHandler))

		// Stencil endpoints (защищённые) — с кластерным роутингом
		mux.HandleFunc("/api/upload", handlers.CORS(auth.AuthMiddleware(
			handlers.UploadHandlerWithCluster(node),
		)))
		mux.HandleFunc("/api/layers", handlers.CORS(auth.AuthMiddleware(
			handlers.LayersHandlerWithCluster(node),
		)))
		mux.HandleFunc("/api/download-all", handlers.CORS(auth.AuthMiddleware(
			handlers.DownloadAllHandlerWithCluster(node),
		)))

		// Кластерные эндпоинты (main)
		mux.HandleFunc("/api/cluster/register", ch.ClusterMiddleware(ch.RegisterHandler))
		mux.HandleFunc("/api/cluster/heartbeat", ch.ClusterMiddleware(ch.HeartbeatHandler))
		mux.HandleFunc("/api/cluster/ping", ch.ClusterMiddleware(ch.PingHandler))

		log.Printf("[cluster] starting as MAIN node %s (%s)", clusterCfg.NodeID, clusterCfg.AdvertiseURL)
	} else {
		// Worker нода: только кластерные эндпоинты, без БД
		ch.WorkerJobHandler = executeWorkerJob

		mux.HandleFunc("/api/cluster/ping", ch.ClusterMiddleware(ch.PingHandler))
		mux.HandleFunc("/api/cluster/job", ch.ClusterMiddleware(ch.JobHandler))

		log.Printf("[cluster] starting as WORKER node %s (%s), main: %s", clusterCfg.NodeID, clusterCfg.AdvertiseURL, clusterCfg.MainURL)
	}

	// Serve built frontend (production) or fallback to public for dev
	distDirs := []string{
		"frontend/public/dist",
		"../frontend/public/dist",
	}
	var distDir string
	for _, d := range distDirs {
		if _, err := os.Stat(d); err == nil {
			distDir = d
			break
		}
	}
	if distDir != "" {
		fs := http.FileServer(http.Dir(distDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Не перехватываем API-запросы
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			path := filepath.Join(distDir, r.URL.Path)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
	} else {
		fs := http.FileServer(http.Dir("frontend/public"))
		mux.Handle("/", http.StripPrefix("/", fs))
	}

	// Запуск кластерных циклов
	node.Start()

	fmt.Printf("StencilForge server listening on :%s (mode: %s)\n", port, clusterCfg.Mode)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// loadEnv загружает переменные из .env файла (простой парсер)
func loadEnv() {
	// Ищем .env в текущей директории и родительской
	paths := []string{".env", "../.env", "../../.env"}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		parseEnvFile(string(data))
		fmt.Printf("Loaded env from %s\n", p)
		break
	}
}

func parseEnvFile(content string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Убираем export prefix если есть
		line = strings.TrimPrefix(line, "export ")
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Убираем кавычки
		val = strings.Trim(val, `"'`)
		if os.Getenv(key) == "" { // не перезаписываем уже установленные
			os.Setenv(key, val)
		}
	}
}

// executeWorkerJob — функция-обработчик заданий на worker-ноде
func executeWorkerJob(w http.ResponseWriter, r *http.Request, job cluster.JobRequest) *cluster.JobResult {
	switch job.Type {
	case "upload":
		return handlers.HandleUploadJob(job)
	case "layers":
		return handlers.HandleLayersJob(job)
	case "download":
		return handlers.HandleDownloadJob(job)
	default:
		return &cluster.JobResult{JobID: job.JobID, Error: "unknown job type: " + job.Type}
	}
}
