package message

type Req struct {
	SendTo      uint64 `json:"send_to"`      // 房间ID
	SendType    uint8  `json:"send_type"`    // 消息类型
	SendContent string `json:"send_content"` // 消息内容
}
