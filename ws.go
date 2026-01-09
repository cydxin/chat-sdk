package chat_sdk

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time 写入超时时间
	writeWait = 10 * time.Second

	// Time pong超时时间
	pongWait = 60 * time.Second

	// Send 对应的ping 必须小于pong
	pingPeriod = (pongWait * 9) / 10

	// Maximum 对等端允许消息大小
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for SDK
	},
}

// Client ws和hub的连接
// 说明：Client 代表“某个具体 websocket 连接”，用户级别可复用的数据放到 UserSession。
type Client struct {
	hub *WsServer

	// 🔗链接
	conn *websocket.Conn

	// 消息缓冲区
	send chan []byte

	// UserID 和用户关联
	UserID uint64

	// 会话ID
	SessionID string

	// UserSession 指向用户级别共享状态（昵称/头像/已读缓存等）
	session *UserSession

	// Name Nickname Avatar
	Name string

	Nickname string

	Avatar string
}

// UserSession 用户级别共享状态（同一用户多设备/多连接复用）
type UserSession struct {
	UserID   uint64
	Name     string
	Nickname string
	Avatar   string

	readMu   sync.Mutex
	readList map[uint64]uint64

	lastSeen time.Time

	// dirty 表示 readList 有更新但尚未落库
	dirty bool
	// lastFlush 上次落库时间
	lastFlush time.Time
}

func (s *UserSession) mergeRead(roomID, lastRead uint64) {
	if roomID == 0 || lastRead == 0 {
		return
	}
	s.readMu.Lock()
	if s.readList == nil {
		s.readList = make(map[uint64]uint64)
	}
	if old := s.readList[roomID]; lastRead > old {
		s.readList[roomID] = lastRead
		s.dirty = true
	}
	s.lastSeen = time.Now()
	s.readMu.Unlock()
}

func (s *UserSession) snapshotRead() map[uint64]uint64 {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	if len(s.readList) == 0 {
		return nil
	}
	snap := make(map[uint64]uint64, len(s.readList))
	for k, v := range s.readList {
		snap[k] = v
	}
	return snap
}

// markFlushed 在落库成功后调用
func (s *UserSession) markFlushed() {
	s.readMu.Lock()
	s.dirty = false
	s.lastFlush = time.Now()
	s.readMu.Unlock()
}

// snapshotReadAndDirty 返回快照及是否 dirty（用于周期 flush）
func (s *UserSession) snapshotReadAndDirty() (map[uint64]uint64, bool) {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	if !s.dirty || len(s.readList) == 0 {
		return nil, false
	}
	snap := make(map[uint64]uint64, len(s.readList))
	for k, v := range s.readList {
		snap[k] = v
	}
	return snap, true
}

// readPump 将消息从client (websocket 连接) 到hub管理。
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { _ = c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("readPump error: %v", err)
			}
			break
		}
		c.hub.handleMessage(c, message)
	}
}

// writePump 将消息从hub管理写到具体的client (websocket 连接)。
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// 一次性发送管道剩余全部的消息，不重新走message, ok := <-c.send，提升性能
			// 额外的消息批量写入数据库保持结果一致
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("writePump 写入ping失败")
				return
			}
		}
	}
}

type WsServer struct {
	clients map[*Client]bool
	// 用户ID ->该用户所有活跃的Websocket连接（支持多设备）
	userClients map[uint64][]*Client

	// 用户级别共享 session
	sessions map[uint64]*UserSession

	// 用户ID -> “延迟移除/flush” 的定时器
	gcTimers map[uint64]*time.Timer

	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	// 回调处理消息
	onMessage func(client *Client, msg []byte)
}

func NewWsServer() *WsServer {
	return &WsServer{
		broadcast:   make(chan []byte),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		userClients: make(map[uint64][]*Client),
		sessions:    make(map[uint64]*UserSession),
		gcTimers:    make(map[uint64]*time.Timer),
	}
}

