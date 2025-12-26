package service

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUserService_UpdatePassword(t *testing.T) {
	gormDB, mock, sqlDB := newMockDB(t)
	defer func() { _ = sqlDB.Close() }()

	us := NewUserService(&Service{DB: gormDB, RDB: nil, TablePrefix: "im_"})

	// 用更宽松的正则，避免不同方言/版本导致的细节差异
	updateRe := regexp.MustCompile("UPDATE `im_user` SET `password`=.*`updated_at`=.* WHERE id = \\?")
	mock.ExpectExec(updateRe.String()).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), uint64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := us.UpdatePassword(1, "newpass"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUserService_SearchUsers(t *testing.T) {
	gormDB, mock, sqlDB := newMockDB(t)
	defer func() { _ = sqlDB.Close() }()

	us := NewUserService(&Service{DB: gormDB, RDB: nil, TablePrefix: "im_"})

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "uid", "username", "nickname", "password", "avatar", "phone", "email", "gender", "birthday", "signature", "online_status", "last_login_at", "last_active_at", "created_at", "updated_at", "deleted_at"}).
		AddRow(uint64(2), "u2", "bob", "Bobby", "hash", "", "", "", 0, nil, "", 0, nil, nil, now, now, nil)

	limit := 10

	// GORM 会生成 LIMIT ?（而不是 LIMIT 10）
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `im_user` WHERE id <> ? AND (username LIKE ? OR nickname LIKE ? OR uid LIKE ?) AND `im_user`.`deleted_at` IS NULL ORDER BY id DESC LIMIT ?")).
		WithArgs(uint64(1), "%bo%", "%bo%", "%bo%", limit).
		WillReturnRows(rows)

	res, err := us.SearchUsers("bo", 1, limit, 0)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1, got %d", len(res))
	}
	if res[0].Username != "bob" {
		t.Fatalf("expected bob, got %s", res[0].Username)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
