package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type VerifyCodePurpose string

const (
	VerifyCodePurposeRegister       VerifyCodePurpose = "register"
	VerifyCodePurposeForgotPassword VerifyCodePurpose = "forgot_password"
	VerifyCodePurposeLogin          VerifyCodePurpose = "login"
)

// VerifyCodeService 负责验证码的生成、存储与校验（Redis）。
// 注意：这里不负责“短信/邮件发送”，调用方可自行集成第三方通道。
// 最小实现：生成 6 位数字验证码，写入 Redis，返回 code 便于调用层发送。
//
// Redis Key: im:verify_code:{purpose}:{identifier}
// TTL: 默认 5 分钟
// Cooldown: 默认 60 秒（防刷，可选；这里实现了）
// Cooldown Key: im:verify_code_cd:{purpose}:{identifier}
//
// identifier 统一使用 string（手机号/邮箱），并做 TrimSpace；邮箱会 ToLower。
// purpose 用于区分注册/找回密码等场景，避免串码。
type VerifyCodeService struct {
	rdb *redis.Client

	ttl      time.Duration
	cooldown time.Duration
}

func NewVerifyCodeService(rdb *redis.Client) *VerifyCodeService {
	return &VerifyCodeService{
		rdb:      rdb,
		ttl:      5 * time.Minute,
		cooldown: 60 * time.Second,
	}
}

func (s *VerifyCodeService) ensure() error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("redis client is nil")
	}
	return nil
}

func (s *VerifyCodeService) normalizeIdentifier(identifier string) string {
	id := strings.TrimSpace(identifier)
	if strings.Contains(id, "@") {
		id = strings.ToLower(id)
	}
	return id
}

func (s *VerifyCodeService) codeKey(purpose VerifyCodePurpose, identifier string) string {
	identifier = s.normalizeIdentifier(identifier)
	return fmt.Sprintf("im:verify_code:%s:%s", purpose, identifier)
}

func (s *VerifyCodeService) cooldownKey(purpose VerifyCodePurpose, identifier string) string {
	identifier = s.normalizeIdentifier(identifier)
	return fmt.Sprintf("im:verify_code_cd:%s:%s", purpose, identifier)
}

func (s *VerifyCodeService) generate6Digits() (string, error) {
	upper := big.NewInt(1000000) // 0..999999
	n, err := rand.Int(rand.Reader, upper)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

type SendCodeResult struct {
	TTLSeconds int64  `json:"ttl_seconds"`
	Code       string `json:"code,omitempty"` // 是否返回由上层/调用方决定；这里总是返回，便于集成发送通道与测试
}

// SendCode 生成验证码并写入 Redis。
// 返回 code 供调用方发送短信/邮件。
func (s *VerifyCodeService) SendCode(ctx context.Context, purpose VerifyCodePurpose, identifier string) (*SendCodeResult, error) {
	if err := s.ensure(); err != nil {
		return nil, err
	}
	identifier = s.normalizeIdentifier(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("identifier is required")
	}
	if purpose == "" {
		return nil, fmt.Errorf("purpose is required")
	}

	// cooldown
	cdKey := s.cooldownKey(purpose, identifier)
	ok, err := s.rdb.SetNX(ctx, cdKey, "1", s.cooldown).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		// 仍然视为成功，但提示稍后再试
		ttl, _ := s.rdb.TTL(ctx, cdKey).Result()
		return &SendCodeResult{TTLSeconds: int64(ttl.Seconds()), Code: ""}, nil
	}

	code, err := s.generate6Digits()
	if err != nil {
		return nil, err
	}

	key := s.codeKey(purpose, identifier)
	if err := s.rdb.Set(ctx, key, code, s.ttl).Err(); err != nil {
		return nil, err
	}

	return &SendCodeResult{TTLSeconds: int64(s.ttl.Seconds()), Code: code}, nil
}

// VerifyCode 校验验证码。成功会删除验证码 key（一次性）。
func (s *VerifyCodeService) VerifyCode(ctx context.Context, purpose VerifyCodePurpose, identifier string, code string) (bool, error) {
	if err := s.ensure(); err != nil {
		return false, err
	}
	identifier = s.normalizeIdentifier(identifier)
	code = strings.TrimSpace(code)
	if identifier == "" {
		return false, fmt.Errorf("identifier is required")
	}
	if purpose == "" {
		return false, fmt.Errorf("purpose is required")
	}
	if code == "" {
		return false, fmt.Errorf("输入验证码")
	}

	key := s.codeKey(purpose, identifier)
	val, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if strings.TrimSpace(val) != code {
		return false, nil
	}
	_ = s.rdb.Del(ctx, key).Err()
	return true, nil
}
