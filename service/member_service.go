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
			"type":       "friend_request",
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
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// 通知申请者
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":       "friend_accepted",
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
	// 移除 FOR UPDATE
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
			"type":       "friend_rejected",
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
	// 删除双向关系
	err := s.DB.Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", user1, user2, user2, user1).
		Delete(&models.Friend{}).Error

	if err != nil {
		return err
	}

	// 通知对方
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":    "friend_deleted",
			"user_id": user1,
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
func (s *MemberService) GetFriendList(userID uint64) ([]uint64, error) {
	var friends []uint64
	err := s.DB.Model(&models.Friend{}).
		Where("user_id = ? AND status = ?", userID, 1).
		Pluck("friend_id", &friends).Error
	return friends, err
}

// GetPendingRequests 获取待处理的好友申请
func (s *MemberService) GetPendingRequests(userID uint64) ([]models.FriendApply, error) {
	var requests []models.FriendApply
	err := s.DB.Model(&models.FriendApply{}).
		Where("to_user_id = ? AND status = ?", userID, models.StatusPending).
		Preload("FromUser").
		Order("created_at DESC").
		Find(&requests).Error
	return requests, err
}

// SearchUsers 搜索用户：按 username/nickname/uid 模糊匹配，排除自己，返回匹配的 userID 列表。
func (s *MemberService) SearchUsers(keyword string, currentUserID int64, limit int) ([]uint64, error) {
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

	var userIDs []uint64
	err := q.Order("id DESC").Limit(limit).Pluck("id", &userIDs).Error
	return userIDs, err
}

// AddRoomMember 添加成员到房间（群聊）
func (s *MemberService) AddRoomMember(roomID uint64, userID uint64, operatorID uint64) error {
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

	// 检查用户是否已经是成员
	var count int64
	err = s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Count(&count).Error

	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("用户已经是房间成员")
	}

	// 添加成员
	newMember := &models.RoomUser{
		RoomID:    roomID,
		UserID:    userID,
		Role:      0, // 普通成员
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.DB.Create(newMember).Error
	if err != nil {
		return err
	}

	// 通知被添加的用户和房间其他成员
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":        "member_added",
			"room_id":     roomID,
			"user_id":     userID,
			"operator_id": operatorID,
		}
		notifBytes, _ := json.Marshal(notification)

		// 获取所有房间成员
		var members []uint64
		s.DB.Model(&models.RoomUser{}).
			Where("room_id = ?", roomID).
			Pluck("user_id", &members)

		for _, memberID := range members {
			s.WsNotifier(memberID, notifBytes)
		}
	}

	return nil
}

// RemoveRoomMember 从房间移除成员
func (s *MemberService) RemoveRoomMember(roomID uint64, userID uint64, operatorID uint64) error {
	// 检查操作者是否是管理员
	var operator models.RoomUser
	err := s.DB.Model(&models.RoomUser{}).
		Where("room_id = ? AND user_id = ?", roomID, operatorID).
		First(&operator).Error

	if err != nil {
		return fmt.Errorf("操作者不是房间成员")
	}

	if operator.Role < 1 {
		return fmt.Errorf("只有管理员可以移除成员")
	}

	// 删除成员
	err = s.DB.Where("room_id = ? AND user_id = ?", roomID, userID).
		Delete(&models.RoomUser{}).Error

	if err != nil {
		return err
	}

	// 通知被移除的用户和房间其他成员
	if s.WsNotifier != nil {
		notification := map[string]interface{}{
			"type":        "member_removed",
			"room_id":     roomID,
			"user_id":     userID,
			"operator_id": operatorID,
		}
		notifBytes, _ := json.Marshal(notification)

		// 通知被移除的用户
		s.WsNotifier(userID, notifBytes)

		// 获取所有房间成员
		var members []uint64
		s.DB.Model(&models.RoomUser{}).
			Where("room_id = ?", roomID).
			Pluck("user_id", &members)

		for _, memberID := range members {
			s.WsNotifier(memberID, notifBytes)
		}
	}

	return nil
}
