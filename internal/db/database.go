package db

import (
	"LagrangeQQ/internal/models"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() error {
	// Create data directory if not exists
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	appDir := filepath.Join(configDir, "LagrangeQQ")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(appDir, "data.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	DB = db

	// Auto Migrate
	err = DB.AutoMigrate(&models.User{}, &models.Group{}, &models.Message{})
	if err != nil {
		return err
	}

	return nil
}

// Data Access Object Methods

func SaveMessage(msg *models.Message) error {
	if msg.MessageID != 0 {
		return DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "message_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"type", "sub_type", "sender_id", "target_id", "group_id", "content", "raw", "elements", "reply_to", "timestamp", "is_send", "is_read", "recall"}),
		}).Create(msg).Error
	}
	return DB.Create(msg).Error
}

func GetMessageByDatabaseID(id int64) (*models.Message, error) {
	var msg models.Message
	res := DB.Preload("Sender").Preload("Group").First(&msg, id)
	if res.Error != nil {
		return nil, res.Error
	}
	return &msg, nil
}

func GetMessages(id int64, isGroup bool, offset int, limit int) ([]models.Message, error) {
	var messages []models.Message
	query := DB.Preload("Sender").Preload("Group").Order("timestamp desc").Offset(offset).Limit(limit)

	if isGroup {
		query = query.Where("group_id = ?", id)
	} else {
		// id is the friend's ID
		query = query.Where("group_id = 0 AND (sender_id = ? OR target_id = ?)", id, id)
	}

	result := query.Find(&messages)
	return messages, result.Error
}

func UpsertUser(user *models.User) error {
	return DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(user).Error
}

func UpsertGroup(group *models.Group) error {
	return DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(group).Error
}

func GetUserByID(id int64) (*models.User, error) {
	var user models.User
	if err := DB.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func GetGroupByID(id int64) (*models.Group, error) {
	var group models.Group
	if err := DB.First(&group, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &group, nil
}

func GetSessionSummaries(limit int) ([]models.ChatSession, error) {
	if limit <= 0 {
		limit = 50
	}
	fetchLimit := limit * 4
	if fetchLimit < limit {
		fetchLimit = limit
	}

	var messages []models.Message
	if err := DB.Preload("Sender").Preload("Group").
		Order("timestamp desc").
		Limit(fetchLimit).
		Find(&messages).Error; err != nil {
		return nil, err
	}

	unreadRows, err := UnreadSummary()
	if err != nil {
		return nil, err
	}
	unreadMap := make(map[string]int64, len(unreadRows))
	for _, row := range unreadRows {
		unreadMap[chatMapKey(row.ChatID, row.IsGroup)] = row.Count
	}

	sessions := make([]models.ChatSession, 0, limit)
	seen := make(map[string]struct{})
	for _, msg := range messages {
		chatID, isGroup := deriveChatID(&msg)
		if chatID == 0 {
			continue
		}
		key := chatMapKey(chatID, isGroup)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		name, avatar := resolveIdentity(&msg, chatID, isGroup)
		session := models.ChatSession{
			ChatID:      chatID,
			IsGroup:     isGroup,
			Name:        name,
			Avatar:      avatar,
			LastMessage: formatPreview(&msg),
			Time:        msg.Timestamp,
			Unread:      unreadMap[key],
		}
		sessions = append(sessions, session)
		if len(sessions) >= limit {
			break
		}
	}
	return sessions, nil
}

func chatMapKey(id int64, isGroup bool) string {
	if isGroup {
		return fmt.Sprintf("g-%d", id)
	}
	return fmt.Sprintf("f-%d", id)
}

func deriveChatID(msg *models.Message) (int64, bool) {
	if msg.GroupID != 0 {
		return msg.GroupID, true
	}
	if msg.IsSend && msg.TargetID != 0 {
		return msg.TargetID, false
	}
	if !msg.IsSend && msg.SenderID != 0 {
		return msg.SenderID, false
	}
	return 0, false
}

func resolveIdentity(msg *models.Message, chatID int64, isGroup bool) (string, string) {
	if isGroup {
		if msg.Group.ID != 0 {
			if msg.Group.Name != "" {
				return msg.Group.Name, msg.Group.AvatarURL
			}
			return fmt.Sprintf("群聊 %d", chatID), msg.Group.AvatarURL
		}
		if grp, _ := GetGroupByID(chatID); grp != nil {
			return grp.Name, grp.AvatarURL
		}
		return fmt.Sprintf("群聊 %d", chatID), fmt.Sprintf("https://p.qlogo.cn/gh/%d/%d/640", chatID, chatID)
	}

	if user, _ := GetUserByID(chatID); user != nil {
		if user.Nickname != "" || user.AvatarURL != "" {
			return user.Nickname, user.AvatarURL
		}
	}

	if msg.Sender.ID != 0 && msg.SenderID == chatID {
		return msg.Sender.Nickname, msg.Sender.AvatarURL
	}

	return fmt.Sprintf("好友 %d", chatID), fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", chatID)
}

func formatPreview(msg *models.Message) string {
	if len(msg.Elements) > 0 {
		for _, el := range msg.Elements {
			switch strings.ToLower(el.Type) {
			case "image":
				return "[图片]"
			case "voice", "record":
				return "[语音]"
			case "face":
				if el.Text != "" {
					return el.Text
				}
				return "[表情]"
			}
		}
	}
	if msg.Content != "" {
		return msg.Content
	}
	return ""
}

func MarkChatRead(id int64, isGroup bool) error {
	query := DB.Model(&models.Message{})
	if isGroup {
		query = query.Where("group_id = ?", id)
	} else {
		query = query.Where("group_id = 0 AND (sender_id = ? OR target_id = ?)", id, id)
	}
	return query.Updates(map[string]interface{}{"is_read": true}).Error
}

func MarkMessageRecalled(messageID int32) (*models.Message, error) {
	var msg models.Message
	if err := DB.Where("message_id = ?", messageID).First(&msg).Error; err != nil {
		return nil, err
	}
	msg.Recall = true
	if err := DB.Save(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

func SearchMessages(keyword string, chatID int64, isGroup bool, limit int) ([]models.Message, error) {
	var messages []models.Message
	if limit <= 0 {
		limit = 50
	}
	query := DB.Preload("Sender").Order("timestamp desc").Limit(limit)
	if isGroup {
		query = query.Where("group_id = ?", chatID)
	} else if chatID != 0 {
		query = query.Where("group_id = 0 AND (sender_id = ? OR target_id = ?)", chatID, chatID)
	}
	like := "%" + keyword + "%"
	query = query.Where("content LIKE ? OR raw LIKE ?", like, like)
	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func UnreadSummary() ([]models.UnreadSummary, error) {
	var rows []models.UnreadSummary
	err := DB.Model(&models.Message{}).
		Select(`CASE WHEN group_id = 0 THEN CASE WHEN is_send THEN target_id ELSE sender_id END ELSE group_id END as chat_id,
			CASE WHEN group_id = 0 THEN 0 ELSE 1 END as is_group,
			COUNT(*) as count`).
		Where("is_read = ?", false).
		Group("chat_id, is_group").
		Scan(&rows).Error
	return rows, err
}
