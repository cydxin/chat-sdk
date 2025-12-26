# Swagger 文档

本 SDK 已集成 Swagger API 文档能力。

## 生成文档

在项目根目录运行：

```bash
go run github.com/swaggo/swag/cmd/swag@latest init --parseDependency --parseInternal --output ./docs --generalInfo docs.go
```

这会在 `docs/` 目录生成：
- `docs.go`（导入注册用）
- `swagger.json`
- `swagger.yaml`

## 调用方使用（Gin 示例）

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/cydxin/chat-sdk"
)

func main() {
    r := gin.Default()
    engine := chat_sdk.NewEngine(...)

    // 注册 Swagger UI
    chat_sdk.RegisterSwagger(r, "/swagger/*any")

    // 挂载你的业务路由...

    r.Run(":8080")
}
```

访问：`http://localhost:8080/swagger/index.html`

## 注意事项

1. **安全提示**：生产环境建议关闭 Swagger UI 或加鉴权。
2. **文档同步**：修改 handler 注解后需重新运行 `swag init`。
3. **自定义配置**：可修改 `docs.go` 里的全局配置（host/basePath/title 等）。

