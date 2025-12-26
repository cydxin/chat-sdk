package models

import (
	"errors"

	"gorm.io/gorm"
)

// MessageDAO 封装 Message 相关的数据库操作
type MessageDAO struct {
	db *gorm.DB
}

// NewMessageDAO 创建 MessageDAO 实例
func NewMessageDAO(db *gorm.DB) *MessageDAO {
	return &MessageDAO{db: db}
}

// Create 创建消息
func (dao *MessageDAO) Create(msg *Message) error {
	return dao.db.Create(msg).Error
}

// FindByID 根据ID查找消息
func (dao *MessageDAO) FindByID(id uint64) (*Message, error) {
	var msg Message
	err := dao.db.Where("id = ?", id).First(&msg).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// FindByRoomID 获取房间消息列表
func (dao *MessageDAO) FindByRoomID(roomID uint64, limit, offset int) ([]Message, error) {
	var messages []Message
	err := dao.db.Where("room_id = ?", roomID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error
	return messages, err
}

// UpdateStatus 更新消息状态
func (dao *MessageDAO) UpdateStatus(id uint64, status int) error {
	return dao.db.Model(&Message{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateContent 更新消息内容 (例如撤回时修改内容)
func (dao *MessageDAO) UpdateContent(id uint64, content string) error {
	return dao.db.Model(&Message{}).Where("id = ?", id).Update("content", content).Error
}

// DeleteForUser 单删消息 (仅对指定用户不可见)
func (dao *MessageDAO) DeleteForUser(userID, messageID uint64) error {
	var status MessageStatus
	err := dao.db.Where("user_id = ? AND message_id = ?", userID, messageID).First(&status).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = MessageStatus{
				UserID:    (userID),
				MessageID: (messageID),
				IsDeleted: true,
			}
			return dao.db.Create(&status).Error
		}
		return err
	}
	return dao.db.Model(&status).Update("is_deleted", true).Error
}

// DeleteForEveryone 双删消息 (对所有人不可见)
func (dao *MessageDAO) DeleteForEveryone(messageID uint64) error {
	return dao.UpdateStatus(messageID, MessageStatusBothDeleted)
}

// FindByRoomIDForUser 获取房间消息列表 (过滤掉用户已删除的消息)
func (dao *MessageDAO) FindByRoomIDForUser(roomID, userID uint64, limit, offset int) ([]Message, error) {
	var messages []Message
	err := dao.db.Table("message").
		Select("message.*").
		Joins("LEFT JOIN message_statuses ON message_statuses.message_id = message.id AND message_statuses.user_id = ?", userID).
		Where("message.room_id = ?", roomID).
		Where("message.status != ?", MessageStatusBothDeleted).
		Where("message_statuses.is_deleted IS NULL OR message_statuses.is_deleted = ?", false).
		Order("message.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error
	return messages, err
}
