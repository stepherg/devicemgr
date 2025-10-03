package policy

import "time"

// FirmwarePolicy represents a resolved firmware configuration for a device.
type FirmwarePolicy struct {
	ID                string            `json:"id,omitempty"`
	Version           string            `json:"version,omitempty"`
	Model             string            `json:"model,omitempty"`
	DownloadURL       string            `json:"downloadUrl,omitempty"`
	Required          bool              `json:"required,omitempty"`
	RebootImmediately bool              `json:"rebootImmediately,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	RetrievedAt       time.Time         `json:"retrievedAt"`
}

// SettingsProfile represents settings profile content (opaque payload retained for higher layers).
type SettingsProfile struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Application string                 `json:"applicationType,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	RetrievedAt time.Time              `json:"retrievedAt"`
}

// TelemetryProfile placeholder for telemetry configuration.
type TelemetryProfile struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Schedule    map[string]interface{} `json:"schedule,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
	RetrievedAt time.Time              `json:"retrievedAt"`
}

// FeatureFlags represents RFC feature enablement state.
type FeatureFlags struct {
	Flags       map[string]bool `json:"flags"`
	RetrievedAt time.Time       `json:"retrievedAt"`
}
