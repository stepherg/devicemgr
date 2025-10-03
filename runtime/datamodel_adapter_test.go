package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dm "github.com/xmidt-org/talaria/devicemgr"
)

func TestDataModelAdapterGet(t *testing.T) {
	// mock translation server
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("names") == "" {
			t.Fatalf("names query missing")
		}
		resp := map[string]any{
			"parameters": map[string]any{
				"Device.X.Sample": map[string]any{"value": 42, "timestamp": time.Now().UnixMilli()},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srvr.Close()

	ad, err := NewDataModelAdapter(DataModelOptions{BaseURL: srvr.URL, Service: "config"})
	if err != nil {
		t.Fatalf("build adapter: %v", err)
	}

	res, err := ad.Get(context.Background(), dm.DeviceID("mac:112233445566"), []string{"Device.X.Sample"}, dm.GetOptions{IncludeAttrs: false})
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(res.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(res.Values))
	}
}

func TestDataModelAdapterSet(t *testing.T) {
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["command"] == nil {
			t.Fatalf("command missing")
		}
		resp := map[string]any{"parameters": map[string]any{"Device.X.Sample": map[string]any{"value": 1}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srvr.Close()

	ad, err := NewDataModelAdapter(DataModelOptions{BaseURL: srvr.URL, Service: "config"})
	if err != nil {
		t.Fatalf("build adapter: %v", err)
	}
	params := []dm.SetParameter{{Name: "Device.X.Sample", Value: 100}}
	res, err := ad.Set(context.Background(), dm.DeviceID("mac:112233445566"), params, dm.SetOptions{})
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if len(res.Applied) != 1 {
		t.Fatalf("expected 1 applied, got %d", len(res.Applied))
	}
}

func TestDeviceAdapterObjectDevices(t *testing.T) {
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/devices" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"devices": []map[string]any{{"id": "mac:aa"}, {"deviceId": "mac:bb"}, {"mac": "mac:cc"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srvr.Close()
	da := NewDeviceAdapter(srvr.URL, nil)
	ids, err := da.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("poll once: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 ids, got %d", len(ids))
	}
}
