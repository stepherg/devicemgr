package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xmidt-org/talaria/devicemgr"
)

// simple upgrader server that echoes json-rpc request
func TestBlizzardAdapterCallAndNotify(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var notifySent bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		go func() {
			// send a notification after a short delay
			time.Sleep(50 * time.Millisecond)
			n := jsonrpcNotification{JSONRPC: "2.0", Method: "device.event", Params: json.RawMessage(`{"x":1}`)}
			b, _ := json.Marshal(n)
			_ = c.WriteMessage(websocket.TextMessage, b)
			notifySent = true
		}()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var req jsonrpcRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				t.Errorf("bad req: %v", err)
				return
			}
			resp := jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
			b, _ := json.Marshal(resp)
			_ = c.WriteMessage(websocket.TextMessage, b)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	ad := NewBlizzardAdapter(u.String(), "001122334455", "svc", devicemgr.StaticAuth{Value: ""})
	ctx := context.Background()
	if err := ad.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer ad.Close()

	sub := ad.Subscribe(4)
	defer sub.Close()

	res, err := ad.Call(ctx, BlizzardCall{Method: "ping", Params: map[string]interface{}{"a": 1}, Timeout: time.Second})
	if err != nil {
		t.Fatalf("call err: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", res.Error)
	}
	if string(res.Result) == "" {
		t.Fatalf("empty result")
	}

	select {
	case evt := <-sub.C():
		if evt.Kind != devicemgr.EventNotification {
			t.Fatalf("expected notification kind, got %s", evt.Kind)
		}
		if evt.Payload == nil {
			t.Fatalf("expected payload in notification event")
		}
	case <-time.After(500 * time.Millisecond):
		if !notifySent {
			t.Fatalf("did not receive notification")
		}
	}
}
