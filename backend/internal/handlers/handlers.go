package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/polentrix/backend/internal/config"
	"github.com/polentrix/backend/internal/memory"
	"github.com/polentrix/backend/internal/ollama"
	"github.com/polentrix/backend/internal/store"
)

// Suggested catalog shown in the UI; downloaded tags not in this list are still included.
var suggestedModels = []string{
	"qwen2.5:0.5b",
	"qwen2.5:1.5b",
	"qwen2.5:3b",
	"qwen2.5:7b",
	"llama3.2:1b",
	"llama3.2:3b",
	"phi3:mini",
	"gemma2:2b",
	"mistral:7b",
}

var promptPresets = []map[string]string{
	{
		"id":     "default",
		"name":   "Default",
		"prompt": "You are Polentrix, a helpful local AI assistant.",
	},
	{
		"id":     "concise",
		"name":   "Concise",
		"prompt": "You are Polentrix. Answer clearly and briefly. Avoid filler.",
	},
	{
		"id":     "coder",
		"name":   "Coding",
		"prompt": "You are Polentrix, a careful coding assistant. Prefer correct, minimal code. Explain briefly when helpful.",
	},
	{
		"id":     "italian",
		"name":   "Italiano",
		"prompt": "Sei Polentrix, un assistente AI locale. Rispondi sempre in italiano, in modo chiaro e cordiale.",
	},
}

type Handler struct {
	client              *ollama.Client
	store               *store.Store
	assembler           *memory.Assembler
	model               string
	defaultSystemPrompt string
	defaultTemperature  float64
	memoryUserID        string
}

func New(client *ollama.Client, st *store.Store, cfg config.Config) *Handler {
	assembler := &memory.Assembler{
		Store:  st,
		Client: client,
		Cfg: memory.Config{
			MaxTokens:      cfg.ContextMaxTokens,
			KeepRecent:     cfg.ContextKeepRecent,
			RetrievalLimit: cfg.MemoryRetrievalLimit,
			UserID:         cfg.MemoryUserID,
			AutoExtract:    cfg.AutoExtractMemories,
		},
	}
	return &Handler{
		client:              client,
		store:               st,
		assembler:           assembler,
		model:               cfg.OllamaModel,
		defaultSystemPrompt: cfg.DefaultSystemPrompt,
		defaultTemperature:  cfg.DefaultTemperature,
		memoryUserID:        cfg.MemoryUserID,
	}
}

type catalogModel struct {
	Name       string `json:"name"`
	Downloaded bool   `json:"downloaded"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":              "ok",
		"model":               h.model,
		"default_system_prompt": h.defaultSystemPrompt,
		"default_temperature": h.defaultTemperature,
	})
}

func (h *Handler) PromptPresets(w http.ResponseWriter, r *http.Request) {
	presets := make([]map[string]string, len(promptPresets))
	copy(presets, promptPresets)
	if h.defaultSystemPrompt != "" {
		presets[0] = map[string]string{
			"id":     "default",
			"name":   "Default",
			"prompt": h.defaultSystemPrompt,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": presets})
}

func (h *Handler) Models(w http.ResponseWriter, r *http.Request) {
	list, err := h.client.ListModels(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	installed := make(map[string]ollama.ModelInfo, len(list.Models))
	for _, m := range list.Models {
		installed[m.Name] = m
		if strings.HasSuffix(m.Name, ":latest") {
			installed[strings.TrimSuffix(m.Name, ":latest")] = m
		}
	}

	seen := make(map[string]bool)
	out := make([]catalogModel, 0, len(suggestedModels)+len(list.Models))

	add := func(name string) {
		if seen[name] {
			return
		}
		entry := catalogModel{Name: name}
		if info, ok := installed[name]; ok {
			if seen[info.Name] {
				seen[name] = true
				return
			}
			entry.Downloaded = true
			entry.Size = info.Size
			entry.ModifiedAt = info.ModifiedAt
			entry.Name = info.Name
			seen[info.Name] = true
		}
		seen[name] = true
		out = append(out, entry)
	}

	if h.model != "" {
		add(h.model)
	}
	for _, name := range suggestedModels {
		add(name)
	}
	for _, m := range list.Models {
		add(m.Name)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"default": h.model,
		"models":  out,
	})
}

type pullBody struct {
	Name string `json:"name"`
}

func (h *Handler) PullModel(w http.ResponseWriter, r *http.Request) {
	var body pullBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	stream, err := h.client.PullStream(r.Context(), strings.TrimSpace(body.Name))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk ollama.PullChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]string{"error": chunk.Error}))
			flusher.Flush()
			break
		}

		payload := map[string]any{
			"status":    chunk.Status,
			"digest":    chunk.Digest,
			"total":     chunk.Total,
			"completed": chunk.Completed,
		}
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(payload))
		flusher.Flush()

		if chunk.Status == "success" {
			break
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]string{"error": err.Error()}))
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

type chatBody struct {
	Messages       []ollama.Message `json:"messages"`
	ConversationID string           `json:"conversation_id,omitempty"`
	Model          string           `json:"model,omitempty"`
	Options        *ollama.Options  `json:"options,omitempty"`
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	model := h.model
	if body.Model != "" {
		model = body.Model
	}

	opts := body.Options
	if opts == nil && h.defaultTemperature > 0 {
		t := h.defaultTemperature
		opts = &ollama.Options{Temperature: &t}
	}

	messages := body.Messages
	convID := strings.TrimSpace(body.ConversationID)
	var lastUser string
	if convID != "" {
		assembled, err := h.assembler.Assemble(r.Context(), convID, model)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		messages = assembled.Messages
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				lastUser = messages[i].Content
				break
			}
		}
	}
	if len(messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "messages or conversation_id required"})
		return
	}

	stream, err := h.client.ChatStream(r.Context(), model, messages, opts)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var assistantBuf strings.Builder

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk ollama.ChatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			msg := chunk.Error
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "not found") || strings.Contains(lower, "pull model") {
				msg = fmt.Sprintf("model %q is not available — pull it in Model settings first", model)
			}
			fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]string{"error": msg}))
			flusher.Flush()
			break
		}

		if chunk.Message.Content != "" {
			assistantBuf.WriteString(chunk.Message.Content)
		}

		payload := map[string]any{
			"content": chunk.Message.Content,
			"done":    chunk.Done,
			"role":    chunk.Message.Role,
		}
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(payload))
		flusher.Flush()

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]string{"error": err.Error()}))
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	if convID != "" && lastUser != "" && assistantBuf.Len() > 0 {
		go func() {
			bg, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			h.assembler.PersistExtractedMemories(bg, convID, model, lastUser, assistantBuf.String())
		}()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"error":"marshal failed"}`)
	}
	return b
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
