package chat_sdk

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cydxin/chat-sdk/message"
	"github.com/cydxin/chat-sdk/middleware"
	model "github.com/cydxin/chat-sdk/models"
	"github.com/cydxin/chat-sdk/service"
	"github.com/gin-gonic/gin"
)

type ChatEngine struct {
	config        *Config
	UserService   *service.UserService
	RoomService   *service.RoomService
	MsgService    *service.MessageService
	MemberService *service.MemberService
	AuthService   *service.AuthService // 鉴权服务
	WsServer      *WsServer
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

		Instance = &ChatEngine{
			config: c,
		}

		// 初始化 WS
		Instance.WsServer = NewWsServer()
		go Instance.WsServer.Run()

		// 初始化基础 Service，注入 WsNotifier 回调
		baseService := &service.Service{
			DB:          c.DB,
			RDB:         c.RDB,
			TablePrefix: c.TablePrefix,
			WsNotifier:  Instance.WsServer.SendToUser, // 注入 WebSocket 通知函数
		}

		// 初始化各个 Service
		Instance.UserService = service.NewUserService(baseService)
		Instance.RoomService = service.NewRoomService(baseService)
		Instance.MsgService = service.NewMessageService(baseService)
		Instance.MemberService = service.NewMemberService(baseService)
		Instance.AuthService = service.NewAuthService(c.RDB) // 初始化鉴权服务

		// 迁移表
		if err := Instance.AutoMigrate(); err != nil {
			log.Printf("AutoMigrate failed: %v", err)
		}

		//  使用闭包处理消息
		Instance.WsServer.onMessage = func(client *Client, msg []byte) {
			// 固定的参数
			var req message.Req
			if err := json.Unmarshal(msg, &req); err != nil {
				log.Printf("Invalid message format: %v", err)
				return
			}

			var senderID uint64
			senderID = client.UserID
			savedMsg, err := Instance.MsgService.SaveMessage(req.SendTo, senderID, req.SendContent, req.SendType)
			if err != nil {
				log.Printf("Failed to save message: %v", err)
				return
			}

			members, err := Instance.RoomService.GetRoomMembers(req.SendTo)
			if err != nil {
				log.Printf("Failed to get room members: %v", err)
				return
			}

			resp := struct {
				ID        uint64    `json:"id"`
				RoomID    uint64    `json:"room_id"`
				SenderID  uint64    `json:"sender_id"`
				MsgType   uint8     `json:"msg_type"`
				Content   string    `json:"content"`
				CreatedAt time.Time `json:"created_at"`
			}{
				ID:        savedMsg.ID,
				RoomID:    savedMsg.RoomID,
				SenderID:  savedMsg.SenderID,
				MsgType:   savedMsg.Type,
				Content:   savedMsg.Content,
				CreatedAt: savedMsg.CreatedAt,
			}

			respBytes, _ := json.Marshal(resp)

			for _, memberID := range members {
				Instance.WsServer.SendToUser(uint64(memberID), respBytes)
			}
		}

	})

	return Instance
}

func (c *ChatEngine) AutoMigrate() error {
	db := c.config.DB
	log.Println("AutoMigrate...")

	// 需要外键约束，建议配在gorm init的时候，这样的话这里就不用引gorm进来
	return db.AutoMigrate(
		&model.User{},
		&model.Room{},
		&model.MessageStatus{},
		&model.Friend{},
		&model.FriendApply{},
		&model.RoomUser{},
		&model.Message{},
		&model.Conversation{},
	)
}

/*
*	提供的HTTP接口在此处，也可以直接自己写controller然后调用service
*	推荐自己写controller，因为这样更灵活
*
*
*
*
 */

// ServeWS 处理 WebSocket 请求，需要传入 userID 和 name
func (c *ChatEngine) ServeWS(w http.ResponseWriter, r *http.Request, userID uint64, name string) {
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
