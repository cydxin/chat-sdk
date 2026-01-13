package service

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cydxin/chat-sdk/cons"
	"github.com/cydxin/chat-sdk/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RoomService struct {
	*Service
}

func NewRoomService(s *Service) *RoomService {
	log.Println("NewRoomService")
	return &RoomService{Service: s}
}

// CreatePrivateRoom 确保两个用户之间存在私聊房间（使用规则生成 RoomAccount）
func (s *RoomService) CreatePrivateRoom(user1, user2 uint64) (*models.Room, error) {
	roomAccount := generatePrivateRoomAccount(user1, user2)

	var room models.Room
	err := s.DB.Where("room_account = ?", roomAccount).First(&room).Error
	if err == nil {
		return &room, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return s.createRoom(1, "", user1, []uint64{user1, user2}, &roomAccount)
}

// CreateGroupRoom 创建群聊房间（生成可分享的群号 RoomAccount）
func (s *RoomService) CreateGroupRoom(name string, creator uint64, members []uint64) (*models.Room, error) {
	groupAccount := fmt.Sprintf("group_%s", uuid.New().String()[:8])
	room, err := s.createRoom(2, name, creator, members, &groupAccount)
	if err != nil {
		return nil, err
	}

	// 自动生成群头像（取自己 + 前8个成员）
	cfg := MergeAvatarsConfig{}
	if s.GroupAvatarMergeConfig != nil {
		if !s.GroupAvatarMergeConfig.Enabled {
			return room, nil
		}
		cfg.CanvasSize = s.GroupAvatarMergeConfig.CanvasSize
		cfg.Padding = s.GroupAvatarMergeConfig.Padding
		cfg.Gap = s.GroupAvatarMergeConfig.Gap
		cfg.Timeout = s.GroupAvatarMergeConfig.Timeout
		cfg.OutputDir = s.GroupAvatarMergeConfig.OutputDir
		cfg.URLPrefix = s.GroupAvatarMergeConfig.URLPrefix
	}
	memberIDs := make([]uint64, 0, 9)
	memberIDs = append(memberIDs, creator)
	for _, uid := range members {
		if uid == 0 || uid == creator {
			continue
		}
		memberIDs = append(memberIDs, uid)
		if len(memberIDs) >= 9 {
			break
		}
	}

	// 批量取头像 URL
	var avatars []string
	if len(memberIDs) > 0 {
		_ = s.DB.Model(&models.User{}).
			Where("id IN ?", memberIDs).
			Pluck("avatar", &avatars).Error
	}
	if len(avatars) > 0 {
		if merged, err := MergeMembersAvatar(avatars, cfg); err == nil && merged != nil {
			_ = s.DB.Model(&models.Room{}).Where("id = ?", room.ID).Update("avatar", merged.URL).Error
			room.Avatar = merged.URL
		}
	}

	return room, nil
}

// createRoom 内部创建房间的通用方法
// roomAccount 如果为 nil，则自动生成一个 UUID
func (s *RoomService) createRoom(roomType uint8, name string, creator uint64, members []uint64, roomAccount *string) (*models.Room, error) {
	var generated string
	if roomAccount != nil {
		generated = *roomAccount
	} else {
		generated = uuid.New().String()
	}

	room := &models.Room{
		RoomAccount: generated,
		Type:        roomType,
		Name:        name,
		CreatorID:   creator,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tx := s.DB.Begin()
	defer tx.Rollback()

	if err := tx.Create(room).Error; err != nil {
		return nil, err
	}
	members = append(members, creator)
	// 添加房间成员
	for _, uid := range members {
		member := &models.RoomUser{
			RoomID:    room.ID,
			UserID:    uid,
			Role:      0, // 普通成员
			JoinTime:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if uid == creator {
			member.Role = 2 // 群主
		}
		if err := tx.Create(member).Error; err != nil {
			return nil, err
		}
	}

	// 同步创建会话：确保成员创建房间后会话列表立即可见
	{
		// 去重成员，避免 creator 重复 append 导致插入重复
		seen := make(map[uint64]struct{}, len(members))
		uniq := make([]uint64, 0, len(members))
		for _, uid := range members {
			if uid == 0 {
				continue
			}
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			uniq = append(uniq, uid)
		}

		now := time.Now()
		for _, uid := range uniq {
			conv := &models.Conversation{UserID: uid, RoomID: room.ID}
			if err := tx.FirstOrCreate(conv, map[string]any{"user_id": uid, "room_id": room.ID}).Error; err != nil {
				return nil, err
			}
			if err := tx.Model(&models.Conversation{}).
				Where("user_id = ? AND room_id = ?", uid, room.ID).
				Updates(map[string]any{"is_visible": true, "updated_at": now}).Error; err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return room, nil
}

// GetRoomByAccount 根据对外房间号/群号查询房间
func (s *RoomService) GetRoomByAccount(account string) (*models.Room, error) {
	var room models.Room
	err := s.DB.Where("room_account = ?", account).First(&room).Error
	return &room, err
}

// GetRoomByID 根据对外房间号/群号查询房间
func (s *RoomService) GetRoomByID(account uint64) (*models.Room, error) {
	var room models.Room
	err := s.DB.First(&room, account).Error
	return &room, err
}

// GetRoomMembers 获取房间成员的用户ID列表
func (s *RoomService) GetRoomMembers(roomID uint64) ([]uint64, error) {
	var members []uint64
	err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ?", roomID).
		Pluck("user_id", &members).Error
	return members, err
}

// RoomDTO 房间列表返回结构
type RoomDTO struct {
	ID          uint64      `json:"id"`
	RoomAccount string      `json:"room_account"`
	Name        string      `json:"name"`
	Avatar      string      `json:"avatar"`
	Type        uint8       `json:"type"`
	LastMessage *MessageDTO `json:"last_message,omitempty"`
	UnreadCount int         `json:"unread_count"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// GroupInfoDTO 群基础信息（不含成员列表）
type GroupInfoDTO struct {
	ID          uint64          `json:"id"`
	RoomAccount string          `json:"room_account"`
	Name        string          `json:"name"`
	Avatar      string          `json:"avatar"`
	CreatorID   uint64          `json:"creator_id"`
	Notices     []RoomNoticeDTO `json:"notices,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// GetGroupInfo 获取群基础信息
func (s *RoomService) GetGroupInfo(roomID uint64) (*GroupInfoDTO, error) {
	var room models.Room
	if err := s.DB.Model(&models.Room{}).
		Select("id, room_account, name, avatar, creator_id, created_at, updated_at, type").
		Where("id = ?", roomID).
		First(&room).Error; err != nil {
		return nil, err
	}
	if room.Type != 2 {
		return nil, fmt.Errorf("此群不存在")
	}
	out := &GroupInfoDTO{
		ID:          room.ID,
		RoomAccount: room.RoomAccount,
		Name:        room.Name,
		Avatar:      room.Avatar,
		CreatorID:   room.CreatorID,
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
	}

	// 附带群公告（默认取最近 20 条，置顶优先）
	var notices []models.RoomNotice
	_ = s.DB.Model(&models.RoomNotice{}).
		Where("room_id = ?", roomID).
		Order("is_pinned desc").
		Order("created_at desc").
		Limit(20).
		Find(&notices).Error
	if len(notices) > 0 {
		out.Notices = make([]RoomNoticeDTO, 0, len(notices))
		for i := range notices {
			out.Notices = append(out.Notices, *toRoomNoticeDTO(&notices[i]))
		}
	}
	return out, nil
}

// QuitGroup 退出群聊
func (s *RoomService) QuitGroup(roomID, UID uint64) error {
	// 通知（尽力而为：落库 + WS）
	err := s.DB.Delete(&models.RoomUser{}, "room_id = ? and user_id =? ", roomID, UID).Error
	if err != nil {
		return err
	}
	// 会话也隐藏掉
	s.DB.Model(&models.Conversation{}).Where("room_id = ? and user_id = ?", roomID, UID).Update("is_visible", false)

	if s.Notify != nil {
		var members []uint64
		_ = s.DB.Model(&models.RoomUser{}).Where("room_id = ?", roomID).Pluck("user_id", &members).Error
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			UID,
			cons.EventRoomMemberQuit,
			map[string]any{"user_id": UID},
			members,
			true,
		)
	}

	return nil
}

// GetUserRooms 获取用户参与的所有房间
func (s *RoomService) GetUserRooms(userID uint) ([]RoomDTO, error) {
	var rooms []models.Room
	roomTable := models.Room{}.TableName()
	roomUserTable := models.RoomUser{}.TableName()

	// 1. 查询用户所在的房间
	err := s.DB.Model(&models.Room{}).
		Joins(fmt.Sprintf("JOIN %s ON %s.id = %s.room_id", roomUserTable, roomTable, roomUserTable)).
		Where(fmt.Sprintf("%s.user_id = ?", roomUserTable), userID).
		Find(&rooms).Error

	if err != nil {
		return nil, err
	}

	if len(rooms) == 0 {
		return []RoomDTO{}, nil
	}

	roomIDs := make([]uint64, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}

	// 2. 批量查询每个房间的最新一条消息
	// 使用子查询找到每个房间最大的消息ID
	// SELECT * FROM messages WHERE id IN (SELECT MAX(id) FROM messages WHERE room_id IN (?) GROUP BY room_id)
	var lastMessages []models.Message
	err = s.DB.Where("id IN (?)",
		s.DB.Model(&models.Message{}).Select("MAX(id)").Where("room_id IN ?", roomIDs).Group("room_id"),
	).Find(&lastMessages).Error

	if err != nil {
		// 记录错误但不中断，可能只是没消息
		log.Printf("GetUserRooms fetch last messages error: %v", err)
	}

	lastMsgMap := make(map[uint64]*models.Message)
	for i := range lastMessages {
		lastMsgMap[lastMessages[i].RoomID] = &lastMessages[i]
	}

	// 3. 批量查询私聊对象的头像和昵称
	// 对于私聊房间 (Type=1)，我们需要找到另一个成员的信息
	privateRoomIDs := make([]uint64, 0)
	for _, r := range rooms {
		if r.Type == 1 {
			privateRoomIDs = append(privateRoomIDs, r.ID)
		}
	}

	otherUserMap := make(map[uint64]models.User) // roomID -> User
	if len(privateRoomIDs) > 0 {
		var roomUsers []models.RoomUser
		// 查找这些房间里，user_id != 当前userID 的记录
		err = s.DB.Preload("User").
			Where("room_id IN ? AND user_id != ?", privateRoomIDs, userID).
			Find(&roomUsers).Error
		if err == nil {
			for _, ru := range roomUsers {
				otherUserMap[ru.RoomID] = ru.User
			}
		}
	}

	// 4. 组装 DTO
	dtos := make([]RoomDTO, len(rooms))
	for i, r := range rooms {
		dto := RoomDTO{
			ID:          r.ID,
			RoomAccount: r.RoomAccount,
			Type:        r.Type,
			UpdatedAt:   r.UpdatedAt,
		}

		// 处理头像和名称
		if r.Type == 1 {
			// 私聊：使用对方的头像和昵称
			if otherUser, ok := otherUserMap[r.ID]; ok {
				dto.Name = otherUser.Nickname
				dto.Avatar = otherUser.Avatar
			} else {
				// 兜底，可能对方退出了或者数据异常
				dto.Name = "未知用户"
				dto.Avatar = "" // 默认头像
			}
		} else {
			// 群聊：使用群名称和群头像
			dto.Name = r.Name
			dto.Avatar = r.Avatar
			if dto.Name == "" {
				dto.Name = "群聊" // 默认群名
			}
		}

		// 处理最新消息
		if msg, ok := lastMsgMap[r.ID]; ok {
			dto.LastMessage = ToMessageDTO(msg)
		}

		dtos[i] = dto
	}

	return dtos, nil
}

// GetGroupList 获取用户参与的群聊列表（Type=2）
func (s *RoomService) GetGroupList(userID uint) ([]RoomDTO, error) {
	var rooms []models.Room
	roomTable := models.Room{}.TableName()
	roomUserTable := models.RoomUser{}.TableName()

	// 1. 查询用户所在的群聊房间
	err := s.DB.Model(&models.Room{}).
		Joins(fmt.Sprintf("JOIN %s ON %s.id = %s.room_id", roomUserTable, roomTable, roomUserTable)).
		Where(fmt.Sprintf("%s.user_id = ? AND %s.type = ?", roomUserTable, roomTable), userID, 2).
		Find(&rooms).Error
	if err != nil {
		return nil, err
	}
	if len(rooms) == 0 {
		return []RoomDTO{}, nil
	}

	roomIDs := make([]uint64, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}

	// 2. 批量查询每个房间的最新一条消息
	var lastMessages []models.Message
	err = s.DB.Where("id IN (?)",
		s.DB.Model(&models.Message{}).Select("MAX(id)").Where("room_id IN ?", roomIDs).Group("room_id"),
	).Find(&lastMessages).Error
	if err != nil {
		log.Printf("GetGroupList fetch last messages error: %v", err)
	}

	lastMsgMap := make(map[uint64]*models.Message)
	for i := range lastMessages {
		lastMsgMap[lastMessages[i].RoomID] = &lastMessages[i]
	}

	// 3. 组装 DTO
	dtos := make([]RoomDTO, len(rooms))
	for i, r := range rooms {
		dto := RoomDTO{
			ID:          r.ID,
			RoomAccount: r.RoomAccount,
			Name:        r.Name,
			Avatar:      r.Avatar,
			Type:        r.Type,
			UpdatedAt:   r.UpdatedAt,
		}
		if dto.Name == "" {
			dto.Name = "群聊"
		}
		if msg, ok := lastMsgMap[r.ID]; ok {
			dto.LastMessage = ToMessageDTO(msg)
		}
		dtos[i] = dto
	}

	return dtos, nil
}

// CheckRoomMember 检查用户是否是房间成员
func (s *RoomService) CheckRoomMember(roomID uint, userID uint) (bool, error) {
	var count int64
	err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Count(&count).Error
	return count > 0, err
}

// generatePrivateRoomAccount 生成私聊会话的固定对外号
func generatePrivateRoomAccount(userID1, userID2 uint64) string {
	if userID1 > userID2 {
		userID1, userID2 = userID2, userID1
	}
	return fmt.Sprintf("private_%d_%d", userID1, userID2)
}

// UpdateGroupInfo 更新群聊信息（名称、头像）
func (s *RoomService) UpdateGroupInfo(operatorID, roomID uint64, name, avatar string) error {
	// 检查权限
	role, err := s.getMemberRole(roomID, operatorID)
	if err != nil {
		return err
	}
	if role < 1 { // 0 用户
		return errors.New("permission denied")
	}

	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	if avatar != "" {
		updates["avatar"] = avatar
	}

	if err := s.DB.Model(&models.Room{}).Where("id = ?", roomID).Updates(updates).Error; err != nil {
		return err
	}
	// 发布通知（尽力而为）
	if s.Notify != nil {
		members, _ := s.GetRoomMembers(roomID)
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			operatorID,
			cons.EventRoomGroupInfoUpdated,
			map[string]any{"name": name, "avatar": avatar},
			members,
			true,
		)
	}
	return nil
}

// SetGroupAdmin 设置/取消管理员
func (s *RoomService) SetGroupAdmin(operatorID, roomID, targetUserID uint64, isAdmin bool) error {
	// Check permission: Owner only
	role, err := s.getMemberRole(roomID, operatorID)
	if err != nil {
		return err
	}
	if role != 2 { // Only owner
		return errors.New("permission denied: only owner can set admin")
	}

	newRole := 0
	if isAdmin {
		newRole = 1
	}

	if err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, targetUserID).
		Update("role", newRole).Error; err != nil {
		return err
	}

	if s.Notify != nil {
		members, _ := s.GetRoomMembers(roomID)
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			operatorID,
			cons.EventRoomAdminSet,
			map[string]any{"target_user_id": targetUserID, "is_admin": isAdmin, "role": newRole},
			members,
			true,
		)
	}
	return nil
}

// SetGroupMuteCountdown 设置群禁言（倒计时）
// durationMinutes: 0 means cancel mute
func (s *RoomService) SetGroupMuteCountdown(operatorID, roomID uint64, durationMinutes int) error {
	role, err := s.getMemberRole(roomID, operatorID)
	if err != nil {
		return err
	}
	if role < 1 {
		return errors.New("permission denied")
	}

	updates := map[string]interface{}{
		"is_mute":    false,
		"mute_until": nil,
	}

	if durationMinutes > 0 {
		t := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
		updates["is_mute"] = true
		updates["mute_until"] = &t
	}

	if err := s.DB.Model(&models.Room{}).Where("id = ?", roomID).Updates(updates).Error; err != nil {
		return err
	}
	if s.Notify != nil {
		members, _ := s.GetRoomMembers(roomID)
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			operatorID,
			cons.EventRoomGroupMuteCountdown,
			map[string]any{"duration_minutes": durationMinutes},
			members,
			true,
		)
	}
	return nil
}

// CancelGroupMuteCountdown 取消群禁言（倒计时模式）
func (s *RoomService) CancelGroupMuteCountdown(operatorID, roomID uint64) error {
	return s.SetGroupMuteCountdown(operatorID, roomID, 0)
}

// SetGroupMuteScheduled 设置群禁言（定时）
// startTime: "HH:MM", durationMinutes: duration
func (s *RoomService) SetGroupMuteScheduled(operatorID, roomID uint64, startTime string, durationMinutes int) error {
	role, err := s.getMemberRole(roomID, operatorID)
	if err != nil {
		return err
	}
	if role < 1 {
		return errors.New("permission denied")
	}

	updates := map[string]interface{}{
		"mute_daily_start_time": startTime,
		"mute_daily_duration":   durationMinutes,
	}

	if err := s.DB.Model(&models.Room{}).Where("id = ?", roomID).Updates(updates).Error; err != nil {
		return err
	}
	if s.Notify != nil {
		members, _ := s.GetRoomMembers(roomID)
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			operatorID,
			cons.EventRoomGroupMuteScheduled,
			map[string]any{"start_time": startTime, "duration_minutes": durationMinutes},
			members,
			true,
		)
	}
	return nil
}

