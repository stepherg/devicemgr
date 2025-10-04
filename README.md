# Device Management Layer

This package provides a *client-side*, additive Device Management Layer (DML) that unifies:

* Talaria runtime device presence (via HTTP polling)
* Tr1d1um parameter GET/SET (WDMP construction client-side)
* xconfadmin policy data (future phases)

## Current Scope

### Phase 1 - Complete ✅

* Core types (`types.go`)
* Errors (`errors.go`)
* Options + defaults (`options.go`)
* Talaria device polling adapter with synthetic online/offline events (`runtime/device_adapter.go`)
* WDMP payload builders (`translate/wdmp.go`)
* DataModel adapter for Tr1d1um translation GET/SET (`runtime/datamodel_adapter.go`)
* Firmware policy adapter (initial read-only methods) + policy type scaffolding (`policy/`)
* Blizzard JSON-RPC adapter with WebSocket support (`runtime/blizzard_adapter.go`)
* Discovery API server (`cmd/devicemgr/main.go`)

### Phase 2 - Planned

* Settings / Telemetry / Feature policy adapters (replace stubs)
* Blizzard adapter enhancements (automatic reconnect with backoff, heartbeat/ping, metrics)
* Caching layer with TTL and freshness tagging
* Row/table WDMP operations (ADD_ROW, REPLACE_ROWS, DELETE_ROW)

### Phase 3 - Future

* Orchestrator for coordinating polling schedules
* Structured logging and metrics instrumentation
* OpenTelemetry tracing spans

## Local Development

Example instantiation:

```go
opts := devicemgr.DefaultOptions()
opts.TalariaBaseURL = "http://localhost:6200"
opts.Tr1d1umBaseURL = "http://localhost:6100/api/v3"
opts.XconfAdminBaseURL = "http://localhost:9001"
opts.Auth.Talaria = devicemgr.StaticAuth{Value: "Basic dXNlcjpwYXNz"}
opts.Auth.Tr1d1um = devicemgr.StaticAuth{Value: "Basic dXNlcjpwYXNz"}
opts.Auth.XconfAdmin = devicemgr.StaticAuth{Value: "Basic dXNlcjpwYXNz"}

adapter := runtime.NewDeviceAdapter(opts.TalariaBaseURL, opts.Auth.Talaria)
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
ids, err := adapter.PollOnce(ctx)
_ = ids; _ = err
sub := adapter.Subscribe(32)
// consume sub.C()

// Data model GET
dmAdapter, _ := runtime.NewDataModelAdapter(runtime.DataModelOptions{BaseURL: opts.Tr1d1umBaseURL, Service: "config", Auth: opts.Auth.Tr1d1um})
gr, err := dmAdapter.Get(ctx, devicemgr.DeviceID("mac:112233445566"), []string{"Device.X.Sample"}, devicemgr.GetOptions{})
_ = gr; _ = err

// Data model SET
_, _ = dmAdapter.Set(ctx, devicemgr.DeviceID("mac:112233445566"), []devicemgr.SetParameter{{Name: "Device.X.Sample", Value: 42}}, devicemgr.SetOptions{})

```

### Discovery API Server

You can run the included discovery API server (assumes local Talaria with Basic auth token `dXNlcjpwYXNz`):

```bash
cd cmd/devicemgr
go run .
```

Or build and run the binary:

```bash
go build -o bin/devicemgr ./cmd/devicemgr
./bin/devicemgr
```

Environment variables:

* `DEVICEMGR_DISCOVERY_ADDR` - Listen address (default: `:8090`)
* `DEVICEMGR_POLL_INTERVAL` - Polling interval (default: `15s`)

## Next Steps

1. Flesh out settings, telemetry, feature adapters (replace stubs)
2. Introduce orchestrator coordinating runtime + policy fetch scheduling
3. Add caching with TTL + stale allowances
4. Instrument metrics and tracing spans
5. Add row/table WDMP operations (Add/Replace/Delete rows)
6. Add Blizzard adapter reconnect, jitter backoff, heartbeat pings, metrics

## Blizzard JSON-RPC Adapter

Experimental adapter enabling JSON-RPC calls and notifications via a gateway WebSocket path (e.g. a Parodus/WRP aware proxy).

Basic usage:

```go
ba := runtime.NewBlizzardAdapter("wss://gateway/blizzard", "001122334455", "svc", devicemgr.StaticAuth{Value: "Bearer <token>"})
ctx := context.Background()
if err := ba.Connect(ctx); err != nil { panic(err) }
defer ba.Close()

sub := ba.Subscribe(16)
go func() {
    for evt := range sub.C() {
        // evt.Payload will contain jsonrpcNotification
    }
}()

res, err := ba.Call(ctx, runtime.BlizzardCall{Method: "Device.Ping", Params: map[string]any{"ts": time.Now().Unix()}})
if err != nil { /* handle */ }
if res.Error != nil { /* rpc level error */ }
fmt.Println(string(res.Result))
```

Current limitations:

* No automatic reconnect/backoff
* No heartbeat/ping; relies on underlying TCP
* Basic close semantics (no graceful drain of pending calls on transient error)

Implemented notification support: notifications now surface as `EventNotification`; request IDs are UUID v4.

Planned improvements:

* Reconnect with exponential backoff & max jitter
* Ping/pong liveness detection and metrics (in-flight count, latency histogram)
* Optional WRP frame mode (bypass gateway) using `wrp-go`
* Graceful pending-call cancellation on connection loss with structured error
* Heartbeat control method integration (see contract doc)

See `docs/blizzard_contract.md` for the evolving message contract.

## License

Apache-2.0
