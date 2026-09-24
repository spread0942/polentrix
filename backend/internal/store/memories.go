package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Memory is a durable fact retained beyond a single turn.
// ConversationID == nil means user-scoped (global for that user).
type Memory struct {
	ID             string  `json:"id"`
	UserID         string  `json:"user_id"`
	ConversationID *string `json:"conversation_id"`
	Content        string  `json:"content"`
	CreatedAt      int64   `json:"created_at"`
	UpdatedAt      int64   `json:"updated_at"`
}

func (s *Store) ListMemories(userID string, conversationID *string) ([]Memory, error) {
	var rows *sql.Rows
	var err error
	if conversationID == nil {
		rows, err = s.db.Query(`
			SELECT id, user_id, conversation_id, content, created_at, updated_at
			FROM memories
			WHERE user_id = $1 AND conversation_id IS NULL
			ORDER BY updated_at DESC
		`, userID)
	} else {
		rows, err = s.db.Query(`
			SELECT id, user_id, conversation_id, content, created_at, updated_at
			FROM memories
			WHERE user_id = $1 AND conversation_id = $2
			ORDER BY updated_at DESC
		`, userID, *conversationID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

// ListAccessibleMemories returns user-global memories plus conversation-scoped ones.
func (s *Store) ListAccessibleMemories(userID, conversationID string) ([]Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, conversation_id, content, created_at, updated_at
		FROM memories
		WHERE user_id = $1
		  AND (conversation_id IS NULL OR conversation_id = $2)
		ORDER BY updated_at DESC
	`, userID, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (s *Store) GetMemory(userID, id string) (*Memory, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, conversation_id, content, created_at, updated_at
		FROM memories WHERE id = $1 AND user_id = $2
	`, id, userID)
	m, err := scanMemory(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (s *Store) CreateMemory(m Memory) (*Memory, error) {
	if m.ID == "" || m.UserID == "" || m.Content == "" {
		return nil, fmt.Errorf("id, user_id, and content required")
	}
	now := nowMS()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	if m.UpdatedAt == 0 {
		m.UpdatedAt = now
	}
	_, err := s.db.Exec(`
		INSERT INTO memories (id, user_id, conversation_id, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, m.ID, m.UserID, m.ConversationID, m.Content, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s.GetMemory(m.UserID, m.ID)
}

func (s *Store) UpdateMemory(userID, id, content string) (*Memory, error) {
	res, err := s.db.Exec(`
		UPDATE memories SET content = $1, updated_at = $2
		WHERE id = $3 AND user_id = $4
	`, content, nowMS(), id, userID)
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
	return s.GetMemory(userID, id)
}

func (s *Store) DeleteMemory(userID, id string) error {
	res, err := s.db.Exec(`DELETE FROM memories WHERE id = $1 AND user_id = $2`, id, userID)
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

// FindSimilarMemory returns an existing memory with the same content (case-insensitive trim).
func (s *Store) FindSimilarMemory(userID string, conversationID *string, content string) (*Memory, error) {
	var row *sql.Row
	if conversationID == nil {
		row = s.db.QueryRow(`
			SELECT id, user_id, conversation_id, content, created_at, updated_at
			FROM memories
			WHERE user_id = $1 AND conversation_id IS NULL
			  AND lower(trim(content)) = lower(trim($2))
			LIMIT 1
		`, userID, content)
	} else {
		row = s.db.QueryRow(`
			SELECT id, user_id, conversation_id, content, created_at, updated_at
			FROM memories
			WHERE user_id = $1 AND conversation_id = $2
			  AND lower(trim(content)) = lower(trim($3))
			LIMIT 1
		`, userID, *conversationID, content)
	}
	m, err := scanMemory(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func scanMemories(rows *sql.Rows) ([]Memory, error) {
	out := make([]Memory, 0)
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMemory(row rowScanner) (Memory, error) {
	var m Memory
	var convID sql.NullString
	err := row.Scan(&m.ID, &m.UserID, &convID, &m.Content, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return m, err
	}
	if convID.Valid {
		v := convID.String
		m.ConversationID = &v
	}
	return m, nil
}
