package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cydxin/chat-sdk/response"
	"github.com/cydxin/chat-sdk/service"
	"github.com/gin-gonic/gin"
)

const (
	// ContextUserIDKey gin context 里保存 user id 的 key
	ContextUserIDKey = "user_id"
	ContextTokenKey  = "token"
)

// AuthOptions 可选配置。
type AuthOptions struct {
	// HeaderKey 默认 Authorization
	HeaderKey string
	// QueryKey 默认 token
	QueryKey string
	// UserIDKey 默认 user_id
	UserIDKey string
	// TokenKey 默认 token
	TokenKey string
}

func (o *AuthOptions) withDefaults() AuthOptions {
	if o == nil {
		return AuthOptions{HeaderKey: "Authorization", QueryKey: "token", UserIDKey: ContextUserIDKey, TokenKey: ContextTokenKey}
	}
	out := *o
	if out.HeaderKey == "" {
		out.HeaderKey = "Authorization"
	}
	if out.QueryKey == "" {
		out.QueryKey = "token"
	}
	if out.UserIDKey == "" {
		out.UserIDKey = ContextUserIDKey
	}
	if out.TokenKey == "" {
		out.TokenKey = ContextTokenKey
	}
	return out
}

/*
	GinAuthMiddleware Gin 鉴权中间件：

- 优先从 Authorization: Bearer <token> 读取
- 如果没有，再从 query 参数读取（默认 token=xxx）
- 校验 token -> userID（Redis）成功后，写入 gin.Context

使用：router.Use(middleware.GinAuthMiddleware(authService, nil))
*/
func GinAuthMiddleware(auth *service.AuthService, opt *AuthOptions) gin.HandlerFunc {
	cfg := opt.withDefaults()

	return func(c *gin.Context) {
		if auth == nil {
			c.Header("Content-Type", "application/json")
			c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
				Code: response.CodeInternalError,
				Msg:  "auth service is nil",
			})
			return
		}

		// 1) header bearer
		token := ""
		ah := strings.TrimSpace(c.GetHeader(cfg.HeaderKey))
		if ah != "" {
			parts := strings.SplitN(ah, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				fmt.Println(parts[1])
				token = strings.TrimSpace(parts[1])
			}
		}

		// 2) query fallback
		if token == "" {
			token = strings.TrimSpace(c.Query(cfg.QueryKey))
		}

		if token == "" {
			c.Header("Content-Type", "application/json")
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Response{
				Code: response.CodeTokenInvalid,
				Msg:  "missing token",
			})
			return
		}

		uid, err := auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.Header("Content-Type", "application/json")
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Response{
				Code: response.CodeTokenInvalid,
				Msg:  err.Error(),
			})
			return
		}

		c.Set(cfg.UserIDKey, uid)
		c.Set(cfg.TokenKey, token)
		c.Next()
	}
}
