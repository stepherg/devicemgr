package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	dm "github.com/xmidt-org/talaria/devicemgr"
	"github.com/xmidt-org/talaria/devicemgr/translate"
)

// DataModelAdapter performs TR-181 style GET/SET operations via Tr1d1um's translation endpoints.
// It constructs WDMP payloads using the clean-room builders and issues HTTP requests against
// the public translation API surface (no direct WRP coupling here – Tr1d1um handles that).
//
// Supported operations in this initial version:
//   - GET / GET_ATTRIBUTES (batch names)
//   - SET / SET_ATTRIBUTES (multiple parameters)
//
// Row/table operations (ADD_ROW, REPLACE_ROWS, DELETE_ROW) can be layered in later by reusing
// the existing builders. Interface exposes only core parameter ops for now.
type DataModelAdapter struct {
	client  *http.Client
	baseURL string // e.g. http://tr1d1um:6100/api/v3
	auth    dm.AuthStrategy
	service string // translation service name (maps to {service} path component)
}

// DataModelOptions configures a new adapter.
type DataModelOptions struct {
	BaseURL        string
	Service        string
	Client         *http.Client
	Auth           dm.AuthStrategy
	RequestTimeout time.Duration
}

// NewDataModelAdapter builds a DataModelAdapter.
func NewDataModelAdapter(o DataModelOptions) (*DataModelAdapter, error) {
	if o.BaseURL == "" {
		return nil, errors.New("BaseURL required")
	}
	if o.Service == "" {
		return nil, errors.New("Service required")
	}
	c := o.Client
	if c == nil {
		c = &http.Client{Timeout: func() time.Duration {
			if o.RequestTimeout > 0 {
				return o.RequestTimeout
			}
			return 15 * time.Second
		}()}
	}
	return &DataModelAdapter{client: c, baseURL: strings.TrimRight(o.BaseURL, "/"), auth: o.Auth, service: o.Service}, nil
}

// GetResult models a consolidated response from a GET/GET_ATTRIBUTES call.
type GetResult struct {
	// Values maps parameter name -> ParameterValue (value + timestamp + freshness) when present.
	Values map[string]dm.ParameterValue
	// RawPayload keeps the raw device JSON payload (opaque to this layer) for callers needing extras.
	RawPayload json.RawMessage
}

// SetResult captures outcome of a SET/SET_ATTRIBUTES call.
type SetResult struct {
	// Modified names that were applied successfully (best-effort parse from response if present).
	Applied []string
	// RawPayload original response.
	RawPayload json.RawMessage
}

// Get issues a multi-name GET or GET_ATTRIBUTES (when opts.Attributes != "").
func (a *DataModelAdapter) Get(ctx context.Context, deviceID dm.DeviceID, names []string, opts dm.GetOptions) (*GetResult, error) {
	if len(names) == 0 {
		return nil, errors.New("names required")
	}
	// Build query per translation transport expectations: names=comma,separated; attributes flag when IncludeAttrs
	q := url.Values{}
	q.Set("names", strings.Join(names, ","))
	if opts.IncludeAttrs {
		// empty value indicates attributes parameter presence selects GET_ATTRIBUTES variant
		q.Set("attributes", "*")
	}
	endpoint := fmt.Sprintf("%s/device/%s/%s?%s", a.baseURL, url.PathEscape(string(deviceID)), url.PathEscape(a.service), q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if a.auth != nil {
		if h, err := a.auth.AuthorizationValue(); err == nil && h != "" {
			req.Header.Set("Authorization", h)
		}
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, dm.ErrDeviceNotFound
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, dm.ErrAccessDenied
	}
	if resp.StatusCode >= 500 {
		return nil, dm.ErrBackendUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Attempt to parse a minimal WDMP-style value map. Different services may vary; we keep it lenient.
	// Common pattern: { "parameters": { "Device.Param": {"value":X,"timestamp":123} } }
	var probe struct {
		Parameters map[string]struct {
			Value     interface{} `json:"value"`
			Timestamp int64       `json:"timestamp"`
		} `json:"parameters"`
	}
	_ = json.Unmarshal(body, &probe) // best effort

	result := &GetResult{Values: map[string]dm.ParameterValue{}, RawPayload: json.RawMessage(body)}
	for name, v := range probe.Parameters {
		result.Values[name] = dm.ParameterValue{
			Name:        name,
			Value:       v.Value,
			RetrievedAt: time.Unix(0, v.Timestamp*int64(time.Millisecond)),
			Freshness:   dm.FreshRecentCache, // cannot differentiate precisely; treat as recent cache
		}
	}
	return result, nil
}

// Set issues a SET or SET_ATTRIBUTES based on supplied parameters.
func (a *DataModelAdapter) Set(ctx context.Context, deviceID dm.DeviceID, params []dm.SetParameter, opts dm.SetOptions) (*SetResult, error) {
	if len(params) == 0 {
		return nil, errors.New("params required")
	}
	// Build WDMP payload JSON using builders.
	payload, err := translate.BuildSet(params, opts.TestAndSet)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/device/%s/%s", a.baseURL, url.PathEscape(string(deviceID)), url.PathEscape(a.service))
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.auth != nil {
		if h, err := a.auth.AuthorizationValue(); err == nil && h != "" {
			req.Header.Set("Authorization", h)
		}
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, dm.ErrDeviceNotFound
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, dm.ErrAccessDenied
	}
	if resp.StatusCode == http.StatusConflict {
		return nil, dm.ErrConflict
	}
	if resp.StatusCode >= 500 {
		return nil, dm.ErrBackendUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// Attempt parse applied names: some responses echo parameters or status structures.
	var probe struct {
		Parameters map[string]any `json:"parameters"`
	}
	_ = json.Unmarshal(body, &probe)
	var applied []string
	for name := range probe.Parameters {
		applied = append(applied, name)
	}
	return &SetResult{Applied: applied, RawPayload: json.RawMessage(body)}, nil
}
