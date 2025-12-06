package db

import (
	"LagrangeQQ/internal/models"
	"os"
	"path/filepath"

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
