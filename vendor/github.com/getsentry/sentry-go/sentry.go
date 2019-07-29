package sentry

import (
	"context"
	"time"
)

// Version Sentry-Go SDK Version
const Version = "0.2.1"

// Init initializes whole SDK by creating new `Client` and binding it to the current `Hub`
func Init(options ClientOptions) error {
	hub := CurrentHub()
	client, err := NewClient(options)
	if err != nil {
		return err
	}
	hub.BindClient(client)
	return nil
}

// AddBreadcrumb records a new breadcrumb.
//
// The total number of breadcrumbs that can be recorded are limited by the
// configuration on the client.
func AddBreadcrumb(breadcrumb *Breadcrumb) {
	hub := CurrentHub()
	hub.AddBreadcrumb(breadcrumb, nil)
}

// CaptureMessage captures an arbitrary message.
func CaptureMessage(message string) *EventID {
	hub := CurrentHub()
	return hub.CaptureMessage(message)
}

// CaptureException captures an error.
func CaptureException(exception error) *EventID {
	hub := CurrentHub()
	return hub.CaptureException(exception)
}

// CaptureEvent captures an event on the currently active client if any.
//
// The event must already be assembled. Typically code would instead use
// the utility methods like `CaptureException`. The return value is the
// event ID. In case Sentry is disabled or event was dropped, the return value will be nil.
func CaptureEvent(event *Event) *EventID {
	hub := CurrentHub()
	return hub.CaptureEvent(event)
}

// Recover captures a panic.
func Recover() *EventID {
	if err := recover(); err != nil {
		hub := CurrentHub()
		return hub.Recover(err)
	}
	return nil
}

// Recover captures a panic and passes relevant context object.
func RecoverWithContext(ctx context.Context) *EventID {
	if err := recover(); err != nil {
		var hub *Hub

		if HasHubOnContext(ctx) {
			hub = GetHubFromContext(ctx)
		} else {
			hub = CurrentHub()
		}

		return hub.RecoverWithContext(ctx, err)
	}
	return nil
}

// WithScope temporarily pushes a scope for a single call.
//
// This function takes one argument, a callback that executes
// in the context of that scope.
//
// This is useful when extra data should be send with a single capture call
// for instance a different level or tags
func WithScope(f func(scope *Scope)) {
	hub := CurrentHub()
	hub.WithScope(f)
}

// ConfigureScope invokes a function that can modify the current scope.
//
// The function is passed a mutable reference to the `Scope` so that modifications
// can be performed.
func ConfigureScope(f func(scope *Scope)) {
	hub := CurrentHub()
	hub.ConfigureScope(f)
}

// PushScope pushes a new scope.
func PushScope() {
	hub := CurrentHub()
	hub.PushScope()
}

// PopScope pushes a new scope.
func PopScope() {
	hub := CurrentHub()
	hub.PopScope()
}

// Flush notifies when all the buffered events have been sent by returning `true`
// or `false` if timeout was reached.
func Flush(timeout time.Duration) bool {
	hub := CurrentHub()
	return hub.Flush(timeout)
}

// LastEventID returns an ID of last captured event.
func LastEventID() EventID {
	hub := CurrentHub()
	return hub.LastEventID()
}
