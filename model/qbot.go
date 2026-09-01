package model

import (
	"time"

	"github.com/gofrs/uuid"
)

// QBotState is the durable, per-player state of the BOT puzzle.
type QBotState struct {
	UserID        uuid.UUID `gorm:"type:char(36);not null;primaryKey"`
	Cleared       bool      `gorm:"type:boolean;not null;default:false"`
	Revision      uint64    `gorm:"type:bigint unsigned;not null;default:0"`
	Action        string    `gorm:"type:varchar(32);not null;default:''"`
	ActionPayload string    `gorm:"type:text;not null"`
	UpdatedAt     time.Time `gorm:"precision:6"`

	User User `gorm:"constraint:q_bot_states_user_id_users_id_foreign,OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*QBotState) TableName() string { return "q_bot_states" }

// QBotDeletedAttachment records one attachment occurrence hidden by the puzzle.
// A file referenced by another message remains visible.
type QBotDeletedAttachment struct {
	UserID    uuid.UUID `gorm:"type:char(36);not null;primaryKey"`
	MessageID uuid.UUID `gorm:"type:char(36);not null;primaryKey"`
	FileID    uuid.UUID `gorm:"type:char(36);not null;primaryKey"`
	CreatedAt time.Time `gorm:"precision:6"`

	User    User      `gorm:"constraint:q_bot_deleted_attachments_user_id_users_id_foreign,OnUpdate:CASCADE,OnDelete:CASCADE"`
	Message *Message  `gorm:"constraint:q_bot_deleted_attachments_message_id_messages_id_foreign,OnUpdate:CASCADE,OnDelete:CASCADE"`
	File    *FileMeta `gorm:"constraint:q_bot_deleted_attachments_file_id_files_id_foreign,OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*QBotDeletedAttachment) TableName() string { return "q_bot_deleted_attachments" }
