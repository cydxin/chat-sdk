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
	userDao           *models.UserDAO
	tokenService      *TokenService
	verifyCodeService *VerifyCodeService
	loginTokenTTL     time.Duration
}

func NewUserService(s *Service) *UserService {
	log.Println("NewUserService")
	return &UserService{
		Service:           s,
		userDao:           models.NewUserDAO(s.DB),
		tokenService:      NewTokenService(s.RDB),
		verifyCodeService: NewVerifyCodeService(s.RDB),
		loginTokenTTL:     7 * 24 * time.Hour,
	}
}

// --- types ---

type UserDTO struct {
	ID            uint64     `json:"id"`
	UID           string     `json:"uid"`
	Username      string     `json:"username"`
	Nickname      string     `json:"nickname"`
	Remark        string     `json:"remark"`         // 好友备注（仅在好友/私聊场景有意义）
	GroupNickname string     `json:"group_nickname"` // 我在该群里的昵称（群成员/会话列表可用）
	Avatar        string     `json:"avatar"`
	Phone         string     `json:"phone"`
	Email         string     `json:"email"`
	Gender        uint8      `json:"gender"`
	Birthday      *time.Time `json:"birthday"`
	Signature     string     `json:"signature"`
	OnlineStatus  uint8      `json:"online_status"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	LastActiveAt  *time.Time `json:"last_active_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	RoomID        uint64     `json:"room_id"`      // 私聊房间ID（与该好友的会话）
	RoomAccount   string     `json:"room_account"` // 私聊房间对外号（与该好友的会话）
}

type RegisterReq struct {
	Username string `json:"username"`
	Phone    string `json:"phone"` // phone/email 二选一
	Email    string `json:"email"` // phone/email 二选一
	Password string `json:"password"`
	Code     string `json:"code"`
}

type LoginReq struct {
	Account  string `json:"account"`            // username/phone/email
	Password string `json:"password,omitempty"` // plaintext（可选：与 code 二选一）
	Code     string `json:"code,omitempty"`     // 验证码（可选：与 password 二选一）
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

type ForgotPasswordReq struct {
	Identifier  string `json:"identifier"` // phone/email/username(这里允许 username，但更推荐 phone/email)
	NewPassword string `json:"new_password"`
	Code        string `json:"code"`
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

func normalizeEmail(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "@") {
		s = strings.ToLower(s)
	}
	return s
}

func pickIdentifier(phone, email string) (string, error) {
	phone = strings.TrimSpace(phone)
	email = normalizeEmail(email)
	if phone == "" && email == "" {
		return "", fmt.Errorf("phone or email is required")
	}
	if phone != "" && email != "" {
		return "", fmt.Errorf("phone and email cannot both be provided")
	}
	if phone != "" {
		return phone, nil
	}
	return email, nil
}

// Register 注册（验证码校验 + 写库）
func (s *UserService) Register(ctx context.Context, req RegisterReq) error {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		return fmt.Errorf("password is required")
	}
	identifier, err := pickIdentifier(req.Phone, req.Email)
	if err != nil {
		return err
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RDB == nil {
		return fmt.Errorf("redis is not configured")
	}

	ok, err := s.verifyCodeService.VerifyCode(ctx, VerifyCodePurposeRegister, identifier, code)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid verification code")
	}

	exists, err := s.userDao.ExistsByAccount(username, req.Phone, req.Email)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	user := &models.User{
		UID:       uuid.New().String(),
		Username:  username,
		Password:  string(hash),
		Phone:     strings.TrimSpace(req.Phone),
		Email:     normalizeEmail(req.Email),
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

// Register 兼容旧调用：不带 ctx 时使用 Background。
func (s *UserService) RegisterLegacy(req RegisterReq) error {
	return s.Register(context.Background(), req)
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
	password := strings.TrimSpace(req.Password)
	code := strings.TrimSpace(req.Code)
	if password == "" && code == "" {
		return nil, fmt.Errorf("password or code is required")
	}
	if password != "" && code != "" {
		return nil, fmt.Errorf("password and code cannot both be provided")
	}

	u, err := s.userDao.FindByAccount(acc)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("invalid account or password")
		}
		return nil, err
	}

	// 1) 密码登录
	if password != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
			return nil, fmt.Errorf("invalid account or password")
		}
	} else {
		// 2) 验证码登录
		if s.RDB == nil {
			return nil, fmt.Errorf("redis is not configured")
		}
		ok, err := s.verifyCodeService.VerifyCode(ctx, VerifyCodePurposeLogin, acc, code)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("invalid verification code")
		}
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

	if s.RDB == nil {
		resp.Token = ""
		return resp, nil
	}

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

// ForgotPassword 忘记密码（验证码校验后更新密码）
func (s *UserService) ForgotPassword(ctx context.Context, req ForgotPasswordReq) error {
	identifier := normalizeAccount(req.Identifier)
	if identifier == "" {
		return fmt.Errorf("identifier is required")
	}
	newPwd := strings.TrimSpace(req.NewPassword)
	if newPwd == "" {
		return fmt.Errorf("new_password is required")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RDB == nil {
		return fmt.Errorf("redis is not configured")
	}

	u, err := s.userDao.FindByAccount(identifier)
	if err != nil {
		return err
	}

	ok, err := s.verifyCodeService.VerifyCode(ctx, VerifyCodePurposeForgotPassword, identifier, code)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid verification code")
	}

	return s.UpdatePassword(u.ID, newPwd)
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
