package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cydxin/chat-sdk/models"
	"gorm.io/gorm"
)

type MemberService struct {
	*Service
}

func NewMemberService(s *Service) *MemberService {
	log.Println("NewMemberService")
	return &MemberService{Service: s}
}

// SendFriendRequest 发送好友申请
func (s *MemberService) SendFriendRequest(fromUser, toUser uint64, message string) error {
	if fromUser == toUser {
		return fmt.Errorf("不能添加自己为好友")
	}
	log.Println(1)
	// 检查是否已经是好友
	isFriend, _ := s.CheckFriendship(fromUser, toUser)
	if isFriend {
		return fmt.Errorf("已经是好友关系")
	}
	log.Println(2)

	// 检查是否已经发送过申请
	var existingRequest models.FriendApply
	err := s.DB.Model(&models.FriendApply{}).
		Where("from_user_id = ? AND to_user_id = ? AND status = ?", fromUser, toUser, models.StatusPending).
		First(&existingRequest).Error
	log.Println(3)

	if err == nil {
		return fmt.Errorf("已经发送过好友申请，请等待对方回应")
	}

	// 创建好友申请
	request := &models.FriendApply{
		FromUserID: fromUser,
		ToUserID:   toUser,
		Status:     models.StatusPending,
		Reason:     message,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	log.Println(4)

	err = s.DB.Create(request).Error
	log.Println(5)

	if err != nil {
		return err
	}

	// 通知对方
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":       EventFriendRequest,
			"request_id": request.ID,
			"from_user":  fromUser,
			"message":    message,
		}
		notifBytes, _ := json.Marshal(notification)
		s.WsNotifier(toUser, notifBytes)
	}
	log.Println(6)

	return nil
}

