package backend

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"LagrangeQQ/internal/db"
	"LagrangeQQ/internal/models"
	"LagrangeQQ/internal/napcat"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Backend provides the main API for the frontend
type Backend struct {
	ctx    context.Context // Wails runtime context
	mu     sync.Mutex
	client *napcat.Client
}

// NewBackend creates a new Backend instance
func NewBackend() *Backend {
	return &Backend{}
}

// SetContext sets the Wails runtime context
func (b *Backend) SetContext(ctx context.Context) {
	b.ctx = ctx
}

// StartNapCat initializes and starts the NapCat client
func (b *Backend) StartNapCat() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				b.notify(fmt.Sprintf("NapCat Panic: %v", r))
			}
		}()

		b.notify("Loading NapCat configuration...")
		config := napcat.LoadConfig()
		b.notify(fmt.Sprintf("Connecting to NapCat at %s...", config.WebSocketURL))

		client := napcat.NewClient(config)
		b.client = client

		// Setup event handlers before connecting
		b.setupEventHandlers()

		if err := client.Connect(b.ctx); err != nil {
			b.notify("Failed to connect to NapCat: " + err.Error())
			b.notify("Please ensure NapCat is running and configured correctly.")
			b.notify(fmt.Sprintf("Expected WebSocket URL: %s", config.WebSocketURL))
			return
		}

		info := client.LoginInfo()
		if info != nil {
			b.notify(fmt.Sprintf("Connected as %s (%d)", info.Nickname, info.UserID))
			runtime.EventsEmit(b.ctx, "LoginSuccess", info.UserID)
		} else {
			b.notify("Connected to NapCat (login info unavailable)")
			runtime.EventsEmit(b.ctx, "LoginSuccess", 0)
		}
	}()
}

// StartLagrange is an alias for StartNapCat for backward compatibility
func (b *Backend) StartLagrange() {
	b.StartNapCat()
}

func (b *Backend) setupEventHandlers() {
	b.client.SetPrivateMessageHandler(func(e *napcat.PrivateMessageEvent) {
		b.handlePrivateMessage(e)
	})

	b.client.SetGroupMessageHandler(func(e *napcat.GroupMessageEvent) {
		b.handleGroupMessage(e)
	})

	b.client.SetFriendRecallHandler(func(e *napcat.FriendRecallEvent) {
		b.handleRecall(e.MessageID, false)
	})

	b.client.SetGroupRecallHandler(func(e *napcat.GroupRecallEvent) {
		b.handleRecall(e.MessageID, true)
	})

	b.client.SetDisconnectHandler(func() {
		log.Println("Disconnected from NapCat")
		runtime.EventsEmit(b.ctx, "Disconnected", nil)
	})
}

func (b *Backend) handlePrivateMessage(e *napcat.PrivateMessageEvent) {
	selfID := int64(0)
	if b.client.LoginInfo() != nil {
		selfID = b.client.LoginInfo().UserID
	}
	isSelf := e.UserID == selfID

	elements := b.parseMessageSegments(e.Message)

	msg := &models.Message{
		MessageID: e.MessageID,
		Type:      "private",
		SenderID:  e.UserID,
		Sender: models.User{
			ID:        e.Sender.UserID,
			Nickname:  e.Sender.Nickname,
			AvatarURL: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", e.Sender.UserID),
			IsFriend:  true,
		},
		TargetID:  selfID,
		Content:   e.RawMessage,
		Raw:       e.RawMessage,
		Elements:  elements,
		Timestamp: e.Time,
		IsSend:    isSelf,
		IsRead:    isSelf,
	}

	// Check for reply
	for _, elem := range elements {
		if elem.Type == "reply" {
			msg.ReplyTo = elem.ReplyID
			break
		}
	}

	_ = db.UpsertUser(&msg.Sender)
	if err := db.SaveMessage(msg); err != nil {
		log.Println("save private message:", err)
	}

	runtime.EventsEmit(b.ctx, "MessageReceived", msg)
}

