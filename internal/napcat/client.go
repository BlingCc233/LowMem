package napcat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Client is a OneBot 11 WebSocket client for NapCat
type Client struct {
	config    Config
	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	connected atomic.Bool
	mu        sync.RWMutex

	// HTTP Client for fallback
	httpClient *HTTPClient

	// Request/response correlation
	pending   map[string]chan *APIResponse
	pendingMu sync.Mutex
	echoSeq   atomic.Int64

	// Event callbacks
	onPrivateMessage func(*PrivateMessageEvent)
	onGroupMessage   func(*GroupMessageEvent)
	onFriendRecall   func(*FriendRecallEvent)
	onGroupRecall    func(*GroupRecallEvent)
	onDisconnect     func()

	// Login info cache
	loginInfo *LoginInfo
}

// NewClient creates a new NapCat client
func NewClient(config Config) *Client {
	c := &Client{
		config:  config,
		pending: make(map[string]chan *APIResponse),
	}
	if config.HTTPURL != "" {
		c.httpClient = NewHTTPClient(config.HTTPURL, config.AccessToken)
	}
	return c
}

// Connect establishes WebSocket connection to NapCat
func (c *Client) Connect(parentCtx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(parentCtx)

	// Try HTTP first to check basic connectivity and get login info
	if c.httpClient != nil {
		info, err := c.GetLoginInfo()
		if err == nil {
			c.loginInfo = info
			log.Printf("Connected via HTTP as %s (%d)", info.Nickname, info.UserID)
		} else {
			log.Printf("HTTP check failed: %v", err)
		}
	}

	header := http.Header{}
	if c.config.AccessToken != "" {
		header.Set("Authorization", "Bearer "+c.config.AccessToken)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.DialContext(c.ctx, c.config.WebSocketURL, header)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.connected.Store(true)

	// Start message reader
	go c.readLoop()

	// Update login info via WebSocket if HTTP failed or just to succeed via WS
	if c.loginInfo == nil {
		info, err := c.GetLoginInfo()
		if err != nil {
			log.Printf("Warning: failed to get login info via WS: %v", err)
		} else {
			c.loginInfo = info
			log.Printf("Connected via WebSocket as %s (%d)", info.Nickname, info.UserID)
		}
	}

	return nil
}

// Close closes the connection
func (c *Client) Close() {
	c.connected.Store(false)
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// IsConnected returns connection status (WebSocket only)
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// IsHTTPAvailable returns true if HTTP client is configured
func (c *Client) IsHTTPAvailable() bool {
	return c.httpClient != nil
}

// LoginInfo returns cached login info
func (c *Client) LoginInfo() *LoginInfo {
	return c.loginInfo
}

// SetPrivateMessageHandler sets private message callback
func (c *Client) SetPrivateMessageHandler(h func(*PrivateMessageEvent)) {
	c.onPrivateMessage = h
}

// SetGroupMessageHandler sets group message callback
func (c *Client) SetGroupMessageHandler(h func(*GroupMessageEvent)) {
	c.onGroupMessage = h
}

// SetFriendRecallHandler sets friend recall callback
func (c *Client) SetFriendRecallHandler(h func(*FriendRecallEvent)) {
	c.onFriendRecall = h
}

// SetGroupRecallHandler sets group recall callback
func (c *Client) SetGroupRecallHandler(h func(*GroupRecallEvent)) {
	c.onGroupRecall = h
}

// SetDisconnectHandler sets disconnect callback
func (c *Client) SetDisconnectHandler(h func()) {
	c.onDisconnect = h
}

// readLoop reads messages from WebSocket
func (c *Client) readLoop() {
	defer func() {
		originalConnected := c.connected.Swap(false)
		if originalConnected && c.onDisconnect != nil {
			c.onDisconnect()
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Println("WebSocket closed normally")
			} else {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		c.handleMessage(message)
	}
}

// handleMessage processes incoming WebSocket message
func (c *Client) handleMessage(data []byte) {
	// First check if it's an API response (has echo field)
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.Echo != "" {
		c.pendingMu.Lock()
		ch, ok := c.pending[resp.Echo]
		if ok {
			delete(c.pending, resp.Echo)
		}
		c.pendingMu.Unlock()
		if ok {
			ch <- &resp
		}
		return
	}

	// Otherwise it's an event
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to parse event: %v", err)
		return
	}

	switch event.PostType {
	case "message":
		c.handleMessageEvent(data, &event)
	case "notice":
		c.handleNoticeEvent(data, &event)
	case "meta_event":
		// Heartbeat, lifecycle - ignore
	}
}

func (c *Client) handleMessageEvent(data []byte, event *Event) {
	switch event.MessageType {
	case "private":
		var e PrivateMessageEvent
		if err := json.Unmarshal(data, &e); err == nil && c.onPrivateMessage != nil {
			c.onPrivateMessage(&e)
		}
	case "group":
		var e GroupMessageEvent
		if err := json.Unmarshal(data, &e); err == nil && c.onGroupMessage != nil {
			c.onGroupMessage(&e)
		}
	}
}

func (c *Client) handleNoticeEvent(data []byte, event *Event) {
	switch event.NoticeType {
	case "friend_recall":
		var e FriendRecallEvent
		if err := json.Unmarshal(data, &e); err == nil && c.onFriendRecall != nil {
			c.onFriendRecall(&e)
		}
	case "group_recall":
		var e GroupRecallEvent
		if err := json.Unmarshal(data, &e); err == nil && c.onGroupRecall != nil {
			c.onGroupRecall(&e)
		}
	}
}

// Call sends an API request via WebSocket if connected, otherwise fallbacks to HTTP
func (c *Client) Call(action string, params interface{}) (*APIResponse, error) {
	// Try WebSocket first
	if c.connected.Load() {
		return c.callWebSocket(action, params)
	}

	// Fallback to HTTP
	if c.httpClient != nil {
		// Log warning only if we were expecting WS to specific calls? No, just debug log
		// log.Printf("WS disconnected, using HTTP fallback for %s", action)
		return c.httpClient.PostAPI(action, params)
	}

	return nil, fmt.Errorf("not connected and no http fallback")
}

// callWebSocket sends an API request over WebSocket
func (c *Client) callWebSocket(action string, params interface{}) (*APIResponse, error) {
	echo := fmt.Sprintf("%d", c.echoSeq.Add(1))
	req := APIRequest{
		Action: action,
		Params: params,
		Echo:   echo,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create response channel
	respCh := make(chan *APIResponse, 1)
	c.pendingMu.Lock()
	c.pending[echo] = respCh
	c.pendingMu.Unlock()

	// Send request
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return nil, fmt.Errorf("connection closed")
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, echo)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("write message: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(30 * time.Second):
		c.pendingMu.Lock()
		delete(c.pending, echo)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("request timeout")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}