// AcceptFriendRequest 同意好友申请
func (s *MemberService) AcceptFriendRequest(requestID uint64, userID uint64) error {
	log.Println(requestID, userID)
	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback() // 确保事务在函数退出时回滚（如果未提交）

	var request models.FriendApply
	err := tx.First(&request, requestID).Error
	if err != nil {
		return err
	}

	// 验证是否是接收者
	if request.ToUserID != userID {
		return fmt.Errorf("无权操作此申请")
	}

	if request.Status != models.StatusPending {
		return fmt.Errorf("该申请已处理")
	}

	// 更新申请状态 (使用乐观锁：Where status = Pending)
	now := time.Now()
	result := tx.Model(&models.FriendApply{}).
		Where("id = ? AND status = ?", requestID, models.StatusPending).
		Updates(map[string]interface{}{
			"status":       models.StatusAgreed,
			"updated_at":   now,
			"processed_at": &now,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("该申请已被处理")
	}

	// 创建好友关系 (双向)
	friends := []models.Friend{
		{
			UserID:    request.FromUserID,
			FriendID:  request.ToUserID,
			Status:    1, // 正常
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			UserID:    request.ToUserID,
			FriendID:  request.FromUserID,
			Status:    1, // 正常
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := tx.Create(&friends).Error; err != nil {
		return err
	}

	// 创建私聊房间（使用规则生成 RoomAccount）
	roomAccount := generatePrivateRoomAccount(request.FromUserID, request.ToUserID)

	// 检查房间是否已存在
	var existingRoom models.Room
	err = tx.Where("room_account = ?", roomAccount).First(&existingRoom).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 如果房间不存在，则创建
	if errors.Is(err, gorm.ErrRecordNotFound) {
		room := &models.Room{
			RoomAccount: roomAccount,
			Type:        1, // 1-私聊
			CreatorID:   request.FromUserID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(room).Error; err != nil {
			return err
		}

		// 添加房间成员
		members := []models.RoomUser{
			{
				RoomID:    room.ID,
				UserID:    request.FromUserID,
				Role:      0,
				JoinTime:  now,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				RoomID:    room.ID,
				UserID:    request.ToUserID,
				Role:      0,
				JoinTime:  now,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		if err := tx.Create(&members).Error; err != nil {
			return err
		}

		// 新建房间时：确保双方会话可见
		for _, uid := range []uint64{request.FromUserID, request.ToUserID} {
			conv := &models.Conversation{UserID: uid, RoomID: room.ID}
			if err := tx.FirstOrCreate(conv, map[string]any{"user_id": uid, "room_id": room.ID}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Conversation{}).
				Where("user_id = ? AND room_id = ?", uid, room.ID).
				Updates(map[string]any{"is_visible": true, "updated_at": now}).Error; err != nil {
				return err
			}
		}
	} else {
		// 房间已存在（通常是删好友后再加回来）：确保双方会话重新展示
		for _, uid := range []uint64{request.FromUserID, request.ToUserID} {
			conv := &models.Conversation{UserID: uid, RoomID: existingRoom.ID}
			if err := tx.FirstOrCreate(conv, map[string]any{"user_id": uid, "room_id": existingRoom.ID}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Conversation{}).
				Where("user_id = ? AND room_id = ?", uid, existingRoom.ID).
				Updates(map[string]any{"is_visible": true, "updated_at": now}).Error; err != nil {
				return err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// 通知申请者
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":       EventFriendAccepted,
			"request_id": requestID,
			"user_id":    userID,
		}
		notifBytes, _ := json.Marshal(notification)
		s.WsNotifier(request.FromUserID, notifBytes)
	}

	return nil
}

// RejectFriendRequest 拒绝好友申请
func (s *MemberService) RejectFriendRequest(requestID uint64, userID uint64) error {
	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	var request models.FriendApply

	if err := tx.First(&request, requestID).Error; err != nil {
		return err
	}

	// 验证是否是接收者
	if request.ToUserID != userID {
		return fmt.Errorf("无权操作此申请")
	}

	if request.Status != models.StatusPending {
		return fmt.Errorf("该申请已处理")
	}

	// 更新申请状态 (使用乐观锁)
	now := time.Now()
	result := tx.Model(&models.FriendApply{}).
		Where("id = ? AND status = ?", requestID, models.StatusPending).
		Updates(map[string]interface{}{
			"status":       models.StatusRefused,
			"updated_at":   now,
			"processed_at": &now,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("该申请已被处理")
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// 通知申请者
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":       EventFriendRejected,
			"request_id": requestID,
			"user_id":    userID,
		}
		notifBytes, _ := json.Marshal(notification)
		s.WsNotifier(request.FromUserID, notifBytes)
	}

	return nil
}

// DeleteFriend 删除好友
func (s *MemberService) DeleteFriend(user1, user2 uint64) error {
	// 以事务保证：删好友 + 隐藏会话 一致
	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	// 1) 删除双向好友关系
	if err := tx.Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", user1, user2, user2, user1).
		Delete(&models.Friend{}).Error; err != nil {
		return err
	}

	// 2) 找到两人的私聊房间，并把对应会话隐藏（仅隐藏这一个房间的会话）
	roomAccount := generatePrivateRoomAccount(user1, user2)
	var room models.Room
	if err := tx.Model(&models.Room{}).
		Select("id").
		Where("room_account = ? AND type = ?", roomAccount, 1).
		First(&room).Error; err == nil {
		if err := tx.Model(&models.Conversation{}).
			Where("room_id = ? AND user_id IN ?", room.ID, []uint64{user1, user2}).
			Updates(map[string]any{"is_visible": false}).Error; err != nil {
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// 通知对方
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":    EventFriendDeleted,
			"user_id": user1,
			"room_id": room.ID,
		}
		notifBytes, _ := json.Marshal(notification)
		s.WsNotifier(user2, notifBytes)

		// 同时通知另一方
		notification["user_id"] = user2
		notifBytes, _ = json.Marshal(notification)
		s.WsNotifier(user1, notifBytes)
	}

	return nil
}

// CheckFriendship 检查是否是好友关系
func (s *MemberService) CheckFriendship(user1, user2 uint64) (bool, error) {
	var count int64
	err := s.DB.Model(&models.Friend{}).
		Where("user_id = ? AND friend_id = ? AND status = ?", user1, user2, 1).
		Count(&count).Error

	return count > 0, err
}

// GetFriendList 获取好友列表
func (s *MemberService) GetFriendList(userID uint64) ([]UserDTO, error) {
	var friends []models.Friend
	err := s.DB.Model(&models.Friend{}).
		Where("user_id = ? AND status = ?", userID, 1).
		Preload("Friend").
		Find(&friends).Error

	if err != nil {
		return nil, err
	}

	dtos := make([]UserDTO, len(friends))
	roomAccounts := make([]string, 0, len(friends))
	accountToIndex := make(map[string]int, len(friends))

	for i, f := range friends {
		dtos[i] = UserDTO{
			ID:           f.Friend.ID,
			UID:          f.Friend.UID,
			Username:     f.Friend.Username,
			Nickname:     f.Friend.Nickname,
			Remark:       f.Remark,
			Avatar:       f.Friend.Avatar,
			Phone:        f.Friend.Phone,
			Email:        f.Friend.Email,
			Gender:       f.Friend.Gender,
			Birthday:     f.Friend.Birthday,
			Signature:    f.Friend.Signature,
			OnlineStatus: f.Friend.OnlineStatus,
			LastLoginAt:  f.Friend.LastLoginAt,
			LastActiveAt: f.Friend.LastActiveAt,
			CreatedAt:    f.Friend.CreatedAt,
			UpdatedAt:    f.Friend.UpdatedAt,
		}

		acc := generatePrivateRoomAccount(userID, f.Friend.ID)
		roomAccounts = append(roomAccounts, acc)
		accountToIndex[acc] = i
	}

	// 批量查询私聊房间
	if len(roomAccounts) > 0 {
		var rooms []models.Room
		_ = s.DB.Model(&models.Room{}).
			Select("id, room_account").
			Where("room_account IN ?", roomAccounts).
			Find(&rooms).Error

		for _, r := range rooms {
			if idx, ok := accountToIndex[r.RoomAccount]; ok {
				dtos[idx].RoomID = r.ID
				dtos[idx].RoomAccount = r.RoomAccount
			}
		}
	}

	return dtos, nil
}

// UserBasicDTO 用户基本信息DTO
type UserBasicDTO struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// FriendApplyDTO 好友申请DTO
type FriendApplyDTO struct {
	ID        uint64       `json:"id"`
	FromUser  UserBasicDTO `json:"from_user"`
	Reason    string       `json:"reason"`
	Status    uint8        `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
}

// GetPendingRequests 获取全部的好友申请
func (s *MemberService) GetPendingRequests(userID uint64) ([]FriendApplyDTO, error) {
	var requests []models.FriendApply
	err := s.DB.Model(&models.FriendApply{}).
		Where("to_user_id = ?", userID).
		Preload("FromUser").
		Order("created_at DESC").
		Find(&requests).Error

	if err != nil {
		return nil, err
	}

	dtos := make([]FriendApplyDTO, len(requests))
	for i, r := range requests {
		dtos[i] = FriendApplyDTO{
			ID: r.ID,
			FromUser: UserBasicDTO{
				ID:       r.FromUser.ID,
				Username: r.FromUser.Username,
				Nickname: r.FromUser.Nickname,
				Avatar:   r.FromUser.Avatar,
			},

			Reason:    r.Reason,
			Status:    r.Status,
			CreatedAt: r.CreatedAt,
		}
	}
	return dtos, nil
}

// SearchUsers 搜索用户：按 username/nickname/uid 模糊匹配，排除自己，返回匹配的 userID 列表。
func (s *MemberService) SearchUsers(keyword string, currentUserID int64, limit int) ([]UserBasicDTO, error) {
	keyword = strings.TrimSpace(keyword)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := s.DB.Model(&models.User{})
	if currentUserID > 0 {
		q = q.Where("id <> ?", currentUserID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR nickname LIKE ? OR uid LIKE ?", like, like, like)
	}

	var users []models.User
	err := q.Select("id, username, nickname, avatar").
		Order("id DESC").
		Limit(limit).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	out := make([]UserBasicDTO, 0, len(users))
	for i := range users {
		u := users[i]
		out = append(out, UserBasicDTO{ID: u.ID, Username: u.Username, Nickname: u.Nickname, Avatar: u.Avatar})
	}
	return out, nil
}

// -------------------- 好友备注（Friend Remark） --------------------

// SetFriendRemark 设置好友备注（user -> friend 的单向备注）
func (s *MemberService) SetFriendRemark(userID, friendID uint64, remark string) error {
	remark = strings.TrimSpace(remark)
	// 允许清空

	res := s.DB.Model(&models.Friend{}).
		Where("user_id = ? AND friend_id = ? AND status = ?", userID, friendID, 1).
		Updates(map[string]any{"remark": remark, "updated_at": time.Now()})

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("not friends")
	}
	return nil
}

// AddRoomMember 添加成员到房间（群聊）
func (s *MemberService) AddRoomMember(roomID uint64, userIDs []uint64, operatorID uint64) error {
	// 基本校验
	if roomID == 0 {
		return fmt.Errorf("room_id is required")
	}
	if operatorID == 0 {
		return fmt.Errorf("operator_id is required")
	}
	if len(userIDs) == 0 {
		return fmt.Errorf("user_ids is required")
	}

	// 检查操作者是否是管理员
	var member models.RoomUser
	err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, operatorID).
		First(&member).Error

	if err != nil {
		return fmt.Errorf("操作者不是房间成员")
	}

	// 假设 Role 1=管理员, 2=群主
	if member.Role < 1 {
		return fmt.Errorf("只有管理员可以添加成员")
	}

	// 去重 + 过滤掉 operator 自己
	uniq := make(map[uint64]struct{}, len(userIDs))
	clean := make([]uint64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid == 0 || uid == operatorID {
			continue
		}
		if _, ok := uniq[uid]; ok {
			continue
		}
		uniq[uid] = struct{}{}
		clean = append(clean, uid)
	}
	if len(clean) == 0 {
		return fmt.Errorf("no valid user_ids")
	}

	// 查询已存在的成员，避免唯一索引冲突
	var existingIDs []uint64
	if err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id IN ?", roomID, clean).
		Pluck("user_id", &existingIDs).Error; err != nil {
		return err
	}
	existingSet := make(map[uint64]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existingSet[id] = struct{}{}
	}

	toAdd := make([]uint64, 0, len(clean))
	toAddUserInfo := make([]map[string]interface{}, 0, len(clean))
	for _, uid := range clean {
		if _, ok := existingSet[uid]; ok {
			continue
		}
		toAdd = append(toAdd, uid)
	}
	if len(toAdd) == 0 {
		return fmt.Errorf("用户已经是房间成员")
	}

	now := time.Now()
	rows := make([]models.RoomUser, 0, len(toAdd))

	// 批量获取用户头像/昵称（优先在线缓存，未命中再查库）
	briefMap, err := models.NewUserDAO(s.DB).BatchGetUserBriefsPreferOnline(toAdd, func(userID uint64) (models.UserBrief, bool, error) {
		if s.OnlineUserGetter == nil {
			return models.UserBrief{}, false, nil
		}
		nn, av, ok := s.OnlineUserGetter(userID)
		if !ok {
			return models.UserBrief{}, false, nil
		}
		return models.UserBrief{UserID: userID, Nickname: nn, Avatar: av}, true, nil
	})
	if err != nil {
		return err
	}

	for _, uid := range toAdd {
		b := briefMap[uid]
		toAddUserInfo = append(toAddUserInfo, map[string]interface{}{
			"user_id":  uid,
			"nickname": b.Nickname,
			"avatar":   b.Avatar,
		})
		rows = append(rows, models.RoomUser{
			RoomID:    roomID,
			UserID:    uid,
			Role:      0, // 普通成员
			JoinTime:  now,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	// 批量写入
	if err := s.DB.Create(&rows).Error; err != nil {
		return err
	}

	// 通知（尽力而为：落库 + WS）
	if s.Notify != nil {
		var members []uint64
		_ = s.DB.Model(&models.RoomUser{}).Where("room_id = ?", roomID).Pluck("user_id", &members).Error
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			operatorID,
			EventRoomMemberAdded,
			map[string]any{"user_ids": toAddUserInfo},
			members,
			true,
		)
	}

	return nil
}

// RemoveRoomMember 从房间移除成员
func (s *MemberService) RemoveRoomMember(roomID uint64, userID uint64, operatorID uint64) error {
	// 事务：移除成员 + 隐藏该成员会话
	tx := s.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	// 检查操作者是否是管理员
	var operator models.RoomUser
	err := tx.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, operatorID).
		First(&operator).Error

	if err != nil {
		return fmt.Errorf("操作者不是房间成员")
	}

	if operator.Role < 1 {
		return fmt.Errorf("只有管理员可以移除成员")
	}

	// 删除成员（幂等：如果目标已不在群里，RowsAffected=0 直接返回 nil，不再重复通知）
	res := tx.Where("room_id = ? AND user_id = ?", roomID, userID).
		Delete(&models.RoomUser{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// 目标用户已不在群里（可能已被踢/已退出）
		return nil
	}

	// 隐藏该成员的会话（从消息列表不展示）
	_ = tx.Model(&models.Conversation{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Updates(map[string]any{"is_visible": false, "updated_at": time.Now()}).Error

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// 通知（尽力而为：落库 + WS）
	if s.Notify != nil {
		var members []uint64
		_ = s.DB.Model(&models.RoomUser{}).Where("room_id = ?", roomID).Pluck("user_id", &members).Error
		_, _ = s.Notify.PublishRoomEvent(
			roomID,
			operatorID,
			EventRoomMemberRemoved,
			map[string]any{"user_id": userID},
			members,
			true,
		)
	}

	return nil
}
