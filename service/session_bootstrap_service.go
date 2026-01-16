package service

import (
	"github.com/cydxin/chat-sdk/repository"
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
	return repository.NewConversationDAO(s.DB).ListVisibleLastReadSnapshot(userID)
}
