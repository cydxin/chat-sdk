package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestMessageBeforeCreate 测试 Message.BeforeCreate 自动生成 MessageID (UUID)
func TestMessageBeforeCreate(t *testing.T) {
	// 创建 mock DB
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open failed: %v", err)
	}

	// 测试用例 1: MessageID 为空时，自动生成 UUID
	t.Run("AutoGenerateUUID", func(t *testing.T) {
		msg := &Message{
			RoomID:   1,
			SenderID: 100,
			Type:     1,
			Content:  "Test message",
		}

		// Mock INSERT 操作
		mock.ExpectExec("INSERT INTO `im_message`").
			WillReturnResult(sqlmock.NewResult(1, 1))

		// 创建消息
		err := db.Create(msg).Error
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// 验证 MessageID 已生成
		if msg.MessageID == "" {
			t.Error("MessageID should be auto-generated, but it's empty")
		}

		// 验证 MessageID 是有效的 UUID
		_, err = uuid.Parse(msg.MessageID)
		if err != nil {
			t.Errorf("MessageID should be a valid UUID, got: %s, error: %v", msg.MessageID, err)
		}

		// 验证所有 mock 期望都被满足
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})

	// 测试用例 2: MessageID 已设置时，不覆盖
	t.Run("PreserveExistingMessageID", func(t *testing.T) {
		customUUID := uuid.New().String()
		msg := &Message{
			MessageID: customUUID,
			RoomID:    1,
			SenderID:  100,
			Type:      1,
			Content:   "Test message",
		}

		// Mock INSERT 操作
		mock.ExpectExec("INSERT INTO `im_message`").
			WillReturnResult(sqlmock.NewResult(1, 1))

		// 创建消息
		err := db.Create(msg).Error
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// 验证 MessageID 未被覆盖
		if msg.MessageID != customUUID {
			t.Errorf("MessageID should be preserved, expected: %s, got: %s", customUUID, msg.MessageID)
		}

		// 验证所有 mock 期望都被满足
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unfulfilled expectations: %v", err)
		}
	})
}

// TestMessageTableName 测试表名生成
func TestMessageTableName(t *testing.T) {
	msg := Message{}
	expected := "im_message"
	if msg.TableName() != expected {
		t.Errorf("TableName() = %s, want %s", msg.TableName(), expected)
	}
}

// TestMessageIDFieldTypes 测试 Message 相关表的 MessageID 字段类型一致性
func TestMessageIDFieldTypes(t *testing.T) {
	// 这个测试确保我们理解了 MessageID 字段的使用
	// Message.ID (uint64) - 内部数据库主键
	// Message.MessageID (string) - 外部 UUID
	// MessageStatus.MessageID (uint64) - 引用 Message.ID
	// Conversation.LastMessageID (uint64) - 引用 Message.ID

	t.Run("MessageFields", func(t *testing.T) {
		msg := Message{}
		
		// 验证字段类型
		var _ uint64 = msg.ID              // ID 应该是 uint64
		var _ string = msg.MessageID       // MessageID 应该是 string (UUID)
		var _ uint64 = msg.RoomID          // RoomID 应该是 uint64
		var _ uint64 = msg.SenderID        // SenderID 应该是 uint64
	})

	t.Run("MessageStatusFields", func(t *testing.T) {
		status := MessageStatus{}
		
		// MessageStatus.MessageID 引用 Message.ID (内部主键)
		var _ uint64 = status.MessageID    // 应该是 uint64
		var _ uint64 = status.UserID       // 应该是 uint64
	})

	t.Run("ConversationFields", func(t *testing.T) {
		conv := Conversation{}
		
		// Conversation.LastMessageID 引用 Message.ID (内部主键)
		if conv.LastMessageID != nil {
			var _ uint64 = *conv.LastMessageID  // 应该是 uint64
		}
	})
}
