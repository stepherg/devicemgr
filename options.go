package devicemgr

import (
	"time"
)

// AuthStrategy acquires an authorization header value (e.g., "Basic ..." or "Bearer ...").
type AuthStrategy interface {
	AuthorizationValue() (string, error)
}

// StaticAuth implements AuthStrategy using a pre-specified token value.
type StaticAuth struct{ Value string }

func (s StaticAuth) AuthorizationValue() (string, error) { return s.Value, nil }

// Options configures the Device Management Layer.
type Options struct {
	TalariaBaseURL    string
	Tr1d1umBaseURL    string // should include /api/v3 prefix
	XconfAdminBaseURL string

	Auth struct {
		Talaria    AuthStrategy
		Tr1d1um    AuthStrategy
		XconfAdmin AuthStrategy
	}

	Services []string // valid tr1d1um translation services

	Polling PollingConfig
	Cache   CacheConfig
}

type PollingConfig struct {
	DeviceList       time.Duration
	FirmwarePolicies time.Duration
	Settings         time.Duration
	Telemetry        time.Duration
	Features         time.Duration
	Rollout          time.Duration
	Global           time.Duration
}

type CacheConfig struct {
	DeviceStateTTL  time.Duration
	ParamTTL        time.Duration
	PolicyTTL       time.Duration
	StaleAcceptable time.Duration
}

// DefaultOptions gives baseline sensible defaults for local dev.
func DefaultOptions() Options {
	opts := Options{}
	opts.Polling = PollingConfig{
		DeviceList:       15 * time.Second,
		FirmwarePolicies: 60 * time.Second,
		Settings:         90 * time.Second,
		Telemetry:        60 * time.Second,
		Features:         60 * time.Second,
		Rollout:          30 * time.Second,
		Global:           120 * time.Second,
	}
	opts.Cache = CacheConfig{
		DeviceStateTTL:  10 * time.Second,
		ParamTTL:        5 * time.Second,
		PolicyTTL:       60 * time.Second,
		StaleAcceptable: 5 * time.Second,
	}
	return opts
}
