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
}

// Table 获取带前缀的表名
func (s *Service) Table(name string) *gorm.DB {
	return s.DB.Table(name)
}
