package message

type Req struct {
	SendTo      uint64 `json:"send_to"`
	SendType    uint8  `json:"send_type"`
	SendContent string `json:"send_content"`
}
