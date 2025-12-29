//go:build ignore
// +build ignore

package main

import (
	"log"

	chat "github.com/cydxin/chat-sdk"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Example: 演示如何运行 Message.MessageID 数据库迁移
//
// 使用场景：
// 当你的数据库中 im_message 表的 message_id 列类型为 bigint，
// 但 Go 代码期望它是 VARCHAR(32) 时，运行此迁移。
//
// ⚠️ 警告：此迁移会清空 message 表的所有数据！
// 在生产环境运行前请务必备份数据。

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

	log.Println("开始检查并迁移 message_id 字段...")

	// 3. 运行迁移
	// 这个函数会：
	// - 检查 message_id 列的类型
	// - 如果是 bigint，则转换为 VARCHAR(32)
	// - 如果已经是 VARCHAR，则跳过迁移
	if err := engine.MigrateMessageIDToUUID(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("✅ 迁移完成！")
	log.Println("现在 message_id 列已经是 VARCHAR(32) 类型")
	log.Println("新创建的消息会自动生成 UUID 作为 MessageID")

	// 4. 测试：创建一条消息验证 UUID 自动生成
	msg, err := engine.MsgService.SaveMessage(1, 100, "测试消息", 1)
	if err != nil {
		log.Fatalf("Failed to create test message: %v", err)
	}

	log.Printf("✅ 测试消息创建成功！")
	log.Printf("   - 内部 ID: %d", msg.ID)
	log.Printf("   - 外部 MessageID (UUID): %s", msg.MessageID)
	log.Printf("   - 内容: %s", msg.Content)
}
