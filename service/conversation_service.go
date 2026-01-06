package service

import (
	"fmt"
	"log"

	"github.com/cydxin/chat-sdk/models"
	"gorm.io/gorm"
)

// ConversationListItemDTO 会话列表项（消息列表）
type ConversationListItemDTO struct {
	ConversationID uint64      `json:"conversation_id"`
	RoomID         uint64      `json:"room_id"`
	UserID         uint64      `json:"user_id"` //私聊时为对方用户ID，群聊时为0
	RoomAccount    string      `json:"room_account"`
	RoomType       uint8       `json:"room_type"` // 1-私聊 2-群聊
	Name           string      `json:"name"`      // 私聊：对方昵称；群聊：群名
	Avatar         string      `json:"avatar"`    // 私聊：对方头像；群聊：群头像
	LastMessage    *MessageDTO `json:"last_message,omitempty"`
	UnreadCount    uint64      `json:"unread_count"`
	UpdatedAt      int64       `json:"updated_at"` // unix seconds for easy sort/render
}

type ConversationService struct {
	*Service
}

func NewConversationService(s *Service) *ConversationService {
	log.Println("NewConversationService")
	return &ConversationService{Service: s}
}

// GetConversationList 获取当前用户的会话列表（消息列表）
func (s *ConversationService) GetConversationList(userID uint64) ([]ConversationListItemDTO, error) {
	var convs []models.Conversation
	err := s.DB.Model(&models.Conversation{}).
		Where("user_id = ? AND is_visible = ?", userID, true).
		Order("updated_at DESC").
		Find(&convs).Error
	if err != nil {
		return nil, err
	}
	if len(convs) == 0 {
		return []ConversationListItemDTO{}, nil
	}

	// 全部 房间ID
	roomIDs := make([]uint64, 0, len(convs))
	for _, c := range convs {
		roomIDs = append(roomIDs, c.RoomID)
	}

	// rooms
	var rooms []models.Room
	if err := s.DB.Model(&models.Room{}).
		Where("id IN ?", roomIDs).
		Find(&rooms).Error; err != nil {
		return nil, err
	}
	roomMap := make(map[uint64]models.Room, len(rooms))
	privateRoomIDs := make([]uint64, 0)
	for _, r := range rooms {
		roomMap[r.ID] = r
		if r.Type == 1 {
			privateRoomIDs = append(privateRoomIDs, r.ID)
		}
	}

	// 其他私人房间用户：Map[roomID]User
	otherUserMap := make(map[uint64]models.User)
	friendRemarkMap := make(map[uint64]string)
	if len(privateRoomIDs) > 0 {
		var roomUsers []models.RoomUser
		// 查找这些私聊房间里，user_id != 当前 userID 的记录
		if err := s.DB.Preload("User").
			Where("room_id IN ? AND user_id <> ?", privateRoomIDs, userID).
			Find(&roomUsers).Error; err == nil {
			for _, ru := range roomUsers {
				otherUserMap[ru.RoomID] = ru.User
			}
		}

		// 取出对方 user_id 列表，用于查 remark
		otherIDs := make([]uint64, 0, len(roomUsers))
		roomToOtherID := make(map[uint64]uint64)
		for _, ru := range roomUsers {
			otherIDs = append(otherIDs, ru.UserID)
			roomToOtherID[ru.RoomID] = ru.UserID
		}
		if len(otherIDs) > 0 {
			var friends []models.Friend
			_ = s.DB.Model(&models.Friend{}).
				Select("friend_id, remark").
				Where("user_id = ? AND friend_id IN ? AND status = ?", userID, otherIDs, 1).
				Find(&friends).Error
			remarkByFriendID := make(map[uint64]string, len(friends))
			for _, f := range friends {
				if f.Remark != "" {
					remarkByFriendID[f.FriendID] = f.Remark
				}
			}
			for roomID, otherID := range roomToOtherID {
				if rmk, ok := remarkByFriendID[otherID]; ok {
					friendRemarkMap[roomID] = rmk
				}
			}
		}
	}

	// 最后一条消息
	lastMsgIDs := make([]uint64, 0)
	for _, c := range convs {
		if c.LastMessageID != nil && *c.LastMessageID > 0 {
			lastMsgIDs = append(lastMsgIDs, *c.LastMessageID)
		}
	}

	lastMsgMap := make(map[uint64]*models.Message)
	if len(lastMsgIDs) > 0 {
		var msgs []models.Message
		if err := s.DB.Model(&models.Message{}).
			Where("id IN ?", lastMsgIDs).
			Find(&msgs).Error; err == nil {
			for i := range msgs {
				m := msgs[i]
				lastMsgMap[m.ID] = &m
			}
		}
	}

	// 用户的群昵称
	groupNicknameMap := make(map[uint64]string)
	{
		var rows []models.RoomUser
		_ = s.DB.Model(&models.RoomUser{}).
			Select("room_id, nickname").
			Where("user_id = ? AND room_id IN ?", userID, roomIDs).
			Find(&rows).Error
		for _, ru := range rows {
			if ru.Nickname != "" {
				groupNicknameMap[ru.RoomID] = ru.Nickname
			}
		}
	}

	out := make([]ConversationListItemDTO, 0, len(convs))
	for _, c := range convs {
		r, ok := roomMap[c.RoomID]
		if !ok {
			// room 被删了，跳过
			continue
		}

		item := ConversationListItemDTO{
			ConversationID: c.ID,
			RoomID:         r.ID,
			// 私聊：对方用户ID；群聊：0（下面 switch 会覆盖修正）
			UserID:      0,
			RoomAccount: r.RoomAccount,
			RoomType:    r.Type,
			UnreadCount: c.UnreadCount,
			UpdatedAt:   c.UpdatedAt.Unix(),
		}

		switch r.Type {
		case 1:
			if other, ok := otherUserMap[r.ID]; ok {
				item.UserID = other.ID
				// 优先好友备注
				if rmk, ok := friendRemarkMap[r.ID]; ok {
					item.Name = rmk
				} else if other.Nickname != "" {
					item.Name = other.Nickname
				} else {
					item.Name = other.Username
				}
				item.Avatar = other.Avatar
			} else {
				item.Name = "未知用户"
				item.Avatar = ""
			}
		case 2:
			item.UserID = 0
			item.Name = r.Name
			if nn, ok := groupNicknameMap[r.ID]; ok {
				item.Name = nn
			}
			item.Avatar = r.Avatar
			if item.Name == "" {
				item.Name = "群聊"
			}
		default:
			item.Name = fmt.Sprintf("room_%d", r.ID)
		}

		if c.LastMessageID != nil {
			if msg, ok := lastMsgMap[*c.LastMessageID]; ok {
				item.LastMessage = ToMessageDTO(msg)
			}
		}

		out = append(out, item)
	}

	return out, nil
}

// EnsureConversationForRoom 确保会话存在（用于首次进入房间或发送消息时创建）
func (s *ConversationService) EnsureConversationForRoom(userID, roomID uint64) error {
	conv := &models.Conversation{UserID: userID, RoomID: roomID}
	if err := s.DB.FirstOrCreate(conv, map[string]any{"user_id": userID, "room_id": roomID}).Error; err != nil {
		return err
	}
	// 确保可见：如果用户曾经隐藏过会话，新消息应该自动让它重新出现在列表里
	return s.DB.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(map[string]any{"is_visible": true}).Error
}

// SoftDeleteConversation 删除会话：当前实现为 hard delete（删除记录即不展示）；如需保留记录可改为加字段。
func (s *ConversationService) SoftDeleteConversation(userID, roomID uint64) error {
	return s.DB.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(map[string]any{"is_visible": false}).Error
}

// UpdateConversationLastMessage 更新会话最后一条消息（只更新当前用户视角）
func (s *ConversationService) UpdateConversationLastMessage(userID, roomID, messageID uint64) error {
	res := s.DB.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(map[string]any{"last_message_id": messageID}).
		Update("updated_at", gorm.Expr("NOW()"))
	return res.Error
}
