package message

type Req struct {
	Type        string `json:"type"`         // WS 消息类型：message/read_ack...
	SendTo      uint64 `json:"send_to"`      // 房间 ID
	SendType    uint8  `json:"send_type"`    // 消息类型
	SendContent string `json:"send_content"` // 消息内容
	PacketID    string `json:"packet_id"`    // 包ID
}
