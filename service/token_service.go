package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// 默认 token 过期时间
	defaultTokenTTL = 7 * 24 * time.Hour
)

// TokenService 专门负责 token 的生成、存储、校验与注销。
// Redis Key 设计：
// - im:token:{token} -> userID (String, TTL)
// - im:user_tokens:{userID} -> Set(token1, token2, ...) (Set, 可选 TTL)
//
// 这样可以：
// - 单 token 注销：DEL tokenKey + SREM userSet
// - 全端注销：SMEMBERS userSet 再批量 DEL tokenKey
// - 支持多端登录/多 token
// - 可选做单点登录：登录时先 RevokeAllTokensByUser
type TokenService struct {
	rdb *redis.Client
}

func NewTokenService(rdb *redis.Client) *TokenService {
	return &TokenService{rdb: rdb}
}

func (s *TokenService) ensure() error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("redis client is nil")
	}
	return nil
}

func (s *TokenService) tokenKey(token string) string {
	return "im:token:" + token
}

func (s *TokenService) userTokensKey(userID uint64) string {
	return fmt.Sprintf("im:user_tokens:%d", userID)
}

// GenerateToken 生成一个随机 token（不包含任何用户信息）。
func (s *TokenService) GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// StoreToken 保存 token -> userID 映射，并把 token 加入 user 的 token 集合。
func (s *TokenService) StoreToken(ctx context.Context, token string, userID uint64, ttl time.Duration) error {
	if err := s.ensure(); err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.tokenKey(token), fmt.Sprintf("%d", userID), ttl)
	pipe.SAdd(ctx, s.userTokensKey(userID), token)
	// user token set 的 TTL 不是必须；这里设置为略大于 token TTL，方便自动清理
	pipe.Expire(ctx, s.userTokensKey(userID), ttl+24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// RefreshTokenTTL 对 token 续期（同时延长 user token set TTL）。
func (s *TokenService) RefreshTokenTTL(ctx context.Context, token string, ttl time.Duration) error {
	if err := s.ensure(); err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}

	uid, err := s.GetUserIDByToken(ctx, token)
	if err != nil {
		return err
	}

	pipe := s.rdb.TxPipeline()
	pipe.Expire(ctx, s.tokenKey(token), ttl)
	pipe.Expire(ctx, s.userTokensKey(uid), ttl+24*time.Hour)
	_, err = pipe.Exec(ctx)
	return err
}

// GetUserIDByToken 根据 token 取 userID。
func (s *TokenService) GetUserIDByToken(ctx context.Context, token string) (uint64, error) {
	if err := s.ensure(); err != nil {
		return 0, err
	}
	val, err := s.rdb.Get(ctx, s.tokenKey(token)).Result()
	if err != nil {
		return 0, err
	}
	uid, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, err
	}
	return uid, nil
}

// RevokeToken 注销 token（只删除 tokenKey，不处理 user set；如需两边一起删用 RemoveUserToken + RevokeToken 或 AuthService.RevokeToken）。
func (s *TokenService) RevokeToken(ctx context.Context, token string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.rdb.Del(ctx, s.tokenKey(token)).Err()
}

// AddUserToken 将 token 加入 user 的 token 集合。
func (s *TokenService) AddUserToken(ctx context.Context, userID uint64, token string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.rdb.SAdd(ctx, s.userTokensKey(userID), token).Err()
}

// RemoveUserToken 从 user 的 token 集合中移除 token。
func (s *TokenService) RemoveUserToken(ctx context.Context, userID uint64, token string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.rdb.SRem(ctx, s.userTokensKey(userID), token).Err()
}

// ListUserTokens 列出用户所有 token（用于全端注销）。
func (s *TokenService) ListUserTokens(ctx context.Context, userID uint64) ([]string, error) {
	if err := s.ensure(); err != nil {
		return nil, err
	}
	return s.rdb.SMembers(ctx, s.userTokensKey(userID)).Result()
}

// RevokeAllTokensByUser 注销用户全部 token。
func (s *TokenService) RevokeAllTokensByUser(ctx context.Context, userID uint64) error {
	if err := s.ensure(); err != nil {
		return err
	}
	tokens, err := s.ListUserTokens(ctx, userID)
	if err != nil {
		// 如果 set 不存在，视为没有 token
		if err == redis.Nil {
			return nil
		}
		return err
	}
	if len(tokens) == 0 {
		_ = s.rdb.Del(ctx, s.userTokensKey(userID)).Err()
		return nil
	}

	pipe := s.rdb.TxPipeline()
	for _, t := range tokens {
		pipe.Del(ctx, s.tokenKey(t))
	}
	pipe.Del(ctx, s.userTokensKey(userID))
	_, err = pipe.Exec(ctx)
	return err
}
