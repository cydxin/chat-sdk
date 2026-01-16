package main

import (
	"log"
	"path/filepath"
	"strconv"

	"github.com/cydxin/chat-sdk"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. 初始化数据库连接
	dsn := "root:password@tcp(127.0.0.1:3306)/chat_db?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	// 2. 初始化 Chat Engine（单例模式，全局只需调用一次）
	// 注意：需要配置 Redis 才能使用 Token 认证
	engine := chat_sdk.NewEngine(
		chat_sdk.WithDB(db),
		//chat_sdk.WithRDB(), // 配置 Redis
		chat_sdk.WithTablePrefix("chat_"), // 自定义表前缀

		// 群头像合成（可不配 OutputDir：默认会用“可执行文件所在目录/uploads/auto_avatar”）
		// 如果你想固定到项目目录（开发环境 go run 常用），可以显式配置：
		chat_sdk.WithGroupAvatarMergeConfig(chat_sdk.GroupAvatarMergeConfig{
			Enabled:   true,
			OutputDir: filepath.Join(".", "uploads", "auto_avatar"),
			URLPrefix: "uploads/auto_avatar",
		}),
	)

	// 3. 创建 Gin 路由
	r := gin.Default()

	// 设置 CORS（如果需要）
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 注册 Swagger UI
	chat_sdk.RegisterSwagger(r, "/swagger/*any")

	// 4. WebSocket 连接路由
	// 客户端连接：ws://localhost:8080/ws?user_id=1001
	r.GET("/ws", func(c *gin.Context) {
		userIDStr := c.Query("user_id")
		if userIDStr == "" {
			c.JSON(400, gin.H{"error": "缺少 user_id 参数"})
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			c.JSON(400, gin.H{"error": "user_id 格式错误"})
			return
		}

		name := c.Query("name")
		if name == "" {
			name = "匿名用户"
		}

		// 升级为 WebSocket 连接
		engine.WsServer.ServeWS(c.Writer, c.Request, uint64(userID), name)
	})

	// 5. API 路由组
	api := r.Group("/api/v1")

	// 消息模块
	messageAPI := api.Group("/message")
	{
		messageAPI.GET("/conversations", engine.GinHandleGetMessageConversations)
		messageAPI.POST("/conversation/hide", engine.GinHandleHideConversation)
		messageAPI.GET("/list", engine.GinHandleGetRoomMessages)
		messageAPI.GET("/detail", engine.GinHandleGetMessageByID)
		messageAPI.POST("/recall", engine.GinHandleRecallMessage)
	}

	// 消息模块
	userAPI := api.Group("/user")
	{
		userAPI.POST("/register", engine.GinHandleUserRegister)
		userAPI.POST("/login", engine.GinHandleUserLogin)
		userAPI.POST("/code/send", engine.GinHandleSendVerifyCode)
		userAPI.POST("/password/forgot", engine.GinHandleForgotPassword)
		userAPI.GET("/info", engine.GinHandleGetUserInfo)
		userAPI.POST("/update", engine.GinHandleUpdateUserInfo)
		userAPI.POST("/avatar", engine.GinHandleUpdateUserAvatar)
		userAPI.POST("/password", engine.GinHandleUpdateUserPassword)
		userAPI.GET("/search", engine.GinHandleSearchUsers)
	}

	// 好友模块
	friendAPI := api.Group("/friend")
	{
		friendAPI.POST("/request", engine.GinHandleSendFriendRequest)
		friendAPI.POST("/accept", engine.GinHandleAcceptFriendRequest)
		friendAPI.POST("/reject", engine.GinHandleRejectFriendRequest)
		friendAPI.DELETE("/delete", engine.GinHandleDeleteFriend)
		friendAPI.POST("/remark", engine.GinHandleSetFriendRemark)
		friendAPI.GET("/list", engine.GinHandleGetFriendList)
		friendAPI.GET("/pending", engine.GinHandleGetPendingRequests)
	}

	// 通知模块
	notifyAPI := api.Group("/notification")
	{
		notifyAPI.GET("/list", engine.GinHandleListNotifications)
		notifyAPI.POST("/read", engine.GinHandleMarkNotificationsRead)
	}

	// 房间模块
	roomAPI := api.Group("/room")
	{
		roomAPI.POST("/private", engine.GinHandleCreatePrivateRoom)
		roomAPI.POST("/group", engine.GinHandleCreateGroupRoom)
		roomAPI.GET("/group/info", engine.GinHandleGetGroupInfo)
		roomAPI.GET("/list", engine.GinHandleGetUserRooms)
		roomAPI.GET("/group/list", engine.GinHandleGetGroupRooms)
		roomAPI.GET("/member/list", engine.GinHandleGetRoomMemberList)
		roomAPI.POST("/member/nickname", engine.GinHandleSetMyGroupNickname)
		roomAPI.POST("/member/add", engine.GinHandleAddRoomMember)
		roomAPI.POST("/member/remove", engine.GinHandleRemoveRoomMember)
	}

	// 6. 启动服务器
	log.Println("Chat Server 启动在 :8080")
	log.Println("Swagger UI: http://localhost:8080/swagger/index.html")
	log.Println("WebSocket 地址: ws://localhost:8080/ws?user_id=YOUR_USER_ID")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}