// CancelGroupMuteScheduled 取消群定时禁言
func (s *RoomService) CancelGroupMuteScheduled(operatorID, roomID uint64) error {
	return s.SetGroupMuteScheduled(operatorID, roomID, "", 0)
}

// SetUserMute 设置指定用户禁言
func (s *RoomService) SetUserMute(operatorID, roomID, targetUserID uint64, durationMinutes int) error {
	operatorRole, err := s.getMemberRole(roomID, operatorID)
	if err != nil {
		return err
	}
	if operatorRole < 1 {
		return errors.New("permission denied")
	}

	// Check target role (optional: admin cannot mute owner, etc. but for now simple check)
	// Usually admin cannot mute other admins or owner.
	targetRole, err := s.getMemberRole(roomID, targetUserID)
	if err == nil && targetRole >= operatorRole {
		return errors.New("permission denied: cannot mute user with equal or higher role")
	}

	updates := map[string]interface{}{
		"is_muted":    false,
		"muted_until": nil,
	}

	if durationMinutes > 0 {
		t := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
		updates["is_muted"] = true
		updates["muted_until"] = &t
	}

	if err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, targetUserID).
		Updates(updates).Error; err != nil {
		return err
	}

	if s.Notify != nil {
		members, _ := s.GetRoomMembers(roomID)
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			operatorID,
			cons.EventRoomUserMute,
			map[string]any{"target_user_id": targetUserID, "duration_minutes": durationMinutes},
			members,
			true,
		)
	}
	return nil
}

