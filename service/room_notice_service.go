package service

import (
	"errors"
	"time"

	"github.com/cydxin/chat-sdk/cons"
	"github.com/cydxin/chat-sdk/models"
	"gorm.io/gorm"
)

type RoomNoticeDTO struct {
	ID        uint64    `json:"id"`
	RoomID    uint64    `json:"room_id"`
	ActorID   uint64    `json:"actor_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	IsPinned  bool      `json:"is_pinned"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toRoomNoticeDTO(n *models.RoomNotice) *RoomNoticeDTO {
	if n == nil {
		return nil
	}
	return &RoomNoticeDTO{
		ID:        n.ID,
		RoomID:    n.RoomID,
		ActorID:   n.ActorID,
		Title:     n.Title,
		Content:   n.Content,
		IsPinned:  n.IsPinned,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

type RoomNoticeService struct{ *Service }

func NewRoomNoticeService(s *Service) *RoomNoticeService { return &RoomNoticeService{Service: s} }

// CreateNotice 发布一条群公告，并发送通知给房间成员。
// 权限：群主/管理员（role>=1）。
func (s *RoomNoticeService) CreateNotice(roomID, actorID uint64, title, content string, pinned bool) (*RoomNoticeDTO, error) {
	if roomID == 0 {
		return nil, errors.New("room_id is required")
	}
	if actorID == 0 {
		return nil, errors.New("actor_id is required")
	}
	if content == "" {
		return nil, errors.New("content is required")
	}

	role, err := getMemberRole(s.DB, roomID, actorID)
	if err != nil {
		return nil, err
	}
	if role < 1 {
		return nil, errors.New("permission denied")
	}

	now := time.Now()
	n := &models.RoomNotice{RoomID: roomID, ActorID: actorID, Title: title, Content: content, IsPinned: pinned, CreatedAt: now, UpdatedAt: now}
	if err := s.DB.Create(n).Error; err != nil {
		return nil, err
	}

	// 通知：落库事件+投递+WS推送
	if s.Notify != nil {
		var members []uint64
		_ = s.DB.Model(&models.RoomUser{}).Where("room_id = ?", roomID).Pluck("user_id", &members).Error
		_, _ = s.Notify.PublishRoomEvent(roomID, actorID, cons.EventRoomNoticeSet, map[string]any{"notice_id": n.ID}, members, true)
	}

	return toRoomNoticeDTO(n), nil
}

// ListNotices 获取群公告列表（默认返回最近若干，置顶优先）。
func (s *RoomNoticeService) ListNotices(roomID uint64, limit int) ([]RoomNoticeDTO, error) {
	if roomID == 0 {
		return nil, errors.New("room_id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	var rows []models.RoomNotice
	if err := s.DB.Model(&models.RoomNotice{}).
		Where("room_id = ?", roomID).
		Order("is_pinned desc").
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]RoomNoticeDTO, 0, len(rows))
	for i := range rows {
		if dto := toRoomNoticeDTO(&rows[i]); dto != nil {
			out = append(out, *dto)
		}
	}
	return out, nil
}

// DeleteNotices 删除群公告。
func (s *RoomNoticeService) DeleteNotices(roomIDS []uint64) error {
	if len(roomIDS) == 0 {
		return nil
	}
	if err := s.DB.Where("room_id IN ?", roomIDS).
		Delete(&models.RoomNotice{}).Error; err != nil {
		return err
	}
	return nil
}

func getMemberRole(db *gorm.DB, roomID, userID uint64) (int, error) {
	type roleRow struct{ Role uint8 }
	var r roleRow
	if err := db.Model(&models.RoomUser{}).
		Select("role").
		Where("room_id = ? AND user_id = ?", roomID, userID).
		First(&r).Error; err != nil {
		return 0, err
	}
	return int(r.Role), nil
}
