package cons

// 统一的房间/群通知事件类型（event_type）
const (
	EventRoomGroupInfoUpdated   = "room.group.info_updated"   // 群信息更新
	EventRoomAdminSet           = "room.admin.set"            // 群管理员设置
	EventRoomGroupMuteCountdown = "room.group.mute.countdown" // 群检测到禁言倒计时结束
	EventRoomGroupMuteScheduled = "room.group.mute.scheduled" // 群定时禁言
	EventRoomUserMute           = "room.user.mute"            // 群用户禁言
	EventRoomMemberAdded        = "room.member.added"         // 群用户添加
	EventRoomMemberRemoved      = "room.member.removed"       // 群用户移除(踢出去)
	EventRoomMemberQuit         = "room.member.quit"          // 群用户退群
	EventRoomNoticeSet          = "room.notice.set"           // 群公告发布/更新
)

// 统一的 用户通知
const (
	EventForward        = "forward"         // 群信息更新
	EventMergeForward   = "merge_forward"   // 群管理员设置
	EventNotification   = "notification"    // 群检测到禁言倒计时结束
	EventFriendDeleted  = "friend_deleted"  // 群定时禁言
	EventRecall         = "recall"          // 消息回撤操作等
	EventFriendRejected = "friend_rejected" // 群用户添加
	EventFriendRequest  = "friend_request"  // 群用户移除(踢出去)
	EventFriendAccepted = "friend_accepted" // 群用户移除(踢出去)
)
