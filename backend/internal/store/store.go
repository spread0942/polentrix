package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
}

func Open(databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT 'New chat',
			system_prompt TEXT NOT NULL DEFAULT '',
			temperature DOUBLE PRECISION,
			top_p DOUBLE PRECISION,
			num_predict INTEGER,
			model TEXT NOT NULL DEFAULT '',
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at BIGINT NOT NULL,
			position INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation
			ON messages(conversation_id, position)`,
		// Phase 2 — short-term context summary
		`ALTER TABLE conversations ADD COLUMN IF NOT EXISTS context_summary TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE conversations ADD COLUMN IF NOT EXISTS summarized_until INTEGER NOT NULL DEFAULT 0`,
		// Phase 2 — long-term memories (user- and conversation-scoped)
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			conversation_id TEXT REFERENCES conversations(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_conversation ON memories(conversation_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_user_conv ON memories(user_id, conversation_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func nowMS() int64 {
	return time.Now().UnixMilli()
}
