package chat_sdk

import (
	"log"
	"net/http"
	"sync"

	"github.com/cydxin/chat-sdk/middleware"
	model "github.com/cydxin/chat-sdk/models"
	"github.com/cydxin/chat-sdk/service"
	"github.com/gin-gonic/gin"
)

type ChatEngine struct {
	config *Config

	UserService         *service.UserService
	RoomService         *service.RoomService
	MsgService          *service.MessageService
	MemberService       *service.MemberService
	AuthService         *service.AuthService // 鉴权服务
	MomentService       *service.MomentService
	ConversationService *service.ConversationService
	NotificationService *service.NotificationService
	WsServer            *WsServer
}

var (
	Instance *ChatEngine
	once     sync.Once
)

// NewEngine 创建实例
// 使用选项模式传入配置，Option回调
func NewEngine(opts ...Option) *ChatEngine {
	once.Do(func() {
		c := &Config{
			TablePrefix: "im_", // Default
		}
		for _, opt := range opts {
			opt(c)
		}

		Instance = &ChatEngine{config: c}

		// 初始化 WS
		Instance.WsServer = NewWsServer()
		go Instance.WsServer.Run()

		// 初始化基础 Service，注入 WsNotifier 回调
		baseService := &service.Service{
			DB:          c.DB,
			RDB:         c.RDB,
			TablePrefix: c.TablePrefix,
			WsNotifier:  Instance.WsServer.SendToUser, // 注入 WebSocket 通知函数
			OnlineUserGetter: func(userID uint64) (string, string, bool) {
				Instance.WsServer.mu.RLock()
				sess := Instance.WsServer.sessions[userID]
				Instance.WsServer.mu.RUnlock()
				if sess == nil {
					return "", "", false
				}
				return sess.Nickname, sess.Avatar, true
			},
		}
		// 注入通知服务（统一落库 + WS 推送 + HTTP 拉取）
		baseService.Notify = service.NewNotificationService(baseService)
		// 注入已读回执服务（延迟落库）
		baseService.ReadReceipt = service.NewReadReceiptService(baseService)
		// 注入 WS 会话加载服务（建连时拉取已读游标）
		baseService.SessionBootstrap = service.NewSessionBootstrapService(baseService)

		// 初始化各个 Service
		Instance.UserService = service.NewUserService(baseService)
		Instance.RoomService = service.NewRoomService(baseService)
		Instance.MsgService = service.NewMessageService(baseService)
		Instance.MemberService = service.NewMemberService(baseService)
		Instance.MomentService = service.NewMomentService(baseService)
		Instance.ConversationService = service.NewConversationService(baseService)
		Instance.NotificationService = baseService.Notify
		Instance.AuthService = service.NewAuthService(c.RDB) // 初始化鉴权服务

		// 迁移表
		if err := Instance.AutoMigrate(); err != nil {
			log.Printf("AutoMigrate failed: %v", err)
		}

		// 绑定 WS 回调（实现见 ws_on_message.go）
		Instance.bindWsHandlers()

	})

	return Instance
}

func (c *ChatEngine) AutoMigrate() error {
	db := c.config.DB
	log.Println("AutoMigrate...")
	return db.AutoMigrate(
		&model.User{},
		&model.Room{},
		&model.MessageStatus{},
		&model.Friend{},
		&model.FriendApply{},
		&model.RoomUser{},
		&model.Message{},
		&model.Conversation{},
		&model.Moment{},
		&model.MomentMedia{},
		&model.MomentComment{},
		&model.RoomNotification{},
		&model.RoomNotificationDelivery{},
	)

}

/*
*	提供的HTTP接口在此处，也可以直接自己写controller然后调用service
*	推荐自己写controller，因为这样更灵活
 */

// ServeWS 处理 WebSocket 请求，需要传入 userID 和 name
func (c *ChatEngine) ServeWS(w http.ResponseWriter, r *http.Request, userID uint64, name string) {
	user, err := Instance.UserService.GetUser(userID)
	if err == nil && user != nil {
		c.WsServer.ServeWS(w, r, userID, name, user.Nickname, user.Avatar)
		return
	}
	c.WsServer.ServeWS(w, r, userID, name)
}

// HandleWS 返回 WebSocket 的Handler
func (c *ChatEngine) HandleWS(userID int64, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.WsServer.ServeWS(w, r, uint64(userID), name)
	}
}

// GinAuthMiddleware 返回配置好的 Gin 鉴权中间件
// 使用 ChatEngine 内部的 AuthService 和 Redis 配置
//
// 使用示例:
//
//	engine := chat_sdk.NewEngine(...)
//	r := gin.Default()
//	r.Use(engine.GinAuthMiddleware(nil)) // 使用默认配置
//	// 或自定义配置
//	r.Use(engine.GinAuthMiddleware(&middleware.AuthOptions{
//	    HeaderKey: "X-Token",
//	    QueryKey: "access_token",
//	}))
func (c *ChatEngine) GinAuthMiddleware(opt *middleware.AuthOptions) gin.HandlerFunc {
	return middleware.GinAuthMiddleware(c.AuthService, opt)
}
