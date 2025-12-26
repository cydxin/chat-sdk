# Chat SDK

一个高性能的即时通讯/客服聊天 Go SDK，支持 WebSocket 实时通讯和完整的好友/群组管理功能。

## 特性

- ✅ **WebSocket 实时通讯**：高性能的双向通讯
- ✅ **单例模式**：使用 `sync.Once` 确保全局唯一实例
- ✅ **灵活配置**：支持自定义表前缀、数据库连接
- ✅ **消息管理**：发送、接收、撤回消息
- ✅ **好友系统**：添加、删除好友，好友申请管理
- ✅ **群组聊天**：创建群组，管理成员
- ✅ **框架无关**：兼容 Gin、Go-Zero 等任何 Go Web 框架

## 安装

```bash
go get github.com/cydxin/chat-sdk
```

## 快速开始

### 1. 初始化引擎

```go
import (
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    chat "github.com/cydxin/chat-sdk"
)

// 连接数据库
db, _ := gorm.Open(mysql.Open("user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4"))

// 创建 Chat Engine（单例模式）
engine := chat.NewEngine(
    chat.WithDB(db),
    chat.WithTablePrefix("my_app_"), // 自定义表前缀
)
```

### 2. 注册路由（Gin 示例）

```go
import "github.com/gin-gonic/gin"

r := gin.Default()

// WebSocket 连接
r.GET("/ws", func(c *gin.Context) {
    userID := c.Query("user_id") // 从认证中间件获取
    userIDInt, _ := strconv.ParseInt(userID, 10, 64)
    engine.WsServer.ServeWS(c.Writer, c.Request, userIDInt)
})

// 消息相关
r.POST("/api/chat/recall", engine.HandleRecallMessage())
r.GET("/api/chat/messages", engine.HandleGetRoomMessages())

// 好友相关
r.POST("/api/friend/request", engine.HandleSendFriendRequest())
r.POST("/api/friend/accept", engine.HandleAcceptFriendRequest())
r.POST("/api/friend/reject", engine.HandleRejectFriendRequest())
r.DELETE("/api/friend/delete", engine.HandleDeleteFriend())
r.GET("/api/friend/list", engine.HandleGetFriendList())
r.GET("/api/friend/pending", engine.HandleGetPendingRequests())

// 房间/群组相关
r.POST("/api/room/private", engine.HandleCreatePrivateRoom())
r.POST("/api/room/member/add", engine.HandleAddRoomMember())
r.POST("/api/room/member/remove", engine.HandleRemoveRoomMember())

r.Run(":8080")
```

### 3. Go-Zero 示例

```go
// 在 handler 中
func ChatWsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := r.Context().Value("userId").(int64)
        svcCtx.ChatEngine.WsServer.ServeWS(w, r, userID)
    }
}

// 其他 Handler
func RecallMessageHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return svcCtx.ChatEngine.HandleRecallMessage()
}
```

## WebSocket 消息协议

### 客户端发送消息

```json
{
  "send_to": 1,           // 房间 ID
  "send_type": 1,         // 消息类型：1-文字 2-图片 3-语音
  "send_content": "hello" // 消息内容
}
```

### 服务端推送消息

```json
{
  "id": 123,
  "room_id": 1,
  "sender_id": 1001,
  "msg_type": 1,
  "content": "hello",
  "created_at": 1702345678
}
```

### 服务端通知类型

#### 消息撤回通知
```json
{
  "type": "recall",
  "message_id": 123,
  "room_id": 1,
  "user_id": 1001
}
```

#### 好友申请通知
```json
{
  "type": "friend_request",
  "request_id": 456,
  "from_user": 1001,
  "message": "你好，加个好友吧"
}
```

#### 好友同意通知
```json
{
  "type": "friend_accepted",
  "request_id": 456,
  "user_id": 1002
}
```

#### 成员添加通知
```json
{
  "type": "member_added",
  "room_id": 1,
  "user_id": 1003,
  "operator_id": 1001
}
```

## API 接口

### 消息管理

#### 撤回消息
```
POST /api/chat/recall
Body: {
  "message_id": 123,
  "user_id": 1001
}
```

