package sentry

import (
	"context"
	"sync"
	"time"
)

type contextKey int

// HubContextKey is a context key used to store Hub on any context.Context type
const HubContextKey = contextKey(1)

// RequestContextKey is a context key used to store http.Request on the context passed to RecoverWithContext
const RequestContextKey = contextKey(2)

// Default maximum number of breadcrumbs added to an event. Can be overwritten `maxBreadcrumbs` option.
const defaultMaxBreadcrumbs = 30

// Absolute maximum number of breadcrumbs added to an event.
// The `maxBreadcrumbs` option cannot be higher than this value.
const maxBreadcrumbs = 100

// Initial instance of the Hub that has no `Client` bound and an empty `Scope`
var currentHub = NewHub(nil, NewScope()) // nolint: gochecknoglobals

// Hub is the central object that can manages scopes and clients.
//
// This can be used to capture events and manage the scope.
// The default hub that is available automatically.
//
// In most situations developers do not need to interface the hub. Instead
// toplevel convenience functions are exposed that will automatically dispatch
// to global (`CurrentHub`) hub.  In some situations this might not be
// possible in which case it might become necessary to manually work with the
// hub. This is for instance the case when working with async code.
type Hub struct {
	sync.RWMutex
	stack       *stack
	lastEventID EventID
}

type layer struct {
	client *Client
	scope  *Scope
}

type stack []*layer

// NewHub returns an instance of a `Hub` with provided `Client` and `Scope` bound.
func NewHub(client *Client, scope *Scope) *Hub {
	hub := Hub{
		stack: &stack{{
			client: client,
			scope:  scope,
		}},
	}
	return &hub
}

// CurrentHub returns an instance of previously initialized `Hub` stored in the global namespace.
func CurrentHub() *Hub {
	return currentHub
}

// LastEventID returns an ID of last captured event for the current `Hub`.
func (hub *Hub) LastEventID() EventID {
	return hub.lastEventID
}

func (hub *Hub) stackTop() *layer {
	hub.RLock()
	defer hub.RUnlock()

	stack := hub.stack
	if stack == nil {
		return nil
	}

	stackLen := len(*stack)
	if stackLen == 0 {
		return nil
	}
	top := (*stack)[stackLen-1]

	return top
}

// Clone returns a copy of the current Hub with top-most scope and client copied over.
func (hub *Hub) Clone() *Hub {
	top := hub.stackTop()
	if top == nil {
		return nil
	}
	scope := top.scope
	if scope != nil {
		scope = scope.Clone()
	}
	return NewHub(top.client, scope)
}

// Scope returns top-level `Scope` of the current `Hub` or `nil` if no `Scope` is bound.
func (hub *Hub) Scope() *Scope {
	top := hub.stackTop()
	if top == nil {
		return nil
	}
	return top.scope
}

// Scope returns top-level `Client` of the current `Hub` or `nil` if no `Client` is bound.
func (hub *Hub) Client() *Client {
	top := hub.stackTop()
	if top == nil {
		return nil
	}
	return top.client
}

// PushScope pushes a new scope for the current `Hub` and reuses previously bound `Client`.
func (hub *Hub) PushScope() *Scope {
	top := hub.stackTop()

	var client *Client
	if top != nil {
		client = top.client
	}

	var scope *Scope
	if top != nil && top.scope != nil {
		scope = top.scope.Clone()
	} else {
		scope = NewScope()
	}

	hub.Lock()
	defer hub.Unlock()

	*hub.stack = append(*hub.stack, &layer{
		client: client,
		scope:  scope,
	})

	return scope
}

// PushScope pops the most recent scope for the current `Hub`.
func (hub *Hub) PopScope() {
	hub.Lock()
	defer hub.Unlock()

	stack := *hub.stack
	stackLen := len(stack)
	if stackLen > 0 {
		*hub.stack = stack[0 : stackLen-1]
	}
}

// BindClient binds a new `Client` for the current `Hub`.
func (hub *Hub) BindClient(client *Client) {
	top := hub.stackTop()
	if top != nil {
		top.client = client
	}
}

// WithScope temporarily pushes a scope for a single call.
//
// A shorthand for:
// PushScope()
// f(scope)
// PopScope()
func (hub *Hub) WithScope(f func(scope *Scope)) {
	scope := hub.PushScope()
	defer hub.PopScope()
	f(scope)
}

// ConfigureScope invokes a function that can modify the current scope.
//
// The function is passed a mutable reference to the `Scope` so that modifications
// can be performed.
func (hub *Hub) ConfigureScope(f func(scope *Scope)) {
	scope := hub.Scope()
	f(scope)
}

