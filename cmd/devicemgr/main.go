package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	dm "github.com/xmidt-org/talaria/devicemgr"
	"github.com/xmidt-org/talaria/devicemgr/internal/server"
	"github.com/xmidt-org/talaria/devicemgr/runtime"
)

// devicemgr: pure discovery API server. It starts the /api/devices endpoint and waits for shutdown.
func main() {
	auth := dm.StaticAuth{Value: "Basic dXNlcjpwYXNz"}
	deviceAdapter := runtime.NewDeviceAdapter("http://talaria:6200", auth)

	addr := os.Getenv("DEVICEMGR_DISCOVERY_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	// Initial poll to seed snapshot
	if _, err := deviceAdapter.PollOnce(context.Background()); err != nil {
		log.Printf("initial poll failed: %v", err)
	}

	// Periodic polling loop (default interval 15s, configurable via DEVICEMGR_POLL_INTERVAL seconds)
	interval := 15 * time.Second
	if v := os.Getenv("DEVICEMGR_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}
	ctxPoll, cancelPoll := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := deviceAdapter.PollOnce(context.Background()); err != nil {
					log.Printf("poll error: %v", err)
				}
			case <-ctxPoll.Done():
				return
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	_, errCh, err := server.StartDiscoveryServer(ctx, server.DiscoveryConfig{ListenAddr: addr, DeviceAdapter: deviceAdapter})
	if err != nil {
		log.Fatalf("failed to start discovery API: %v", err)
	}
	go func() {
		if err := <-errCh; err != nil {
			log.Printf("discovery API error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("devicemgr discovery API running on %s (GET /api/devices)", addr)
	<-sigCh
	log.Printf("shutdown signal received; stopping server")
	cancelPoll()
	cancel()
}
