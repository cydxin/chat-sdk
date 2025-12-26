package main

import (
	"log"
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
	userAPI := api.Group("/user")
	{
		userAPI.POST("/register", gin.WrapH(engine.HandleUserRegister()))
		userAPI.POST("/login", gin.WrapH(engine.HandleUserLogin()))
		userAPI.GET("/info", gin.WrapH(engine.HandleGetUserInfo()))
		userAPI.POST("/update", gin.WrapH(engine.HandleUpdateUserInfo()))
		userAPI.POST("/avatar", gin.WrapH(engine.HandleUpdateUserAvatar()))
		userAPI.POST("/password", gin.WrapH(engine.HandleUpdateUserPassword()))
		userAPI.GET("/search", gin.WrapH(engine.HandleSearchUsers()))
	}

	// 好友模块
	friendAPI := api.Group("/friend")
	{
		friendAPI.POST("/request", gin.WrapH(engine.HandleSendFriendRequest()))
		friendAPI.POST("/accept", gin.WrapH(engine.HandleAcceptFriendRequest()))
		friendAPI.POST("/reject", gin.WrapH(engine.HandleRejectFriendRequest()))
		friendAPI.DELETE("/delete", gin.WrapH(engine.HandleDeleteFriend()))
		friendAPI.GET("/list", gin.WrapH(engine.HandleGetFriendList()))
		friendAPI.GET("/pending", gin.WrapH(engine.HandleGetPendingRequests()))
	}

	// 房间模块
	roomAPI := api.Group("/room")
	{
		roomAPI.POST("/private", gin.WrapH(engine.HandleCreatePrivateRoom()))
		roomAPI.POST("/group", gin.WrapH(engine.HandleCreateGroupRoom()))
		roomAPI.POST("/member/add", gin.WrapH(engine.HandleAddRoomMember()))
		roomAPI.POST("/member/remove", gin.WrapH(engine.HandleRemoveRoomMember()))
	}

	// 6. 启动服务器
	log.Println("Chat Server 启动在 :8080")
	log.Println("Swagger UI: http://localhost:8080/swagger/index.html")
	log.Println("WebSocket 地址: ws://localhost:8080/ws?user_id=YOUR_USER_ID")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}
