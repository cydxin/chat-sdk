package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RoomNotification 群(房间)内的操作通知事件（事件只存一份）
// 用于：
// - WS 即时通知的消息体来源
// - 离线/新设备通过 HTTP 拉取近两天通知
//
// 事件与投递分离：RoomNotificationDelivery 记录“某用户收到了某事件(未读/已读)”
// 这样事件 payload 不会因为群成员多而重复存多份。
type RoomNotification struct {
	ID        uint64         `gorm:"primarykey"`
	RoomID    uint64         `gorm:"index;not null"`
	ActorID   uint64         `gorm:"index;not null"`
	EventType string         `gorm:"size:64;index;not null"`
	Payload   datatypes.JSON `gorm:"type:json"`
	CreatedAt time.Time      `gorm:"index"`

	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (RoomNotification) TableName() string { return prefix + "room_notification" }

// RoomNotificationDelivery 用户投递表（每个用户一条，用于未读/已读与离线拉取）
// 建议唯一索引 (user_id, event_id) 用于幂等。
type RoomNotificationDelivery struct {
	ID      uint64 `gorm:"primarykey"`
	UserID  uint64 `gorm:"index:idx_user_created,priority:1;not null;uniqueIndex:idx_user_event"`
	EventID uint64 `gorm:"not null;uniqueIndex:idx_user_event"`
	RoomID  uint64 `gorm:"index;not null"`

	IsRead bool `gorm:"default:false;index"`
	ReadAt *time.Time

	CreatedAt time.Time      `gorm:"index:idx_user_created,priority:2"`
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// 关联（用于查询 preload/join）
	Event RoomNotification `gorm:"foreignKey:EventID"`
}

func (RoomNotificationDelivery) TableName() string { return prefix + "room_notification_delivery" }
