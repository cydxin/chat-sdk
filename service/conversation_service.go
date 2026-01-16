package service

import (
	"fmt"
	"log"
	"time"

	"github.com/cydxin/chat-sdk/models"
	"github.com/cydxin/chat-sdk/repository"
)

// ConversationListItemDTO 会话列表项（消息列表）
type ConversationListItemDTO struct {
	ConversationID uint64      `json:"conversation_id"`
	RoomID         uint64      `json:"room_id"`
	UserID         uint64      `json:"user_id"` //私聊时为对方用户ID，群聊时为0
	RoomAccount    string      `json:"room_account"`
	RoomType       uint8       `json:"room_type"` // 1-私聊 2-群聊
	Name           string      `json:"name"`      // 私聊：对方昵称；群聊：群名
	Avatar         string      `json:"avatar"`    // 私聊：对方头像；群聊：群头像
	LastMessage    *MessageDTO `json:"last_message,omitempty"`
	UnreadCount    uint64      `json:"unread_count"`

	// MentionMessageIDs 未读区间内命中的 @ 消息ID（message.type=8）。
	// 数组为空表示该房间当前未读里没有 @。
	MentionMessageIDs []uint64 `json:"mention_message_ids"`

	UpdatedAt int64 `json:"updated_at"` // unix seconds for easy sort/render
}

type ConversationService struct {
	*Service
}

func NewConversationService(s *Service) *ConversationService {
	log.Println("NewConversationService")
	return &ConversationService{Service: s}
}

