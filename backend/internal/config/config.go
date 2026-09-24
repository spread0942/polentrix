package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                string
	OllamaBaseURL       string
	OllamaModel         string
	DatabaseURL         string
	DefaultSystemPrompt string
	DefaultTemperature  float64

	// Memory / context window (Phase 2)
	MemoryUserID           string
	ContextMaxTokens       int
	ContextKeepRecent      int
	MemoryRetrievalLimit   int
	AutoExtractMemories    bool
}

func Load() Config {
	return Config{
		Port:                getenv("PORT", "8080"),
		OllamaBaseURL:       getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:         getenv("OLLAMA_MODEL", "qwen2.5:0.5b"),
		DatabaseURL:         getenv("DATABASE_URL", "postgres://polentrix:polentrix@localhost:5432/polentrix?sslmode=disable"),
		DefaultSystemPrompt: getenv("DEFAULT_SYSTEM_PROMPT", "You are Polentrix, a helpful local AI assistant."),
		DefaultTemperature:  getenvFloat("DEFAULT_TEMPERATURE", 0.7),

		MemoryUserID:         getenv("MEMORY_USER_ID", "local"),
		ContextMaxTokens:     getenvInt("CONTEXT_MAX_TOKENS", 6000),
		ContextKeepRecent:    getenvInt("CONTEXT_KEEP_RECENT", 12),
		MemoryRetrievalLimit: getenvInt("MEMORY_RETRIEVAL_LIMIT", 8),
		AutoExtractMemories:  getenvBool("AUTO_EXTRACT_MEMORIES", true),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