// CancelUserMute 取消指定用户禁言
func (s *RoomService) CancelUserMute(operatorID, roomID, targetUserID uint64) error {
	return s.SetUserMute(operatorID, roomID, targetUserID, 0)
}

// -------------------- 群成员列表（Member List） --------------------

// -------------------- 群昵称（我在群里的昵称） --------------------

// SetMyGroupNickname 设置当前用户在指定群聊里的昵称（room_user.nickname）
// nickname 允许为空：为空表示清空群昵称，显示端会回退到备注/用户昵称/用户名。
func (s *RoomService) SetMyGroupNickname(userID, roomID uint64, nickname string) error {
	// 必须是成员
	var count int64
	if err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("非群成员")
	}

	return s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Updates(map[string]any{"nickname": nickname, "updated_at": time.Now()}).Error
}

// RoomMemberListItemDTO 群成员列表项
// display_name 按优先级：好友备注 > 群昵称 > 用户昵称 > 用户名
type RoomMemberListItemDTO struct {
	UserID      uint64 `json:"user_id"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	Remark      string `json:"remark"`         // 好友备注（当前用户视角）
	GroupNick   string `json:"group_nickname"` // 群昵称（room_user.nickname）
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	Role        uint8  `json:"role"`
	IsMuted     bool   `json:"is_muted"`
}

// GetRoomMemberList 获取房间成员列表（展示名按：备注 > 群昵称 > 昵称 > 用户名）
func (s *RoomService) GetRoomMemberList(roomID uint64, viewerUserID uint64) ([]RoomMemberListItemDTO, error) {
	// 1) 拉出 room_user + user
	var roomUsers []models.RoomUser
	err := s.DB.Preload("User").
		Where("room_id = ?", roomID).
		Find(&roomUsers).Error
	if err != nil {
		return nil, err
	}
	if len(roomUsers) == 0 {
		return []RoomMemberListItemDTO{}, nil
	}

	memberIDs := make([]uint64, 0, len(roomUsers))
	for _, ru := range roomUsers {
		memberIDs = append(memberIDs, ru.UserID)
	}

	// 2) 取 viewer -> member 的好友备注 (friend.remark)
	remarkMap := make(map[uint64]string)
	{
		var friends []models.Friend
		_ = s.DB.Model(&models.Friend{}).
			Select("friend_id, remark").
			Where("user_id = ? AND friend_id IN ? AND status = ?", viewerUserID, memberIDs, 1).
			Find(&friends).Error
		for _, f := range friends {
			if f.Remark != "" {
				remarkMap[f.FriendID] = f.Remark
			}
		}
	}

	// 3) 组装 DTO
	out := make([]RoomMemberListItemDTO, 0, len(roomUsers))
	for _, ru := range roomUsers {
		u := ru.User
		item := RoomMemberListItemDTO{
			UserID:    ru.UserID,
			Username:  u.Username,
			Nickname:  u.Nickname,
			Remark:    remarkMap[ru.UserID],
			GroupNick: ru.Nickname,
			Avatar:    u.Avatar,
			Role:      ru.Role,
			IsMuted:   ru.IsMuted,
		}

		// display_name 优先级：备注 > 群昵称 > 用户昵称 > 用户名
		switch {
		case item.Remark != "":
			item.DisplayName = item.Remark
		case item.GroupNick != "":
			item.DisplayName = item.GroupNick
		case item.Nickname != "":
			item.DisplayName = item.Nickname
		default:
			item.DisplayName = item.Username
		}

		out = append(out, item)
	}

	return out, nil
}

// Helper
func (s *RoomService) getMemberRole(roomID, userID uint64) (int, error) {
	var member models.RoomUser
	err := s.DB.Select("role").Where("room_id = ? AND user_id = ?", roomID, userID).First(&member).Error
	if err != nil {
		return 0, err
	}
	return int(member.Role), nil
}
