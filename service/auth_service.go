package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// AuthService 提供“鉴权核心能力”，供调用方自建中间件/拦截器使用。
// - 解析 token（Bearer 优先，其次 query）
// - 校验 token -> userID（Redis）
// - 注销 token / 注销用户全部 token
//
// Gin 等框架的中间件建议作为单独适配层，内部调用该 service。
type AuthService struct {
	token *TokenService
}

func NewAuthService(rdb *redis.Client) *AuthService {
	return &AuthService{token: NewTokenService(rdb)}
}

// ExtractToken 从 HTTP 请求中提取 token：优先 Authorization: Bearer，其次 query: token。
func (a *AuthService) ExtractToken(r *http.Request) string {
	if r == nil {
		return ""
	}

	// Authorization: Bearer <token>
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	if ah != "" {
		parts := strings.SplitN(ah, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// query: ?token=xxx
	q := r.URL.Query().Get("token")
	return strings.TrimSpace(q)
}

// Authenticate 根据 token 获取 userID。
func (a *AuthService) Authenticate(ctx context.Context, token string) (uint64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, fmt.Errorf("missing token")
	}
	return a.token.GetUserIDByToken(ctx, token)
}

// AuthenticateRequest 从请求里抽 token 并鉴权。
func (a *AuthService) AuthenticateRequest(ctx context.Context, r *http.Request) (uint64, string, error) {
	t := a.ExtractToken(r)
	uid, err := a.Authenticate(ctx, t)
	return uid, t, err
}

// RevokeToken 注销单个 token。
func (a *AuthService) RevokeToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	uid, err := a.token.GetUserIDByToken(ctx, token)
	if err == nil {
		_ = a.token.RemoveUserToken(ctx, uid, token)
	}
	return a.token.RevokeToken(ctx, token)
}

// RevokeAllTokensByUser 注销用户全部 token。
func (a *AuthService) RevokeAllTokensByUser(ctx context.Context, userID uint64) error {
	return a.token.RevokeAllTokensByUser(ctx, userID)
}

// RefreshTokenTTL 对 token 续期（可选能力，用于滑动过期）。
func (a *AuthService) RefreshTokenTTL(ctx context.Context, token string, ttl time.Duration) error {
	return a.token.RefreshTokenTTL(ctx, token, ttl)
}