func (h *WsServer) Run() {
	flushTicker := time.NewTicker(60 * time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case <-flushTicker.C:
			// 在线周期 flush：只 flush dirty 的 session
			// 这里不在 h.mu.Lock 下做 DB IO，避免阻塞 ws 主循环。
			h.mu.RLock()
			// copy sessions snapshot
			sessions := make([]*UserSession, 0, len(h.sessions))
			for _, s := range h.sessions {
				sessions = append(sessions, s)
			}
			h.mu.RUnlock()

			for _, sess := range sessions {
				if sess == nil {
					continue
				}
				snap, dirty := sess.snapshotReadAndDirty()
				if !dirty || snap == nil {
					continue
				}

				if Instance != nil && Instance.MsgService != nil && Instance.MsgService.ReadReceipt != nil {
					if err := Instance.MsgService.ReadReceipt.FlushUserRead(sess.UserID, snap); err == nil {
						sess.markFlushed()
					}
				}
			}

		case client := <-h.register:
			h.mu.Lock()
			// 1) 复用/创建用户级 session
			sess := h.sessions[client.UserID]
			if sess == nil {
				sess = &UserSession{UserID: client.UserID, Name: client.Name, Nickname: client.Nickname, Avatar: client.Avatar, lastSeen: time.Now()}
				h.sessions[client.UserID] = sess
			} else {
				// 更新用户资料（以最新连接为准）
				sess.Name = client.Name
				sess.Nickname = client.Nickname
				sess.Avatar = client.Avatar
				sess.lastSeen = time.Now()
			}
			client.session = sess

			// 2) 取消gcTime（用户又上线了）
			if t, ok := h.gcTimers[client.UserID]; ok {
				t.Stop()
				delete(h.gcTimers, client.UserID)
			}

			h.clients[client] = true
			h.userClients[client.UserID] = append(h.userClients[client.UserID], client)
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)

				if userConns, exists := h.userClients[client.UserID]; exists {
					for i, conn := range userConns {
						if conn == client {
							h.userClients[client.UserID] = append(userConns[:i], userConns[i+1:]...)
							break
						}
					}
					if len(h.userClients[client.UserID]) == 0 {
						// 不立刻 delete：交给 timer 决定是否清理，给断开-重连留窗口
					}
				}
			}

			// 3) 启动/重置 5 分钟 GC：仅当用户确实无任何连接时才 flush + 清理
			uid := client.UserID
			if t, ok := h.gcTimers[uid]; ok {
				t.Stop()
			}
			h.gcTimers[uid] = time.AfterFunc(5*time.Minute, func() {
				// timer 回调里不要直接用 client 指针（可能已复用/已变化），用 uid 查当前状态
				h.mu.RLock()
				conns := h.userClients[uid]
				sess := h.sessions[uid]
				h.mu.RUnlock()

				if len(conns) > 0 {
					// 用户重新上线了，不 flush
					return
				}

				// flush session readList
				if sess != nil {
					snap := sess.snapshotRead()
					if snap != nil {
						if Instance != nil && Instance.MsgService != nil && Instance.MsgService.ReadReceipt != nil {
							_ = Instance.MsgService.ReadReceipt.FlushUserRead(uid, snap)
						}
					}
				}

				// 清理 maps
				h.mu.Lock()
				delete(h.userClients, uid)
				delete(h.sessions, uid)
				delete(h.gcTimers, uid)
				h.mu.Unlock()
			})

			h.mu.Unlock()

		case message := <-h.broadcast:
			// 注意：不能在 RLock 下修改 map / close channel，否则会引发竞态/崩溃。
			var toRemove []*Client
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					toRemove = append(toRemove, client)
				}
			}
			h.mu.RUnlock()

			if len(toRemove) > 0 {
				h.mu.Lock()
				for _, client := range toRemove {
					if _, ok := h.clients[client]; !ok {
						continue
					}
					delete(h.clients, client)
					// 从 userClients 中移除
					if userConns, exists := h.userClients[client.UserID]; exists {
						for i, conn := range userConns {
							if conn == client {
								h.userClients[client.UserID] = append(userConns[:i], userConns[i+1:]...)
								break
							}
						}
						if len(h.userClients[client.UserID]) == 0 {
							delete(h.userClients, client.UserID)
						}
					}
					// close 之前再确认一次，避免 panic（多处 close 的竞态）
					select {
					case <-client.send:
						// channel 可能已被关闭并读到零值；下面安全 close 仍可能 panic，故用 recover
					default:
					}
					func() {
						defer func() { _ = recover() }()
						close(client.send)
					}()
				}
				h.mu.Unlock()
			}
		}
	}
}

func (h *WsServer) handleMessage(client *Client, msg []byte) {
	if h.onMessage != nil {
		h.onMessage(client, msg)
	}
}
func (h *WsServer) SetOnMessage(fn func(client *Client, msg []byte)) {
	h.onMessage = fn
}

// ServeWS 处理ws的请求
func (h *WsServer) ServeWS(w http.ResponseWriter, r *http.Request, userID uint64, name string, extras ...string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	nickname := ""
	avatar := ""
	if len(extras) > 0 {
		nickname = extras[0]
	}
	if len(extras) > 1 {
		avatar = extras[1]
	}

	// 复用/创建用户级 session
	h.mu.Lock()
	sess := h.sessions[userID]
	created := false
	if sess == nil {
		created = true
		sess = &UserSession{UserID: userID, Name: name, Nickname: nickname, Avatar: avatar, lastSeen: time.Now()}
		h.sessions[userID] = sess
	} else {
		sess.Name = name
		sess.Nickname = nickname
		sess.Avatar = avatar
		sess.lastSeen = time.Now()
	}
	// cancel GC timer（用户又上线了）
	if t, ok := h.gcTimers[userID]; ok {
		t.Stop()
		delete(h.gcTimers, userID)
	}
	h.mu.Unlock()

	// 建连时从 DB 加载可见会话的 last_read_msg_id 到 session.readList
	// 只在 session 新建或当前 readList 为空时加载，避免每次重连都打 DB。
	if Instance != nil && Instance.MsgService != nil && Instance.MsgService.SessionBootstrap != nil {
		sess.readMu.Lock()
		empty := len(sess.readList) == 0
		sess.readMu.Unlock()
		if created || empty {
			if m, err := Instance.MsgService.SessionBootstrap.GetVisibleConversationLastReads(userID); err == nil {
				for roomID, lastRead := range m {
					sess.mergeRead(roomID, lastRead)
				}
				// 初始化加载不算未落库变更
				sess.readMu.Lock()
				sess.dirty = false
				sess.readMu.Unlock()
			}
		}
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		UserID:   userID,
		Name:     name,
		Nickname: nickname,
		Avatar:   avatar,
		session:  sess,
	}
	client.hub.register <- client
	log.Println("注册进去: ", client.UserID)

	go client.writePump()
	go client.readPump()

	// 不要 select{} 永久阻塞 handler；连接生命周期由 readPump/writePump 控制。
}

// SendToUser 发送消息到用户
func (h *WsServer) SendToUser(userID uint64, msg []byte) {
	h.mu.RLock()
	clients := h.userClients[userID]
	keys := len(h.userClients)
	h.mu.RUnlock()

	log.Printf("SendToUser user=%d userKeys=%d conns=%d", userID, keys, len(clients))
	for _, client := range clients {
		select {
		case client.send <- msg:
		default:
			// 丢弃避免阻塞
		}
	}
}
