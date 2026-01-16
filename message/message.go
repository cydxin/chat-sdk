package message

type Req struct {
	Type        string `json:"type"`         // WS 消息类型：message/read_ack...
	SendTo      uint64 `json:"send_to"`      // 房间 ID
	SendType    uint8  `json:"send_type"`    // 消息类型 1-文本 2-图片 3-语音 4-视频 5-文件 6-位置 7-引用 8-艾特@ 8-引用的同时@
	SendContent string `json:"send_content"` // 消息内容
	Extra       Extra  `json:"extra"`        // 消息扩展
	PacketID    string `json:"packet_id"`    // 包ID
}

type Extra struct {
	MessageID      uint64        `json:"message_id,omitempty"`      // 被引用的消息 ID
	MessageType    uint8         `json:"message_type,omitempty"`    // 前端使用的类型
	UserID         uint64        `json:"user_id,omitempty"`         // 相关用户 ID
	MessageContent string        `json:"message_content,omitempty"` // 被引用的消息内容
	MentionedUsers []uint64      `json:"mentioned_users,omitempty"` // 被@的用户列表
	Location       *LocationInfo `json:"location,omitempty"`        // 位置信息
	FileInfo       *FileInfo     `json:"file_info,omitempty"`       // 文件信息 用不上 直接文件地址实现
}

type LocationInfo struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Address   string  `json:"address"`
}

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
	Ext  string `json:"ext"`
}
