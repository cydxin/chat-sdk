package service

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/cydxin/chat-sdk/cons"
	"github.com/cydxin/chat-sdk/models"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

// NotificationService 统一处理：群/房间内操作通知
// 约定：先落库(事件+投递)，再尽力通过 WS 推送；离线/新设备通过 HTTP 拉取。
type NotificationService struct {
	*Service
}

func NewNotificationService(s *Service) *NotificationService {
	return &NotificationService{Service: s}
}

// PublishRoomEvent 创建一条房间事件，并投递给 members。
// includeActor=是否也投递给操作者（部分事件想让操作者也看到，例如设置管理员/禁言）。
func (s *NotificationService) PublishRoomEvent(roomID, actorID uint64, eventType string, payload any, members []uint64, includeActor bool) (*models.RoomNotification, error) {
	if roomID == 0 {
		return nil, errors.New("room_id is required")
	}
	if actorID == 0 {
		return nil, errors.New("actor_id is required")
	}
	if eventType == "" {
		return nil, errors.New("event_type is required")
	}

	// 序列化 payload
	var pl datatypes.JSON
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		pl = b
	}

	now := time.Now()

	// 事件 + 投递建议同事务，确保离线拉取一定能看到。
	tx := s.DB.Begin()
	defer tx.Rollback()

	evt := &models.RoomNotification{
		RoomID:    roomID,
		ActorID:   actorID,
		EventType: eventType,
		Payload:   pl,
		CreatedAt: now,
	}
	if err := tx.Create(evt).Error; err != nil {
		return nil, err
	}

	// 处理 members
	// - 去重
	// - 可选排除 actor
	uniq := make(map[uint64]struct{}, len(members)+1)
	clean := make([]uint64, 0, len(members)+1)
	for _, uid := range members {
		if uid == 0 {
			continue
		}
		if !includeActor && uid == actorID {
			continue
		}
		if _, ok := uniq[uid]; ok {
			continue
		}
		uniq[uid] = struct{}{}
		clean = append(clean, uid)
	}
	if includeActor {
		if _, ok := uniq[actorID]; !ok {
			clean = append(clean, actorID)
		}
	}
	switch eventType {
	case cons.EventRoomMemberRemoved:
		// 把移除的人也放进通知里
		var removeID uint64
		tmp := payload.(map[string]interface{})
		if v, ok := tmp["user_id"]; ok {
			removeID = v.(uint64)
			clean = append(clean, removeID)
		}
	case cons.EventRoomMemberQuit:
		// 自己退出的话

	default:
	}

	rows := make([]models.RoomNotificationDelivery, 0, len(clean))
	for _, uid := range clean {
		rows = append(rows, models.RoomNotificationDelivery{
			UserID:    uid,
			EventID:   evt.ID,
			RoomID:    roomID,
			IsRead:    false,
			CreatedAt: now,
		})
	}
	if len(rows) > 0 {
		// OnConflict DoNothing: 避免并发/重试重复投递
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// WS 推送（尽力而为：失败不影响主流程）
	s.pushRoomEventToUsers(evt, clean)

	return evt, nil
}

func (s *NotificationService) pushRoomEventToUsers(evt *models.RoomNotification, userIDs []uint64) {
	if s.WsNotifier == nil || evt == nil {
		return
	}

	msg := struct {
		Type      string         `json:"type"`
		EventID   uint64         `json:"event_id"`
		RoomID    uint64         `json:"room_id"`
		ActorID   uint64         `json:"actor_id"`
		EventType string         `json:"event_type"`
		Payload   datatypes.JSON `json:"payload,omitempty"`
		CreatedAt time.Time      `json:"created_at"`
	}{
		Type:      cons.EventNotification,
		EventID:   evt.ID,
		RoomID:    evt.RoomID,
		ActorID:   evt.ActorID,
		EventType: evt.EventType,
		Payload:   evt.Payload,
		CreatedAt: evt.CreatedAt,
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for _, uid := range userIDs {
		s.WsNotifier(uid, b)
	}
}

// NotificationDTO HTTP 返回结构
// ID 使用 delivery_id 作为游标分页的主键。
// Event* 字段来自 RoomNotification。
type NotificationDTO struct {
	ID        uint64         `json:"id"` // delivery_id
	EventID   uint64         `json:"event_id"`
	RoomID    uint64         `json:"room_id"`
	ActorID   uint64         `json:"actor_id"`
	EventType string         `json:"event_type"`
	Payload   datatypes.JSON `json:"payload,omitempty"`
	IsRead    bool           `json:"is_read"`
	CreatedAt time.Time      `json:"created_at"`
}

// ListUserNotifications 拉取用户通知（默认按 delivery id 倒序）
// - sinceDays: 近 N 天（建议默认 2）
// - cursor: 分页游标（传 0 表示从最新开始；否则取 id < cursor）
func (s *NotificationService) ListUserNotifications(userID uint64, sinceDays int, cursor uint64, limit int, roomID *uint64, unreadOnly bool) ([]NotificationDTO, uint64, error) {
	if userID == 0 {
		return nil, 0, errors.New("user_id is required")
	}
	if sinceDays <= 0 {
		sinceDays = 2
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	since := time.Now().Add(-time.Duration(sinceDays) * 24 * time.Hour)

	q := s.DB.Model(&models.RoomNotificationDelivery{}).
		Where("user_id = ? AND created_at >= ?", userID, since)
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	if roomID != nil && *roomID > 0 {
		q = q.Where("room_id = ?", *roomID)
	}
	if unreadOnly {
		q = q.Where("is_read = ?", false)
	}

	// join 事件表拿 payload
	var rows []models.RoomNotificationDelivery
	if err := q.Preload("Event").Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]NotificationDTO, 0, len(rows))
	var nextCursor uint64
	for _, r := range rows {
		out = append(out, NotificationDTO{
			ID:        r.ID,
			EventID:   r.EventID,
			RoomID:    r.RoomID,
			ActorID:   r.Event.ActorID,
			EventType: r.Event.EventType,
			Payload:   r.Event.Payload,
			IsRead:    r.IsRead,
			CreatedAt: r.CreatedAt,
		})
		nextCursor = r.ID
	}

	return out, nextCursor, nil
}

// MarkReadByIDs 批量标记已读
func (s *NotificationService) MarkReadByIDs(userID uint64, ids []uint64) error {
	if userID == 0 {
		return errors.New("user_id is required")
	}
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	return s.DB.Model(&models.RoomNotificationDelivery{}).
		Where("user_id = ? AND id IN ?", userID, ids).
		Updates(map[string]any{"is_read": true, "read_at": &now}).Error
}
