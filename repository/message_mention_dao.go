package repository

import (
	"github.com/cydxin/chat-sdk/models"
	"gorm.io/gorm"
)

// RoomMentionRange 表示某个房间内“未读区间”里，命中的 @ 消息ID 列表。
// 注意：这里的 @ 以 message.type=8（前端 send_type=8）为准。
type RoomMentionRange struct {
	RoomID     uint64
	MentionIDs []uint64
}

type MessageMentionDAO struct {
	db *gorm.DB
}

func NewMessageMentionDAO(db *gorm.DB) *MessageMentionDAO {
	return &MessageMentionDAO{db: db}
}

func (dao *MessageMentionDAO) WithDB(db *gorm.DB) *MessageMentionDAO {
	if db == nil {
		return dao
	}
	return &MessageMentionDAO{db: db}
}

// BatchListMentionMessageIDsInRanges 批量查询“未读区间 (lastRead, lastMsgID]”内 type=8 的消息ID。
//
// 实现策略：
// - 用 (room_id, id区间) 做 OR 组合；
// - 只 select room_id,id 两列；
// - 上层再按 room_id 聚合成 map。
//
// 说明：如果 ranges 很多（比如上千个群），OR 会变长；你当前会话列表通常不会过大，满足需求。
func (dao *MessageMentionDAO) BatchListMentionMessageIDsInRanges(ranges []RoomIDRange) ([]struct {
	RoomID uint64
	ID     uint64
}, error) {
	if len(ranges) == 0 {
		return []struct {
			RoomID uint64
			ID     uint64
		}{}, nil
	}

	q := dao.db.Model(&models.Message{}).
		Select("room_id, id").
		Where("type = ?", 8)

	for i, rg := range ranges {
		if rg.RoomID == 0 || rg.MinExclusive == 0 || rg.MaxInclusive == 0 {
			continue
		}
		if rg.MinExclusive >= rg.MaxInclusive {
			continue
		}
		cond := "room_id = ? AND id > ? AND id <= ?"
		args := []any{rg.RoomID, rg.MinExclusive, rg.MaxInclusive}
		if i == 0 {
			q = q.Where(cond, args...)
		} else {
			q = q.Or(cond, args...)
		}
	}

	var rows []struct {
		RoomID uint64
		ID     uint64
	}
	if err := q.Order("id DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// RoomIDRange 通用的 room+id 区间描述： (MinExclusive, MaxInclusive]
type RoomIDRange struct {
	RoomID       uint64
	MinExclusive uint64
	MaxInclusive uint64
}
