package memory

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/polentrix/backend/internal/ollama"
	"github.com/polentrix/backend/internal/store"
)

// Config controls short-term context window and long-term retrieval.
type Config struct {
	MaxTokens       int
	KeepRecent      int
	RetrievalLimit  int
	UserID          string
	AutoExtract     bool
}

// EstimateTokens approximates token count (chars/4) without a tokenizer.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	n := (len(text) + 3) / 4
	if n < 1 {
		return 1
	}
	return n
}

func EstimateMessages(msgs []ollama.Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateTokens(m.Role) + EstimateTokens(m.Content) + 4
	}
	return total
}

// FormatMemories builds a prompt block from retrieved memories.
func FormatMemories(memories []store.Memory) string {
	if len(memories) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Long-term memory (facts you should remember about the user and this conversation):\n")
	for i, m := range memories {
		scope := "user"
		if m.ConversationID != nil {
			scope = "conversation"
		}
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, scope, strings.TrimSpace(m.Content))
	}
	b.WriteString("Use these facts when relevant. Do not invent memories.")
	return b.String()
}

// FormatSummary builds a short-term summary injection message.
func FormatSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	return "Earlier conversation summary:\n" + summary
}

// RetrieveRelevant scores memories against a query using keyword overlap.
// No embeddings yet (those arrive with Phase 3 RAG); this is a lightweight baseline.
func RetrieveRelevant(memories []store.Memory, query string, limit int) []store.Memory {
	if limit <= 0 || len(memories) == 0 {
		return nil
	}
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		// No query tokens — return most recently updated up to limit.
		if len(memories) > limit {
			return memories[:limit]
		}
		return memories
	}

	type scored struct {
		m     store.Memory
		score float64
	}
	ranked := make([]scored, 0, len(memories))
	for _, m := range memories {
		s := overlapScore(qTokens, tokenize(m.Content))
		if s <= 0 {
			continue
		}
		ranked = append(ranked, scored{m: m, score: s})
	}
	// Insertion sort by score desc (small N).
	for i := 1; i < len(ranked); i++ {
		j := i
		for j > 0 && ranked[j].score > ranked[j-1].score {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
			j--
		}
	}
	if len(ranked) == 0 {
		// Fall back to newest memories so the model still has some context.
		if len(memories) > limit {
			return memories[:limit]
		}
		return memories
	}
	out := make([]store.Memory, 0, limit)
	for i := 0; i < len(ranked) && i < limit; i++ {
		out = append(out, ranked[i].m)
	}
	return out
}