// GetConversationList 获取当前用户的会话列表（消息列表）
func (s *ConversationService) GetConversationList(userID uint64) ([]ConversationListItemDTO, error) {
	var convs []models.Conversation
	convs, err := repository.NewConversationDAO(s.DB).ListVisibleByUserID(userID)
	if err != nil {
		return nil, err
	}
	if len(convs) == 0 {
		return []ConversationListItemDTO{}, nil
	}

	// 全部 房间ID
	roomIDs := make([]uint64, 0, len(convs))
	// convMap: roomID -> conv
	convMap := make(map[uint64]models.Conversation, len(convs))
	for _, c := range convs {
		roomIDs = append(roomIDs, c.RoomID)
		convMap[c.RoomID] = c
	}

	// rooms
	var rooms []models.Room
	if err := s.DB.Model(&models.Room{}).
		Where("id IN ?", roomIDs).
		Find(&rooms).Error; err != nil {
		return nil, err
	}
	roomMap := make(map[uint64]models.Room, len(rooms))
	privateRoomIDs := make([]uint64, 0)
	// 用于批量查询 last message
	lastMsgIDs := make([]uint64, 0, len(rooms))
	seenMsg := make(map[uint64]struct{}, len(rooms))
	for _, r := range rooms {
		roomMap[r.ID] = r
		if r.Type == 1 {
			privateRoomIDs = append(privateRoomIDs, r.ID)
		}
		if r.LastMessageID != nil && *r.LastMessageID > 0 {
			mid := *r.LastMessageID
			if _, ok := seenMsg[mid]; !ok {
				seenMsg[mid] = struct{}{}
				lastMsgIDs = append(lastMsgIDs, mid)
			}
		}
	}

	// 批量查询最后一条消息（含 sender）
	lastMsgMap := make(map[uint64]*MessageDTO, len(lastMsgIDs)) // key: room_id
	if len(lastMsgIDs) > 0 {
		var msgs []models.Message
		if err := s.DB.Model(&models.Message{}).
			Preload("Sender").
			Where("id IN ?", lastMsgIDs).
			Find(&msgs).Error; err != nil {
			return nil, err
		}
		msgByID := make(map[uint64]models.Message, len(msgs))
		for i := range msgs {
			msgByID[msgs[i].ID] = msgs[i]
		}
		for _, r := range rooms {
			if r.LastMessageID == nil || *r.LastMessageID == 0 {
				continue
			}
			m, ok := msgByID[*r.LastMessageID]
			if !ok {
				continue
			}
			lastMsgMap[r.ID] = ToMessageDTO(&m)
		}
	}

	// 预计算未读数：roomID -> unread
	// 设计：ReadList 只保存“有未读的房间”以及对应 last_read_msg_id。
	// - 命中 ReadList：用 (lastRead, lastMsgID] 统计未读数。
	// - 未命中 ReadList：视为 0（说明该房间没有未读）。
	unreadMap := make(map[uint64]uint64, len(roomIDs))

	sessionReads := map[uint64]uint64{}
	if s.SessionReadGetter != nil {
		if m := s.SessionReadGetter(userID); len(m) > 0 {
			sessionReads = m
		}
	}
	log.Printf("sessionReads: %+v", sessionReads)
	type rng struct {
		roomID    uint64
		lastRead  uint64
		lastMsgID uint64
	}
	ranges := make([]rng, 0, len(roomIDs))
	for _, rid := range roomIDs {
		r, ok := roomMap[rid]
		if !ok {
			unreadMap[rid] = 0
			continue
		}
		if r.LastMessageID == nil || *r.LastMessageID == 0 {
			unreadMap[rid] = 0
			continue
		}
		lastMsgID := *r.LastMessageID

		// 未命中 sessionReads：按你的规则，代表没有未读
		lastRead, ok := sessionReads[rid]
		if !ok {
			unreadMap[rid] = 0
			continue
		}

		if lastRead >= lastMsgID {
			unreadMap[rid] = 0
			continue
		}
		ranges = append(ranges, rng{roomID: rid, lastRead: lastRead, lastMsgID: lastMsgID})
		unreadMap[rid] = 0
	}

	if len(ranges) > 0 {
		q := s.DB.Model(&models.Message{}).
			Select("room_id, COUNT(1) AS cnt")
		for i, rg := range ranges {
			cond := "room_id = ? AND id > ? AND id <= ?"
			args := []any{rg.roomID, rg.lastRead, rg.lastMsgID}
			if i == 0 {
				q = q.Where(cond, args...)
			} else {
				q = q.Or(cond, args...)
			}
		}
		q = q.Group("room_id")

		type row struct {
			RoomID uint64
			Cnt    int64
		}
		var rows []row
		if err := q.Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.Cnt < 0 {
				unreadMap[r.RoomID] = 0
				continue
			}
			unreadMap[r.RoomID] = uint64(r.Cnt)
		}
	}

	// mentionIDsByRoom：roomID -> 未读区间内的 @ 消息ID列表（type=8）
	mentionIDsByRoom := make(map[uint64][]uint64, len(ranges))
	if len(ranges) > 0 {
		qidRanges := make([]repository.RoomIDRange, 0, len(ranges))
		for _, rg := range ranges {
			qidRanges = append(qidRanges, repository.RoomIDRange{
				RoomID:       rg.roomID,
				MinExclusive: rg.lastRead,
				MaxInclusive: rg.lastMsgID,
			})
		}

		rows, err := repository.NewMessageMentionDAO(s.DB).BatchListMentionMessageIDsInRanges(qidRanges)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			mentionIDsByRoom[row.RoomID] = append(mentionIDsByRoom[row.RoomID], row.ID)
		}
	}

	// 其他私人房间用户：Map[roomID]User
	otherUserMap := make(map[uint64]models.User)
	friendRemarkMap := make(map[uint64]string)
	if len(privateRoomIDs) > 0 {
		var roomUsers []models.RoomUser
		// 查找这些私聊房间里，user_id != 当前 userID 的记录
		if err := s.DB.Preload("User").
			Where("room_id IN ? AND user_id <> ?", privateRoomIDs, userID).
			Find(&roomUsers).Error; err == nil {
			for _, ru := range roomUsers {
				otherUserMap[ru.RoomID] = ru.User
			}
		}

		// 取出对方 user_id 列表，用于查 remark
		otherIDs := make([]uint64, 0, len(roomUsers))
		roomToOtherID := make(map[uint64]uint64)
		for _, ru := range roomUsers {
			otherIDs = append(otherIDs, ru.UserID)
			roomToOtherID[ru.RoomID] = ru.UserID
		}
		if len(otherIDs) > 0 {
			var friends []models.Friend
			_ = s.DB.Model(&models.Friend{}).
				Select("friend_id, remark").
				Where("user_id = ? AND friend_id IN ? AND status = ?", userID, otherIDs, 1).
				Find(&friends).Error
			remarkByFriendID := make(map[uint64]string, len(friends))
			for _, f := range friends {
				if f.Remark != "" {
					remarkByFriendID[f.FriendID] = f.Remark
				}
			}
			for roomID, otherID := range roomToOtherID {
				if rmk, ok := remarkByFriendID[otherID]; ok {
					friendRemarkMap[roomID] = rmk
				}
			}
		}
	}

	// 用户的群昵称
	groupNicknameMap := make(map[uint64]string)
	{
		var rows []models.RoomUser
		_ = s.DB.Model(&models.RoomUser{}).
			Select("room_id, nickname").
			Where("user_id = ? AND room_id IN ?", userID, roomIDs).
			Find(&rows).Error
		for _, ru := range rows {
			if ru.Nickname != "" {
				groupNicknameMap[ru.RoomID] = ru.Nickname
			}
		}
	}

	out := make([]ConversationListItemDTO, 0, len(convs))
	for _, c := range convs {
		r, ok := roomMap[c.RoomID]
		if !ok {
			// room 被删了，跳过
			continue
		}
		mids := mentionIDsByRoom[r.ID]
		if mids == nil {
			mids = []uint64{}
		}

		item := ConversationListItemDTO{
			ConversationID:    c.ID,
			RoomID:            r.ID,
			UserID:            0,
			RoomAccount:       r.RoomAccount,
			RoomType:          r.Type,
			UnreadCount:       unreadMap[r.ID],
			UpdatedAt:         c.UpdatedAt.Unix(),
			LastMessage:       lastMsgMap[r.ID],
			MentionMessageIDs: mids,
		}

		switch r.Type {
		case 1:
			if other, ok := otherUserMap[r.ID]; ok {
				item.UserID = other.ID
				// 优先好友备注
				if rmk, ok := friendRemarkMap[r.ID]; ok {
					item.Name = rmk
				} else if other.Nickname != "" {
					item.Name = other.Nickname
				} else {
					item.Name = other.Username
				}
				item.Avatar = other.Avatar
			} else {
				item.Name = "未知用户"
				item.Avatar = ""
			}
		case 2:
			item.UserID = 0
			item.Name = r.Name
			if nn, ok := groupNicknameMap[r.ID]; ok {
				item.Name = nn
			}
			item.Avatar = r.Avatar
			if item.Name == "" {
				item.Name = "群聊"
			}
		default:
			item.Name = fmt.Sprintf("room_%d", r.ID)
		}

		out = append(out, item)
	}

	return out, nil
}

