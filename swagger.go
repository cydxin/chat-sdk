package chat_sdk

import (
	_ "github.com/cydxin/chat-sdk/docs"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterSwagger 在 Gin 路由上注册 Swagger UI。
// 默认路由：/swagger/*any
//
// 使用示例：
//
//	r := gin.Default()
//	chat_sdk.RegisterSwagger(r, "/swagger/*any")
//	r.Run(":8080")
//
// 访问：http://localhost:8080/swagger/index.html
func RegisterSwagger(r *gin.Engine, path string) {
	if path == "" {
		path = "/swagger/*any"
	}
	r.GET(path, ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// RegisterSwaggerWithGroup 在 Gin 路由组上注册 Swagger UI。
func RegisterSwaggerWithGroup(g *gin.RouterGroup, path string) {
	if path == "" {
		path = "/swagger/*any"
	}
	g.GET(path, ginSwagger.WrapHandler(swaggerFiles.Handler))
}
