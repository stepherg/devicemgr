package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/xmidt-org/talaria/devicemgr/runtime"
)

// DeviceInfo represents a device in discovery response.
// Minimal for UI selection; can be extended later.
type DeviceInfo struct {
	ID       string    `json:"id"`
	Online   bool      `json:"online"`
	LastSeen time.Time `json:"lastSeen,omitempty"`
}

// DevicesHandler builds an HTTP handler serving current devices snapshot.
func DevicesHandler(adapter *runtime.DeviceAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, last := adapter.Snapshot()
		out := struct {
			Devices  []DeviceInfo `json:"devices"`
			Count    int          `json:"count"`
			LastPoll time.Time    `json:"lastPoll"`
		}{LastPoll: last}
		out.Devices = make([]DeviceInfo, 0, len(ids))
		for _, id := range ids {
			out.Devices = append(out.Devices, DeviceInfo{ID: id, Online: true, LastSeen: last})
		}
		out.Count = len(out.Devices)
		w.Header().Set("Content-Type", "application/json")
		writeCORS(w)
		json.NewEncoder(w).Encode(out)
	}
}

func writeCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}
