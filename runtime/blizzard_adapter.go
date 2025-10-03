package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/xmidt-org/talaria/devicemgr"
)

// BlizzardAdapter maintains a logical JSON-RPC channel to a device service exposed
// through a Parodus / WRP path (optionally via a gateway websocket).
// It does not (yet) implement automatic reconnect or backpressure metrics.
// JSON-RPC Request shape we send: {"jsonrpc":"2.0", "id":"<uuid>", "method":..., "params":...}
// Responses are matched by id. Notifications (no id) become events.
//
// Transport modes (current implementation):
//  - WebSocket direct to a gateway that forwards JSON-RPC to the device (recommended)
// Future extension: wrap raw WRP frames if gateway not available.

type BlizzardAdapter struct {
	baseWS   string // websocket base URL (e.g. wss://host/blizzard)
	auth     devicemgr.AuthStrategy
	deviceID string
	service  string

	dialer *websocket.Dialer
	connMu sync.RWMutex
	conn   *websocket.Conn

	pendingMu sync.Mutex
	pending   map[string]chan json.RawMessage

	listenersMu sync.RWMutex
	listeners   []*blizzardEventSub

	closed chan struct{}
}

// local eventSub mirrors implementation from device_adapter but keeps channel sendable
type blizzardEventSub struct {
	ch        chan devicemgr.Event
	closeOnce sync.Once
}

func (e *blizzardEventSub) C() <-chan devicemgr.Event { return e.ch }
func (e *blizzardEventSub) Close() error              { e.closeOnce.Do(func() { close(e.ch) }); return nil }

// BlizzardCall represents an outbound JSON-RPC call.
// Params should be JSON-marshalable.
type BlizzardCall struct {
	Method  string
	Params  interface{}
	Timeout time.Duration // optional; default 5s
}

// BlizzardResult contains a decoded JSON-RPC result or error struct.
type BlizzardResult struct {
	Result json.RawMessage
	Error  *RPCError
}

// RPCError models standard JSON-RPC error.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type jsonrpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// NewBlizzardAdapter creates a new adapter. baseWS should be a websocket URL prefix
// without trailing slash. DeviceID and service identify the logical endpoint.
func NewBlizzardAdapter(baseWS, deviceID, service string, auth devicemgr.AuthStrategy) *BlizzardAdapter {
	return &BlizzardAdapter{
		baseWS:   baseWS,
		auth:     auth,
		deviceID: deviceID,
		service:  service,
		dialer:   &websocket.Dialer{HandshakeTimeout: 10 * time.Second},
		pending:  make(map[string]chan json.RawMessage),
		closed:   make(chan struct{}),
	}
}

// Connect establishes the websocket.
func (b *BlizzardAdapter) Connect(ctx context.Context) error {
	u, err := url.Parse(b.baseWS)
	if err != nil {
		return err
	}
	// path join simplistic
	u.Path = fmt.Sprintf("%s/%s/%s", u.Path, b.deviceID, b.service)

	header := http.Header{}
	if b.auth != nil {
		if v, e := b.auth.AuthorizationValue(); e == nil && v != "" {
			header.Set("Authorization", v)
		}
	}
	conn, _, err := b.dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return err
	}
	b.connMu.Lock()
	b.conn = conn
	b.connMu.Unlock()
	go b.readLoop()
	return nil
}

// reconnect attempts a single reconnect using the same parameters.
func (b *BlizzardAdapter) reconnect(ctx context.Context) error {
	u, err := url.Parse(b.baseWS)
	if err != nil {
		return err
	}
	u.Path = fmt.Sprintf("%s/%s/%s", u.Path, b.deviceID, b.service)
	header := http.Header{}
	if b.auth != nil {
		if v, e := b.auth.AuthorizationValue(); e == nil && v != "" {
			header.Set("Authorization", v)
		}
	}
	conn, _, err := b.dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return err
	}
	b.connMu.Lock()
	if b.conn != nil {
		_ = b.conn.Close()
	}
	b.conn = conn
	b.connMu.Unlock()
	return nil
}

// Close terminates the connection and all pending calls.
func (b *BlizzardAdapter) Close() error {
	select {
	case <-b.closed:
		return nil
	default:
		close(b.closed)
	}
	b.connMu.Lock()
	c := b.conn
	b.conn = nil
	b.connMu.Unlock()
	if c != nil {
		_ = c.Close()
	}
	b.pendingMu.Lock()
	for id, ch := range b.pending {
		close(ch)
		delete(b.pending, id)
	}
	b.pendingMu.Unlock()
	b.listenersMu.Lock()
	b.listeners = nil
	b.listenersMu.Unlock()
	return nil
}

