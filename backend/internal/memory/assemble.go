package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/polentrix/backend/internal/ollama"
	"github.com/polentrix/backend/internal/store"
)

// Assembler builds the LLM message list for a conversation turn.
type Assembler struct {
	Store  *store.Store
	Client *ollama.Client
	Cfg    Config
}

// AssembleResult is the prepared chat context.
type AssembleResult struct {
	Messages        []ollama.Message
	MemoriesUsed    []store.Memory
	DidSummarize    bool
	ContextSummary  string
	SummarizedUntil int
	SystemPrompt    string
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Assemble loads conversation state, manages the context window, retrieves
// long-term memories, and returns messages ready for the LLM.
func (a *Assembler) Assemble(ctx context.Context, conversationID, model string) (*AssembleResult, error) {
	conv, err := a.Store.GetConversation(conversationID, true)
	if err != nil {
		return nil, err
	}

	systemBase := conv.SystemPrompt
	systemTokens := EstimateTokens(systemBase) + 64

	window := PlanWindow(
		conv.Messages,
		conv.ContextSummary,
		conv.SummarizedUntil,
		a.Cfg.MaxTokens,
		a.Cfg.KeepRecent,
		systemTokens,
	)

	if window.NeedsSummary && len(window.ToSummarize) > 0 {
		chunk, sumErr := SummarizeMessages(ctx, a.Client, model, window.Summary, window.ToSummarize)
		if sumErr != nil {
			log.Printf("memory: summarize failed: %v (sending truncated recent messages)", sumErr)
			window.NeedsSummary = false
			window.ToSummarize = nil
		} else {
			window = ApplySummary(window, chunk)
			if err := a.Store.UpdateContextSummary(conversationID, window.Summary, window.SummarizedUntil); err != nil {
				log.Printf("memory: persist summary failed: %v", err)
			}
		}
	}

	query := ""
	for i := len(conv.Messages) - 1; i >= 0; i-- {
		if conv.Messages[i].Role == "user" && conv.Messages[i].Content != "" {
			query = conv.Messages[i].Content
			break
		}
	}

	allMem, err := a.Store.ListAccessibleMemories(a.Cfg.UserID, conversationID)
	if err != nil {
		return nil, err
	}
	relevant := RetrieveRelevant(allMem, query, a.Cfg.RetrievalLimit)

	system := BuildSystemPrompt(systemBase, relevant, window.Summary)

	msgs := make([]ollama.Message, 0, 1+len(window.Recent))
	if system != "" {
		msgs = append(msgs, ollama.Message{Role: "system", Content: system})
	}
	msgs = append(msgs, ToOllama(window.Recent)...)

	return &AssembleResult{
		Messages:        msgs,
		MemoriesUsed:    relevant,
		DidSummarize:    window.DidSummarize,
		ContextSummary:  window.Summary,
		SummarizedUntil: window.SummarizedUntil,
		SystemPrompt:    system,
	}, nil
}

// PersistExtractedMemories stores facts from a turn as conversation-scoped memories.
func (a *Assembler) PersistExtractedMemories(ctx context.Context, conversationID, model, userMsg, assistantMsg string) {
	if !a.Cfg.AutoExtract {
		return
	}
	facts, err := ExtractMemories(ctx, a.Client, model, userMsg, assistantMsg)
	if err != nil {
		log.Printf("memory: extract failed: %v", err)
		return
	}
	convID := conversationID
	for _, fact := range facts {
		if _, err := a.Store.FindSimilarMemory(a.Cfg.UserID, &convID, fact); err == nil {
			continue
		}
		_, err := a.Store.CreateMemory(store.Memory{
			ID:             newID(),
			UserID:         a.Cfg.UserID,
			ConversationID: &convID,
			Content:        fact,
		})
		if err != nil {
			log.Printf("memory: create extracted failed: %v", err)
		}
	}
}
