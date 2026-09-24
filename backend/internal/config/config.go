package config

import "os"

type Config struct {
	Port          string
	OllamaBaseURL string
	OllamaModel   string
}

func Load() Config {
	return Config{
		Port:          getenv("PORT", "8080"),
		OllamaBaseURL: getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:   getenv("OLLAMA_MODEL", "qwen2.5:0.5b"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