func (b *Backend) handleGroupMessage(e *napcat.GroupMessageEvent) {
	selfID := int64(0)
	if b.client.LoginInfo() != nil {
		selfID = b.client.LoginInfo().UserID
	}
	isSelf := e.UserID == selfID

	senderName := e.Sender.Nickname
	if e.Sender.Card != "" {
		senderName = e.Sender.Card
	}

	elements := b.parseMessageSegments(e.Message)

	msg := &models.Message{
		MessageID: e.MessageID,
		Type:      "group",
		SenderID:  e.UserID,
		Sender: models.User{
			ID:        e.Sender.UserID,
			Nickname:  senderName,
			AvatarURL: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", e.Sender.UserID),
		},
		GroupID:   e.GroupID,
		Content:   e.RawMessage,
		Raw:       e.RawMessage,
		Elements:  elements,
		Timestamp: e.Time,
		IsSend:    isSelf,
		IsRead:    isSelf,
	}

	// Check for reply
	for _, elem := range elements {
		if elem.Type == "reply" {
			msg.ReplyTo = elem.ReplyID
			break
		}
	}

	_ = db.UpsertUser(&msg.Sender)
	if err := db.SaveMessage(msg); err != nil {
		log.Println("save group message:", err)
	}

	runtime.EventsEmit(b.ctx, "MessageReceived", msg)
}

func (b *Backend) handleRecall(messageID int32, isGroup bool) {
	msg, err := db.MarkMessageRecalled(messageID)
	if err != nil {
		log.Println("mark recall:", err)
		return
	}
	runtime.EventsEmit(b.ctx, "MessageRecalled", msg)
}

func (b *Backend) notify(msg string) {
	log.Println(msg)
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "Log", msg)
	}
}

// GetFriends returns the friend list
func (b *Backend) GetFriends() ([]models.User, error) {
	// If WS is connected or HTTP is available
	if b.client != nil && (b.client.IsConnected() || b.client.IsHTTPAvailable()) {
		friends, err := b.client.GetFriendList()
		if err != nil {
			return nil, err
		}

		users := make([]models.User, 0, len(friends))
		for _, f := range friends {
			u := models.User{
				ID:        f.UserID,
				Nickname:  f.Nickname,
				AvatarURL: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", f.UserID),
				IsFriend:  true,
			}
			if f.Remark != "" {
				u.Nickname = f.Remark
			}
			_ = db.UpsertUser(&u)
			users = append(users, u)
		}
		return users, nil
	}
	// Fallback to database
	var users []models.User
	err := db.DB.Find(&users, "is_friend = ?", true).Error
	return users, err
}

// GetGroups returns the group list
func (b *Backend) GetGroups() ([]models.Group, error) {
	if b.client != nil && (b.client.IsConnected() || b.client.IsHTTPAvailable()) {
		groups, err := b.client.GetGroupList()
		if err != nil {
			return nil, err
		}

		result := make([]models.Group, 0, len(groups))
		for _, g := range groups {
			grp := models.Group{
				ID:          g.GroupID,
				Name:        g.GroupName,
				AvatarURL:   fmt.Sprintf("https://p.qlogo.cn/gh/%d/%d/640", g.GroupID, g.GroupID),
				MemberCount: g.MemberCount,
			}
			_ = db.UpsertGroup(&grp)
			result = append(result, grp)
		}
		return result, nil
	}
	// Fallback to database
	var groups []models.Group
	err := db.DB.Find(&groups).Error
	return groups, err
}

// GetMessages returns messages for a chat
func (b *Backend) GetMessages(id int64, isGroup bool) ([]models.Message, error) {
	return db.GetMessages(id, isGroup, 0, 50)
}

// GetSessions returns aggregated chat previews for the sidebar.
func (b *Backend) GetSessions() ([]models.ChatSession, error) {
	return db.GetSessionSummaries(64)
}

