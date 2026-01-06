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
type Client struct {
	hub *WsServer

	// 🔗链接
	conn *websocket.Conn

	// 消息缓冲区
	send chan []byte

	// UserID 和用户关联
	UserID uint64

	// Name Nickname Avatar
	Name string

	Nickname string

	Avatar string
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
	}
}

func (h *WsServer) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.userClients[client.UserID] = append(h.userClients[client.UserID], client)
			//log.Printf("ws register user=%d totalClients=%d userKeys=%d", client.UserID, len(h.clients), len(h.userClients))
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
						delete(h.userClients, client.UserID)
					}
				}
			}
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

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		UserID:   userID,
		Name:     name,
		Nickname: nickname,
		Avatar:   avatar,
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
