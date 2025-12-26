package service

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newMockDB 用 go-sqlmock 创建一个可被 GORM 使用的 *gorm.DB。
// 说明：我们用 mysql dialector 只是为了让 GORM 生成的 SQL/占位符风格稳定（? 占位符），
// 实际不会连接真实 MySQL。
func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()

	sqldb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	// SkipDefaultTransaction: 避免 GORM 默认在每次写操作开启事务，简化 sqlmock 断言
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqldb, SkipInitializeWithVersion: true}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		_ = sqldb.Close()
		t.Fatalf("gorm.Open: %v", err)
	}

	return db, mock, sqldb
}
