package store

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

type Conversation struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	SystemPrompt    string    `json:"system_prompt"`
	Temperature     *float64  `json:"temperature"`
	TopP            *float64  `json:"top_p"`
	NumPredict      *int      `json:"num_predict"`
	Model           string    `json:"model"`
	ContextSummary  string    `json:"context_summary"`
	SummarizedUntil int       `json:"summarized_until"`
	CreatedAt       int64     `json:"created_at"`
	UpdatedAt       int64     `json:"updated_at"`
	Messages        []Message `json:"messages,omitempty"`
}

type Message struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	CreatedAt      int64  `json:"created_at"`
	Position       int    `json:"position"`
}

func (s *Store) ListConversations() ([]Conversation, error) {
	rows, err := s.db.Query(`
		SELECT id, title, system_prompt, temperature, top_p, num_predict, model,
			context_summary, summarized_until, created_at, updated_at
		FROM conversations
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Conversation, 0)
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetConversation(id string, withMessages bool) (*Conversation, error) {
	row := s.db.QueryRow(`
		SELECT id, title, system_prompt, temperature, top_p, num_predict, model,
			context_summary, summarized_until, created_at, updated_at
		FROM conversations WHERE id = $1
	`, id)

	c, err := scanConversation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if withMessages {
		msgs, err := s.ListMessages(id)
		if err != nil {
			return nil, err
		}
		c.Messages = msgs
	}
	return &c, nil
}

func (s *Store) CreateConversation(c Conversation) (*Conversation, error) {
	now := nowMS()
	if c.ID == "" {
		return nil, fmt.Errorf("id required")
	}
	if c.Title == "" {
		c.Title = "New chat"
	}
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = now
	}

	_, err := s.db.Exec(`
		INSERT INTO conversations (
			id, title, system_prompt, temperature, top_p, num_predict, model,
			context_summary, summarized_until, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, c.ID, c.Title, c.SystemPrompt, c.Temperature, c.TopP, c.NumPredict, c.Model,
		c.ContextSummary, c.SummarizedUntil, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s.GetConversation(c.ID, true)
}

// UpdateConversation writes the provided fields. Pass the full conversation
// state for settings; callers typically load, merge a patch, then save.
func (s *Store) UpdateConversation(c Conversation) (*Conversation, error) {
	if c.ID == "" {
		return nil, fmt.Errorf("id required")
	}
	if _, err := s.GetConversation(c.ID, false); err != nil {
		return nil, err
	}
	if c.Title == "" {
		c.Title = "New chat"
	}
	c.UpdatedAt = nowMS()

	_, err := s.db.Exec(`
		UPDATE conversations
		SET title = $1, system_prompt = $2, temperature = $3, top_p = $4, num_predict = $5, model = $6,
			context_summary = $7, summarized_until = $8, updated_at = $9
		WHERE id = $10
	`, c.Title, c.SystemPrompt, c.Temperature, c.TopP, c.NumPredict, c.Model,
		c.ContextSummary, c.SummarizedUntil, c.UpdatedAt, c.ID)
	if err != nil {
		return nil, err
	}
	return s.GetConversation(c.ID, true)
}

func (s *Store) DeleteConversation(id string) error {
	res, err := s.db.Exec(`DELETE FROM conversations WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TouchConversation(id string) error {
	_, err := s.db.Exec(`UPDATE conversations SET updated_at = $1 WHERE id = $2`, nowMS(), id)
	return err
}

func (s *Store) ListMessages(conversationID string) ([]Message, error) {
	rows, err := s.db.Query(`
		SELECT id, conversation_id, role, content, created_at, position
		FROM messages
		WHERE conversation_id = $1
		ORDER BY position ASC, created_at ASC
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt, &m.Position); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CreateMessage(m Message) (*Message, error) {
	if m.ID == "" || m.ConversationID == "" || m.Role == "" {
		return nil, fmt.Errorf("id, conversation_id, and role required")
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = nowMS()
	}
	if m.Position == 0 {
		var max sql.NullInt64
		err := s.db.QueryRow(
			`SELECT MAX(position) FROM messages WHERE conversation_id = $1`,
			m.ConversationID,
		).Scan(&max)
		if err != nil {
			return nil, err
		}
		if max.Valid {
			m.Position = int(max.Int64) + 1
		} else {
			m.Position = 1
		}
	}

	_, err := s.db.Exec(`
		INSERT INTO messages (id, conversation_id, role, content, created_at, position)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, m.ID, m.ConversationID, m.Role, m.Content, m.CreatedAt, m.Position)
	if err != nil {
		return nil, err
	}
	_ = s.TouchConversation(m.ConversationID)
	return &m, nil
}

func (s *Store) UpdateMessage(conversationID, messageID, content string) (*Message, error) {
	res, err := s.db.Exec(`
		UPDATE messages SET content = $1 WHERE id = $2 AND conversation_id = $3
	`, content, messageID, conversationID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	_ = s.TouchConversation(conversationID)

	var m Message
	err = s.db.QueryRow(`
		SELECT id, conversation_id, role, content, created_at, position
		FROM messages WHERE id = $1 AND conversation_id = $2
	`, messageID, conversationID).Scan(
		&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt, &m.Position,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) DeleteMessage(conversationID, messageID string) error {
	res, err := s.db.Exec(`
		DELETE FROM messages WHERE id = $1 AND conversation_id = $2
	`, messageID, conversationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	_ = s.TouchConversation(conversationID)
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanConversation(row rowScanner) (Conversation, error) {
	var c Conversation
	var temp, topP sql.NullFloat64
	var numPredict sql.NullInt64
	err := row.Scan(
		&c.ID, &c.Title, &c.SystemPrompt,
		&temp, &topP, &numPredict,
		&c.Model, &c.ContextSummary, &c.SummarizedUntil,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	if temp.Valid {
		v := temp.Float64
		c.Temperature = &v
	}
	if topP.Valid {
		v := topP.Float64
		c.TopP = &v
	}
	if numPredict.Valid {
		v := int(numPredict.Int64)
		c.NumPredict = &v
	}
	return c, nil
}

// UpdateContextSummary persists a short-term conversation summary.
func (s *Store) UpdateContextSummary(id, summary string, summarizedUntil int) error {
	_, err := s.db.Exec(`
		UPDATE conversations
		SET context_summary = $1, summarized_until = $2, updated_at = $3
		WHERE id = $4
	`, summary, summarizedUntil, nowMS(), id)
	return err
}
