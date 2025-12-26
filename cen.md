1. Engine ([engine.go](engine.go)): 核心入口，负责初始化服务、自动迁移数据库表、管理WebSocket服务，并对外提供 http.HandlerFunc 以便集成到 Gin 或 go-zero 等框架中。
2. Service ([service.go](service.go)): 业务逻辑层，包含 RoomService (处理房间创建、私聊逻辑) 和 MessageService (消息存储)。
3. WebSocket ([ws.go](ws.go)): 封装了 WebSocket 的连接管理、消息广播和心跳机制。
4. Model ([table.go](table.go)): 定义数据表结构，去除了硬编码的表名，支持动态前缀。
5. Config ([option.go](option.go)): 保持原有的配置模式。