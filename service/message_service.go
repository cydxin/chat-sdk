package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cydxin/chat-sdk/models"
	"gorm.io/datatypes"
)

// MessageDTO 消息数据传输对象（避免 Swagger 递归）
type MessageDTO struct {
	ID           uint64         `json:"id"`
	MessageID    string         `json:"message_id"`
	RoomID       uint64         `json:"room_id"`
	SenderID     uint64         `json:"sender_id"`
	ReplyToMsgID *uint64        `json:"reply_to_msg_id,omitempty"`
	Type         uint8          `json:"type"`
	Content      string         `json:"content"`
	Extra        datatypes.JSON `json:"extra,omitempty"`
	IsSystem     bool           `json:"is_system"`
	IsEncrypted  bool           `json:"is_encrypted"`
	Status       uint8          `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// SenderDTO 发送人信息（用于消息列表返回）
type SenderDTO struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// MessageListItemDTO 消息列表项（带发送人信息；不返回 Room，避免冗余/递归）
type MessageListItemDTO struct {
	ID           uint64         `json:"id"`
	RoomID       uint64         `json:"room_id"`
	SenderID     uint64         `json:"sender_id"`
	Sender       *SenderDTO     `json:"sender,omitempty"`
	ReplyToMsgID *uint64        `json:"reply_to_msg_id,omitempty"`
	Type         uint8          `json:"type"`
	Content      string         `json:"content"`
	Extra        datatypes.JSON `json:"extra,omitempty"`
	IsSystem     bool           `json:"is_system"`
	IsEncrypted  bool           `json:"is_encrypted"`
	Status       uint8          `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ToMessageDTO 将 Message 转换为 MessageDTO
func ToMessageDTO(msg *models.Message) *MessageDTO {
	if msg == nil {
		return nil
	}
	return &MessageDTO{
		ID: msg.ID,
		//MessageID:    msg.MessageID,
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

func toSenderDTO(u *models.User) *SenderDTO {
	if u == nil {
		return nil
	}
	return &SenderDTO{ID: u.ID, Username: u.Username, Nickname: u.Nickname, Avatar: u.Avatar}
}

func toMessageListItemDTO(m *models.Message) *MessageListItemDTO {
	if m == nil {
		return nil
	}
	return &MessageListItemDTO{
		ID:           m.ID,
		RoomID:       m.RoomID,
		SenderID:     m.SenderID,
		Sender:       toSenderDTO(&m.Sender),
		ReplyToMsgID: m.ReplyToMsgID,
		Type:         m.Type,
		Content:      m.Content,
		Extra:        m.Extra,
		IsSystem:     m.IsSystem,
		IsEncrypted:  m.IsEncrypted,
		Status:       m.Status,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// ToMessageDTOs 已弃用：当前仓库内无引用。
// 如需批量转换可直接用 ToMessageDTO + 循环。
// func ToMessageDTOs(msgs []models.Message) []MessageDTO {
// 	dtos := make([]MessageDTO, len(msgs))
// 	for i, msg := range msgs {
// 		if dto := ToMessageDTO(&msg); dto != nil {
// 			dtos[i] = *dto
// 		}
// 	}
// 	return dtos
// }

func toMessageListItemDTOs(msgs []models.Message) []MessageListItemDTO {
	out := make([]MessageListItemDTO, 0, len(msgs))
	for i := range msgs {
		if dto := toMessageListItemDTO(&msgs[i]); dto != nil {
			out = append(out, *dto)
		}
	}
	return out
}

type MessageService struct {
	*Service
	messageDAO *models.MessageDAO
	// SessionBootstrap 用于 WS 建连时加载会话已读游标（由 engine 注入）
	SessionBootstrap *SessionBootstrapService
}

func NewMessageService(s *Service) *MessageService {
	log.Println("NewMessageService")
	return &MessageService{Service: s, messageDAO: models.NewMessageDAO(s.DB), SessionBootstrap: s.SessionBootstrap}
}

// SaveMessage 保存消息到数据库
func (s *MessageService) SaveMessage(roomID uint64, senderID uint64, content string, msgType uint8) (*models.Message, error) {
	if err := s.checkMuteStatus(roomID, senderID); err != nil {
		return nil, err
	}

	msg := &models.Message{
		//MessageID: uuid.New().String(), // 生成唯一的消息 ID
		RoomID:   roomID,
		SenderID: senderID,
		Type:     msgType,
		Content:  content,
		Status:   models.MessageStatusSent, // 默认状态为已发送
	}
	err := s.messageDAO.Create(msg)
	if err != nil {
		return nil, err
	}
	log.Println(msg.ID, " 最后的消息 ID")
	s.DB.Model(&models.Room{}).Where("id = ?", roomID).UpdateColumn("last_message_id", msg.ID)
	//now := time.Now()

	return msg, nil
}

func (s *MessageService) checkMuteStatus(roomID, userID uint64) error {
	var room models.Room
	if err := s.DB.First(&room, roomID).Error; err != nil {
		return err
	}

	var member models.RoomUser
	if err := s.DB.Where("room_id = ? AND user_id = ?", roomID, userID).First(&member).Error; err != nil {
		return err // Not a member?
	}

	// Admin/Owner bypass mute
	if member.Role > 0 {
		return nil
	}

	now := time.Now()

	// 1. Check User Mute
	if member.IsMuted && member.MutedUntil != nil && member.MutedUntil.After(now) {
		return fmt.Errorf("你已经被禁至 %s", member.MutedUntil.Format("2006-01-02 15:04:05"))
	}

	// 2. Check Global Mute (Countdown)
	if room.IsMute && room.MuteUntil != nil && room.MuteUntil.After(now) {
		return fmt.Errorf("群开启禁言至 %s", room.MuteUntil.Format("2006-01-02 15:04:05"))
	}

	// 3. Check Global Mute (Scheduled)
	if room.MuteDailyDuration > 0 && room.MuteDailyStartTime != "" {
		// Parse start time
		t, err := time.Parse("15:04", room.MuteDailyStartTime)
		if err == nil {
			// Check two windows: starting today and starting yesterday
			startToday := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			endToday := startToday.Add(time.Duration(room.MuteDailyDuration) * time.Minute)

			if now.After(startToday) && now.Before(endToday) {
				return fmt.Errorf("群每日禁言 %s 禁言 %d分钟", room.MuteDailyStartTime, room.MuteDailyDuration)
			}

			startYesterday := startToday.Add(-24 * time.Hour)
			endYesterday := startYesterday.Add(time.Duration(room.MuteDailyDuration) * time.Minute)
			if now.After(startYesterday) && now.Before(endYesterday) {
				return fmt.Errorf("群每日禁言 %s 禁言 %d分钟", room.MuteDailyStartTime, room.MuteDailyDuration)
			}
		}
	}

	return nil
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
			"type":        EventRecall,
			"recall_type": recallType,
			"message_id":  messageID,
			"room_id":     msg.RoomID,
			"user_id":     userID,
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

// GetRoomMessagesDTO 获取房间消息列表（分页，带发送人信息，返回 DTO）
func (s *MessageService) GetRoomMessagesDTO(roomID uint64, limit, messID int) ([]MessageListItemDTO, error) {
	var msgs []models.Message
	// 这里不走 DAO：需要 preload sender
	//err
	query := s.DB.Model(&models.Message{}).
		Preload("Sender").
		Where("room_id = ?", roomID)
	if messID > 0 {
		query = query.Where("id < ?", messID)
	}
	err := query.Order("created_at DESC").
		Limit(limit).
		Find(&msgs).Error
	if err != nil {
		return nil, err
	}
	return toMessageListItemDTOs(msgs), nil
}

// GetMessageByID 根据ID获取消息
func (s *MessageService) GetMessageByID(messageID uint64) (*models.Message, error) {
	dao := s.messageDAO
	return dao.FindByID(messageID)
}
