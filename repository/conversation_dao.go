package repository

import (
	"time"

	"github.com/cydxin/chat-sdk/models"
	"gorm.io/gorm"
)

// ConversationDAO 封装 Conversation 相关的数据库操作
//
// 约定：
// - 只做“数据访问”（CRUD/查询封装），不做业务编排（权限、通知等）。
// - 事务边界应由 service 控制；如需在事务中执行，请使用 WithDB(tx)。
type ConversationDAO struct {
	db *gorm.DB
}

func NewConversationDAO(db *gorm.DB) *ConversationDAO {
	return &ConversationDAO{db: db}
}

// WithDB 用于在事务（tx）中复用 DAO
func (dao *ConversationDAO) WithDB(db *gorm.DB) *ConversationDAO {
	if db == nil {
		return dao
	}
	return &ConversationDAO{db: db}
}

// ListVisibleByUserID 获取用户可见会话列表
func (dao *ConversationDAO) ListVisibleByUserID(userID uint64) ([]models.Conversation, error) {
	var convs []models.Conversation
	err := dao.db.Model(&models.Conversation{}).
		Where("user_id = ? AND is_visible = ?", userID, true).
		Order("updated_at DESC").
		Find(&convs).Error
	return convs, err
}

// EnsureConversationForRoom 确保会话存在，并设置为可见
func (dao *ConversationDAO) EnsureConversationForRoom(userID, roomID uint64) error {
	conv := &models.Conversation{UserID: userID, RoomID: roomID}
	if err := dao.db.FirstOrCreate(conv, map[string]any{"user_id": userID, "room_id": roomID}).Error; err != nil {
		return err
	}
	return dao.db.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(map[string]any{"is_visible": true}).Error
}

// SetRoomConversationVisibleByRoomID 将指定房间的所有会话设置为可见
func (dao *ConversationDAO) SetRoomConversationVisibleByRoomID(roomID uint64) error {
	return dao.db.Model(&models.Conversation{}).
		Where("is_visible = 0 AND room_id = ?", roomID).
		Updates(map[string]any{"is_visible": true}).Error
}

// SoftDeleteConversation 将会话设为不可见（软删除）
func (dao *ConversationDAO) SoftDeleteConversation(userID, roomID uint64) error {
	return dao.db.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(map[string]any{"is_visible": false}).Error
}

// UpdateLastReadMsgID 更新会话的最后已读消息ID
func (dao *ConversationDAO) UpdateLastReadMsgID(userID, roomID, lastReadMsgID uint64, now time.Time) error {
	updates := map[string]any{"last_read_msg_id": lastReadMsgID}
	if !now.IsZero() {
		updates["updated_at"] = now
	}
	return dao.db.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(updates).Error
}

// ClearLastReadMsgID 清空 last_read_msg_id（可用于隐藏会话或兼容策略切换）
func (dao *ConversationDAO) ClearLastReadMsgID(userID, roomID uint64, now time.Time) error {
	updates := map[string]any{"last_read_msg_id": nil}
	if !now.IsZero() {
		updates["updated_at"] = now
	}
	return dao.db.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(updates).Error
}

// ListVisibleLastReadSnapshot 获取用户所有可见会话的 last_read_msg_id 快照。
// 返回：room_id -> last_read_msg_id（NULL 视为 0）。
func (dao *ConversationDAO) ListVisibleLastReadSnapshot(userID uint64) (map[uint64]uint64, error) {
	if userID == 0 {
		return map[uint64]uint64{}, nil
	}

	type row struct {
		RoomID        uint64
		LastReadMsgID *uint64
	}
	var rows []row
	if err := dao.db.Model(&models.Conversation{}).
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

// UpdateLastMessageIDByUserRoom 更新会话 last_message_id，并同步 updated_at。
// 说明：当前你的模型里 Conversation.last_message_id 被注释掉了；如果你未来重新启用该字段，这个方法会直接生效。
func (dao *ConversationDAO) UpdateLastMessageIDByUserRoom(userID, roomID, messageID uint64, now time.Time) error {
	if userID == 0 || roomID == 0 || messageID == 0 {
		return nil
	}
	updates := map[string]any{"last_message_id": messageID}
	if !now.IsZero() {
		updates["updated_at"] = now
	}
	return dao.db.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(updates).Error
}

// EnsureConversationsVisibleBulk 批量确保这些用户在指定房间的会话存在且可见。
// 用于：创建群/私聊、拉人入群等场景。
func (dao *ConversationDAO) EnsureConversationsVisibleBulk(roomID uint64, userIDs []uint64, now time.Time) error {
	if roomID == 0 || len(userIDs) == 0 {
		return nil
	}

	// 去重
	seen := make(map[uint64]struct{}, len(userIDs))
	uniq := make([]uint64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid == 0 {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		uniq = append(uniq, uid)
	}
	if len(uniq) == 0 {
		return nil
	}

	for _, uid := range uniq {
		conv := &models.Conversation{UserID: uid, RoomID: roomID}
		if err := dao.db.FirstOrCreate(conv, map[string]any{"user_id": uid, "room_id": roomID}).Error; err != nil {
			return err
		}
	}

	updates := map[string]any{"is_visible": true}
	if !now.IsZero() {
		updates["updated_at"] = now
	}
	return dao.db.Model(&models.Conversation{}).
		Where("room_id = ? AND user_id IN ?", roomID, uniq).
		Updates(updates).Error
}
