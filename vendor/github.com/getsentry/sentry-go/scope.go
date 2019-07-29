package sentry

import (
	"reflect"
	"sync"
	"time"
)

// Scope holds contextual data for the current scope.
//
// The scope is an object that can cloned efficiently and stores data that
// is locally relevant to an event.  For instance the scope will hold recorded
// breadcrumbs and similar information.
//
// The scope can be interacted with in two ways:
//
// 1. the scope is routinely updated with information by functions such as
//    `AddBreadcrumb` which will modify the currently top-most scope.
// 2. the topmost scope can also be configured through the `ConfigureScope`
//    method.
//
// Note that the scope can only be modified but not inspected.
// Only the client can use the scope to extract information currently.
type Scope struct {
	sync.RWMutex
	breadcrumbs     []*Breadcrumb
	user            User
	tags            map[string]string
	contexts        map[string]interface{}
	extra           map[string]interface{}
	fingerprint     []string
	level           Level
	transaction     string
	request         Request
	eventProcessors []EventProcessor
}

func NewScope() *Scope {
	scope := Scope{
		breadcrumbs: make([]*Breadcrumb, 0),
		tags:        make(map[string]string),
		contexts:    make(map[string]interface{}),
		extra:       make(map[string]interface{}),
		fingerprint: make([]string, 0),
	}

	return &scope
}

// AddBreadcrumb adds new breadcrumb to the current scope
// and optionaly throws the old one if limit is reached.
func (scope *Scope) AddBreadcrumb(breadcrumb *Breadcrumb, limit int) {
	if breadcrumb.Timestamp == 0 {
		breadcrumb.Timestamp = time.Now().Unix()
	}

	scope.Lock()
	defer scope.Unlock()

	breadcrumbs := append(scope.breadcrumbs, breadcrumb)
	if len(breadcrumbs) > limit {
		scope.breadcrumbs = breadcrumbs[1 : limit+1]
	} else {
		scope.breadcrumbs = breadcrumbs
	}
}

// ClearBreadcrumbs clears all breadcrumbs from the current scope.
func (scope *Scope) ClearBreadcrumbs() {
	scope.breadcrumbs = []*Breadcrumb{}
}

// SetUser sets new user for the current scope.
func (scope *Scope) SetUser(user User) {
	scope.user = user
}

// SetRequest sets new user for the current scope.
func (scope *Scope) SetRequest(request Request) {
	scope.request = request
}

// SetTag adds a tag to the current scope.
func (scope *Scope) SetTag(key, value string) {
	scope.Lock()
	defer scope.Unlock()

	scope.tags[key] = value
}

// SetTags assigns multiple tags to the current scope.
func (scope *Scope) SetTags(tags map[string]string) {
	scope.Lock()
	defer scope.Unlock()

	for k, v := range tags {
		scope.tags[k] = v
	}
}

// RemoveTag removes a tag from the current scope.
func (scope *Scope) RemoveTag(key string) {
	scope.Lock()
	defer scope.Unlock()

	delete(scope.tags, key)
}

// SetContext adds a context to the current scope.
func (scope *Scope) SetContext(key string, value interface{}) {
	scope.Lock()
	defer scope.Unlock()

	scope.contexts[key] = value
}

// SetContexts assigns multiple contexts to the current scope.
func (scope *Scope) SetContexts(contexts map[string]interface{}) {
	scope.Lock()
	defer scope.Unlock()

	for k, v := range contexts {
		scope.contexts[k] = v
	}
}

// RemoveContext removes a context from the current scope.
func (scope *Scope) RemoveContext(key string) {
	scope.Lock()
	defer scope.Unlock()

	delete(scope.contexts, key)
}

// SetExtra adds an extra to the current scope.
func (scope *Scope) SetExtra(key string, value interface{}) {
	scope.Lock()
	defer scope.Unlock()

	scope.extra[key] = value
}

