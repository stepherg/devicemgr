package policy

import (
	"context"
	"fmt"
	"time"
)

// FirmwareAdapter provides read-only access to firmware policies.
type FirmwareAdapter struct {
	c *Client
}

func NewFirmwareAdapter(c *Client) *FirmwareAdapter { return &FirmwareAdapter{c: c} }

// GetConfigByID fetches a firmware config by its ID.
func (f *FirmwareAdapter) GetConfigByID(ctx context.Context, id string) (*FirmwarePolicy, error) {
	var raw struct {
		ID      string `json:"id"`
		Version string `json:"firmwareVersion"`
		Model   string `json:"model"`
		URL     string `json:"firmwareDownloadProtocol"`
	}
	if err := f.c.getJSON(ctx, "/xconfAdminService/firmwareconfig/"+id, &raw); err != nil {
		return nil, err
	}
	return &FirmwarePolicy{ID: raw.ID, Version: raw.Version, Model: raw.Model, DownloadURL: raw.URL, RetrievedAt: time.Now()}, nil
}

// ResolveForModel returns the first config for a model (simplified: calls model list endpoint and finds first matching config).
func (f *FirmwareAdapter) ResolveForModel(ctx context.Context, model string) (*FirmwarePolicy, error) {
	// Simplified placeholder: real logic would query model-specific endpoint.
	var list []struct {
		ID              string `json:"id"`
		FirmwareVersion string `json:"firmwareVersion"`
		Model           string `json:"model"`
	}
	if err := f.c.getJSON(ctx, "/xconfAdminService/firmwareconfig", &list); err != nil {
		return nil, err
	}
	for _, item := range list {
		if item.Model == model {
			return &FirmwarePolicy{ID: item.ID, Version: item.FirmwareVersion, Model: item.Model, RetrievedAt: time.Now()}, nil
		}
	}
	return nil, fmt.Errorf("no firmware config for model %s", model)
}
