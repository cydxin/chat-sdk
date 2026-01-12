package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	prefix = "im_"
)

// User 用户表
type User struct {
	ID           uint64     `gorm:"primarykey"`
	UID          string     `gorm:"size:36;uniqueIndex;not null"`      // 对外用户 ID
	Username     string     `gorm:"size:50;uniqueIndex;not null"`      // 用户名
	Nickname     string     `gorm:"size:100;not null"`                 // 昵称
	Password     string     `gorm:"size:255;not null"`                 // 密码
	Avatar       string     `gorm:"size:500"`                          // 头像
	Phone        string     `gorm:"size:20;uniqueIndex;default:null"`  // 手机号
	Email        string     `gorm:"size:100;uniqueIndex;default:null"` // 邮箱
	Gender       uint8      `gorm:"type:tinyint;default:0"`            // 性别: 0-未知 1-男 2-女
	Birthday     *time.Time // 生日
	Signature    string     `gorm:"size:255"`               // 个性签名
	OnlineStatus uint8      `gorm:"type:tinyint;default:0"` // 在线状态: 0-离线 1-在线
	LastLoginAt  *time.Time // 最后登录时间
	LastActiveAt *time.Time // 最后活跃时间
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	// 关联关系
	Friends      []Friend      `gorm:"foreignKey:UserID"`
	FriendApplys []FriendApply `gorm:"foreignKey:FromUserID"`
	Rooms        []RoomUser    `gorm:"foreignKey:UserID"`
	Messages     []Message     `gorm:"foreignKey:SenderID"`
}

func (User) TableName() string {
	return prefix + "user"
}

// 请求状态
const (
	StatusPending = 0
	StatusAgreed  = 1
	StatusRefused = 2
)

// Friend 好友关系表
type Friend struct {
	ID        uint64 `gorm:"primarykey"`
	UserID    uint64 `gorm:"index;not null"`         // 用户 ID
	FriendID  uint64 `gorm:"index;not null"`         // 好友 ID
	Remark    string `gorm:"size:100"`               // 备注
	GroupName string `gorm:"size:50"`                // 分组名
	IsStar    bool   `gorm:"default:false"`          // 是否星标好友
	IsMuted   bool   `gorm:"default:false"`          // 是否免打扰
	Status    uint8  `gorm:"type:tinyint;default:1"` // 状态: 1-正常 2-拉黑
	CreatedAt time.Time
	UpdatedAt time.Time

	// 关联关系
	User   User `gorm:"foreignKey:UserID"`
	Friend User `gorm:"foreignKey:FriendID"`

	// 复合索引
	Indexes []gorm.Index `gorm:"-"`
}

func (f *Friend) TableName() string {
	return prefix + "friend"
}

// FriendApply 好友申请表
type FriendApply struct {
	ID          uint64 `gorm:"primarykey"`
	FromUserID  uint64 `gorm:"not null;index:idx_from" json:"from_user"` // 申请用户 ID
	ToUserID    uint64 `gorm:"not null;index:idx_to" json:"to_user"`     // 目标用户 ID
	Reason      string `gorm:"size:255"`                                 // 申请理由
	Remark      string `gorm:"size:100"`                                 // 备注
	Status      uint8  `gorm:"type:tinyint;index:idx_status;default:0"`  // 状态: 0-待处理 1-同意 2-拒绝
	Reply       string `gorm:"size:255"`                                 // 回复消息
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ProcessedAt *time.Time // 处理时间

	// 关联关系
	FromUser User `gorm:"foreignKey:FromUserID"`
	ToUser   User `gorm:"foreignKey:ToUserID"`
}

func (FriendApply) TableName() string {
	return prefix + "friend_apply"
}

// Room 聊天房间表
type Room struct {
	ID uint64 `gorm:"primarykey"`

	// RoomAccount 对外房间号/群号（可用 10 位数字字符串或自定义规则），用于搜索/分享；
	// 不参与任何外键关联，避免再被 GORM 推断成 bigint。
	RoomAccount string `gorm:"column:room_account;type:varchar(32);uniqueIndex;not null"`

	Name          string  `gorm:"size:100"`               // 房间名称
	Avatar        string  `gorm:"size:500"`               // 房间头像
	Type          uint8   `gorm:"type:tinyint;default:1"` // 类型: 1-私聊 2-群聊
	CreatorID     uint64  `gorm:"index"`                  // 创建者 ID
	Description   string  `gorm:"size:500"`               // 描述
	MemberLimit   int     `gorm:"default:200"`            // 成员上限
	IsEncrypted   bool    `gorm:"default:false"`          // 是否端到端加密
	LastMessageID *uint64 `gorm:"index"`                  // 最后一条消息 ID

	// 新增禁言相关字段
	IsMute             bool       `gorm:"default:false"` // 全员禁言开关
	MuteUntil          *time.Time `gorm:"default:null"`  // 全员禁言截止时间（倒计时模式）
	MuteDailyStartTime string     `gorm:"size:5"`        // 每日禁言开始时间 "HH:MM"
	MuteDailyDuration  int        `gorm:"default:0"`     // 每日禁言持续时长（分钟）

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// 关联关系
	Creator   User       `gorm:"foreignKey:CreatorID"`
	RoomUsers []RoomUser `gorm:"foreignKey:RoomID;references:ID"`
	Messages  []Message  `gorm:"foreignKey:RoomID;references:ID"`
}

