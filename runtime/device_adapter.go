package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/xmidt-org/talaria/devicemgr"
)

// DeviceAdapter polls the Talaria /devices endpoint and provides synthetic events.
type DeviceAdapter struct {
	baseURL string
	client  *http.Client
	auth    devicemgr.AuthStrategy

	mu        sync.RWMutex
	lastIDs   map[string]struct{}
	listeners []chan devicemgr.Event
	lastPoll  time.Time
}

type talariaDevicesResponse struct {
	Devices json.RawMessage `json:"devices"` // can be array of strings or array of objects
}

func NewDeviceAdapter(baseURL string, auth devicemgr.AuthStrategy) *DeviceAdapter {
	return &DeviceAdapter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		auth:    auth,
		lastIDs: make(map[string]struct{}),
	}
}

// PollOnce fetches the current devices and emits synthetic online/offline events.
func (d *DeviceAdapter) PollOnce(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v2/devices", d.baseURL), nil)
	if err != nil {
		return nil, err
	}
	if d.auth != nil {
		if v, e := d.auth.AuthorizationValue(); e == nil {
			req.Header.Set("Authorization", v)
		}
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var parsed talariaDevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	// attempt to parse devices
	var rawAny []interface{}
	if err := json.Unmarshal(parsed.Devices, &rawAny); err != nil {
		return nil, fmt.Errorf("unexpected devices format: %w", err)
	}
	ids := make([]string, 0, len(rawAny))
	for _, elem := range rawAny {
		switch v := elem.(type) {
		case string:
			ids = append(ids, v)
		case map[string]interface{}:
			// Accept common keys
			for _, k := range []string{"id", "deviceId", "deviceID", "mac"} {
				if val, ok := v[k]; ok {
					if s, ok := val.(string); ok && s != "" {
						ids = append(ids, s)
						break
					}
				}
			}
		}
	}
	d.emitDiff(ids)
	return ids, nil
}

func (d *DeviceAdapter) emitDiff(current []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	currSet := make(map[string]struct{}, len(current))
	for _, id := range current {
		currSet[id] = struct{}{}
	}
	d.lastPoll = time.Now()
	// online events
	for id := range currSet {
		if _, existed := d.lastIDs[id]; !existed {
			d.broadcast(devicemgr.Event{Kind: devicemgr.EventOnline, DeviceID: devicemgr.DeviceID(id), OccurredAt: time.Now(), Source: "synthetic-poll"})
		}
	}
	// offline events
	for id := range d.lastIDs {
		if _, still := currSet[id]; !still {
			d.broadcast(devicemgr.Event{Kind: devicemgr.EventOffline, DeviceID: devicemgr.DeviceID(id), OccurredAt: time.Now(), Source: "synthetic-poll"})
		}
	}
	d.lastIDs = currSet
}

func (d *DeviceAdapter) broadcast(e devicemgr.Event) {
	for _, ch := range d.listeners {
		select {
		case ch <- e:
		default: /* drop if slow */
		}
	}
}

// Snapshot returns current known device IDs plus last poll time.
func (d *DeviceAdapter) Snapshot() (ids []string, lastPoll time.Time) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ids = make([]string, 0, len(d.lastIDs))
	for id := range d.lastIDs {
		ids = append(ids, id)
	}
	return ids, d.lastPoll
}

// Subscribe returns an event subscription channel.
func (d *DeviceAdapter) Subscribe(buffer int) devicemgr.EventSubscription {
	ch := make(chan devicemgr.Event, buffer)
	d.mu.Lock()
	d.listeners = append(d.listeners, ch)
	d.mu.Unlock()
	return &eventSub{ch: ch, closeFn: func() { close(ch) }}
}

type eventSub struct {
	ch      <-chan devicemgr.Event
	closeFn func()
}

func (e *eventSub) C() <-chan devicemgr.Event { return e.ch }
func (e *eventSub) Close() error {
	if e.closeFn != nil {
		e.closeFn()
		e.closeFn = nil
	}
	return nil
}
