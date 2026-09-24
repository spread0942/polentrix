package memory

import (
	"strings"
	"testing"

	"github.com/polentrix/backend/internal/store"
)

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatal("empty should be 0")
	}
	n := EstimateTokens("abcd")
	if n != 1 {
		t.Fatalf("got %d want 1", n)
	}
	n = EstimateTokens(strings.Repeat("x", 40))
	if n != 10 {
		t.Fatalf("got %d want 10", n)
	}
}

func TestRetrieveRelevant(t *testing.T) {
	memories := []store.Memory{
		{ID: "1", Content: "User prefers dark mode in the editor"},
		{ID: "2", Content: "User's name is Alice and lives in Rome"},
		{ID: "3", Content: "Working on a Go backend for Polentrix"},
	}
	got := RetrieveRelevant(memories, "What is my name?", 2)
	if len(got) == 0 {
		t.Fatal("expected at least one memory")
	}
	found := false
	for _, m := range got {
		if strings.Contains(m.Content, "Alice") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Alice memory, got %#v", got)
	}
}

func TestPlanWindowSummarizesWhenLarge(t *testing.T) {
	msgs := make([]store.Message, 0, 30)
	for i := 1; i <= 30; i++ {
		msgs = append(msgs, store.Message{
			ID:       string(rune('a' + (i % 26))),
			Role:     "user",
			Content:  strings.Repeat("hello world ", 20),
			Position: i,
		})
	}
	res := PlanWindow(msgs, "", 0, 800, 6, 50)
	if !res.NeedsSummary {
		t.Fatal("expected NeedsSummary for oversized conversation")
	}
	if len(res.Recent) == 0 {
		t.Fatal("expected recent messages")
	}
	if len(res.ToSummarize) == 0 {
		t.Fatal("expected messages to summarize")
	}
}

func TestPlanWindowKeepsSmallConversations(t *testing.T) {
	msgs := []store.Message{
		{Role: "user", Content: "hi", Position: 1},
		{Role: "assistant", Content: "hello", Position: 2},
	}
	res := PlanWindow(msgs, "", 0, 6000, 12, 50)
	if res.NeedsSummary {
		t.Fatal("small conversation should not need summary")
	}
	if len(res.Recent) != 2 {
		t.Fatalf("got %d recent", len(res.Recent))
	}
}

func TestApplySummary(t *testing.T) {
	res := WindowResult{
		Summary:     "Old facts.",
		ToSummarize: []store.Message{{Position: 5}, {Position: 8}},
	}
	res = ApplySummary(res, "New facts about Alice.")
	if !res.DidSummarize {
		t.Fatal("expected DidSummarize")
	}
	if res.SummarizedUntil != 8 {
		t.Fatalf("summarized_until=%d", res.SummarizedUntil)
	}
	if !strings.Contains(res.Summary, "Old facts") || !strings.Contains(res.Summary, "Alice") {
		t.Fatalf("summary=%q", res.Summary)
	}
}

func TestFormatMemoriesAndBuildSystem(t *testing.T) {
	conv := "c1"
	mems := []store.Memory{
		{Content: "Likes espresso", ConversationID: nil},
		{Content: "Building Phase 2", ConversationID: &conv},
	}
	block := FormatMemories(mems)
	if !strings.Contains(block, "espresso") || !strings.Contains(block, "Phase 2") {
		t.Fatalf("block=%q", block)
	}
	sys := BuildSystemPrompt("You are Polentrix.", mems, "Earlier they discussed Docker.")
	if !strings.Contains(sys, "Polentrix") {
		t.Fatal("missing base")
	}
	if !strings.Contains(sys, "espresso") {
		t.Fatal("missing memories")
	}
	if !strings.Contains(sys, "Docker") {
		t.Fatal("missing summary")
	}
}

func TestToOllamaSkipsEmptyAssistant(t *testing.T) {
	msgs := []store.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: ""},
		{Role: "system", Content: "ignore"},
		{Role: "assistant", Content: "ok"},
	}
	out := ToOllama(msgs)
	if len(out) != 2 {
		t.Fatalf("got %#v", out)
	}
	if out[0].Role != "user" || out[1].Content != "ok" {
		t.Fatalf("got %#v", out)
	}
}
