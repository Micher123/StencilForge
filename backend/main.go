package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"stencilforge/auth"
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

	mux := http.NewServeMux()

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

	// Stencil endpoints (защищённые)
	mux.HandleFunc("/api/upload", handlers.CORS(auth.AuthMiddleware(handlers.UploadHandler)))
	mux.HandleFunc("/api/layers", handlers.CORS(auth.AuthMiddleware(handlers.LayersHandler)))
	mux.HandleFunc("/api/download-all", handlers.CORS(auth.AuthMiddleware(handlers.DownloadAllHandler)))

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

	fmt.Printf("StencilForge server listening on :%s\n", port)
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
