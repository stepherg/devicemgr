package policy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dm "github.com/xmidt-org/talaria/devicemgr"
)

type staticAuth struct{ v string }

func (s staticAuth) AuthorizationValue() (string, error) { return s.v, nil }

func TestFirmwareGetConfigByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xconfAdminService/firmwareconfig/fw123" {
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "fw123", "firmwareVersion": "1.2.3", "model": "X1"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Auth: staticAuth{""}}
	fa := NewFirmwareAdapter(c)
	fp, err := fa.GetConfigByID(context.Background(), "fw123")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fp.ID != "fw123" || fp.Version != "1.2.3" || fp.Model != "X1" {
		t.Fatalf("unexpected firmware policy: %+v", fp)
	}
	_, err = fa.GetConfigByID(context.Background(), "missing")
	if err == nil || err != dm.ErrPolicyNotFound {
		t.Fatalf("expected ErrPolicyNotFound got %v", err)
	}
}
