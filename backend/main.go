package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"stencilforge/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/upload", handlers.CORS(handlers.UploadHandler))
	mux.HandleFunc("/api/layers", handlers.CORS(handlers.LayersHandler))
	mux.HandleFunc("/api/download-all", handlers.CORS(handlers.DownloadAllHandler))

	// Serve built frontend (production) or fallback to public for dev
	distDir := "../frontend/public/dist"
	if _, err := os.Stat(distDir); err == nil {
		fs := http.FileServer(http.Dir(distDir))
		mux.Handle("/", http.StripPrefix("/", fs))
	} else {
		fs := http.FileServer(http.Dir("../frontend/public"))
		mux.Handle("/", http.StripPrefix("/", fs))
	}

	fmt.Printf("StencilForge server listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
