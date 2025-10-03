package policy

import (
	"context"
	"errors"
)

type TelemetryAdapter struct{ c *Client }

func NewTelemetryAdapter(c *Client) *TelemetryAdapter { return &TelemetryAdapter{c: c} }
func (t *TelemetryAdapter) GetProfileByID(ctx context.Context, id string) (*TelemetryProfile, error) {
	return nil, errors.New("not implemented")
}