// CaptureEvent calls the method of a same name on currently bound `Client` instance
// passing it a top-level `Scope`.
// Returns `EventID` if successfully, or `nil` if there's no `Scope` or `Client` available.
func (hub *Hub) CaptureEvent(event *Event) *EventID {
	client, scope := hub.Client(), hub.Scope()
	if client == nil || scope == nil {
		return nil
	}
	return client.CaptureEvent(event, nil, scope)
}

// CaptureMessage calls the method of a same name on currently bound `Client` instance
// passing it a top-level `Scope`.
// Returns `EventID` if successfully, or `nil` if there's no `Scope` or `Client` available.
func (hub *Hub) CaptureMessage(message string) *EventID {
	client, scope := hub.Client(), hub.Scope()
	if client == nil || scope == nil {
		return nil
	}
	return client.CaptureMessage(message, nil, scope)
}

// CaptureException calls the method of a same name on currently bound `Client` instance
// passing it a top-level `Scope`.
// Returns `EventID` if successfully, or `nil` if there's no `Scope` or `Client` available.
func (hub *Hub) CaptureException(exception error) *EventID {
	client, scope := hub.Client(), hub.Scope()
	if client == nil || scope == nil {
		return nil
	}
	return client.CaptureException(exception, &EventHint{OriginalException: exception}, scope)
}

// AddBreadcrumb records a new breadcrumb.
//
// The total number of breadcrumbs that can be recorded are limited by the
// configuration on the client.
func (hub *Hub) AddBreadcrumb(breadcrumb *Breadcrumb, hint *BreadcrumbHint) {
	client := hub.Client()

	// If there's no client, just store it on the scope straight away
	if client == nil {
		hub.Scope().AddBreadcrumb(breadcrumb, maxBreadcrumbs)
		return
	}

	options := client.Options()
	max := defaultMaxBreadcrumbs

	if options.MaxBreadcrumbs != 0 {
		max = options.MaxBreadcrumbs
	}

	if max < 0 {
		return
	}

	if options.BeforeBreadcrumb != nil {
		h := &BreadcrumbHint{}
		if hint != nil {
			h = hint
		}
		if breadcrumb = options.BeforeBreadcrumb(breadcrumb, h); breadcrumb == nil {
			Logger.Println("breadcrumb dropped due to BeforeBreadcrumb callback.")
			return
		}
	}

	if max > maxBreadcrumbs {
		max = maxBreadcrumbs
	}
	hub.Scope().AddBreadcrumb(breadcrumb, max)
}

// Recover calls the method of a same name on currently bound `Client` instance
// passing it a top-level `Scope`.
// Returns `EventID` if successfully, or `nil` if there's no `Scope` or `Client` available.
func (hub *Hub) Recover(err interface{}) *EventID {
	if err == nil {
		err = recover()
	}
	client, scope := hub.Client(), hub.Scope()
	if client == nil || scope == nil {
		return nil
	}
	return client.Recover(err, &EventHint{RecoveredException: err}, scope)
}

// RecoverWithContext calls the method of a same name on currently bound `Client` instance
// passing it a top-level `Scope`.
// Returns `EventID` if successfully, or `nil` if there's no `Scope` or `Client` available.
func (hub *Hub) RecoverWithContext(ctx context.Context, err interface{}) *EventID {
	if err == nil {
		err = recover()
	}
	client, scope := hub.Client(), hub.Scope()
	if client == nil || scope == nil {
		return nil
	}
	return client.RecoverWithContext(ctx, err, &EventHint{RecoveredException: err}, scope)
}

// Flush calls the method of a same name on currently bound `Client` instance.
func (hub *Hub) Flush(timeout time.Duration) bool {
	client := hub.Client()

	if client == nil {
		return false
	}

	return client.Flush(timeout)
}

// HasHubOnContext checks whether `Hub` instance is bound to a given `Context` struct.
func HasHubOnContext(ctx context.Context) bool {
	_, ok := ctx.Value(HubContextKey).(*Hub)
	return ok
}

// GetHubFromContext tries to retrieve `Hub` instance from the given `Context` struct
// or return `nil` if one is not found.
func GetHubFromContext(ctx context.Context) *Hub {
	if hub, ok := ctx.Value(HubContextKey).(*Hub); ok {
		return hub
	}
	return nil
}

// SetHubOnContext stores given `Hub` instance on the `Context` struct and returns a new `Context`.
func SetHubOnContext(ctx context.Context, hub *Hub) context.Context {
	return context.WithValue(ctx, HubContextKey, hub)
}
