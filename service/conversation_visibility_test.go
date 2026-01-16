package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cydxin/chat-sdk/repository"
)

func TestConversationDAO_EnsureConversationsVisibleBulk(t *testing.T) {
	db, mock, sqldb := newMockDB(t)
	defer func() { _ = sqldb.Close() }()

	dao := repository.NewConversationDAO(db)

	roomID := uint64(10)
	userIDs := []uint64{1, 2, 2, 0, 3}
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// GORM FirstOrCreate for MySQL emits:
	// - SELECT * FROM `im_conversation` WHERE (`user_id` = ? AND `room_id` = ?) ORDER BY `im_conversation`.`id` LIMIT ?
	// - if not found: INSERT INTO `im_conversation` (`user_id`,`room_id`,...) VALUES (?,?,...) RETURNING? (mysql uses last insert id)
	// Our DAO loops uniq userIDs and calls FirstOrCreate per user.
	// We don't assert full SQL shape; just assert the key operations happen in order.

	for _, uid := range []uint64{1, 2, 3} {
		mock.ExpectQuery("SELECT \\* FROM `im_conversation` WHERE `im_conversation`\\.`room_id` = \\? AND `im_conversation`\\.`user_id` = \\? ORDER BY `im_conversation`\\.`id` LIMIT \\?").
			WithArgs(roomID, uid, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		mock.ExpectExec("INSERT INTO `im_conversation`").
			WithArgs(uid, roomID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	mock.ExpectExec("UPDATE `im_conversation` SET .* WHERE room_id = \\? AND user_id IN \\(\\?,\\?,\\?\\)").
		WithArgs(true, now, roomID, 1, 2, 3).
		WillReturnResult(sqlmock.NewResult(0, 3))

	if err := dao.EnsureConversationsVisibleBulk(roomID, userIDs, now); err != nil {
		t.Fatalf("EnsureConversationsVisibleBulk err: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}
