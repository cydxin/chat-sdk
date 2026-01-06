package service

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMemberService_SearchUsers(t *testing.T) {
	gormDB, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	ms := NewMemberService(&Service{DB: gormDB, TablePrefix: "im_"})

	rows := sqlmock.NewRows([]string{"id", "username", "nickname", "avatar"}).
		AddRow(uint64(2), "bob", "Bobby", "http://avatar")
	limit := 10

	// SearchUsers(keyword="bo", currentUserID=1, limit=10)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, username, nickname, avatar FROM `im_user` WHERE id <> ? AND (username LIKE ? OR nickname LIKE ? OR uid LIKE ?) AND `im_user`.`deleted_at` IS NULL ORDER BY id DESC LIMIT ?")).
		WithArgs(int64(1), "%bo%", "%bo%", "%bo%", limit).
		WillReturnRows(rows)

	users, err := ms.SearchUsers("bo", 1, limit)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(users) != 1 || users[0].ID != 2 {
		t.Fatalf("expected [{ID:2 ...}], got %#v", users)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
