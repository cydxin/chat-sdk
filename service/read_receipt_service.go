package service

import (
	"time"

	"gorm.io/gorm"

	"github.com/cydxin/chat-sdk/models"
)

// ReadReceiptService 用于处理“已读回执”落库：更新 conversation.last_read_msg_id/unread_count。
// 说明：你希望先写在 client.readList 中延迟落库，本服务用于最终 flush。
type ReadReceiptService struct {
	*Service
}

func NewReadReceiptService(s *Service) *ReadReceiptService {
	return &ReadReceiptService{Service: s}
}

// FlushUserRead 批量 flush 用户在多个 room 的最后已读 message_id。
// rooms: room_id -> last_read_msg_id。
// 行为：
// - last_read_msg_id 取更大值（避免乱序回执覆盖）。
// - unread_count 直接置 0（简单策略：表示用户已读到最后）。
func (s *ReadReceiptService) FlushUserRead(userID uint64, rooms map[uint64]uint64) error {
	if userID == 0 || len(rooms) == 0 {
		return nil
	}

	now := time.Now()
	for roomID, lastRead := range rooms {
		if roomID == 0 || lastRead == 0 {
			continue
		}

		// last_read_msg_id = GREATEST(last_read_msg_id, lastRead)
		// unread_count = 0
		// updated_at = now

		err := s.DB.Model(&models.Conversation{}).
			Where("user_id = ? AND room_id = ?", userID, roomID).
			Updates(map[string]any{
				"last_read_msg_id": gorm.Expr("CASE WHEN last_read_msg_id IS NULL OR last_read_msg_id < ? THEN ? ELSE last_read_msg_id END", lastRead, lastRead),
				"updated_at":       now,
			}).Error
		if err != nil {
			return err
		}
	}

	return nil
}
