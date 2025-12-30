package models

import (
	"gorm.io/gorm"
	"time"
)

// 朋友圈相关模型

// Moment 动态主表
// 标题 + 媒体（图片最多9张 或 视频1个）
type Moment struct {
	ID          uint64 `gorm:"primarykey"`
	UserID      uint64 `gorm:"index;not null"`                  // 发布者
	Title       string `gorm:"size:200"`                        // 标题
	MediaType   uint8  `gorm:"type:tinyint;not null;default:1"` // 1-图片 2-视频
	ImagesCount uint8  `gorm:"type:tinyint;default:0"`          // 图片数量
	CommentsCnt uint64 `gorm:"default:0"`                       // 评论数量（冗余）
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`

	User   User          `gorm:"foreignKey:UserID"`
	Medias []MomentMedia `gorm:"foreignKey:MomentID"`
}

func (Moment) TableName() string { return prefix + "moment" }

// MomentMedia 动态媒体表
// 存储图片或视频地址；视频时通常一条记录，图片时最多9条
type MomentMedia struct {
	ID        uint64 `gorm:"primarykey"`
	MomentID  uint64 `gorm:"index;not null"`
	Type      uint8  `gorm:"type:tinyint;not null;default:1"` // 1-图片 2-视频
	URL       string `gorm:"size:1000;not null"`
	ThumbURL  string `gorm:"size:1000"` // 可选缩略图
	SortOrder int    `gorm:"default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (MomentMedia) TableName() string { return prefix + "moment_media" }

// MomentComment 动态评论表
// 支持二级评论（通过 ParentID 指向父评论）
type MomentComment struct {
	ID        uint64  `gorm:"primarykey"`
	MomentID  uint64  `gorm:"index;not null"`
	UserID    uint64  `gorm:"index;not null"`
	ParentID  *uint64 `gorm:"index"` // nil 为顶级评论
	Content   string  `gorm:"type:text;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User   User   `gorm:"foreignKey:UserID"`
	Moment Moment `gorm:"foreignKey:MomentID"`
}

func (MomentComment) TableName() string { return prefix + "moment_comment" }
