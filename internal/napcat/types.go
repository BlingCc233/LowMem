// Package napcat provides a OneBot 11 WebSocket client for NapCat
package napcat

import (
	"encoding/json"
)

// APIRequest represents a OneBot 11 API request
type APIRequest struct {
	Action string      `json:"action"`
	Params interface{} `json:"params,omitempty"`
	Echo   string      `json:"echo,omitempty"`
}

// APIResponse represents a OneBot 11 API response
type APIResponse struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Data    json.RawMessage `json:"data,omitempty"`
	Echo    string          `json:"echo,omitempty"`
	Message string          `json:"message,omitempty"`
	Wording string          `json:"wording,omitempty"`
}

// IsOK returns true if the response indicates success
func (r *APIResponse) IsOK() bool {
	return r.Status == "ok" || r.RetCode == 0
}

// MessageSegment represents a segment of a message (CQ code equivalent)
type MessageSegment struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// NewTextSegment creates a text message segment
func NewTextSegment(text string) MessageSegment {
	return MessageSegment{
		Type: "text",
		Data: map[string]interface{}{"text": text},
	}
}

// NewImageSegment creates an image message segment
func NewImageSegment(file string) MessageSegment {
	return MessageSegment{
		Type: "image",
		Data: map[string]interface{}{"file": file},
	}
}

// NewAtSegment creates an at message segment
func NewAtSegment(qq int64) MessageSegment {
	return MessageSegment{
		Type: "at",
		Data: map[string]interface{}{"qq": qq},
	}
}

// NewReplySegment creates a reply message segment
func NewReplySegment(id int32) MessageSegment {
	return MessageSegment{
		Type: "reply",
		Data: map[string]interface{}{"id": id},
	}
}

// NewFaceSegment creates a face emoji segment
func NewFaceSegment(id int) MessageSegment {
	return MessageSegment{
		Type: "face",
		Data: map[string]interface{}{"id": id},
	}
}

// NewRecordSegment creates a voice record segment
func NewRecordSegment(file string) MessageSegment {
	return MessageSegment{
		Type: "record",
		Data: map[string]interface{}{"file": file},
	}
}

// Event represents a base OneBot 11 event
type Event struct {
	Time          int64  `json:"time"`
	SelfID        int64  `json:"self_id"`
	PostType      string `json:"post_type"`
	MetaEventType string `json:"meta_event_type,omitempty"`
	MessageType   string `json:"message_type,omitempty"`
	NoticeType    string `json:"notice_type,omitempty"`
	RequestType   string `json:"request_type,omitempty"`
	SubType       string `json:"sub_type,omitempty"`
}

// Sender represents message sender info
type Sender struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card,omitempty"` // Group card/nickname
	Sex      string `json:"sex,omitempty"`
	Age      int    `json:"age,omitempty"`
	Role     string `json:"role,omitempty"` // Group role: owner/admin/member
}

// PrivateMessageEvent represents a private message event
type PrivateMessageEvent struct {
	Event
	MessageID  int32           `json:"message_id"`
	UserID     int64           `json:"user_id"`
	Message    json.RawMessage `json:"message"` // Can be string or array
	RawMessage string          `json:"raw_message"`
	Font       int             `json:"font"`
	Sender     Sender          `json:"sender"`
}

// GroupMessageEvent represents a group message event
type GroupMessageEvent struct {
	Event
	MessageID  int32           `json:"message_id"`
	GroupID    int64           `json:"group_id"`
	UserID     int64           `json:"user_id"`
	Anonymous  interface{}     `json:"anonymous,omitempty"`
	Message    json.RawMessage `json:"message"` // Can be string or array
	RawMessage string          `json:"raw_message"`
	Font       int             `json:"font"`
	Sender     Sender          `json:"sender"`
}

// FriendRecallEvent represents a friend message recall event
type FriendRecallEvent struct {
	Event
	UserID    int64 `json:"user_id"`
	MessageID int32 `json:"message_id"`
}

// GroupRecallEvent represents a group message recall event
type GroupRecallEvent struct {
	Event
	GroupID    int64 `json:"group_id"`
	UserID     int64 `json:"user_id"`
	OperatorID int64 `json:"operator_id"`
	MessageID  int32 `json:"message_id"`
}

// LoginInfo represents login info response
type LoginInfo struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
}

// Friend represents a friend entry
type Friend struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
}

// Group represents a group entry
type Group struct {
	GroupID        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	MemberCount    int32  `json:"member_count"`
	MaxMemberCount int32  `json:"max_member_count"`
}

// SendMsgResponse represents send message response
type SendMsgResponse struct {
	MessageID int32 `json:"message_id"`
}

// Config holds NapCat connection settings
type Config struct {
	WebSocketURL string `json:"websocket_url"`
	HTTPURL      string `json:"http_url"`
	AccessToken  string `json:"access_token,omitempty"`
}

// ImageInfo represents image info
type ImageInfo struct {
	Size     int    `json:"size"`
	Filename string `json:"filename"`
	Url      string `json:"url"`
}

// GroupMemberInfo represents group member info
type GroupMemberInfo struct {
	GroupID         int64  `json:"group_id"`
	UserID          int64  `json:"user_id"`
	Nickname        string `json:"nickname"`
	Card            string `json:"card"`
	Sex             string `json:"sex"`
	Age             int    `json:"age"`
	Area            string `json:"area"`
	JoinTime        int32  `json:"join_time"`
	LastSentTime    int32  `json:"last_sent_time"`
	Level           string `json:"level"`
	Role            string `json:"role"`
	Unfriendly      bool   `json:"unfriendly"`
	Title           string `json:"title"`
	TitleExpireTime int32  `json:"title_expire_time"`
	CardChangeable  bool   `json:"card_changeable"`
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		WebSocketURL: "ws://127.0.0.1:3001",
		HTTPURL:      "http://127.0.0.1:3000",
	}
}