func (Room) TableName() string {
	return prefix + "room"
}

// RoomUser 房间成员表
type RoomUser struct {
	ID         uint64     `gorm:"primarykey"`
	RoomID     uint64     `gorm:"index:idx_room_user,unique;not null"` // 房间 ID (对应 Room.ID)
	UserID     uint64     `gorm:"index:idx_room_user,unique;not null"` // 用户 ID
	Role       uint8      `gorm:"type:tinyint;default:0"`              // 角色: 0-普通成员 1-管理员 2-群主
	Nickname   string     `gorm:"size:100"`                            // 在群里的昵称
	IsMuted    bool       `gorm:"default:false"`                       // 是否被禁言
	MutedUntil *time.Time // 禁言截止时间
	JoinSource string     `gorm:"size:50"`                   // 加入来源
	JoinTime   time.Time  `gorm:"default:CURRENT_TIMESTAMP"` // 加入时间
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// 关联关系
	Room Room `gorm:"foreignKey:RoomID;references:ID"`
	User User `gorm:"foreignKey:UserID"`
}

func (RoomUser) TableName() string {
	return prefix + "room_user"
}

// Message 消息表
type Message struct {
	ID uint64 `gorm:"primarykey"`
	//MessageUUID  string         `gorm:"size:36;uniqueIndex;not null"` // 对外消息 ID
	RoomID       uint64         `gorm:"index;not null"`         // 房间 ID (对应 Room.ID)
	SenderID     uint64         `gorm:"index;not null"`         // 发送者 ID
	ReplyToMsgID *uint64        `gorm:"index"`                  // 回复的消息 ID
	Type         uint8          `gorm:"type:tinyint;default:1"` // 消息类型: 1-文本 2-图片 3-语音 4-视频 5-文件 6-位置
	Content      string         `gorm:"type:text;not null"`     // 消息内容
	Extra        datatypes.JSON `gorm:"column:extra;type:json"`
	IsSystem     bool           `gorm:"default:false"`          // 是否为系统消息
	IsEncrypted  bool           `gorm:"default:false"`          // 是否加密
	Status       uint8          `gorm:"type:tinyint;default:0"` // 状态: 0-发送中 1-已发送 2-已送达 3-已读 4-撤回（会在聊天窗口留下痕迹） 5-删除（自己不可见） 6/7-双删（Sender/非Sender删除)在私聊中互相可以删除，但在群中你只能删除自己的，已经管理员进行删除
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	// 关联关系
	Room    Room     `gorm:"foreignKey:RoomID;references:ID"`
	Sender  User     `gorm:"foreignKey:SenderID"`
	ReplyTo *Message `gorm:"foreignKey:ReplyToMsgID"`
}

func (Message) TableName() string {
	return prefix + "message"
}

const (
	MessageStatusSending       = 0 //发送中
	MessageStatusSent          = 1 //已发送
	MessageStatusDelivered     = 2 //已送达
	MessageStatusRead          = 3 //已读
	MessageStatusRecalled      = 4 //撤回
	MessageStatusDeleted       = 5 //删除
	MessageStatusBothDeleted   = 6 //双删
	MessageStatusMangerDeleted = 7 //群主管理员双删
)

// MessageStatus 消息状态表（记录每个用户的已读状态）
type MessageStatus struct {
	ID        uint64     `gorm:"primarykey"`
	MessageID uint64     `gorm:"index:idx_msg_user,unique;not null"` // 消息 ID
	UserID    uint64     `gorm:"index:idx_msg_user,unique;not null"` // 用户 ID
	RoomID    uint64     `gorm:"index:idx_msg_user,unique;not null"` // 房间 ID
	IsRead    bool       `gorm:"default:false"`                      // 是否已读
	IsDeleted bool       `gorm:"default:false"`                      // 是否删除
	ReadAt    *time.Time // 阅读时间
	CreatedAt time.Time
	UpdatedAt time.Time

	// 关联关系
	Message Message `gorm:"foreignKey:MessageID"`
	User    User    `gorm:"foreignKey:UserID"`
}

func (MessageStatus) TableName() string {
	return prefix + "message_status"
}

// Conversation 会话表（每个用户的聊天会话列表）
type Conversation struct {
	ID     uint64 `gorm:"primarykey"`
	UserID uint64 `gorm:"index:idx_user_room,unique;not null"` // 用户 ID
	RoomID uint64 `gorm:"index:idx_user_room,unique;not null"` // 房间 ID (对应 Room.ID)
	//LastMessageID *uint64 `gorm:"index"`                               // 最后一条消息 ID
	//UnreadCount   uint64  `gorm:"default:0"`     // 未读消息数
	IsMuted       bool    `gorm:"default:false"` // 是否免打扰
	IsPinned      bool    `gorm:"default:false"` // 是否置顶
	IsVisible     bool    `gorm:"default:true"`  // 是否在消息列表展示（用户维度）
	LastReadMsgID *uint64 `gorm:"index"`         // 最后阅读的消息 ID
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// 关联关系
	User User `gorm:"foreignKey:UserID"`
	Room Room `gorm:"foreignKey:RoomID;references:ID"`
	//LastMessage *Message `gorm:"foreignKey:LastMessageID"`
	LastReadMsg *Message `gorm:"foreignKey:LastReadMsgID"`
}

func (Conversation) TableName() string {
	return prefix + "conversation"
}
