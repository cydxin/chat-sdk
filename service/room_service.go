package service

import (
	"errors"
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

// CreatePrivateRoom 确保两个用户之间存在私聊房间
func (s *RoomService) CreatePrivateRoom(user1, user2 uint64) (*models.Room, error) {
	var room models.Room

	err := s.DB.Model(&models.Room{}).
		Joins("JOIN room_users u1 ON rooms.id = u1.room_id").
		Joins("JOIN room_users u2 ON rooms.id = u2.room_id").
		Where("u1.user_id = ? AND u2.user_id = ? AND rooms.type = ?", user1, user2, 1).
		First(&room).Error

	if err == nil {
		return &room, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 创建新房间
	return s.createRoom(1, "", user1, []uint64{user1, user2})
}

// CreateGroupRoom 创建群聊房间
func (s *RoomService) CreateGroupRoom(name string, creator uint64, members []uint64) (*models.Room, error) {
	return s.createRoom(2, name, creator, members)
}

func (s *RoomService) createRoom(roomType uint8, name string, creator uint64, members []uint64) (*models.Room, error) {
	room := &models.Room{
		RoomID:    uuid.New().String(),
		Type:      roomType,
		Name:      name,
		CreatorID: creator,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tx := s.DB.Begin()

	if err := tx.Create(room).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	for _, uid := range members {
		member := &models.RoomUser{
			RoomID:    room.ID,
			UserID:    uid,
			Role:      0, // 普通成员
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if uid == creator {
			member.Role = 2 // 群主
		}
		if err := tx.Create(member).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	tx.Commit()
	return room, nil
}

// GetRoomMembers 获取房间成员的用户ID列表
func (s *RoomService) GetRoomMembers(roomID uint64) ([]uint, error) {
	var members []uint
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
