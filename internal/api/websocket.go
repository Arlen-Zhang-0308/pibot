package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/executor"
)

// toolExecPayload is a lightweight struct for marshalling tool-event WS messages.
type toolExecPayload struct {
	Tool    string `json:"tool"`
	Args    string `json:"args,omitempty"`
	Content string `json:"content,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local network use
	},
}

// WebSocket message types
const (
	MsgTypeChat          = "chat"
	MsgTypeExec          = "exec"
	MsgTypeAbort         = "abort"
	MsgTypeAbortAck      = "abort_ack"
	MsgTypeStream        = "stream"
	MsgTypeStreamEnd     = "stream_end"
	MsgTypeExecResult    = "exec_result"
	MsgTypeError         = "error"
	MsgTypePending       = "pending"
	MsgTypeToolExecuting = "tool_executing"
	MsgTypeToolOutput    = "tool_output"
	MsgTypeToolFinished  = "tool_finished"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// ChatPayload represents a chat message payload
type ChatPayload struct {
	Messages    []ai.Message `json:"messages"`
	Provider    string       `json:"provider,omitempty"`
	AlwaysAllow bool         `json:"always_allow,omitempty"`
}

// ExecPayload represents a command execution payload
type ExecPayload struct {
	Command string `json:"command"`
}

// Client represents a WebSocket client
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	server        *Server
	mu            sync.Mutex
	abortCancel   context.CancelFunc // cancels the current in-flight chat completion
	abortStreamID uint64             // monotonic ID to match cancel to the current stream
	nextStreamID  uint64
}

// Hub manages WebSocket clients
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		}
	}
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:    s.wsHub,
		conn:   conn,
		send:   make(chan []byte, 256),
		server: s,
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.sendError("Invalid message format")
			continue
		}

		c.handleMessage(msg)
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (c *Client) handleMessage(msg WSMessage) {
	switch msg.Type {
	case MsgTypeChat:
		// Run in a goroutine so readPump is free to receive abort (or other) messages
		// while the chat completion is in progress.
		go c.handleChatMessage(msg)
	case MsgTypeExec:
		c.handleExecMessage(msg)
	case MsgTypeAbort:
		c.mu.Lock()
		if c.abortCancel != nil {
			c.abortCancel()
			c.abortCancel = nil
		}
		c.mu.Unlock()
		// Cancel all pending executor gates so no confirmation dialogs remain blocked.
		c.server.executor.CancelAll()
		// Tell the frontend to clear its pending queue.
		c.sendMessage(WSMessage{
			Type:    MsgTypeAbortAck,
			Payload: mustMarshal(map[string]string{}),
		})
	default:
		c.sendError("Unknown message type")
	}
}

// handleChatMessage handles chat messages with streaming and tool support
func (c *Client) handleChatMessage(msg WSMessage) {
	var payload ChatPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError("Invalid chat payload")
		return
	}

	if len(payload.Messages) == 0 {
		c.sendError("No messages provided")
		return
	}

	// Create a channel for streaming
	ch := make(chan string, 100)
	baseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	// Register the cancel function so an abort message can stop this completion.
	c.mu.Lock()
	if c.abortCancel != nil {
		c.abortCancel() // cancel any previous in-flight request
	}
	c.nextStreamID++
	myStreamID := c.nextStreamID
	c.abortCancel = cancel
	c.abortStreamID = myStreamID
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.abortStreamID == myStreamID {
			c.abortCancel = nil
		}
		c.mu.Unlock()
		cancel()
	}()

	// Propagate always-allow flag through context so Executor can skip pending flow.
	ctx := context.WithValue(baseCtx, executor.AlwaysAllowKey, payload.AlwaysAllow)

	// Inject a notify function so the Executor can push pending WS messages back to
	// this client while blocking inside an AI tool call.
	notifyFn := func(result *executor.ExecutionResult) {
		c.sendMessage(WSMessage{
			Type:    MsgTypePending,
			ID:      msg.ID,
			Payload: mustMarshal(result),
		})
	}
	ctx = context.WithValue(ctx, executor.NotifyPendingKey, notifyFn)

	// Inject a real-time output stream callback so command stdout/stderr
	// lines are pushed to the client as they are produced.
	outputStreamFn := func(line string) {
		c.sendMessage(WSMessage{
			Type: MsgTypeToolOutput,
			ID:   msg.ID,
			Payload: mustMarshal(toolExecPayload{
				Content: line,
			}),
		})
	}
	ctx = context.WithValue(ctx, executor.OutputStreamKey, outputStreamFn)

	// Inject a tool-event callback so the client is notified when tools
	// start executing and when they finish.
	toolEventFn := func(evt ai.ToolEvent) {
		var msgType string
		switch evt.Kind {
		case ai.ToolEventExecuting:
			msgType = MsgTypeToolExecuting
		case ai.ToolEventOutput:
			msgType = MsgTypeToolOutput
		case ai.ToolEventFinished:
			msgType = MsgTypeToolFinished
		default:
			return
		}
		c.sendMessage(WSMessage{
			Type: msgType,
			ID:   msg.ID,
			Payload: mustMarshal(toolExecPayload{
				Tool:    evt.Tool,
				Args:    evt.Args,
				Content: evt.Content,
				IsError: evt.IsError,
			}),
		})
	}
	ctx = context.WithValue(ctx, ai.ToolEventKey, toolEventFn)

	// Determine provider
	provider := payload.Provider
	if provider == "" {
		provider = c.server.config.GetAI().DefaultProvider
	}

	// Start streaming with tool support
	go func() {
		err := c.server.chatSession.StreamChatWithTools(ctx, provider, payload.Messages, ch)
		if err != nil && ctx.Err() == nil {
			// Only forward errors that aren't due to intentional cancellation.
			c.sendError(err.Error())
		}
	}()

	// Send chunks as they arrive
	for chunk := range ch {
		c.sendMessage(WSMessage{
			Type: MsgTypeStream,
			ID:   msg.ID,
			Payload: mustMarshal(map[string]string{
				"content": chunk,
			}),
		})
	}

	// Always send stream_end so the frontend resets its state, regardless of
	// whether the stream completed normally or was aborted.
	status := "complete"
	if ctx.Err() != nil {
		status = "aborted"
	}
	c.sendMessage(WSMessage{
		Type: MsgTypeStreamEnd,
		ID:   msg.ID,
		Payload: mustMarshal(map[string]string{
			"status": status,
		}),
	})
}

// handleExecMessage handles command execution messages
func (c *Client) handleExecMessage(msg WSMessage) {
	var payload ExecPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError("Invalid exec payload")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := c.server.executor.Execute(ctx, payload.Command)
	if err != nil {
		c.sendError(err.Error())
		return
	}

	if result.Pending {
		c.sendMessage(WSMessage{
			Type:    MsgTypePending,
			ID:      msg.ID,
			Payload: mustMarshal(result),
		})
	} else {
		c.sendMessage(WSMessage{
			Type:    MsgTypeExecResult,
			ID:      msg.ID,
			Payload: mustMarshal(result),
		})
	}
}

// sendMessage sends a message to the client
func (c *Client) sendMessage(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		// Client buffer full, skip message
	}
}

// sendError sends an error message to the client
func (c *Client) sendError(errMsg string) {
	c.sendMessage(WSMessage{
		Type: MsgTypeError,
		Payload: mustMarshal(map[string]string{
			"error": errMsg,
		}),
	})
}

// mustMarshal marshals to JSON or panics
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
