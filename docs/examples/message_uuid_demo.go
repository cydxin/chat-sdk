//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"log"

	chat "github.com/cydxin/chat-sdk"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Example: 演示 Message.MessageID 自动生成 UUID 的功能
//
// 从 v1.1+ 版本开始，Message.MessageID 会自动生成 UUID
// 无需手动设置，GORM BeforeCreate hook 会自动处理

func main() {
	// 1. 连接数据库
	dsn := "user:password@tcp(127.0.0.1:3306)/chatdb?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 2. 创建 ChatEngine 实例
	engine := chat.NewEngine(
		chat.WithDB(db),
		chat.WithTablePrefix("im_"),
	)

	// 3. 创建消息（MessageID 会自动生成）
	log.Println("创建消息...")
	msg, err := engine.MsgService.SaveMessage(
		1,              // roomID
		100,            // senderID
		"Hello World!", // content
		1,              // msgType (1=text)
	)
	if err != nil {
		log.Fatalf("Failed to create message: %v", err)
	}

	// 4. 查看生成的 ID
	log.Printf("✅ 消息创建成功！")
	log.Printf("   - 内部数据库 ID: %d (用于数据库内部引用)", msg.ID)
	log.Printf("   - 外部 MessageID: %s (UUID，用于 API 响应)", msg.MessageID)
	log.Printf("   - 房间 ID: %d", msg.RoomID)
	log.Printf("   - 发送者 ID: %d", msg.SenderID)
	log.Printf("   - 内容: %s", msg.Content)

	// 5. 演示：在 API 响应中使用 MessageID
	log.Println("\n在 API 响应中使用:")
	type MessageResponse struct {
		MessageID string `json:"message_id"` // 使用 UUID
		Content   string `json:"content"`
		SenderID  uint64 `json:"sender_id"`
		RoomID    uint64 `json:"room_id"`
	}

	resp := MessageResponse{
		MessageID: msg.MessageID, // 使用外部 UUID
		Content:   msg.Content,
		SenderID:  msg.SenderID,
		RoomID:    msg.RoomID,
	}

	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	log.Printf("API Response:\n%s", string(jsonData))

	// 6. 演示：获取消息列表
	log.Println("\n获取房间消息列表...")
	messages, err := engine.MsgService.GetRoomMessages(1, 10, 0)
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}

	log.Printf("找到 %d 条消息:", len(messages))
	for i, m := range messages {
		log.Printf("  [%d] ID=%d, MessageID=%s, Content=%s",
			i+1, m.ID, m.MessageID, m.Content)
	}

	// 7. 重要说明
	log.Println("\n📝 重要说明:")
	log.Println("   1. Message.ID (uint64): 内部数据库主键，用于数据库内的外键引用")
	log.Println("   2. Message.MessageID (string): 外部 UUID，用于 API 响应和客户端")
	log.Println("   3. MessageStatus.MessageID 引用的是 Message.ID (内部主键)")
	log.Println("   4. Conversation.LastMessageID 引用的也是 Message.ID (内部主键)")
	log.Println("   5. UUID 会在创建消息时自动生成，无需手动设置")
}
