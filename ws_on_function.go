package chat_sdk

import (
	"encoding/json"
	"log"
	"time"

	"github.com/cydxin/chat-sdk/message"
	"github.com/cydxin/chat-sdk/models"
)

// bindWsHandlers 将 WS 回调从 engine.go 抽出来，避免 engine.go 臃肿。
// 说明：放在 chat_sdk 包根目录（同 WsServer/engine.go 同级），
// 这样可以直接访问 Instance 与 Client 类型，避免 service 层循环依赖。
func (c *ChatEngine) bindWsHandlersOnMessage() {
	c.WsServer.onMessage = func(client *Client, msg []byte) {
		// 1) 先尝试解析 type
		var typeProbe struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(msg, &typeProbe)
		// 已读回执
		if typeProbe.Type == message.WsTypeReadAck {
			var ack message.ReadAckReq
			if err := json.Unmarshal(msg, &ack); err != nil {
				return
			}
			if client == nil || ack.RoomID == 0 || ack.LastReadMsgID == 0 {
				return
			}
			// 写入 session.readList（用户级共享内存）
			if client.session != nil {
				client.session.mergeRead(ack.RoomID, ack.LastReadMsgID)
			}
			return
		}

		// 发送消息
		var req message.Req
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Printf("Invalid message format: %v", err)
			return
		}
		if client == nil {
			return
		}

		room, err := Instance.RoomService.GetRoomByID(req.SendTo)
		if err != nil {
			log.Printf("Room not found: %d, error: %v", req.SendTo, err)
			return
		}
		senderID := client.UserID
		// 1) 私聊拉黑校验（基于 friend.status=2）
		if room.Type == 1 {
			blocked, err := isBlockedPrivate(room.ID, senderID)
			if err != nil {
				log.Printf("blocked check failed: %v", err)
				return
			}
			if blocked {
				sendWsError(senderID, "你们已互相拉黑/被对方拉黑，无法发送消息", req.PacketID)
				return
			}
		}
		// 2) 群聊成员存在性校验（防止退群/被踢还继续发）
		if room.Type == 2 {
			ok, err := isRoomMember(room.ID, senderID)
			if err != nil {
				log.Printf("member check failed: %v", err)
				return
			}
			if !ok {
				sendWsError(senderID, "你已不是群成员，无法发送消息", req.PacketID)
				return
			}
		}
		// 3) 保存消息（内部已处理群禁言/个人禁言）
		savedMsg, err := Instance.MsgService.SaveMessage(room.ID, senderID, req.SendContent, req.SendType, req.Extra)
		if err != nil {
			sendWsError(senderID, err.Error(), req.PacketID)
			return
		}

		extraBytes, _ := json.Marshal(req.Extra)
		// 写入session
		if client.session != nil {
			client.session.mergeRead(room.ID, savedMsg.ID)
		}
		members, err := Instance.RoomService.GetRoomMembers(room.ID)
		if err != nil {
			log.Printf("Failed to get room members: %v", err)
			return
		}
		_ = Instance.ConversationService.SetConversationVisible(room.ID)
		resp := struct {
			Type           string          `json:"type"`
			PacketID       string          `json:"packet_id"`
			ID             uint64          `json:"id"`
			RoomID         uint64          `json:"room_id"`
			RoomType       uint8           `json:"room_type"`
			SenderID       uint64          `json:"sender_id"`
			SenderNickname string          `json:"sender_nickname"`
			SenderAvatar   string          `json:"sender_avatar"`
			MsgType        uint8           `json:"msg_type"`
			Content        string          `json:"content"`
			Extra          json.RawMessage `json:"extra,omitempty"`
			CreatedAt      time.Time       `json:"created_at"`
		}{
			Type:      "message",
			PacketID:  req.PacketID,
			ID:        savedMsg.ID,
			RoomID:    room.ID,
			RoomType:  room.Type,
			SenderID:  savedMsg.SenderID,
			MsgType:   savedMsg.Type,
			Content:   savedMsg.Content,
			Extra:     extraBytes,
			CreatedAt: savedMsg.CreatedAt,
		}

		// 建议：无论私聊/群聊都带上 sender 昵称/头像，客户端无需再查。
		resp.SenderNickname = client.Nickname
		resp.SenderAvatar = client.Avatar

		respBytes, _ := json.Marshal(resp)
		for _, memberID := range members {
			Instance.WsServer.SendToUser(memberID, respBytes)
		}
	}
}

func sendWsError(userID uint64, msg string, packetID ...string) {
	if Instance == nil || Instance.WsServer == nil {
		return
	}
	payload := map[string]any{"type": "error", "message": msg, "packet_id": packetID[0]}
	b, _ := json.Marshal(payload)
	Instance.WsServer.SendToUser(userID, b)
}

func isRoomMember(roomID, userID uint64) (bool, error) {
	var count int64
	if err := Instance.MsgService.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// isBlockedPrivate 私聊拉黑校验：只要任意一方 friend.status=2，即视为无法发送。
func isBlockedPrivate(roomID, senderID uint64) (bool, error) {
	// 私聊房间成员只有两人
	var userIDs []uint64
	if err := Instance.MsgService.DB.Model(&models.RoomUser{}).
		Where("room_id = ?", roomID).
		Pluck("user_id", &userIDs).Error; err != nil {
		return false, err
	}
	var peerID uint64
	for _, uid := range userIDs {
		if uid != senderID {
			peerID = uid
			break
		}
	}
	if peerID == 0 {
		return false, nil
	}

	// 任意方向 status=2 都视为拉黑
	var cnt int64
	if err := Instance.MsgService.DB.Model(&models.Friend{}).
		Where("(user_id = ? AND friend_id = ? OR user_id = ? AND friend_id = ?) AND status = ?", senderID, peerID, peerID, senderID, 2).
		Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}
