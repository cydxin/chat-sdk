package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cydxin/chat-sdk/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	*Service
	userDao       *models.UserDAO
	tokenService  *TokenService
	loginTokenTTL time.Duration
}

func NewUserService(s *Service) *UserService {
	log.Println("NewUserService")
	return &UserService{
		Service:       s,
		userDao:       models.NewUserDAO(s.DB),
		tokenService:  NewTokenService(s.RDB),
		loginTokenTTL: 7 * 24 * time.Hour,
	}
}

// --- types ---

type UserDTO struct {
	ID           uint64     `json:"id"`
	UID          string     `json:"uid"`
	Username     string     `json:"username"`
	Nickname     string     `json:"nickname"`
	Avatar       string     `json:"avatar"`
	Phone        string     `json:"phone"`
	Email        string     `json:"email"`
	Gender       uint8      `json:"gender"`
	Birthday     *time.Time `json:"birthday"`
	Signature    string     `json:"signature"`
	OnlineStatus uint8      `json:"online_status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastActiveAt *time.Time `json:"last_active_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	//Nickname string `json:"nickname"`
	//Avatar   string `json:"avatar"`
	//Phone    string `json:"phone"`
	//Email    string `json:"email"`
}

type LoginReq struct {
	Account  string `json:"account"`  // username/phone/email
	Password string `json:"password"` // plaintext
}

type UpdateUserReq struct {
	Nickname  *string    `json:"nickname"`
	Phone     *string    `json:"phone"`
	Email     *string    `json:"email"`
	Gender    *uint8     `json:"gender"`
	Birthday  *time.Time `json:"birthday"`
	Signature *string    `json:"signature"`
}

type UpdatePasswordReq struct {
	NewPassword string `json:"new_password"`
}

type SearchUsersReq struct {
	Keyword string `json:"keyword"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

type LoginResp struct {
	Token string  `json:"token"`
	User  UserDTO `json:"user"`
}

// --- 现 ---

func toUserDTO(u *models.User) *UserDTO {
	if u == nil {
		return nil
	}
	return &UserDTO{
		ID:           u.ID,
		UID:          u.UID,
		Username:     u.Username,
		Nickname:     u.Nickname,
		Avatar:       u.Avatar,
		Phone:        u.Phone,
		Email:        u.Email,
		Gender:       u.Gender,
		Birthday:     u.Birthday,
		Signature:    u.Signature,
		OnlineStatus: u.OnlineStatus,
		LastLoginAt:  u.LastLoginAt,
		LastActiveAt: u.LastActiveAt,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func normalizeAccount(s string) string {
	return strings.TrimSpace(s)
}

// Register 注册（成功不返回用户数据）
func (s *UserService) Register(req RegisterReq) error {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return fmt.Errorf("password is required")
	}

	exists, err := s.userDao.ExistsByUsername(username)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("username already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	user := &models.User{
		UID:       uuid.New().String(),
		Username:  username,
		Password:  string(hash),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if user.Nickname == "" {
		user.Nickname = user.Username
	}

	if err := s.userDao.Create(user); err != nil {
		return err
	}
	return nil
}

// Login 登录（兼容旧接口：只返回用户信息，不返回 token）
func (s *UserService) Login(req LoginReq) (*UserDTO, error) {
	resp, err := s.LoginWithToken(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// LoginWithToken 登录并写 Redis token，返回 token + 用户信息
func (s *UserService) LoginWithToken(ctx context.Context, req LoginReq) (*LoginResp, error) {
	acc := normalizeAccount(req.Account)
	if acc == "" {
		return nil, fmt.Errorf("account is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return nil, fmt.Errorf("password is required")
	}

	var u *models.User
	var err error

	if strings.Contains(acc, "@") {
		u, err = s.userDao.FindByEmail(acc)
	} else {
		u, err = s.userDao.FindByUsername(acc)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			u, err = s.userDao.FindByPhone(acc)
		}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("invalid account or password")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid account or password")
	}

	now := time.Now()
	_ = s.userDao.UpdateFields(u.ID, map[string]any{
		"last_login_at":  &now,
		"last_active_at": &now,
		"online_status":  1,
	})

	fresh, err := s.userDao.FindByID(u.ID)
	if err != nil {
		fresh = u
	}

	resp := &LoginResp{User: *toUserDTO(fresh)}

	// 没有配置 Redis 时：返回用户信息但不发 token（调用层可自行决定是否允许）
	if s.RDB == nil {
		resp.Token = ""
		return resp, nil
	}

	// 生成并存储 token
	token, err := s.tokenService.GenerateToken()
	if err != nil {
		return nil, err
	}
	if err := s.tokenService.StoreToken(ctx, token, fresh.ID, s.loginTokenTTL); err != nil {
		return nil, err
	}
	resp.Token = token
	return resp, nil
}

// GetUser 获取用户信息（脱敏）
func (s *UserService) GetUser(userID uint64) (*UserDTO, error) {
	u, err := s.userDao.FindByID(userID)
	if err != nil {
		return nil, err
	}
	return toUserDTO(u), nil
}

// UpdateAvatar 更新用户头像
func (s *UserService) UpdateAvatar(userID uint64, avatarURL string) (*UserDTO, error) {
	if err := s.userDao.UpdateAvatar(userID, strings.TrimSpace(avatarURL)); err != nil {
		return nil, err
	}
	return s.GetUser(userID)
}

// UpdateUser 更新用户信息
func (s *UserService) UpdateUser(userID uint64, req UpdateUserReq) (*UserDTO, error) {
	updates := make(map[string]any)

	if req.Nickname != nil {
		updates["nickname"] = strings.TrimSpace(*req.Nickname)
	}
	if req.Phone != nil {
		updates["phone"] = strings.TrimSpace(*req.Phone)
	}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.Birthday != nil {
		updates["birthday"] = req.Birthday
	}
	if req.Signature != nil {
		updates["signature"] = strings.TrimSpace(*req.Signature)
	}

	if err := s.userDao.UpdateFields(userID, updates); err != nil {
		return nil, err
	}
	return s.GetUser(userID)
}

// UpdatePassword 更新用户密码（上层自行做验证码/鉴权；这仅负责写库）
func (s *UserService) UpdatePassword(userID uint64, newPassword string) error {
	newPassword = strings.TrimSpace(newPassword)
	if newPassword == "" {
		return fmt.Errorf("new password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.userDao.UpdatePassword(userID, string(hash))
}

// SearchUsers 按关键字搜索用户（username/nickname/uid），返回脱敏数据
func (s *UserService) SearchUsers(keyword string, excludeUserID uint64, limit, offset int) ([]UserDTO, error) {
	users, err := s.userDao.SearchUsers(keyword, excludeUserID, limit, offset)
	if err != nil {
		return nil, err
	}

	out := make([]UserDTO, 0, len(users))
	for i := range users {
		dto := toUserDTO(&users[i])
		if dto != nil {
			out = append(out, *dto)
		}
	}
	return out, nil
}
