# middleware

该目录提供对常见 Web 框架的可选适配（例如 gin），核心鉴权逻辑仍在 `service.AuthService`。

## Gin 使用示例

```go
r := gin.Default()
engine := chat_sdk.NewEngine(chat_sdk.WithDB(db), chat_sdk.WithRDB(rdb))

authSvc := service.NewAuthService(rdb)

r.Use(middleware.GinAuthMiddleware(authSvc, nil))

// 在 handler 里读取
uidAny, _ := c.Get(middleware.ContextUserIDKey)
userID := uidAny.(uint64)
```

默认行为：
- 优先读取 `Authorization: Bearer <token>`
- 如果没有，再读取 query `?token=xxx`

