package policy

import (
	"context"
	"errors"
)

type SettingsAdapter struct{ c *Client }

func NewSettingsAdapter(c *Client) *SettingsAdapter { return &SettingsAdapter{c: c} }
func (s *SettingsAdapter) GetProfileByID(ctx context.Context, id string) (*SettingsProfile, error) {
	return nil, errors.New("not implemented")
}
