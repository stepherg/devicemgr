package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	dm "github.com/xmidt-org/talaria/devicemgr"
)

// Client is a lightweight helper around http.Client for xconfadmin calls.
type Client struct {
	BaseURL string
	Auth    dm.AuthStrategy
	HTTP    *http.Client
}

func NewClient(baseURL string, auth dm.AuthStrategy) *Client {
	return &Client{BaseURL: trimRightSlash(baseURL), Auth: auth, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func trimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// getJSON performs an HTTP GET and decodes JSON into out; returns sentinel errors from devicemgr where feasible.
func (c *Client) getJSON(ctx context.Context, path string, out interface{}) error {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	if c.Auth != nil {
		if v, e := c.Auth.AuthorizationValue(); e == nil && v != "" {
			req.Header.Set("Authorization", v)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case http.StatusOK:
		if out != nil {
			if err := json.Unmarshal(b, out); err != nil {
				return fmt.Errorf("decode: %w", err)
			}
		}
		return nil
	case http.StatusNotFound:
		return dm.ErrPolicyNotFound
	case http.StatusForbidden:
		return dm.ErrAccessDenied
	case http.StatusConflict:
		return dm.ErrConflict
	default:
		if resp.StatusCode >= 500 {
			return dm.ErrBackendUnavailable
		}
		return errors.New(resp.Status)
	}
}
