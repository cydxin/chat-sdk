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

	rows := sqlmock.NewRows([]string{"id"}).AddRow(uint64(2))
	limit := 10

	// SearchUsers(keyword="bo", currentUserID=1, limit=10)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `im_user` WHERE id <> ? AND (username LIKE ? OR nickname LIKE ? OR uid LIKE ?) AND `im_user`.`deleted_at` IS NULL ORDER BY id DESC LIMIT ?")).
		WithArgs(int64(1), "%bo%", "%bo%", "%bo%", limit).
		WillReturnRows(rows)

	ids, err := ms.SearchUsers("bo", 1, limit)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected [2], got %#v", ids)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
