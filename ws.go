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

	// Name
	Name string
}

// readPump 将消息从client (websocket 连接) 到hub管理。
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		// json消息处理 todo:使用更高性能的protobuf
		// 用回调实现
		//  {"send_to":"房间号","send_type":"发送类型 1文字 2图片 3语音 4应用 5分享","send_content":"发送内容"}
		// e.g {"send_to":1,"send_type":1,"send_content":"hello"}
		c.hub.handleMessage(c, message)
	}
}

// writePump 将消息从hub管理写到具体的client (websocket 连接)。
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub 已经关闭了此ws
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 一次性发送管道剩余全部的消息，不重新走message, ok := <-c.send，提升性能
			// 额外的消息批量写入数据库保持结果一致
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type WsServer struct {
	clients map[*Client]bool
	// userID -> all active websocket connections for that user (supports multi-device)
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
			log.Printf("ws register user=%d totalClients=%d userKeys=%d", client.UserID, len(h.clients), len(h.userClients))
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
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
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
func (h *WsServer) ServeWS(w http.ResponseWriter, r *http.Request, userID uint64, name string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: h, conn: conn, send: make(chan []byte, 256), UserID: userID, Name: name}
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
