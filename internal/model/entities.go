package model

import "time"

// Conversation stores user-assistant turns.
type Conversation struct {
	ID        int64     `xorm:"'id' pk autoincr"`
	SessionID string    `xorm:"'session_id' index notnull"`
	Role      string    `xorm:"'role' varchar(16) notnull"`
	Content   string    `xorm:"'content' text notnull"`
	CreatedAt time.Time `xorm:"'created_at' created"`
}

// Memory stores long-term memory snippets.
type Memory struct {
	ID         int64     `xorm:"'id' pk autoincr"`
	Kind       string    `xorm:"'kind' varchar(32) index notnull"`
	Content    string    `xorm:"'content' text notnull"`
	Importance int       `xorm:"'importance' default 0"`
	LastAccess time.Time `xorm:"'last_access' index"`
	CreatedAt  time.Time `xorm:"'created_at' created"`
	UpdatedAt  time.Time `xorm:"'updated_at' updated"`
}

// IndexChunk stores extracted file chunks.
type IndexChunk struct {
	ID        int64     `xorm:"'id' pk autoincr"`
	Path      string    `xorm:"'path' index notnull"`
	ChunkNo   int       `xorm:"'chunk_no' notnull"`
	Content   string    `xorm:"'content' text notnull"`
	UpdatedAt time.Time `xorm:"'updated_at' updated"`
}