func tokenize(s string) map[string]struct{} {
	s = strings.ToLower(s)
	out := make(map[string]struct{})
	var b strings.Builder
	flush := func() {
		if b.Len() < 2 {
			b.Reset()
			return
		}
		w := b.String()
		b.Reset()
		if isStopWord(w) {
			return
		}
		out[w] = struct{}{}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func overlapScore(query, doc map[string]struct{}) float64 {
	if len(query) == 0 || len(doc) == 0 {
		return 0
	}
	hits := 0
	for t := range query {
		if _, ok := doc[t]; ok {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return float64(hits) / float64(len(query))
}

func isStopWord(w string) bool {
	switch w {
	case "a", "an", "the", "is", "are", "was", "were", "be", "been", "being",
		"to", "of", "in", "on", "for", "with", "at", "by", "from", "as",
		"and", "or", "but", "if", "then", "so", "that", "this", "these", "those",
		"i", "you", "he", "she", "it", "we", "they", "me", "my", "your", "our",
		"do", "does", "did", "have", "has", "had", "not", "no", "yes",
		"what", "when", "where", "who", "why", "how", "can", "will", "would",
		"please", "thanks", "thank":
		return true
	default:
		return false
	}
}

// WindowResult is the short-term context after applying the token budget.
type WindowResult struct {
	Summary         string
	SummarizedUntil int
	Recent          []store.Message
	DidSummarize    bool
	NeedsSummary    bool
	ToSummarize     []store.Message
}

// PlanWindow decides whether older messages need summarization and which recent
// messages to keep verbatim.
func PlanWindow(msgs []store.Message, existingSummary string, summarizedUntil, maxTokens, keepRecent int, systemTokens int) WindowResult {
	if keepRecent < 2 {
		keepRecent = 2
	}
	if maxTokens < 500 {
		maxTokens = 500
	}

	active := make([]store.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Position > summarizedUntil {
			active = append(active, m)
		}
	}

	summaryTokens := 0
	if strings.TrimSpace(existingSummary) != "" {
		summaryTokens = EstimateTokens(FormatSummary(existingSummary)) + 8
	}

	budget := maxTokens - systemTokens - summaryTokens
	if budget < 200 {
		budget = 200
	}

	activeTokens := 0
	for _, m := range active {
		activeTokens += EstimateTokens(m.Content) + EstimateTokens(m.Role) + 4
	}

	res := WindowResult{
		Summary:         existingSummary,
		SummarizedUntil: summarizedUntil,
	}

	// Soft threshold: only compress when well over budget or very long.
	if activeTokens <= budget && len(active) <= keepRecent*2 {
		res.Recent = active
		return res
	}

	keep := keepRecent
	if keep > len(active) {
		keep = len(active)
	}
	keepBudget := (budget * 3) / 5
	for keep > 2 {
		tok := 0
		for _, m := range active[len(active)-keep:] {
			tok += EstimateTokens(m.Content) + EstimateTokens(m.Role) + 4
		}
		if tok <= keepBudget {
			break
		}
		keep--
	}

	split := len(active) - keep
	if split <= 0 {
		res.Recent = active
		return res
	}

	res.ToSummarize = active[:split]
	res.Recent = active[split:]
	res.NeedsSummary = true
	return res
}

// ApplySummary merges a new summary chunk into the window result.
func ApplySummary(res WindowResult, newChunk string) WindowResult {
	newChunk = strings.TrimSpace(newChunk)
	if newChunk == "" {
		res.NeedsSummary = false
		return res
	}
	if strings.TrimSpace(res.Summary) == "" {
		res.Summary = newChunk
	} else {
		res.Summary = strings.TrimSpace(res.Summary) + "\n" + newChunk
	}
	if len(res.ToSummarize) > 0 {
		res.SummarizedUntil = res.ToSummarize[len(res.ToSummarize)-1].Position
	}
	res.DidSummarize = true
	res.NeedsSummary = false
	res.ToSummarize = nil
	return res
}

// SummarizeMessages asks the LLM to compress older turns into a short summary.
func SummarizeMessages(ctx context.Context, client *ollama.Client, model string, priorSummary string, msgs []store.Message) (string, error) {
	if len(msgs) == 0 {
		return "", nil
	}
	var transcript strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&transcript, "%s: %s\n", m.Role, m.Content)
	}
	prompt := "Summarize the following conversation excerpt into a concise paragraph of durable facts and decisions. " +
		"Keep names, preferences, goals, and open tasks. Omit chitchat. Write in the third person."
	user := transcript.String()
	if strings.TrimSpace(priorSummary) != "" {
		user = "Existing summary so far:\n" + priorSummary + "\n\nNew excerpt to fold in:\n" + user
	}
	out, err := client.Chat(ctx, model, []ollama.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: user},
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Content), nil
}

// ExtractMemories asks the LLM for durable facts from a recent exchange.
func ExtractMemories(ctx context.Context, client *ollama.Client, model, userMsg, assistantMsg string) ([]string, error) {
	userMsg = strings.TrimSpace(userMsg)
	if userMsg == "" {
		return nil, nil
	}
	prompt := `Extract durable personal facts worth remembering long-term from this exchange.
Return one fact per line. If there is nothing worth remembering, reply with exactly: NONE
Do not invent facts. Prefer stable preferences, identity, projects, and constraints.`
	exchange := "User: " + userMsg + "\nAssistant: " + strings.TrimSpace(assistantMsg)
	out, err := client.Chat(ctx, model, []ollama.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: exchange},
	}, nil)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(out.Content)
	if text == "" || strings.EqualFold(text, "NONE") {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	facts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*•0123456789.) ")
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "NONE") {
			continue
		}
		if len(line) > 500 {
			line = line[:500]
		}
		facts = append(facts, line)
	}
	return facts, nil
}

// BuildSystemPrompt combines base system prompt, memories, and optional short-term summary.
func BuildSystemPrompt(base string, memories []store.Memory, summary string) string {
	parts := make([]string, 0, 3)
	base = strings.TrimSpace(base)
	if base != "" {
		parts = append(parts, base)
	}
	if block := FormatMemories(memories); block != "" {
		parts = append(parts, block)
	}
	if block := FormatSummary(summary); block != "" {
		parts = append(parts, block)
	}
	return strings.Join(parts, "\n\n")
}

// ToOllama converts store messages to ollama messages (skips empty assistant placeholders).
func ToOllama(msgs []store.Message) []ollama.Message {
	out := make([]ollama.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "assistant" && strings.TrimSpace(m.Content) == "" {
			continue
		}
		if m.Role == "system" {
			continue
		}
		out = append(out, ollama.Message{Role: m.Role, Content: m.Content})
	}
	return out
}