// SendMessage sends a message
func (b *Backend) SendMessage(targetID int64, isGroup bool, content string) (*models.Message, error) {
	if b.client == nil || (!b.client.IsConnected() && !b.client.IsHTTPAvailable()) {
		return nil, fmt.Errorf("not connected")
	}

	segments := b.parseMessageContent(content)

	var result *napcat.SendMsgResponse
	var err error

	if isGroup {
		result, err = b.client.SendGroupMsg(targetID, segments)
	} else {
		result, err = b.client.SendPrivateMsg(targetID, segments)
	}

	if err != nil {
		return nil, err
	}

	selfID := int64(0)
	selfNick := ""
	if b.client.LoginInfo() != nil {
		selfID = b.client.LoginInfo().UserID
		selfNick = b.client.LoginInfo().Nickname
	}

	msgType := "private"
	groupID := int64(0)
	if isGroup {
		msgType = "group"
		groupID = targetID
	}

	msg := &models.Message{
		MessageID: result.MessageID,
		Type:      msgType,
		SenderID:  selfID,
		Sender: models.User{
			ID:        selfID,
			Nickname:  selfNick,
			AvatarURL: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", selfID),
		},
		TargetID:  targetID,
		GroupID:   groupID,
		Content:   content,
		Raw:       content,
		Elements:  segmentsToModelElements(b.client, segments),
		Timestamp: time.Now().Unix(),
		IsSend:    true,
		IsRead:    true,
	}

	if err := db.SaveMessage(msg); err != nil {
		log.Println("save sent message:", err)
	}

	return msg, nil
}

// parseMessageContent parses CQ code style message content into message segments
func (b *Backend) parseMessageContent(content string) []napcat.MessageSegment {
	return parseCQCode(content)
}

func parseCQCode(content string) []napcat.MessageSegment {
	var segments []napcat.MessageSegment

	// Simple parser for CQ codes
	// Format: [CQ:type,key=value,key=value]
	i := 0
	for i < len(content) {
		if strings.HasPrefix(content[i:], "[CQ:") {
			// Find end of CQ code
			end := strings.Index(content[i:], "]")
			if end == -1 {
				// No closing bracket, treat as text
				segments = append(segments, napcat.NewTextSegment(content[i:]))
				break
			}

			cqCode := content[i+4 : i+end]
			parts := strings.SplitN(cqCode, ",", 2)
			cqType := parts[0]

			params := make(map[string]string)
			if len(parts) > 1 {
				for _, param := range strings.Split(parts[1], ",") {
					kv := strings.SplitN(param, "=", 2)
					if len(kv) == 2 {
						params[kv[0]] = kv[1]
					}
				}
			}

			switch cqType {
			case "reply":
				if id, err := strconv.ParseInt(params["id"], 10, 32); err == nil {
					segments = append(segments, napcat.NewReplySegment(int32(id)))
				}
			case "at":
				if qq, err := strconv.ParseInt(params["qq"], 10, 64); err == nil {
					segments = append(segments, napcat.NewAtSegment(qq))
				}
			case "image":
				// Support both "file" and "url"
				file := params["file"]
				if file == "" {
					file = params["url"]
				}
				if file != "" {
					segments = append(segments, napcat.NewImageSegment(file))
				}
			case "record":
				file := params["file"]
				if file == "" {
					file = params["url"]
				}
				if file != "" {
					segments = append(segments, napcat.NewRecordSegment(file))
				}
			case "face":
				if id, err := strconv.Atoi(params["id"]); err == nil {
					segments = append(segments, napcat.NewFaceSegment(id))
				}
			default:
				// Unknown CQ code, keep as text
				segments = append(segments, napcat.NewTextSegment(content[i:i+end+1]))
			}

			i += end + 1
		} else {
			// Find next CQ code or end of string
			next := strings.Index(content[i:], "[CQ:")
			if next == -1 {
				// No more CQ codes
				if text := content[i:]; text != "" {
					segments = append(segments, napcat.NewTextSegment(text))
				}
				break
			}
			if text := content[i : i+next]; text != "" {
				segments = append(segments, napcat.NewTextSegment(text))
			}
			i += next
		}
	}

	if len(segments) == 0 {
		segments = append(segments, napcat.NewTextSegment(content))
	}

	return segments
}

