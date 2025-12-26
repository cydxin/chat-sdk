 # 项目结构说明

```
chat-sdk/
├── engine.go              # 核心引擎，初始化和管理所有组件
├── handlers.go            # HTTP Handler 函数集合
├── ws.go                  # WebSocket 服务器实现
├── option.go              # 配置选项（WithDB, WithTablePrefix 等）
├── table.go               # 旧的表定义（已废弃）
├── service.go             # 旧的服务层（已废弃，迁移到 service/ 目录）
│
├── model/                 # 数据模型层
│   └── table.go          # 所有数据表结构定义
│
├── message/               # 消息协议定义
│   └── message.go        # WebSocket 消息结构
│
├── service/               # 业务逻辑层（核心设计）
│   ├── base.go           # 基础服务，包含 WsNotifier 回调
│   ├── room_service.go   # 房间/群组相关服务
│   ├── message_service.go # 消息相关服务（发送、撤回等）
│   └── member_service.go  # 成员/好友管理服务
│
├── example/               # 使用示例
│   └── main.go           # Gin 框架集成示例
│
├── README.md             # 用户文档
├── go.mod                # Go 模块定义
└── go.sum                # 依赖校验
```

## 核心设计思想

### 1. 单例模式（Singleton Pattern）

**文件**: `engine.go`

使用 `sync.Once` 确保 `ChatEngine` 全局唯一：

```go
var (
    Instance *ChatEngine
    once     sync.Once
)

func NewEngine(opts ...Option) *ChatEngine {
    once.Do(func() {
        // 初始化逻辑只执行一次
    })
    return Instance
}
```

**优势**:
- 避免重复初始化
- 全局共享 WebSocket 连接池
- 统一的配置管理

### 2. 函数注入（Dependency Injection via Function）

**文件**: `service/base.go`

通过函数注入避免循环依赖：

```go
type Service struct {
    DB          *gorm.DB
    TablePrefix string
    // 注入 WebSocket 通知函数，避免直接依赖 ws.go
    WsNotifier  func(userID uint64, message []byte)
}
```

**调用链**:
```
engine.go (NewEngine)
    ↓
创建 baseService，注入 WsServer.SendToUser
    ↓
各个 Service 通过 WsNotifier 发送通知
```

**避免的循环依赖**:
```
❌ service -> ws -> service (循环依赖)
✅ service -> (function callback) <- engine -> ws (解耦)
```

### 3. 闭包（Closure）

**文件**: `engine.go`

使用闭包处理 WebSocket 消息：

```go
Instance.WsServer.onMessage = func(client *Client, msg []byte) {
    // 闭包捕获 Instance，可以访问所有 Service
    savedMsg, _ := Instance.MsgService.SaveMessage(...)
    members, _ := Instance.RoomService.GetRoomMembers(...)
    // ...
}
```

**优势**:
- 灵活的消息处理逻辑
- 可以访问外部作用域变量
- 支持动态修改处理逻辑

### 4. 选项模式（Options Pattern）

**文件**: `option.go`

使用函数选项模式配置引擎：

```go
type Option func(*Config)

func WithDB(db *gorm.DB) Option {
    return func(c *Config) {
        c.DB = db
    }
}

// 使用
engine := NewEngine(
    WithDB(db),
    WithTablePrefix("my_app_"),
)
```

**优势**:
- 可扩展的配置方式
- 可选参数更友好
- 向后兼容性强

### 5. 表前缀动态配置

**设计**: 不使用 GORM 的 `TableName()` 方法，而是在运行时动态拼接

```go
// service/base.go
func (s *Service) Table(name string) *gorm.DB {
    return s.DB.Table(s.TablePrefix + name)
}

// 使用
s.Table("chat_messages").Where(...).Find(...)
```

**原因**:
- `TableName()` 方法是静态的，无法感知配置
- 动态拼接支持用户自定义前缀
- 更适合 SDK 场景

### 6. WebSocket 批量发送优化

**文件**: `ws.go`

```go
// writePump 中的优化
w.Write(message) // 写入第一条消息

// 一次性取出管道中剩余的所有消息
n := len(c.send)
for i := 0; i < n; i++ {
    w.Write(<-c.send)
}
```

**优势**:
- 减少系统调用次数
- 合并多条消息到一个 WebSocket 帧
- 高并发场景性能提升明显

### 7. 用户/客服分离

**文件**: `ws.go`

```go
type WsServer struct {
    userClients map[int64][]*Client // 普通用户
    kfClients   map[int64][]*Client // 客服
    // ...
}

// 根据 UserID 区分
if client.UserID > 10000 {
    h.userClients[client.UserID] = append(...)
} else {
    h.kfClients[client.UserID] = append(...)
}
```

**优势**:
- 客服系统和用户系统分离
- 便于实现客服分配逻辑
- 可独立扩展客服功能

## 数据流

### 消息发送流程

```
客户端 WebSocket
    ↓ JSON: {"send_to":1, "send_type":1, "send_content":"hello"}
readPump (ws.go)
    ↓
handleMessage → onMessage 闭包 (engine.go)
    ↓
MsgService.SaveMessage (存入数据库)
    ↓
RoomService.GetRoomMembers (获取房间成员)
    ↓
遍历成员 → WsServer.SendToUser (推送给在线成员)
    ↓
writePump → 客户端 WebSocket
```

### 消息撤回流程

```
HTTP POST /api/chat/recall
    ↓
HandleRecallMessage (handlers.go)
    ↓
MsgService.RecallMessage (service/message_service.go)
    ↓
1. 更新数据库（标记为已撤回）
2. 调用 WsNotifier 回调
    ↓
WsServer.SendToUser (通知房间所有成员)
    ↓
客户端收到 {"type":"recall", ...}
```

### 好友申请流程

```
HTTP POST /api/friend/request
    ↓
HandleSendFriendRequest (handlers.go)
    ↓
MemberService.SendFriendRequest (service/member_service.go)
    ↓
1. 创建好友申请记录
2. 调用 WsNotifier 回调
    ↓
WsServer.SendToUser (通知目标用户)
    ↓
客户端收到 {"type":"friend_request", ...}
```

## 后续优化加强

### 1. 自定义消息处理

```go
engine.WsServer.SetOnMessage(func(client *Client, msg []byte) {
    // 自定义逻辑
})
```

### 2. 接入现有用户表

修改 `service/member_service.go` 的 `SearchUsers` 方法

### 3. 接入消息队列

在 `MsgService.SaveMessage` 后发送到 MQ：

```go
savedMsg, _ := Instance.MsgService.SaveMessage(...)
// 发送到 RabbitMQ/Kafka
producer.Publish(savedMsg)
```

### 4. 分布式部署

使用 Redis Pub/Sub 同步多实例的 WebSocket 消息：

```go
// 在 SendToUser 中
redis.Publish("ws:broadcast", message)

// 订阅其他实例的消息
redis.Subscribe("ws:broadcast", func(msg) {
    localWsServer.SendToUser(...)
})
```


