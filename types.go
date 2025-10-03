package devicemgr

import "time"

type DeviceID string

type Freshness int

const (
	FreshRealTime Freshness = iota
	FreshRecentCache
	FreshStale
)

type DeviceState struct {
	ID          DeviceID
	Online      bool
	ConnectedAt *time.Time
	Metadata    map[string]string
	Freshness   Freshness
	Source      string
}

type ParameterValue struct {
	Name        string
	Value       interface{}
	Type        string
	Attributes  map[string]interface{}
	RetrievedAt time.Time
	Freshness   Freshness
}

type SetParameter struct {
	Name       string
	Value      interface{}
	TypeHint   string
	Attributes map[string]interface{}
}

type CASCondition struct {
	OldCID string
	NewCID string
}

type GetOptions struct {
	Names        []string
	IncludeAttrs bool
}

type SetOptions struct {
	TestAndSet *CASCondition
	Atomic     bool
}

type EventKind string

const (
	EventOnline       EventKind = "online"
	EventOffline      EventKind = "offline"
	EventNotification EventKind = "notification"
)

type Event struct {
	Kind       EventKind
	DeviceID   DeviceID
	OccurredAt time.Time
	Source     string
	Payload    interface{}
}

type EventSubscription interface {
	C() <-chan Event
	Close() error
}
