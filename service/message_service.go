package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cydxin/chat-sdk/message"
	"github.com/cydxin/chat-sdk/models"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
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
func (s *MessageService) SaveMessage(roomID uint64, senderID uint64, content string, msgType uint8, extra message.Extra) (*models.Message, error) {
	if err := s.checkMuteStatus(roomID, senderID); err != nil {
		return nil, err
	}

	extraBytes, err := json.Marshal(extra)
	if err != nil {
		return nil, err
	}

	msg := &models.Message{
		//MessageID: uuid.New().String(), // 生成唯一的消息 ID
		RoomID:   roomID,
		SenderID: senderID,
		Type:     msgType,
		Content:  content,
		Status:   models.MessageStatusSent, // 默认状态为已发送
		Extra:    datatypes.JSON(extraBytes),
	}
	err = s.messageDAO.Create(msg)
	if err != nil {
		return nil, err
	}
	log.Println(msg.ID, " 最后的消息 ID")
	s.DB.Model(&models.Room{}).Where("id = ?", roomID).UpdateColumn("last_message_id", msg.ID)

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

// RecallMessages 批量撤回/删除消息。
// 返回：成功的 message_id 列表，以及失败原因（按 message_id）。
func (s *MessageService) RecallMessages(messageIDs []uint64, userID uint64, recallType uint8) (okIDs []uint64, failed map[uint64]string, err error) {
	failed = make(map[uint64]string)
	if userID == 0 {
		return nil, map[uint64]string{0: "user_id is required"}, fmt.Errorf("user_id is required")
	}
	if len(messageIDs) == 0 {
		return []uint64{}, failed, nil
	}

	// 去重 + 清洗
	uniq := make(map[uint64]struct{}, len(messageIDs))
	ids := make([]uint64, 0, len(messageIDs))
	for _, id := range messageIDs {
		if id == 0 {
			continue
		}
		if _, ok := uniq[id]; ok {
			continue
		}
		uniq[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []uint64{}, failed, nil
	}

	// 批量查消息
	var msgs []models.Message
	if err := s.DB.Model(&models.Message{}).
		Where("id IN ?", ids).
		Find(&msgs).Error; err != nil {
		return nil, nil, err
	}
	msgByID := make(map[uint64]models.Message, len(msgs))
	roomIDsSet := make(map[uint64]struct{}, len(msgs))
	for _, m := range msgs {
		msgByID[m.ID] = m
		roomIDsSet[m.RoomID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := msgByID[id]; !ok {
			failed[id] = "message not found"
		}
	}

	if len(msgByID) == 0 {
		return []uint64{}, failed, nil
	}

	// 批量查房间类型（用于群聊双删权限）
	roomIDs := make([]uint64, 0, len(roomIDsSet))
	for rid := range roomIDsSet {
		roomIDs = append(roomIDs, rid)
	}
	var rooms []models.Room
	if err := s.DB.Model(&models.Room{}).
		Select("id, type").
		Where("id IN ?", roomIDs).
		Find(&rooms).Error; err != nil {
		return nil, nil, err
	}
	roomTypeByID := make(map[uint64]uint8, len(rooms))
	for _, r := range rooms {
		roomTypeByID[r.ID] = r.Type
	}

	now := time.Now()

	// 单事务执行批量变更
	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, nil, tx.Error
	}
	defer tx.Rollback()

	// 需要更新 message.status 的 IDs（撤回/双删）
	setStatusIDs := make([]uint64, 0, len(ids))
	setStatusTo := 0

	// 需要插入/更新 message_status（单删）
	statusRows := make([]models.MessageStatus, 0)
	statusUpdateIDs := make([]uint64, 0)

	for _, id := range ids {
		m, ok := msgByID[id]
		if !ok {
			continue
		}

		switch recallType {
		case models.MessageStatusRecalled:
			if m.SenderID != userID {
				failed[id] = "撤回只能操作自己的消息"
				continue
			}
			if now.Sub(m.CreatedAt) > 2*time.Minute {
				failed[id] = "消息撤回时间已过"
				continue
			}
			setStatusIDs = append(setStatusIDs, id)
			setStatusTo = models.MessageStatusRecalled
			okIDs = append(okIDs, id)

		case models.MessageStatusDeleted:
			// 单删：写入 message_status（用户维度）
			statusRows = append(statusRows, models.MessageStatus{UserID: userID, MessageID: id, RoomID: m.RoomID, IsDeleted: true, CreatedAt: now, UpdatedAt: now})
			statusUpdateIDs = append(statusUpdateIDs, id)
			okIDs = append(okIDs, id)

		case models.MessageStatusBothDeleted:
			// 群聊双删：只能删除自己的
			if roomTypeByID[m.RoomID] == 2 {
				if m.SenderID != userID {
					failed[id] = "群聊双删只能操作自己的消息"
					continue
				}
			}
			setStatusIDs = append(setStatusIDs, id)
			setStatusTo = models.MessageStatusBothDeleted
			okIDs = append(okIDs, id)
		default:
			failed[id] = "不支持的操作类型"
			continue
		}
	}

	// 批量更新 message.status
	if len(setStatusIDs) > 0 {
		if err := tx.Model(&models.Message{}).
			Where("id IN ?", setStatusIDs).
			Update("status", setStatusTo).Error; err != nil {
			return nil, nil, err
		}
	}

	// 单删：批量 upsert message_status.is_deleted=true
	if len(statusRows) > 0 {
		// 先插入（唯一键冲突则忽略），再统一 update
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&statusRows).Error; err != nil {
			return nil, nil, err
		}
		if err := tx.Model(&models.MessageStatus{}).
			Where("user_id = ? AND message_id IN ?", userID, statusUpdateIDs).
			Updates(map[string]any{"is_deleted": true, "updated_at": now}).Error; err != nil {
			return nil, nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}

	// 通知：撤回/双删才通知（单删不打扰）
	needNotify := recallType == models.MessageStatusRecalled || recallType == models.MessageStatusBothDeleted
	if needNotify {
		// 按 room 聚合 message_ids
		roomToMsgIDs := make(map[uint64][]uint64)
		for _, id := range okIDs {
			m, ok := msgByID[id]
			if !ok {
				continue
			}
			roomToMsgIDs[m.RoomID] = append(roomToMsgIDs[m.RoomID], id)
		}

		for roomID, mids := range roomToMsgIDs {
			// members
			var members []uint64
			_ = s.DB.Model(&models.RoomUser{}).
				Where("room_id = ?", roomID).
				Pluck("user_id", &members).Error

			payload := map[string]any{
				"recall_type":  recallType,
				"message_ids":  mids,
				"room_id":      roomID,
				"operator_id":  userID,
				"operator_uid": userID,
			}

			// 有 Notify 就用统一通知落库+WS；没有则保留旧 WS notifier
			if s.Notify != nil {
				_, _ = s.Notify.PublishRoomEvent(roomID, userID, EventRecall, payload, members, true)
			} else if s.WsNotifier != nil {
				notification := map[string]any{
					"type":        EventRecall,
					"recall_type": recallType,
					"message_ids": mids,
					"room_id":     roomID,
					"user_id":     userID,
				}
				b, _ := json.Marshal(notification)
				for _, memberID := range members {
					s.WsNotifier(memberID, b)
				}
			}
		}
	}

	return okIDs, failed, nil
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
