package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// User represents the local user or a contact
type User struct {
	ID        int64  `gorm:"primaryKey" json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	IsFriend  bool   `json:"is_friend"` // True if this user is a friend
}

// Group represents a chat group
type Group struct {
	ID          int64  `gorm:"primaryKey" json:"group_id"`
	Name        string `json:"group_name"`
	AvatarURL   string `json:"avatar_url"`
	MemberCount int32  `json:"member_count"`
}

// Message represents a chat message
type Message struct {
	ID        int64  `gorm:"primaryKey;autoIncrement" json:"database_id"` // Local DB ID
	MessageID int32  `gorm:"uniqueIndex" json:"message_id"`              // OneBot Message ID
	Type      string `json:"message_type"`                                // "private" or "group"
	SubType   string `json:"sub_type"`                                    // "friend", "normal", "anonymous", "notice"
	SenderID  int64  `json:"sender_id"`
	Sender    User   `gorm:"foreignKey:SenderID;references:ID" json:"sender"`
	TargetID  int64  `json:"target_id"`                                     // Receiver ID for private messages
	GroupID   int64  `json:"group_id"`                                      // 0 for private messages
	Group     Group  `gorm:"foreignKey:GroupID;references:ID" json:"group"` // Optional
	Content   string `json:"content"`                                       // Simplified text used for previews/search
	Raw       string `json:"raw_message"`                                   // Full raw CQ code
	Elements  MessageElements `gorm:"type:TEXT" json:"elements,omitempty"` // Structured elements for rendering (text/image/voice/...)
	ReplyTo   int32           `json:"reply_to,omitempty"`                   // Reply target message_id if any
	Timestamp int64           `json:"time"`
	IsSend    bool            `json:"is_send"` // True if sent by self
	IsRead    bool            `json:"is_read"`
	Recall    bool            `json:"recall"` // True if recalled

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// MessageElement describes a single renderable piece of a message.
// Kept lightweight so it can be marshalled to JSON for persistence.
type MessageElement struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	URL     string `json:"url,omitempty"`
	File    string `json:"file,omitempty"`
	QQ      string `json:"qq,omitempty"`
	Name    string `json:"name,omitempty"`
	ReplyID int32  `json:"reply_id,omitempty"`
}

type MessageElements []MessageElement

func (e MessageElements) Value() (driver.Value, error) {
	if len(e) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (e *MessageElements) Scan(value interface{}) error {
	if value == nil {
		*e = MessageElements{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, e)
	case string:
		return json.Unmarshal([]byte(v), e)
	default:
		return errors.New("unsupported type for MessageElements")
	}
}

// ContactListResponse used for frontend display
type ContactListResponse struct {
	Friends []User  `json:"friends"`
	Groups  []Group `json:"groups"`
}

// UnreadSummary is an aggregated unread counter per chat, used for badges.
type UnreadSummary struct {
	ChatID  int64 `json:"chat_id"`
	IsGroup bool  `json:"is_group"`
	Count   int64 `json:"count"`
}
