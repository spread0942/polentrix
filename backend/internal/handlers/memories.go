package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/polentrix/backend/internal/store"
)

type createMemoryBody struct {
	ID             string  `json:"id"`
	Content        string  `json:"content"`
	ConversationID *string `json:"conversation_id"`
}

type patchMemoryBody struct {
	Content string `json:"content"`
}

func (h *Handler) ListMemories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	scope := q.Get("scope") // "user" | "conversation" | "all" (default all when conversation_id set)
	convID := strings.TrimSpace(q.Get("conversation_id"))

	var list []store.Memory
	var err error

	switch {
	case scope == "user" || (convID == "" && scope != "conversation"):
		list, err = h.store.ListMemories(h.memoryUserID, nil)
	case convID != "":
		if scope == "conversation" {
			list, err = h.store.ListMemories(h.memoryUserID, &convID)
		} else {
			list, err = h.store.ListAccessibleMemories(h.memoryUserID, convID)
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "conversation_id required for conversation scope"})
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": list})
}

func (h *Handler) CreateMemory(w http.ResponseWriter, r *http.Request) {
	var body createMemoryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	id := strings.TrimSpace(body.ID)
	content := strings.TrimSpace(body.Content)
	if id == "" || content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and content required"})
		return
	}

	var convID *string
	if body.ConversationID != nil {
		c := strings.TrimSpace(*body.ConversationID)
		if c == "" {
			convID = nil
		} else {
			if _, err := h.store.GetConversation(c, false); err != nil {
				writeStoreError(w, err)
				return
			}
			convID = &c
		}
	}

	m, err := h.store.CreateMemory(store.Memory{
		ID:             id,
		UserID:         h.memoryUserID,
		ConversationID: convID,
		Content:        content,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *Handler) GetMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := h.store.GetMemory(h.memoryUserID, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) PatchMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body patchMemoryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content required"})
		return
	}
	m, err := h.store.UpdateMemory(h.memoryUserID, id, content)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteMemory(h.memoryUserID, id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