// Call issues a JSON-RPC request and waits for a response.
func (b *BlizzardAdapter) Call(ctx context.Context, call BlizzardCall) (*BlizzardResult, error) {
	if call.Method == "" {
		return nil, errors.New("method required")
	}
	if call.Timeout <= 0 {
		call.Timeout = 5 * time.Second
	}

	id := uuid.NewString()
	req := jsonrpcRequest{JSONRPC: "2.0", ID: id, Method: call.Method, Params: call.Params}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ch := make(chan json.RawMessage, 1)
	b.pendingMu.Lock()
	b.pending[id] = ch
	b.pendingMu.Unlock()

	b.connMu.RLock()
	c := b.conn
	b.connMu.RUnlock()
	if c == nil {
		return nil, errors.New("not connected")
	}
	if err = c.WriteMessage(websocket.TextMessage, payload); err != nil {
		b.pendingMu.Lock()
		delete(b.pending, id)
		b.pendingMu.Unlock()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, call.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		b.pendingMu.Lock()
		delete(b.pending, id)
		b.pendingMu.Unlock()
		return nil, ctx.Err()
	case respBytes, ok := <-ch:
		if !ok {
			return nil, errors.New("connection closed")
		}
		var resp jsonrpcResponse
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			return nil, err
		}
		return &BlizzardResult{Result: resp.Result, Error: resp.Error}, nil
	}
}

// Subscribe returns notifications (JSON-RPC messages without id) as events.
func (b *BlizzardAdapter) Subscribe(buffer int) devicemgr.EventSubscription {
	es := &blizzardEventSub{ch: make(chan devicemgr.Event, buffer)}
	b.listenersMu.Lock()
	b.listeners = append(b.listeners, es)
	b.listenersMu.Unlock()
	return es
}

func (b *BlizzardAdapter) broadcast(evt devicemgr.Event) {
	select {
	case <-b.closed:
		return
	default:
	}
	b.listenersMu.RLock()
	listeners := append([]*blizzardEventSub(nil), b.listeners...)
	b.listenersMu.RUnlock()
	for _, es := range listeners {
		if es == nil {
			continue
		}
		select {
		case es.ch <- evt:
		default:
		}
	}
}

func (b *BlizzardAdapter) readLoop() {
	b.connMu.RLock()
	c := b.conn
	b.connMu.RUnlock()
	if c == nil {
		return
	}
	retried := false
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			if !retried {
				retried = true
				b.broadcast(devicemgr.Event{Kind: devicemgr.EventOffline, DeviceID: devicemgr.DeviceID(b.deviceID), OccurredAt: time.Now(), Source: "blizzard-adapter", Payload: fmt.Sprintf("read error, retrying once: %v", err)})
				// brief delay then attempt reconnect
				time.Sleep(300 * time.Millisecond)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if recErr := b.reconnect(ctx); recErr == nil {
					cancel()
					b.connMu.RLock()
					c = b.conn
					b.connMu.RUnlock()
					if c == nil {
						_ = b.Close()
						return
					}
					continue
				}
				cancel()
			}
			b.broadcast(devicemgr.Event{Kind: devicemgr.EventOffline, DeviceID: devicemgr.DeviceID(b.deviceID), OccurredAt: time.Now(), Source: "blizzard-adapter", Payload: err.Error()})
			_ = b.Close()
			return
		}
		// Attempt to decode as response
		var resp jsonrpcResponse
		if err := json.Unmarshal(data, &resp); err == nil && resp.ID != "" && (resp.Result != nil || resp.Error != nil) {
			b.pendingMu.Lock()
			ch, found := b.pending[resp.ID]
			if found {
				delete(b.pending, resp.ID)
			}
			b.pendingMu.Unlock()
			if found {
				select {
				case ch <- data:
					close(ch)
				default:
					close(ch)
				}
			}
			continue
		}
		// If no ID -> notification
		var note jsonrpcNotification
		if err := json.Unmarshal(data, &note); err != nil || note.Method == "" {
			continue
		}
		// JSON-RPC notification
		b.broadcast(devicemgr.Event{Kind: devicemgr.EventNotification, DeviceID: devicemgr.DeviceID(b.deviceID), OccurredAt: time.Now(), Source: "blizzard-adapter", Payload: note})
	}
}
