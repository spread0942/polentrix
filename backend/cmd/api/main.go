package main

import (
	"log"
	"net/http"
	"os"

	"github.com/polentrix/backend/internal/config"
	"github.com/polentrix/backend/internal/handlers"
	"github.com/polentrix/backend/internal/ollama"
	"github.com/polentrix/backend/internal/store"
)

func main() {
	cfg := config.Load()

	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		log.Printf("database: %v", err)
		os.Exit(1)
	}
	defer st.Close()

	client := ollama.NewClient(cfg.OllamaBaseURL)
	h := handlers.New(client, st, cfg.OllamaModel, cfg.DefaultSystemPrompt, cfg.DefaultTemperature)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/models", h.Models)
	mux.HandleFunc("POST /api/models/pull", h.PullModel)
	mux.HandleFunc("GET /api/prompt-presets", h.PromptPresets)
	mux.HandleFunc("POST /api/chat", h.Chat)

	mux.HandleFunc("GET /api/conversations", h.ListConversations)
	mux.HandleFunc("POST /api/conversations", h.CreateConversation)
	mux.HandleFunc("GET /api/conversations/{id}", h.GetConversation)
	mux.HandleFunc("PATCH /api/conversations/{id}", h.PatchConversation)
	mux.HandleFunc("DELETE /api/conversations/{id}", h.DeleteConversation)
	mux.HandleFunc("POST /api/conversations/{id}/messages", h.CreateMessage)
	mux.HandleFunc("PATCH /api/conversations/{id}/messages/{mid}", h.PatchMessage)
	mux.HandleFunc("DELETE /api/conversations/{id}/messages/{mid}", h.DeleteMessage)

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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
