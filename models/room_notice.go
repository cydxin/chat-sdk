package models

import (
	"time"

	"gorm.io/gorm"
)

// RoomNotice 群公告（支持多条，按 pinned/created_at 排序展示）。
// - 便于分页/历史保留
// - 便于按权限审计（谁发布的）
// - 便于做通知投递（落库事件 + WS）
type RoomNotice struct {
	ID       uint64 `gorm:"primarykey"`
	RoomID   uint64 `gorm:"index;not null"`
	ActorID  uint64 `gorm:"index;not null"` // 发布人
	Title    string `gorm:"size:200"`
	Content  string `gorm:"type:text;not null"`
	IsPinned bool   `gorm:"default:false;index"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (RoomNotice) TableName() string { return prefix + "room_notice" }
