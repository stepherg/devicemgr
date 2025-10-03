package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	api "github.com/xmidt-org/talaria/devicemgr/internal/http"
	"github.com/xmidt-org/talaria/devicemgr/runtime"
)

// DiscoveryConfig configures the discovery (device listing) HTTP server.
type DiscoveryConfig struct {
	ListenAddr    string                 // address to bind (e.g. :8090)
	DeviceAdapter *runtime.DeviceAdapter // required
	Logger        *log.Logger            // optional; defaults to log.Default()
	ReadTimeout   time.Duration          // optional
	WriteTimeout  time.Duration          // optional
	IdleTimeout   time.Duration          // optional
}

var ErrNilAdapter = errors.New("discovery server: device adapter is nil")

// StartDiscoveryServer starts an HTTP server exposing /api/devices using the provided adapter.
// It returns the *http.Server, a channel that will receive a terminal error (if any), and an error for immediate startup issues.
// The server stops when the supplied context is canceled.
func StartDiscoveryServer(ctx context.Context, cfg DiscoveryConfig) (*http.Server, <-chan error, error) {
	if cfg.DeviceAdapter == nil {
		return nil, nil, ErrNilAdapter
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8090"
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/devices", api.DevicesHandler(cfg.DeviceAdapter))

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  durationOr(cfg.ReadTimeout, 10*time.Second),
		WriteTimeout: durationOr(cfg.WriteTimeout, 10*time.Second),
		IdleTimeout:  durationOr(cfg.IdleTimeout, 60*time.Second),
	}

	errCh := make(chan error, 1)

	go func() {
		cfg.Logger.Printf("discovery API listening on %s (GET /api/devices)", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Shutdown watcher
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	return srv, errCh, nil
}

func durationOr(v time.Duration, d time.Duration) time.Duration {
	if v <= 0 {
		return d
	}
	return v
}
