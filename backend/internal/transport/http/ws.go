package http

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	pokerapp "github.com/teamcutter/go-poker/internal/application/poker"
	"github.com/teamcutter/go-poker/internal/domain/poker"
	"github.com/teamcutter/go-poker/internal/transport/http/middleware"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type client struct {
	conn     *websocket.Conn
	send     chan []byte
	done     chan struct{}
	userID   string
	closeOne sync.Once
}

func (c *client) close() {
	c.closeOne.Do(func() { close(c.done) })
}

type wsMsg struct {
	Type   string   `json:"type"`
	Code   string   `json:"code,omitempty"`
	Action string   `json:"action,omitempty"`
	Amount int64    `json:"amount,omitempty"`
	Table  TableDTO `json:"table,omitempty"`
	Error  *string  `json:"error,omitempty"`
}

type wsClientMsg struct {
	Type   string `json:"type"`
	Action string `json:"action,omitempty"`
	Amount int64  `json:"amount,omitempty"`
}

type WSHandler struct {
	service  *pokerapp.Service
	sessions middleware.SessionParser
	log      *zap.Logger
	mu       sync.Mutex
	watchers map[*client][]string
}

func NewWSHandler(service *pokerapp.Service, sessions middleware.SessionParser, log *zap.Logger) *WSHandler {
	h := &WSHandler{
		service:  service,
		sessions: sessions,
		log:      log,
		watchers: make(map[*client][]string),
	}
	service.SetNotifier(func(code string, tb *poker.Table) {
		h.broadcast(code, tb)
	})
	return h
}

func (h *WSHandler) broadcast(code string, tb *poker.Table) {
	h.mu.Lock()
	clients := make([]*client, 0)
	for c, codes := range h.watchers {
		for _, c2 := range codes {
			if c2 == code {
				clients = append(clients, c)
				break
			}
		}
	}
	h.mu.Unlock()
	for _, c := range clients {
		payload, err := json.Marshal(wsMsg{Type: "table_state", Code: code, Table: toTableDTOFor(tb, c.userID, h.service.Bankroll(c.userID), h.service.TurnRemaining(code))})
		if err != nil {
			continue
		}
		select {
		case c.send <- payload:
		case <-c.done:
		default:
		}
	}
}

func (h *WSHandler) Connect(c echo.Context) error {
	code := c.Param("code")
	token := c.QueryParam("token")
	if token == "" {
		return fail(c, http.StatusUnauthorized, "unauthorized", "missing token")
	}
	claims, err := h.sessions.Parse(token)
	if err != nil {
		return fail(c, http.StatusUnauthorized, "unauthorized", "invalid session")
	}
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	cl := &client{conn: conn, send: make(chan []byte, 64), done: make(chan struct{}), userID: claims.PublicID}
	h.mu.Lock()
	h.watchers[cl] = []string{code}
	h.mu.Unlock()
	go h.writeLoop(cl)
	h.service.MarkPresent(code, claims.PublicID)
	h.join(code, cl)
	h.readLoop(code, claims.PublicID, cl)
	return nil
}

func (h *WSHandler) readLoop(code, userID string, cl *client) {
	defer func() {
		// Dropping the socket starts the away clock rather than seating the
		// player out immediately, so a backgrounded app or a brief network
		// blip does not cost them their seat mid-hand.
		h.service.MarkAway(code, userID)
		h.unwatch(cl)
		cl.close()
		_ = cl.conn.Close()
	}()
	for {
		_, data, err := cl.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg wsClientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == "act" {
			if err := h.service.Act(code, userID, msg.Action, msg.Amount); err != nil {
				h.sendError(cl, err)
			}
		}
	}
}

func (h *WSHandler) join(code string, cl *client) {
	tb, err := h.service.Get(code)
	if err != nil {
		h.sendError(cl, err)
		return
	}
	h.sendState(cl, code, tb)
}

func (h *WSHandler) unwatch(cl *client) {
	h.mu.Lock()
	delete(h.watchers, cl)
	h.mu.Unlock()
}

func (h *WSHandler) sendState(cl *client, code string, tb *poker.Table) {
	payload, _ := json.Marshal(wsMsg{Type: "table_state", Code: code, Table: toTableDTOFor(tb, cl.userID, h.service.Bankroll(cl.userID), h.service.TurnRemaining(code))})
	select {
	case cl.send <- payload:
	default:
	}
}

func (h *WSHandler) sendError(cl *client, err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	payload, _ := json.Marshal(wsMsg{Type: "error", Error: &msg})
	select {
	case cl.send <- payload:
	default:
	}
}

func (h *WSHandler) writeLoop(cl *client) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg := <-cl.send:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := cl.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				cl.close()
				return
			}
		case <-ticker.C:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				cl.close()
				return
			}
		case <-cl.done:
			return
		}
	}
}
