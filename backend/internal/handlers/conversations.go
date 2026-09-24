package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/polentrix/backend/internal/store"
)

type createConversationBody struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	SystemPrompt *string  `json:"system_prompt"`
	Temperature  *float64 `json:"temperature"`
	TopP         *float64 `json:"top_p"`
	NumPredict   *int     `json:"num_predict"`
	Model        string   `json:"model"`
}

type patchConversationBody struct {
	Title        *string  `json:"title"`
	SystemPrompt *string  `json:"system_prompt"`
	Temperature  *float64 `json:"temperature"`
	TopP         *float64 `json:"top_p"`
	NumPredict   *int     `json:"num_predict"`
	Model        *string  `json:"model"`
	// When true, clear nullable generation fields back to server defaults.
	ClearTemperature bool `json:"clear_temperature"`
	ClearTopP        bool `json:"clear_top_p"`
	ClearNumPredict  bool `json:"clear_num_predict"`
}

type createMessageBody struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type patchMessageBody struct {
	Content string `json:"content"`
}

func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListConversations()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": list})
}

func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var body createConversationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(body.ID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	systemPrompt := h.defaultSystemPrompt
	if body.SystemPrompt != nil {
		systemPrompt = *body.SystemPrompt
	}
	temp := h.defaultTemperature
	temperature := &temp
	if body.Temperature != nil {
		temperature = body.Temperature
	}

	c, err := h.store.CreateConversation(store.Conversation{
		ID:           strings.TrimSpace(body.ID),
		Title:        body.Title,
		SystemPrompt: systemPrompt,
		Temperature:  temperature,
		TopP:         body.TopP,
		NumPredict:   body.NumPredict,
		Model:        body.Model,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.store.GetConversation(id, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) PatchConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur, err := h.store.GetConversation(id, false)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	var body patchConversationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if body.Title != nil {
		cur.Title = *body.Title
	}
	if body.SystemPrompt != nil {
		cur.SystemPrompt = *body.SystemPrompt
	}
	if body.ClearTemperature {
		cur.Temperature = nil
	} else if body.Temperature != nil {
		cur.Temperature = body.Temperature
	}
	if body.ClearTopP {
		cur.TopP = nil
	} else if body.TopP != nil {
		cur.TopP = body.TopP
	}
	if body.ClearNumPredict {
		cur.NumPredict = nil
	} else if body.NumPredict != nil {
		cur.NumPredict = body.NumPredict
	}
	if body.Model != nil {
		cur.Model = *body.Model
	}

	updated, err := h.store.UpdateConversation(*cur)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteConversation(id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	conversationID := r.PathValue("id")
	if _, err := h.store.GetConversation(conversationID, false); err != nil {
		writeStoreError(w, err)
		return
	}

	var body createMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(body.ID) == "" || strings.TrimSpace(body.Role) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and role required"})
		return
	}

	m, err := h.store.CreateMessage(store.Message{
		ID:             strings.TrimSpace(body.ID),
		ConversationID: conversationID,
		Role:           strings.TrimSpace(body.Role),
		Content:        body.Content,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *Handler) PatchMessage(w http.ResponseWriter, r *http.Request) {
	conversationID := r.PathValue("id")
	messageID := r.PathValue("mid")

	var body patchMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	m, err := h.store.UpdateMessage(conversationID, messageID, body.Content)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	conversationID := r.PathValue("id")
	messageID := r.PathValue("mid")
	if err := h.store.DeleteMessage(conversationID, messageID); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
