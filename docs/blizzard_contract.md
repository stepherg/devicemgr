# Blizzard JSON-RPC Message Contract (Draft)

Status: Draft / Incremental – aligns with current `BlizzardAdapter` implementation.

## Scope
Defines client <-> gateway <-> device logical contract for JSON-RPC 2.0 messages delivered over WebSocket (and optionally encapsulated within WRP when traversing Parodus).

## Addressing
Gateway WebSocket URL pattern (example):
```
wss://<host>/blizzard/{deviceID}/{service}
```
- `deviceID`: canonical 12-hex MAC (no separators) OR `mac:001122334455` (gateway normalizes).
- `service`: logical micro-service / agent capability name inside the device runtime.

WRP (future optional):
- Source: `dml:<cluster>` (or configured)
- Destination: `mac:001122334455/{service}`
- Content-Type: `application/json`

## Authentication
HTTP `Authorization` header forwarded (e.g., `Bearer <token>`). Gateway determines authorization policy; adapter is opaque.

## JSON-RPC Envelope
All requests MUST follow JSON-RPC 2.0:
```json
{
  "jsonrpc": "2.0",
  "id": "<uuid-v4>",
  "method": "Namespace.Action",
  "params": { ... }
}
```
- `id`: UUID v4 (string) – unique per outstanding call.
- `method`: hierarchical segments separated by '.' (recommend max depth 3).
- `params`: object or array; MAY be omitted.

### Responses
```json
{
  "jsonrpc": "2.0",
  "id": "<same-id>",
  "result": { ... }
}
```
OR
```json
{
  "jsonrpc": "2.0",
  "id": "<same-id>",
  "error": { "code": <int>, "message": "...", "data": { ... } }
}
```
- Exactly one of `result` or `error` present.
- Adapter surfaces both via `BlizzardResult` (Result or Error).

### Notifications
Server/device-originated messages without `id`:
```json
{
  "jsonrpc": "2.0",
  "method": "Device.Event",
  "params": { "event": "foo", "ts": 1738539200 }
}
```
Mapped to `devicemgr.Event` with `Kind=notification` (new) and `Payload=jsonrpcNotification` struct.

## Error Codes (Proposed)
| Range | Usage |
|-------|-------|
| -32768 .. -32000 | Reserved (JSON-RPC spec) |
| -32000 | Internal device runtime error |
| -32001 | Method not found (runtime) |
| -32002 | Validation / schema error |
| -32003 | Unauthorized (device-level ACL) |
| -32004 | Busy (concurrency throttle) |
| -32005 | Timeout (operation expired inside device) |
| -32100 .. -32199 | Transport / gateway injected (e.g., auth failure, routing) |

Implementers SHOULD include structured `data` with fields:
```json
{"cause":"<string>","details":{...},"retryable":true}
```

## Correlation
- `id` echoes the request.
- For notifications referencing a prior request (asynchronous completion), include `params.correlationId` referencing the original `id`.

## Backpressure & Flow Control
Current draft: best-effort buffering; gateway SHOULD apply per-connection send queue limits.
Future: optional window acknowledgements via a control method `Control.Ack`.

## Heartbeats (Future)
Reserved method `Control.Ping` (request) / `Control.Pong` (response or notification) with example payload:
```json
{"jsonrpc":"2.0","method":"Control.Pong","params":{"ts":<unixNano>}}
```

## Security Considerations
- Enforce auth at gateway edge.
- Rate-limit per deviceID/service tuple.
- Validate `method` whitelist server-side to prevent arbitrary reflection.
- Size limits: request & notification payload <= 256KB recommended.

## Versioning
Add a top-level optional header (WRP metadata or WebSocket subprotocol) `X-Blizzard-Version: 1`. Breaking changes increment this (2, 3...).

## Open Items
- Formal schema for `params` per method (JSON Schema bundle).
- Batch request support (JSON-RPC array) – currently NOT supported.
- WRP encapsulation mapping (add `blizzard/jsonrpc` content-type alias?).

---
This document will evolve alongside adapter feature additions (reconnect, heartbeat, metrics).
