package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cydxin/chat-sdk/models"
)

// MessageDTO 消息数据传输对象（避免 Swagger 递归）
type MessageDTO struct {
	ID           uint64    `json:"id"`
	MessageID    string    `json:"message_id"`
	RoomID       uint64    `json:"room_id"`
	SenderID     uint64    `json:"sender_id"`
	ReplyToMsgID *uint64   `json:"reply_to_msg_id,omitempty"`
	Type         uint8     `json:"type"`
	Content      string    `json:"content"`
	Extra        string    `json:"extra,omitempty"`
	IsSystem     bool      `json:"is_system"`
	IsEncrypted  bool      `json:"is_encrypted"`
	Status       uint8     `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ToMessageDTO 将 Message 转换为 MessageDTO
func ToMessageDTO(msg *models.Message) *MessageDTO {
	if msg == nil {
		return nil
	}
	return &MessageDTO{
		ID:           msg.ID,
		MessageID:    msg.MessageID,
		RoomID:       msg.RoomID,
		SenderID:     msg.SenderID,
		ReplyToMsgID: msg.ReplyToMsgID,
		Type:         msg.Type,
		Content:      msg.Content,
		Extra:        msg.Extra,
		IsSystem:     msg.IsSystem,
		IsEncrypted:  msg.IsEncrypted,
		Status:       msg.Status,
		CreatedAt:    msg.CreatedAt,
		UpdatedAt:    msg.UpdatedAt,
	}
}

// ToMessageDTOs 批量转换
func ToMessageDTOs(msgs []models.Message) []MessageDTO {
	dtos := make([]MessageDTO, len(msgs))
	for i, msg := range msgs {
		if dto := ToMessageDTO(&msg); dto != nil {
			dtos[i] = *dto
		}
	}
	return dtos
}

type MessageService struct {
	*Service
	messageDAO *models.MessageDAO
}

func NewMessageService(s *Service) *MessageService {
	log.Println("NewMessageService")
	return &MessageService{Service: s, messageDAO: models.NewMessageDAO(s.DB)}
}

// SaveMessage 保存消息到数据库
func (s *MessageService) SaveMessage(roomID uint64, senderID uint64, content string, msgType uint8) (*models.Message, error) {
	msg := &models.Message{
		RoomID:   roomID,
		SenderID: senderID,
		Type:     msgType,
		Content:  content,
	}
	err := s.messageDAO.Create(msg)
	return msg, err
}

/*
RecallMessage 撤回消息 删除消息 双删消息
撤回：只能撤回自己的 + 2分钟限制;
双删：
- 群聊：只能双删自己的消息
- 私聊：不限制发送者（按你的需求：无限制）;
单删：只影响自己视图（任何消息都允许删除）
*/
func (s *MessageService) RecallMessage(messageID, userID uint64, recallType uint8) error {
	dao := s.messageDAO
	msg, err := dao.FindByID(messageID)
	if err != nil {
		return err
	}

	switch recallType {
	case models.MessageStatusRecalled:
		if msg.SenderID != userID {
			return fmt.Errorf("撤回只能操作自己的消息")
		}
		if time.Since(msg.CreatedAt) > 2*time.Minute {
			return fmt.Errorf("消息撤回时间已过")
		}
		if err := dao.UpdateStatus(messageID, models.MessageStatusRecalled); err != nil {
			return err
		}
	case models.MessageStatusDeleted:
		if err := dao.DeleteForUser(userID, messageID); err != nil {
			return err
		}
	case models.MessageStatusBothDeleted:
		var room models.Room
		if err := s.DB.Where("id = ?", msg.RoomID).First(&room).Error; err != nil {
			return err
		}
		if room.Type == 2 {
			if msg.SenderID != userID {
				return fmt.Errorf("群聊双删只能操作自己的消息")
			}
		}

		if err := dao.DeleteForEveryone(messageID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的操作类型")
	}

	// 通知房间成员（单删通常不需要通知别人；为了避免打扰，这里只在撤回/双删时通知）
	needNotify := recallType == models.MessageStatusRecalled || recallType == models.MessageStatusBothDeleted
	if needNotify && s.WsNotifier != nil {
		// 获取房间成员
		var members []uint64
		s.DB.Model(models.RoomUser{}).
			Where("room_id = ?", msg.RoomID).
			Pluck("user_id", &members)

		// 构造通知消息
		notification := map[string]interface{}{
			"type":       recallType,
			"message_id": messageID,
			"room_id":    msg.RoomID,
			"user_id":    userID,
		}
		notifBytes, _ := json.Marshal(notification)

		// 通知所有成员
		for _, memberID := range members {
			s.WsNotifier(memberID, notifBytes)
		}
	}

	return nil
}

// GetRoomMessages 获取房间消息列表（分页）
func (s *MessageService) GetRoomMessages(roomID uint64, limit, offset int) ([]models.Message, error) {
	dao := s.messageDAO
	return dao.FindByRoomID(roomID, limit, offset)
}

// GetMessageByID 根据ID获取消息
func (s *MessageService) GetMessageByID(messageID uint64) (*models.Message, error) {
	dao := s.messageDAO
	return dao.FindByID(messageID)
}
