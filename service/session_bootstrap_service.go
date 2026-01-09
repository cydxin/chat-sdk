package service

import (
	"github.com/cydxin/chat-sdk/models"
)

// SessionBootstrapService 用于 WS 连接建立时，加载用户会话相关的轻量状态到内存。
// 当前用于：加载 IsVisible=true 的会话 last_read_msg_id。
type SessionBootstrapService struct {
	*Service
}

func NewSessionBootstrapService(s *Service) *SessionBootstrapService {
	return &SessionBootstrapService{Service: s}
}

// GetVisibleConversationLastReads 返回当前用户所有可见会话的已读游标。
// Key: room_id, Value: last_read_msg_id（为 0 表示未读过任何消息/NULL）。
func (s *SessionBootstrapService) GetVisibleConversationLastReads(userID uint64) (map[uint64]uint64, error) {
	if userID == 0 {
		return map[uint64]uint64{}, nil
	}

	type row struct {
		RoomID        uint64
		LastReadMsgID *uint64
	}
	var rows []row
	if err := s.DB.Model(&models.Conversation{}).
		Select("room_id, last_read_msg_id").
		Where("user_id = ? AND is_visible = ?", userID, true).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[uint64]uint64, len(rows))
	for _, r := range rows {
		if r.RoomID == 0 {
			continue
		}
		if r.LastReadMsgID == nil {
			out[r.RoomID] = 0
			continue
		}
		out[r.RoomID] = *r.LastReadMsgID
	}
	return out, nil
}
