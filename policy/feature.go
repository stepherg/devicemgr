package policy

import (
	"context"
	"errors"
)

type FeatureAdapter struct{ c *Client }

func NewFeatureAdapter(c *Client) *FeatureAdapter { return &FeatureAdapter{c: c} }
func (f *FeatureAdapter) GetFlags(ctx context.Context, ids []string) (*FeatureFlags, error) {
	return nil, errors.New("not implemented")
}
