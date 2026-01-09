package message

// WS 上行消息类型
const (
	WsTypeMessage = "message"  // 默认：发送消息
	WsTypeReadAck = "read_ack" // 已读回执（client -> server）
)

// ReadAckReq 已读回执：表示当前用户在某房间已读到某条消息。
// last_read_msg_id 推荐填“当前房间最新 message_id”。
type ReadAckReq struct {
	Type          string `json:"type"`             // read_ack
	RoomID        uint64 `json:"room_id"`          // 房间 ID
	LastReadMsgID uint64 `json:"last_read_msg_id"` // 最后已读消息 ID
	PacketID      string `json:"packet_id"`        // 可选：客户端匹配 ack
}