func segmentsToModelElements(client *napcat.Client, segments []napcat.MessageSegment) models.MessageElements {
	result := make(models.MessageElements, 0, len(segments))
	for _, seg := range segments {
		elem := models.MessageElement{Type: seg.Type}
		if elem.Type == "record" {
			elem.Type = "voice"
		}
		if text, ok := seg.Data["text"].(string); ok {
			elem.Text = text
		}
		// Try to find URL in "url" or "file"
		if url, ok := seg.Data["url"].(string); ok {
			elem.URL = resolveMediaURL(url)
		}
		if file, ok := seg.Data["file"].(string); ok {
			elem.File = file
			// If URL is missing, decide how to resolve based on type and available helpers.
			if elem.URL == "" {
				// For image segments we can usually resolve via NapCat's get_image API.
				if elem.Type == "image" && client != nil && (client.IsConnected() || client.IsHTTPAvailable()) {
					if info, err := client.GetImage(file); err == nil && info.Url != "" {
						elem.URL = resolveMediaURL(info.Url)
					}
				}
				// Fallback: treat the file as a path/URL directly.
				if elem.URL == "" {
					elem.URL = resolveMediaURL(file)
				}
			}
		}

		if qq, ok := seg.Data["qq"]; ok {
			elem.QQ = fmt.Sprintf("%v", qq)
		}
		if id, ok := seg.Data["id"]; ok {
			// Different segment types reuse the key "id" for different meanings.
			// - reply: reply target message id
			// - face:  emoji id
			var idVal int32
			switch v := id.(type) {
			case int32:
				idVal = v
			case int64:
				idVal = int32(v)
			case int:
				idVal = int32(v)
			case float64:
				idVal = int32(v)
			}
			if idVal != 0 {
				switch elem.Type {
				case "reply":
					elem.ReplyID = idVal
				case "face":
					elem.ID = idVal
				}
			}
		}
		// If it's an image but has no URL, it's broken. Do not add to elements, or add fallback.
		// Front-end renders empty src as broken image/border.
		if elem.Type == "image" && elem.URL == "" {
			// Fallback: show text indicating failure instead of empty bubble
			elem.Type = "text"
			elem.Text = "[图片加载失败]"
		}

		result = append(result, elem)
	}
	return result
}

func (b *Backend) parseMessageSegments(raw json.RawMessage) models.MessageElements {
	// Try to parse as array of segments first
	var segments []map[string]interface{}
	if err := json.Unmarshal(raw, &segments); err == nil {
		// Use helper to convert raw map segments to napcat segments first, or directly to model
		// Keeping existing direct parsing for now
		var result models.MessageElements
		for _, seg := range segments {
			segType, _ := seg["type"].(string)
			data, _ := seg["data"].(map[string]interface{})

			elem := models.MessageElement{Type: segType}
			if elem.Type == "record" {
				elem.Type = "voice"
			}
			if text, ok := data["text"].(string); ok {
				elem.Text = text
			}
			if url, ok := data["url"].(string); ok {
				elem.URL = url
			}
			if file, ok := data["file"].(string); ok {
				elem.File = file
			}
			// Fallback: if file is empty but url exists (or vice versa fix)
			if elem.URL == "" && elem.File != "" && strings.HasPrefix(elem.File, "http") {
				elem.URL = elem.File
			}

			// Try to fetch image URL if missing
			if (elem.Type == "image" || elem.Type == "record") && elem.URL == "" && elem.File != "" && b.client != nil {
				// Only fetch if it doesn't look like a URL or local path we already have
				if !strings.HasPrefix(elem.File, "http") && !strings.HasPrefix(elem.File, "data:") {
					if info, err := b.client.GetImage(elem.File); err == nil && info.Url != "" {
						elem.URL = resolveMediaURL(info.Url)
					}
				}
			}

			if qq, ok := data["qq"]; ok {
				elem.QQ = fmt.Sprintf("%v", qq)
			}
			if name, ok := data["name"].(string); ok {
				elem.Name = name
			}
			if id, ok := data["id"]; ok {
				var idVal int32
				switch v := id.(type) {
				case float64:
					idVal = int32(v)
				case int:
					idVal = int32(v)
				case int32:
					idVal = v
				case int64:
					idVal = int32(v)
				}
				if idVal != 0 {
					switch elem.Type {
					case "reply":
						elem.ReplyID = idVal
					case "face":
						elem.ID = idVal
					}
				}
			}

			// Fallback for broken images
			if elem.Type == "image" && elem.URL == "" {
				elem.Type = "text"
				elem.Text = "[图片加载失败]"
			}

			result = append(result, elem)
		}
		return result
	}

	// Fallback: parse as string (CQ codes)
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		// IMPORTANT: Parse the CQ code string!
		segments := parseCQCode(str)
		return segmentsToModelElements(b.client, segments)
	}

	return nil
}

func resolveMediaURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "file://") ||
		strings.HasPrefix(value, "data:") {
		return value
	}

	cleaned := filepath.ToSlash(value)
	if strings.HasPrefix(cleaned, "/") {
		return "file://" + cleaned
	}
	return "file:///" + cleaned
}

// RecallMessage recalls a message
func (b *Backend) RecallMessage(messageID int32) error {
	if b.client == nil || (!b.client.IsConnected() && !b.client.IsHTTPAvailable()) {
		return fmt.Errorf("not connected")
	}
	return b.client.DeleteMsg(messageID)
}

// ForwardMessage forwards a message to another chat
func (b *Backend) ForwardMessage(messageDBID int64, targetID int64, isGroup bool) (*models.Message, error) {
	msg, err := db.GetMessageByDatabaseID(messageDBID)
	if err != nil {
		return nil, err
	}
	return b.SendMessage(targetID, isGroup, msg.Raw)
}

// SearchMessages searches messages
func (b *Backend) SearchMessages(keyword string, chatID int64, isGroup bool) ([]models.Message, error) {
	return db.SearchMessages(keyword, chatID, isGroup, 50)
}

// MarkChatRead marks a chat as read
func (b *Backend) MarkChatRead(chatID int64, isGroup bool) error {
	return db.MarkChatRead(chatID, isGroup)
}

// UnreadSummary returns unread message counts
func (b *Backend) UnreadSummary() ([]models.UnreadSummary, error) {
	return db.UnreadSummary()
}

// UploadImage uploads an image and returns file path for sending
func (b *Backend) UploadImage(base64Data string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}

	configDir, _ := os.UserConfigDir()
	cacheDir := filepath.Join(configDir, "LagrangeQQ", "cache", "images")
	_ = os.MkdirAll(cacheDir, 0755)

	filename := fmt.Sprintf("%d.jpg", time.Now().UnixNano())
	fpath := filepath.Join(cacheDir, filename)

	if err := os.WriteFile(fpath, data, 0644); err != nil {
		return "", err
	}

	return fpath, nil
}

// UploadVoice uploads a voice file
func (b *Backend) UploadVoice(base64Data string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}

	configDir, _ := os.UserConfigDir()
	cacheDir := filepath.Join(configDir, "LagrangeQQ", "cache", "voices")
	_ = os.MkdirAll(cacheDir, 0755)

	filename := fmt.Sprintf("%d.amr", time.Now().UnixNano())
	fpath := filepath.Join(cacheDir, filename)

	if err := os.WriteFile(fpath, data, 0644); err != nil {
		return "", err
	}

	return fpath, nil
}

// GetSelfInfo returns current user info
func (b *Backend) GetSelfInfo() map[string]interface{} {
	if b.client == nil || (!b.client.IsConnected() && !b.client.IsHTTPAvailable()) {
		return nil
	}
	info := b.client.LoginInfo()
	if info == nil {
		return nil
	}
	return map[string]interface{}{
		"uin":      info.UserID,
		"nickname": info.Nickname,
	}
}