// SetExtras assigns multiple extras to the current scope.
func (scope *Scope) SetExtras(extra map[string]interface{}) {
	scope.Lock()
	defer scope.Unlock()

	for k, v := range extra {
		scope.extra[k] = v
	}
}

// RemoveExtra removes a extra from the current scope.
func (scope *Scope) RemoveExtra(key string) {
	scope.Lock()
	defer scope.Unlock()

	delete(scope.extra, key)
}

// SetFingerprint sets new fingerprint for the current scope.
func (scope *Scope) SetFingerprint(fingerprint []string) {
	scope.fingerprint = fingerprint
}

// SetLevel sets new level for the current scope.
func (scope *Scope) SetLevel(level Level) {
	scope.level = level
}

// SetTransaction sets new transaction name for the current transaction.
func (scope *Scope) SetTransaction(transactionName string) {
	scope.transaction = transactionName
}

// Clone returns a copy of the current scope with all data copied over.
func (scope *Scope) Clone() *Scope {
	scope.RLock()
	defer scope.RUnlock()

	clone := NewScope()
	clone.user = scope.user
	clone.breadcrumbs = make([]*Breadcrumb, len(scope.breadcrumbs))
	copy(clone.breadcrumbs, scope.breadcrumbs)
	for key, value := range scope.tags {
		clone.tags[key] = value
	}
	for key, value := range scope.contexts {
		clone.contexts[key] = value
	}
	for key, value := range scope.extra {
		clone.extra[key] = value
	}
	clone.fingerprint = make([]string, len(scope.fingerprint))
	copy(clone.fingerprint, scope.fingerprint)
	clone.level = scope.level
	clone.transaction = scope.transaction
	clone.request = scope.request

	return clone
}

// Clear removed the data from the current scope.
func (scope *Scope) Clear() {
	*scope = *NewScope()
}

// AddEventProcessor adds an event processor to the current scope.
func (scope *Scope) AddEventProcessor(processor EventProcessor) {
	scope.Lock()
	defer scope.Unlock()

	scope.eventProcessors = append(scope.eventProcessors, processor)
}

// ApplyToEvent takes the data from the current scope and attaches it to the event.
func (scope *Scope) ApplyToEvent(event *Event, hint *EventHint) *Event {
	scope.RLock()
	defer scope.RUnlock()

	if len(scope.breadcrumbs) > 0 {
		if event.Breadcrumbs == nil {
			event.Breadcrumbs = []*Breadcrumb{}
		}

		event.Breadcrumbs = append(event.Breadcrumbs, scope.breadcrumbs...)
	}

	if len(scope.tags) > 0 {
		if event.Tags == nil {
			event.Tags = make(map[string]string)
		}

		for key, value := range scope.tags {
			event.Tags[key] = value
		}
	}

	if len(scope.contexts) > 0 {
		if event.Contexts == nil {
			event.Contexts = make(map[string]interface{})
		}

		for key, value := range scope.contexts {
			event.Contexts[key] = value
		}
	}

	if len(scope.extra) > 0 {
		if event.Extra == nil {
			event.Extra = make(map[string]interface{})
		}

		for key, value := range scope.extra {
			event.Extra[key] = value
		}
	}

	if (reflect.DeepEqual(event.User, User{})) {
		event.User = scope.user
	}

	if (event.Fingerprint == nil || len(event.Fingerprint) == 0) &&
		len(scope.fingerprint) > 0 {
		event.Fingerprint = make([]string, len(scope.fingerprint))
		copy(event.Fingerprint, scope.fingerprint)
	}

	if scope.level != "" {
		event.Level = scope.level
	}

	if scope.transaction != "" {
		event.Transaction = scope.transaction
	}

	if (reflect.DeepEqual(event.Request, Request{})) {
		event.Request = scope.request
	}

	for _, processor := range scope.eventProcessors {
		id := event.EventID
		event = processor(event, hint)
		if event == nil {
			Logger.Printf("Event dropped by one of the Scope EventProcessors: %s\n", id)
			return nil
		}
	}

	return event
}
