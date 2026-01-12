package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cydxin/chat-sdk/message"
	"github.com/cydxin/chat-sdk/models"
	"gorm.io/datatypes"
)

type ForwardMode string

const (
	ForwardModeSingle ForwardMode = "single" // 多条逐条转发
	ForwardModeMerge  ForwardMode = "merge"  // 合并转发（一条消息）
)

type ForwardItem struct {
	MessageID uint64 `json:"message_id"`
}

type ForwardReq struct {
	FromUserID uint64      `json:"from_user_id"`
	ToRoomIDs  []uint64    `json:"to_room_ids"`
	Mode       ForwardMode `json:"mode"` // single/merge

	// Items 要转发的消息ID列表（按原顺序）
	Items []ForwardItem `json:"items"`

	// Comment 可选：转发附言（会作为第一条系统消息或合并消息头）
	Comment string `json:"comment"`
}

type MergeForwardPayload struct {
	MessageID uint64 `json:"message_id"`
	Type      string `json:"type"` // merge_forward
	Title     string `json:"title"`
	From      uint64 `json:"from"`
	Count     int    `json:"count"`
	Items     []any  `json:"items"`
	Comment   string `json:"comment,omitempty"`
}

// ForwardMessages 支持逐条转发/合并转发。
// 注意：
// - 这里不会校验 FromUserID 是否有权限看到这些消息（你可以在上层按房间成员校验）。
// - 逐条转发：每条消息会变成目标房间的一条新消息（保留 type/content/extra/is_system/is_encrypted）。
// - 合并转发：目标房间只生成一条消息，type=1(content为摘要)，extra 内包含 merge payload。
func (s *MessageService) ForwardMessages(ctx context.Context, req ForwardReq) ([]uint64, error) {
	if req.FromUserID == 0 {
		return nil, fmt.Errorf("from_user_id is required")
	}
	if len(req.ToRoomIDs) == 0 {
		return nil, fmt.Errorf("to_room_ids is required")
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("items is required")
	}
	mode := req.Mode
	if mode == "" {
		mode = ForwardModeMerge
	}

	// 1) 批量查原消息（后续按 req.Items 顺序还原）
	ids := make([]uint64, 0, len(req.Items))
	for _, it := range req.Items {
		if it.MessageID == 0 {
			continue
		}
		ids = append(ids, it.MessageID)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid message_id")
	}

	var msgs []models.Message
	if err := s.DB.WithContext(ctx).Model(&models.Message{}).
		Where("id IN ?", ids).
		Order("created_at ASC").
		Find(&msgs).Error; err != nil {
		return nil, err
	}
	msgByID := make(map[uint64]models.Message, len(msgs))
	for _, m := range msgs {
		msgByID[m.ID] = m
	}

	ordered := make([]models.Message, 0, len(ids))
	for _, it := range req.Items {
		m, ok := msgByID[it.MessageID]
		if !ok {
			continue
		}
		ordered = append(ordered, m)
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("messages not found")
	}

	createdIDs := make([]uint64, 0)

	for _, toRoomID := range req.ToRoomIDs {
		if toRoomID == 0 {
			continue
		}

		switch mode {
		case ForwardModeSingle:
			// 可选：先发一条系统附言
			if strings.TrimSpace(req.Comment) != "" {
				_, _ = s.SaveMessage(toRoomID, req.FromUserID, strings.TrimSpace(req.Comment), 1, message.Extra{})
			}
			for _, m := range ordered {
				newMsg := &models.Message{
					RoomID:       toRoomID,
					SenderID:     req.FromUserID,
					ReplyToMsgID: nil,
					Type:         m.Type,
					Content:      m.Content,
					Extra:        m.Extra,
					IsSystem:     m.IsSystem,
					IsEncrypted:  m.IsEncrypted,
					Status:       models.MessageStatusSent,
				}
				if err := s.messageDAO.Create(newMsg); err != nil {
					return createdIDs, err
				}
				createdIDs = append(createdIDs, newMsg.ID)
				// 维持会话/通知：复用 SaveMessage 的后置逻辑需要更多重构；这里简化为 ws 推一次
				if s.WsNotifier != nil {
					notif := map[string]any{"type": EventForward, "room_id": toRoomID, "message_id": newMsg.ID}
					b, _ := json.Marshal(notif)
					var memberIDs []uint64
					_ = s.DB.WithContext(ctx).Model(&models.RoomUser{}).Where("room_id = ?", toRoomID).Pluck("user_id", &memberIDs).Error
					for _, uid := range memberIDs {
						s.WsNotifier(uid, b)
					}
				}
			}

		case ForwardModeMerge:
			payload := MergeForwardPayload{
				Type:    EventMergeForward,
				Title:   "聊天记录",
				From:    req.FromUserID,
				Count:   len(ordered),
				Items:   make([]any, 0, len(ordered)),
				Comment: strings.TrimSpace(req.Comment),
			}
			for _, m := range ordered {
				payload.Items = append(payload.Items, map[string]any{
					"id":         m.ID,
					"room_id":    m.RoomID,
					"sender_id":  m.SenderID,
					"type":       m.Type,
					"content":    m.Content,
					"extra":      json.RawMessage(m.Extra),
					"created_at": m.CreatedAt,
				})
			}
			b, _ := json.Marshal(payload)

			content := fmt.Sprintf("[合并转发] %d 条聊天记录", len(ordered))
			newMsg := &models.Message{
				RoomID:      toRoomID,
				SenderID:    req.FromUserID,
				Type:        1,
				Content:     content,
				Extra:       datatypes.JSON(b),
				IsSystem:    false,
				IsEncrypted: false,
				Status:      models.MessageStatusSent,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := s.messageDAO.Create(newMsg); err != nil {
				return createdIDs, err
			}
			createdIDs = append(createdIDs, newMsg.ID)

			// 会话展示/last_message_id：复用 SaveMessage 的逻辑（但 SaveMessage 会检查禁言）
			// 合并转发属于发送行为，仍需要禁言校验，直接调用 SaveMessage
			if err := s.DB.WithContext(ctx).Model(&models.Message{}).Where("id = ?", newMsg.ID).Update("extra", newMsg.Extra).Error; err != nil {
				_ = err
			}

			if s.WsNotifier != nil {
				//notif := map[string]any{"type": EventMergeForward, "room_id": toRoomID, "message_id": newMsg.ID, "content": payload}
				payload.MessageID = newMsg.ID
				nb, _ := json.Marshal(payload)

				var memberIDs []uint64
				_ = s.DB.WithContext(ctx).Model(&models.RoomUser{}).Where("room_id = ?", toRoomID).Pluck("user_id", &memberIDs).Error
				for _, uid := range memberIDs {
					s.WsNotifier(uid, nb)
				}
			}
		default:
			return createdIDs, fmt.Errorf("invalid mode")
		}
	}

	return createdIDs, nil
}