#### 获取房间消息
```
GET /api/chat/messages?room_id=1&limit=20&offset=0
```

### 好友管理

#### 发送好友申请
```
POST /api/friend/request
Body: {
  "from_user": 1001,
  "to_user": 1002,
  "message": "加个好友"
}
```

#### 同意好友申请
```
POST /api/friend/accept
Body: {
  "request_id": 456,
  "user_id": 1002
}
```

#### 拒绝好友申请
```
POST /api/friend/reject
Body: {
  "request_id": 456,
  "user_id": 1002
}
```

#### 删除好友
```
DELETE /api/friend/delete
Body: {
  "user1": 1001,
  "user2": 1002
}
```

#### 获取好友列表
```
GET /api/friend/list?user_id=1001
```

#### 获取待处理申请
```
GET /api/friend/pending?user_id=1001
```

### 房间/群组管理

#### 创建/获取私聊房间
```
POST /api/room/private
Body: {
  "user_id": 1001,
  "target_id": 1002
}
```

#### 添加群成员
```
POST /api/room/member/add
Body: {
  "room_id": 1,
  "user_id": 1003,
  "operator_id": 1001
}
```

#### 移除群成员
```
POST /api/room/member/remove
Body: {
  "room_id": 1,
  "user_id": 1003,
  "operator_id": 1001
}
```

## 数据库表结构

SDK 会自动创建以下表（带配置的前缀）：

- `{prefix}chat_rooms` - 聊天房间表
- `{prefix}chat_members` - 房间成员表
- `{prefix}chat_messages` - 消息表
- `{prefix}chat_notifications` - 通知表
- `{prefix}friend_requests` - 好友申请表
- `{prefix}friendships` - 好友关系表

## 高级特性

### 1. 避免循环依赖的设计

Service 层通过**函数注入**的方式使用 WebSocket 通知：

```go
baseService := &service.Service{
    DB:          db,
    TablePrefix: "my_app_",
    WsNotifier:  engine.WsServer.SendToUser, // 注入 WebSocket 通知函数
}
```

### 2. 单例模式

使用 `sync.Once` 确保 Engine 全局唯一：

```go
var (
    Instance *ChatEngine
    once     sync.Once
)

func NewEngine(opts ...Option) *ChatEngine {
    once.Do(func() {
        // 初始化逻辑
    })
    return Instance
}
```

### 3. 批量发送优化

WebSocket 的 `writePump` 使用了批量发送优化：

```go
// 一次性发送管道剩余全部的消息
n := len(c.send)
for i := 0; i < n; i++ {
    w.Write(<-c.send)
}
```

### 4. 用户/客服分离

支持根据 UserID 区分普通用户和客服（UserID > 10000 为普通用户）：

```go
if client.UserID > 10000 {
    h.userClients[client.UserID] = append(h.userClients[client.UserID], client)
} else {
    h.kfClients[client.UserID] = append(h.kfClients[client.UserID], client)
}
```

## 性能优化建议

1. **连接池**：使用 GORM 的连接池配置
2. **消息队列**：高并发场景可接入 Redis/RabbitMQ
3. **分布式**：多实例部署时使用 Redis Pub/Sub 同步 WebSocket 消息
4. **数据库索引**：已自动创建必要的索引

## 扩展开发

### 自定义消息处理

```go
// 在初始化后设置自定义消息处理
engine.WsServer.SetOnMessage(func(client *Client, msg []byte) {
    // 自定义逻辑
})
```

### 接入现有用户系统

修改 `service/member_service.go` 中的 `SearchUsers` 方法：

```go
func (s *MemberService) SearchUsers(keyword string, currentUserID int64, limit int) ([]int64, error) {
    var userIDs []int64
    err := s.DB.Table("your_users_table").
        Where("username LIKE ? OR nickname LIKE ?", "%"+keyword+"%", "%"+keyword+"%").
        Where("id != ?", currentUserID).
        Limit(limit).
        Pluck("id", &userIDs).Error
    return userIDs, err
}
```

## License

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！

