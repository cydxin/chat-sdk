package service

import (
	"errors"
	"fmt"
	"log"
	"time"

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
	return s.createRoom(2, name, creator, members, &groupAccount)
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

	// 添加房间成员
	for _, uid := range members {
		member := &models.RoomUser{
			RoomID:    room.ID, // 注意：这里使用的是数字 ID，不是 RoomID 字符串
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

// GetRoomByAccount 根据对外房间号/群号查询房间
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

// GetUserRooms 获取用户参与的所有房间
func (s *RoomService) GetUserRooms(userID uint) ([]models.Room, error) {
	var rooms []models.Room
	err := s.DB.Model(&models.Room{}).
		Joins("JOIN room_users ON rooms.id = room_users.room_id").
		Where("room_users.user_id = ?", userID).
		Find(&rooms).Error

	return rooms, err
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
