package service

import (
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
}

// Table 获取带前缀的表名
func (s *Service) Table(name string) *gorm.DB {
	return s.DB.Table(name)
}
