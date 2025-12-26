// Package chat_sdk 提供即时通讯 SDK 核心能力
// @title Chat SDK API
// @version 1.0
// @description 即时通讯 SDK 的 RESTful API 文档，包含用户管理、消息、房间、好友等模块
// @description
// @description ## 业务状态码说明
// @description | Code | 说明 |
// @description |------|------|
// @description | 0 | 成功 |
// @description | 10001 | 参数错误 |
// @description | 10002 | 用户不存在 |
// @description | 10003 | 密码错误（登录失败） |
// @description | 10004 | Token 无效 |
// @description | 10005 | 权限不足 |
// @description | 99999 | 内部错误 |
// @description
// @description ## HTTP 状态码说明
// @description - **200**: 业务请求成功（根据 response.code 判断业务状态）
// @description - **401**: 认证失败（未登录/Token 无效/登录失败）
// @description - **403**: 权限不足
// @description - **500**: 服务器内部错误
// @description
// @description ## 响应格式
// @description 所有接口统一返回格式：
// @description ```json
// @description {
// @description   "code": 0,
// @description   "msg": "success",
// @description   "data": {}
// @description }
// @description ```
//
// @termsOfService https://github.com/cydxin/chat-sdk
//
// @contact.name API Support
// @contact.url https://github.com/cydxin/chat-sdk/issues
// @contact.email support@example.com
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:6789
// @BasePath /api/v1
// @schemes http https
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 格式：Bearer <token>
//
// @securityDefinitions.apikey QueryToken
// @in query
// @name token
// @description 用于 WebSocket 等无法传 header 的场景
package chat_sdk