// SendImageMessage sends an image message from file path
func (b *Backend) SendImageMessage(targetID int64, isGroup bool, imagePath string) (*models.Message, error) {
	if b.client == nil || (!b.client.IsConnected() && !b.client.IsHTTPAvailable()) {
		return nil, fmt.Errorf("not connected")
	}

	// Check if it's a URL or file path
	var fileRef string
	var modelFileRef string

	if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
		fileRef = imagePath
		modelFileRef = imagePath
	} else {
		// For local files, read and encode to base64 to support remote NapCat
		data, err := os.ReadFile(imagePath)
		if err != nil {
			return nil, fmt.Errorf("read image file: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		fileRef = "base64://" + b64
		// For local display, use Data URI to avoid file:// blocked issues
		// Detect mime type roughly from extension or default to png/jpg
		// Simplified: just use image/jpeg or png based on ext? or just generic?
		// Browser handles data:image/png;base64,... well.
		ext := strings.ToLower(filepath.Ext(imagePath))
		mime := "image/jpeg"
		if ext == ".png" {
			mime = "image/png"
		} else if ext == ".gif" {
			mime = "image/gif"
		} else if ext == ".webp" {
			mime = "image/webp"
		}
		modelFileRef = fmt.Sprintf("data:%s;base64,%s", mime, b64)
	}

	segments := []napcat.MessageSegment{napcat.NewImageSegment(fileRef)}

	var result *napcat.SendMsgResponse
	var err error

	if isGroup {
		result, err = b.client.SendGroupMsg(targetID, segments)
	} else {
		result, err = b.client.SendPrivateMsg(targetID, segments)
	}

	if err != nil {
		return nil, err
	}

	selfID := int64(0)
	if b.client.LoginInfo() != nil {
		selfID = b.client.LoginInfo().UserID
	}

	msgType := "private"
	groupID := int64(0)
	if isGroup {
		msgType = "group"
		groupID = targetID
	}

	msg := &models.Message{
		MessageID: result.MessageID,
		Type:      msgType,
		SenderID:  selfID,
		TargetID:  targetID,
		GroupID:   groupID,
		Content:   "[图片]",
		Raw:       fmt.Sprintf("[CQ:image,file=%s]", modelFileRef),
		Elements:  models.MessageElements{{Type: "image", File: modelFileRef, URL: modelFileRef}},
		Timestamp: time.Now().Unix(),
		IsSend:    true,
		IsRead:    true,
	}

	if err := db.SaveMessage(msg); err != nil {
		log.Println("save sent message:", err)
	}

	return msg, nil
}

// OpenImage opens a file dialog to select an image
func (b *Backend) OpenImage() (string, error) {
	selection, err := runtime.OpenFileDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "Select Image",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Images",
				Pattern:     "*.png;*.jpg;*.jpeg;*.gif;*.webp",
			},
		},
	})
	return selection, err
}

// SendVoiceMessage sends a voice message from file path
func (b *Backend) SendVoiceMessage(targetID int64, isGroup bool, voicePath string) (*models.Message, error) {
	if b.client == nil || (!b.client.IsConnected() && !b.client.IsHTTPAvailable()) {
		return nil, fmt.Errorf("not connected")
	}

	var fileRef string
	var modelFileRef string

	// For local files, read and encode to base64
	if strings.HasPrefix(voicePath, "http://") || strings.HasPrefix(voicePath, "https://") {
		fileRef = voicePath
		modelFileRef = voicePath
	} else {
		data, err := os.ReadFile(voicePath)
		if err != nil {
			return nil, fmt.Errorf("read voice file: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		fileRef = "base64://" + b64
		// Use data URI for audio playback
		// Usually amr/silk? Chrome might not play raw amr?
		// If it's from UploadVoice (amr), browser can't play it directly usually.
		// But let's try data uri. If it's amr, it might fail anyway unless we have a decoder.
		// NOTE: Webview usually can't play AMR.
		// But for now, let's at least give it the data.
		modelFileRef = "data:audio/amr;base64," + b64
	}
	segments := []napcat.MessageSegment{napcat.NewRecordSegment(fileRef)}

	var result *napcat.SendMsgResponse
	var err error

	if isGroup {
		result, err = b.client.SendGroupMsg(targetID, segments)
	} else {
		result, err = b.client.SendPrivateMsg(targetID, segments)
	}

	if err != nil {
		return nil, err
	}

	selfID := int64(0)
	if b.client.LoginInfo() != nil {
		selfID = b.client.LoginInfo().UserID
	}

	msgType := "private"
	groupID := int64(0)
	if isGroup {
		msgType = "group"
		groupID = targetID
	}

	msg := &models.Message{
		MessageID: result.MessageID,
		Type:      msgType,
		SenderID:  selfID,
		TargetID:  targetID,
		GroupID:   groupID,
		Content:   "[语音]",
		Raw:       fmt.Sprintf("[CQ:record,file=%s]", modelFileRef),
		Elements:  models.MessageElements{{Type: "voice", File: modelFileRef, URL: modelFileRef}},
		Timestamp: time.Now().Unix(),
		IsSend:    true,
		IsRead:    true,
	}

	if err := db.SaveMessage(msg); err != nil {
		log.Println("save sent message:", err)
	}

	return msg, nil
}

// Shutdown gracefully shuts down the backend
func (b *Backend) Shutdown() {
	if b.client != nil {
		b.client.Close()
	}
}

// unused but keeping for potential future use
func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
