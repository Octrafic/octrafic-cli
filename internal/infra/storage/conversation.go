package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Conversation represents a chat conversation for a project
type Conversation struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a single message in a conversation
type Message struct {
	ID             int64                  `json:"id"`
	ConversationID string                 `json:"conversation_id"`
	Type           string                 `json:"type"` // 'user', 'assistant', 'api_request', 'api_response', 'system'
	Content        string                 `json:"content"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

const conversationsDir = "conversations"

// GetConversationPath returns the path to a conversation database file
func GetConversationPath(projectID, conversationID string) (string, error) {
	// Conversations are only for named projects (not temporary)
	projectPath, err := GetProjectPathByType(projectID, false)
	if err != nil {
		return "", err
	}

	convDir := filepath.Join(projectPath, conversationsDir)
	if err := os.MkdirAll(convDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create conversations directory: %w", err)
	}

	return filepath.Join(convDir, conversationID+".db"), nil
}

// initConversationDB initializes the database schema for a conversation
func initConversationDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversation (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		title TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL,
		type TEXT NOT NULL,
		content TEXT NOT NULL,
		metadata TEXT,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversation(id)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	`

	_, err := db.Exec(schema)
	return err
}

// CreateConversation creates a new conversation
func CreateConversation(projectID, conversationID, title string) (*Conversation, error) {
	if title == "" {
		title = "Untitled Conversation"
	}

	conv := &Conversation{
		ID:        conversationID,
		ProjectID: projectID,
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dbPath, err := GetConversationPath(projectID, conversationID)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation database: %w", err)
	}
	defer db.Close()

	if err := initConversationDB(db); err != nil {
		return nil, fmt.Errorf("failed to initialize conversation database: %w", err)
	}

	query := `INSERT INTO conversation (id, project_id, title, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?)`
	_, err = db.Exec(query, conv.ID, conv.ProjectID, conv.Title, conv.CreatedAt, conv.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert conversation: %w", err)
	}

	return conv, nil
}

// LoadConversation loads a conversation by ID
func LoadConversation(projectID, conversationID string) (*Conversation, error) {
	dbPath, err := GetConversationPath(projectID, conversationID)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("conversation not found: %s", conversationID)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation database: %w", err)
	}
	defer db.Close()

	conv := &Conversation{}
	query := `SELECT id, project_id, title, created_at, updated_at FROM conversation WHERE id = ?`
	err = db.QueryRow(query, conversationID).Scan(
		&conv.ID,
		&conv.ProjectID,
		&conv.Title,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation: %w", err)
	}

	return conv, nil
}

// ListConversations returns all conversations for a project
func ListConversations(projectID string) ([]*Conversation, error) {
	// Conversations are only for named projects (not temporary)
	projectPath, err := GetProjectPathByType(projectID, false)
	if err != nil {
		return nil, err
	}

	convDir := filepath.Join(projectPath, conversationsDir)
	if _, err := os.Stat(convDir); os.IsNotExist(err) {
		return []*Conversation{}, nil
	}

	entries, err := os.ReadDir(convDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversations directory: %w", err)
	}

	var conversations []*Conversation
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		conversationID := entry.Name()[:len(entry.Name())-3] // Remove .db extension
		conv, err := LoadConversation(projectID, conversationID)
		if err != nil {
			continue // Skip corrupted conversations
		}
		conversations = append(conversations, conv)
	}

	// Sort by updated_at descending (most recent first)
	for i := 0; i < len(conversations); i++ {
		for j := i + 1; j < len(conversations); j++ {
			if conversations[i].UpdatedAt.Before(conversations[j].UpdatedAt) {
				conversations[i], conversations[j] = conversations[j], conversations[i]
			}
		}
	}

	return conversations, nil
}

// SaveMessage saves a message to the conversation
func SaveMessage(projectID, conversationID, messageType, content string, metadata map[string]interface{}) error {
	dbPath, err := GetConversationPath(projectID, conversationID)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open conversation database: %w", err)
	}
	defer db.Close()

	var metadataJSON *string
	if metadata != nil && len(metadata) > 0 {
		jsonBytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		str := string(jsonBytes)
		metadataJSON = &str
	}

	query := `INSERT INTO messages (conversation_id, type, content, metadata, timestamp) 
	          VALUES (?, ?, ?, ?, ?)`
	_, err = db.Exec(query, conversationID, messageType, content, metadataJSON, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Update conversation updated_at
	updateQuery := `UPDATE conversation SET updated_at = ? WHERE id = ?`
	_, err = db.Exec(updateQuery, time.Now(), conversationID)
	if err != nil {
		return fmt.Errorf("failed to update conversation timestamp: %w", err)
	}

	return nil
}

// GetMessages retrieves all messages from a conversation
func GetMessages(projectID, conversationID string) ([]*Message, error) {
	dbPath, err := GetConversationPath(projectID, conversationID)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation database: %w", err)
	}
	defer db.Close()

	query := `SELECT id, conversation_id, type, content, metadata, timestamp 
	          FROM messages 
	          ORDER BY timestamp ASC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		var metadataJSON *string

		err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.Type,
			&msg.Content,
			&metadataJSON,
			&msg.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if metadataJSON != nil {
			if err := json.Unmarshal([]byte(*metadataJSON), &msg.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// DeleteConversation deletes a conversation and all its messages
func DeleteConversation(projectID, conversationID string) error {
	dbPath, err := GetConversationPath(projectID, conversationID)
	if err != nil {
		return err
	}

	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}
