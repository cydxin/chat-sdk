package service

import (
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Service 基础服务，包含数据库和配置
type Service struct {
	DB          *gorm.DB
	RDB         *redis.Client
	TablePrefix string
	// WsNotifier 用于发送 WebSocket 通知的回调函数
	// 避免循环依赖，通过函数注入的方式
	WsNotifier func(userID uint64, message []byte)

	// Notify 通知服务（统一落库 + WS 推送 + HTTP 拉取）
	Notify *NotificationService

	// ReadReceipt 已读回执服务（延迟落库）
	ReadReceipt *ReadReceiptService

	// SessionBootstrap WS 建连时加载会话状态（如已读游标）
	SessionBootstrap *SessionBootstrapService

	// OnlineUserGetter 用于获取在线用户信息（可选）。
	// 只用于读昵称/头像等展示字段，避免 service 层直接引用 WsServer。
	OnlineUserGetter func(userID uint64) (nickname string, avatar string, ok bool)

	// SessionReadGetter 获取用户会话里的已读游标快照（room_id -> last_read_msg_id）。
	// 用于未读数计算/快速恢复，不要求用户当前一定在线。
	SessionReadGetter func(userID uint64) map[uint64]uint64

	// GroupAvatarMergeConfig 群头像合成配置（由 engine 注入，可选）
	GroupAvatarMergeConfig *GroupAvatarMergeConfig
}

// Table 获取带前缀的表名
func (s *Service) Table(name string) *gorm.DB {
	return s.DB.Table(name)
}

// GroupAvatarMergeConfig 群头像合成配置（service 层使用，不依赖 chat_sdk 包）。
type GroupAvatarMergeConfig struct {
	Enabled    bool
	CanvasSize int
	Padding    int
	Gap        int
	Timeout    time.Duration
	OutputDir  string
	URLPrefix  string
}