// EnsureConversationForRoom 确保会话存在（用于首次进入房间或发送消息时创建）
func (s *ConversationService) EnsureConversationForRoom(userID, roomID uint64) error {
	return repository.NewConversationDAO(s.DB).EnsureConversationForRoom(userID, roomID)
}

// SetConversationVisible 设置会话可见
func (s *ConversationService) SetConversationVisible(roomID uint64) error {
	return repository.NewConversationDAO(s.DB).SetRoomConversationVisibleByRoomID(roomID)
}

// SoftDeleteConversation 删除会话：当前实现为 hard delete（删除记录即不展示）；如需保留记录可改为加字段。
func (s *ConversationService) SoftDeleteConversation(userID, roomID uint64) error {
	return repository.NewConversationDAO(s.DB).SoftDeleteConversation(userID, roomID)
}

// UpdateConversationLastMessage 更新会话最后一条消息（只更新当前用户视角）
func (s *ConversationService) UpdateConversationLastMessage(userID, roomID, messageID uint64) error {
	// 仍保留 gorm.ErrRecordNotFound 的语义：DAO 内部对 0 值直接忽略
	return repository.NewConversationDAO(s.DB).UpdateLastMessageIDByUserRoom(userID, roomID, messageID, time.Time{})
}

// EnsureRoomConversationsVisible 确保指定房间在这些用户维度下的会话存在且可见。
// 场景：WS 收到新消息时，把被隐藏/删除（is_visible=false 或记录缺失）的会话重新打开。
func (s *ConversationService) EnsureRoomConversationsVisible(roomID uint64, userIDs []uint64) error {
	return repository.NewConversationDAO(s.DB).EnsureConversationsVisibleBulk(roomID, userIDs, time.Now())
}
