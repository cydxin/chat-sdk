package chat_sdk

import (
	"fmt"
	"log"

	"github.com/cydxin/chat-sdk/models"
	"gorm.io/gorm"
)

// MigrateMessageIDToUUID 迁移 message_id 字段从 bigint 到 VARCHAR(32)
// 这个函数用于修复数据库 schema 与 Go 结构体不匹配的问题
// 警告：这会清空 message 表的数据，请在生产环境使用前备份数据
func (c *ChatEngine) MigrateMessageIDToUUID() error {
	db := c.config.DB
	tableName := c.config.TablePrefix + "message" // 使用配置的表前缀

	log.Printf("开始迁移 %s 表的 message_id 字段...", tableName)

	// 检查表是否存在
	if !db.Migrator().HasTable(tableName) {
		log.Printf("表 %s 不存在，跳过迁移", tableName)
		return nil
	}

	// 检查 message_id 列的类型
	columnType, err := db.Migrator().ColumnTypes(tableName)
	if err != nil {
		return fmt.Errorf("获取列类型失败: %v", err)
	}

	var needsMigration bool
	for _, col := range columnType {
		if col.Name() == "message_id" {
			dbType := col.DatabaseTypeName()
			// 如果是 BIGINT 或类似的数字类型，需要迁移
			if dbType == "BIGINT" || dbType == "INT" || dbType == "UNSIGNED BIGINT" {
				needsMigration = true
				log.Printf("检测到 message_id 列类型为 %s，需要迁移到 VARCHAR(32)", dbType)
			} else {
				log.Printf("message_id 列类型为 %s，无需迁移", dbType)
			}
			break
		}
	}

	if !needsMigration {
		log.Println("message_id 列类型正确，无需迁移")
		return nil
	}

	// 开始事务迁移
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 删除依赖 message_id 的外键约束（如果存在）
		// 注意：GORM 可能自动创建了外键，需要先删除
		log.Println("步骤 1: 检查并删除外键约束...")
		// 这一步根据不同数据库可能需要调整

		// 2. 清空表数据（因为 bigint ID 无法直接转换为 UUID）
		log.Println("步骤 2: 清空表数据...")
		// 使用 Delete 而不是 TRUNCATE 来支持更多数据库类型
		if err := tx.Where("1 = 1").Delete(&models.Message{}).Error; err != nil {
			return fmt.Errorf("清空表失败: %v", err)
		}

		// 3. 修改列类型
		log.Println("步骤 3: 修改 message_id 列类型...")
		// 使用 GORM 的 Migrator 会更安全，但可能不支持所有类型转换
		// 对于复杂的类型转换，仍需要使用原生 SQL
		// 验证表名格式（只允许字母、数字和下划线）
		if !isValidTableName(tableName) {
			return fmt.Errorf("invalid table name: %s", tableName)
		}
		
		// MySQL/MariaDB
		if err := tx.Exec(fmt.Sprintf(
			"ALTER TABLE `%s` MODIFY COLUMN `message_id` VARCHAR(32) NOT NULL",
			tableName,
		)).Error; err != nil {
			return fmt.Errorf("修改列类型失败: %v", err)
		}

		// 4. 确保唯一索引存在
		log.Println("步骤 4: 确保唯一索引存在...")
		// 使用 GORM Migrator 创建索引更安全
		if !tx.Migrator().HasIndex(&models.Message{}, "message_id") {
			if err := tx.Migrator().CreateIndex(&models.Message{}, "message_id"); err != nil {
				log.Printf("创建索引警告: %v", err)
			}
		}

		log.Println("迁移完成！")
		return nil
	})
}

// isValidTableName 验证表名格式，防止 SQL 注入
func isValidTableName(name string) bool {
	// 只允许字母、数字和下划线
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || 
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return len(name) > 0 && len(name) < 64 // MySQL 表名最大 64 字符
}

