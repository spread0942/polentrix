package main

import (
	"log"
	"net/http"
	"os"

	"github.com/polentrix/backend/internal/config"
	"github.com/polentrix/backend/internal/handlers"
	"github.com/polentrix/backend/internal/ollama"
)

func main() {
	cfg := config.Load()
	client := ollama.NewClient(cfg.OllamaBaseURL)
	h := handlers.New(client, cfg.OllamaModel)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/models", h.Models)
	mux.HandleFunc("POST /api/models/pull", h.PullModel)
	mux.HandleFunc("POST /api/chat", h.Chat)

	addr := ":" + cfg.Port
	log.Printf("listening on %s (ollama=%s model=%s)", addr, cfg.OllamaBaseURL, cfg.OllamaModel)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
