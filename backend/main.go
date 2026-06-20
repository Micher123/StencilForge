package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"stencilforge/auth"
	"stencilforge/db"
	"stencilforge/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Инициализация БД
	dataDir := os.Getenv("STENCILFORGE_DATA_DIR")
	if dataDir == "" {
		// по умолчанию в домашней директории пользователя
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

	// Stencil endpoints (защищённые)
	mux.HandleFunc("/api/upload", handlers.CORS(auth.AuthMiddleware(handlers.UploadHandler)))
	mux.HandleFunc("/api/layers", handlers.CORS(auth.AuthMiddleware(handlers.LayersHandler)))
	mux.HandleFunc("/api/download-all", handlers.CORS(auth.AuthMiddleware(handlers.DownloadAllHandler)))

	// Serve built frontend (production) or fallback to public for dev
	// Try several possible locations for dist directory
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
		// Не-static файлы отдаём index.html (SPA routing)
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
