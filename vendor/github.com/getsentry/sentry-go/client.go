package sentry

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"sort"
	"time"
)

// Logger is an instance of log.Logger that is use to provide debug information about running Sentry Client
// can be enabled by either using `Logger.SetOutput` directly or with `Debug` client option
var Logger = log.New(ioutil.Discard, "[Sentry] ", log.LstdFlags) // nolint: gochecknoglobals

type EventProcessor func(event *Event, hint *EventHint) *Event

type EventModifier interface {
	ApplyToEvent(event *Event, hint *EventHint) *Event
}

var globalEventProcessors []EventProcessor // nolint: gochecknoglobals

func AddGlobalEventProcessor(processor EventProcessor) {
	globalEventProcessors = append(globalEventProcessors, processor)
}

// Integration allows for registering a functions that modify or discard captured events.
type Integration interface {
	Name() string
	SetupOnce(client *Client)
}

// ClientOptions that configures a SDK Client
type ClientOptions struct {
	// The DSN to use. If the DSN is not set, the client is effectively disabled.
	Dsn string
	// In debug mode, the debug information is printed to stdout to help you understand what
	// sentry is doing.
	Debug bool
	// Configures whether SDK should generate and attach stacktraces to pure capture message calls.
	AttachStacktrace bool
	// The sample rate for event submission (0.0 - 1.0, defaults to 1.0).
	SampleRate float32
	// List of regexp strings that will be used to match against event's message
	// and if applicable, caught errors type and value.
	// If the match is found, then a whole event will be dropped.
	IgnoreErrors []string
	// Before send callback.
	BeforeSend func(event *Event, hint *EventHint) *Event
	// Before breadcrumb add callback.
	BeforeBreadcrumb func(breadcrumb *Breadcrumb, hint *BreadcrumbHint) *Breadcrumb
	// Integrations to be installed on the current Client, receives default integrations
	Integrations func([]Integration) []Integration
	// io.Writer implementation that should be used with the `Debug` mode
	DebugWriter io.Writer
	// The transport to use.
	// This is an instance of a struct implementing `Transport` interface.
	// Defaults to `httpTransport` from `transport.go`
	Transport Transport
	// The server name to be reported.
	ServerName string
	// The release to be sent with events.
	Release string
	// The dist to be sent with events.
	Dist string
	// The environment to be sent with events.
	Environment string
	// Maximum number of breadcrumbs.
	MaxBreadcrumbs int
	// An optional pointer to `http.Transport` that will be used with a default HTTPTransport.
	HTTPTransport *http.Transport
	// An optional HTTP proxy to use.
	// This will default to the `http_proxy` environment variable.
	// or `https_proxy` if that one exists.
	HTTPProxy string
	// An optional HTTPS proxy to use.
	// This will default to the `HTTPS_PROXY` environment variable
	// or `http_proxy` if that one exists.
	HTTPSProxy string
	// An optionsl CaCerts to use.
	// Defaults to `gocertifi.CACerts()`.
	CaCerts *x509.CertPool
}

// Client is the underlying processor that's used by the main API and `Hub` instances.
type Client struct {
	options         ClientOptions
	dsn             *Dsn
	eventProcessors []EventProcessor
	integrations    []Integration
	Transport       Transport
}

// NewClient creates and returns an instance of `Client` configured using `ClientOptions`.
func NewClient(options ClientOptions) (*Client, error) {
	if options.Debug {
		debugWriter := options.DebugWriter
		if debugWriter == nil {
			debugWriter = os.Stdout
		}
		Logger.SetOutput(debugWriter)
	}

	if options.Dsn == "" {
		options.Dsn = os.Getenv("SENTRY_DSN")
	}

	if options.Release == "" {
		options.Release = os.Getenv("SENTRY_RELEASE")
	}

	if options.Environment == "" {
		options.Environment = os.Getenv("SENTRY_ENVIRONMENT")
	}

	var dsn *Dsn
	if options.Dsn != "" {
		var err error
		dsn, err = NewDsn(options.Dsn)
		if err != nil {
			return nil, err
		}
	}

	client := Client{
		options: options,
		dsn:     dsn,
	}

	client.setupTransport()
	client.setupIntegrations()

	return &client, nil
}

func (client *Client) setupTransport() {
	transport := client.options.Transport

	if transport == nil {
		if client.options.Dsn == "" {
			transport = new(noopTransport)
		} else {
			transport = NewHTTPTransport()
		}
	}

	transport.Configure(client.options)
	client.Transport = transport
}

func (client *Client) setupIntegrations() {
	integrations := []Integration{
		new(contextifyFramesIntegration),
		new(environmentIntegration),
		new(modulesIntegration),
		new(ignoreErrorsIntegration),
	}

	if client.options.Integrations != nil {
		integrations = client.options.Integrations(integrations)
	}

	for _, integration := range integrations {
		if client.integrationAlreadyInstalled(integration.Name()) {
			Logger.Printf("Integration %s is already installed\n", integration.Name())
			continue
		}
		client.integrations = append(client.integrations, integration)
		integration.SetupOnce(client)
		Logger.Printf("Integration installed: %s\n", integration.Name())
	}
}

// AddEventProcessor adds an event processor to the client.
func (client *Client) AddEventProcessor(processor EventProcessor) {
	client.eventProcessors = append(client.eventProcessors, processor)
}

// Options return `ClientOptions` for the current `Client`.
func (client Client) Options() ClientOptions {
	return client.options
}

// CaptureMessage captures an arbitrary message.
func (client *Client) CaptureMessage(message string, hint *EventHint, scope EventModifier) *EventID {
	event := client.eventFromMessage(message, LevelInfo)
	return client.CaptureEvent(event, hint, scope)
}

// CaptureException captures an error.
func (client *Client) CaptureException(exception error, hint *EventHint, scope EventModifier) *EventID {
	event := client.eventFromException(exception, LevelError)
	return client.CaptureEvent(event, hint, scope)
}

// CaptureEvent captures an event on the currently active client if any.
//
// The event must already be assembled. Typically code would instead use
// the utility methods like `CaptureException`. The return value is the
// event ID. In case Sentry is disabled or event was dropped, the return value will be nil.
func (client *Client) CaptureEvent(event *Event, hint *EventHint, scope EventModifier) *EventID {
	return client.processEvent(event, hint, scope)
}

// Recover captures a panic.
// Returns `EventID` if successfully, or `nil` if there's no error to recover from.
func (client *Client) Recover(err interface{}, hint *EventHint, scope EventModifier) *EventID {
	if err == nil {
		err = recover()
	}

	if err != nil {
		if err, ok := err.(error); ok {
			event := client.eventFromException(err, LevelFatal)
			return client.CaptureEvent(event, hint, scope)
		}

		if err, ok := err.(string); ok {
			event := client.eventFromMessage(err, LevelFatal)
			return client.CaptureEvent(event, hint, scope)
		}
	}

	return nil
}

// Recover captures a panic and passes relevant context object.
// Returns `EventID` if successfully, or `nil` if there's no error to recover from.
func (client *Client) RecoverWithContext(
	ctx context.Context,
	err interface{},
	hint *EventHint,
	scope EventModifier,
) *EventID {
	if err == nil {
		err = recover()
	}

	if err != nil {
		if hint.Context == nil && ctx != nil {
			hint.Context = ctx
		}

		if err, ok := err.(error); ok {
			event := client.eventFromException(err, LevelFatal)
			return client.CaptureEvent(event, hint, scope)
		}

		if err, ok := err.(string); ok {
			event := client.eventFromMessage(err, LevelFatal)
			return client.CaptureEvent(event, hint, scope)
		}
	}

	return nil
}

// Flush notifies when all the buffered events have been sent by returning `true`
// or `false` if timeout was reached. It calls `Flush` method of the configured `Transport`.
func (client *Client) Flush(timeout time.Duration) bool {
	return client.Transport.Flush(timeout)
}

func (client *Client) eventFromMessage(message string, level Level) *Event {
	event := NewEvent()
	event.Level = level
	event.Message = message

	if client.Options().AttachStacktrace {
		event.Threads = []Thread{{
			Stacktrace: NewStacktrace(),
			Crashed:    false,
			Current:    true,
		}}
	}

	return event
}

func (client *Client) eventFromException(exception error, level Level) *Event {
	if exception == nil {
		event := NewEvent()
		event.Level = level
		event.Message = fmt.Sprintf("Called %s with nil value", callerFunctionName())
		return event
	}

	stacktrace := ExtractStacktrace(exception)

	if stacktrace == nil {
		stacktrace = NewStacktrace()
	}

	event := NewEvent()
	event.Level = level
	event.Exception = []Exception{{
		Value:      exception.Error(),
		Type:       reflect.TypeOf(exception).String(),
		Stacktrace: stacktrace,
	}}
	return event
}

func (client *Client) processEvent(event *Event, hint *EventHint, scope EventModifier) *EventID {
	options := client.Options()

	// TODO: Reconsider if its worth going away from default implementation
	// of other SDKs. In Go zero value (default) for float32 is 0.0,
	// which means that if someone uses ClientOptions{} struct directly
	// and we would not check for 0 here, we'd skip all events by default
	if options.SampleRate != 0.0 {
		randomFloat := rand.New(rand.NewSource(time.Now().UnixNano())).Float32()
		if randomFloat > options.SampleRate {
			Logger.Println("Event dropped due to SampleRate hit.")
			return nil
		}
	}

	if event = client.prepareEvent(event, hint, scope); event == nil {
		return nil
	}

	if options.BeforeSend != nil {
		h := &EventHint{}
		if hint != nil {
			h = hint
		}
		if event = options.BeforeSend(event, h); event == nil {
			Logger.Println("Event dropped due to BeforeSend callback.")
			return nil
		}
	}

	client.Transport.SendEvent(event)

	return &event.EventID
}

func (client *Client) prepareEvent(event *Event, hint *EventHint, scope EventModifier) *Event {
	if event.EventID == "" {
		event.EventID = EventID(uuid())
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	if event.Level == "" {
		event.Level = LevelInfo
	}

	if event.ServerName == "" {
		if client.Options().ServerName != "" {
			event.ServerName = client.Options().ServerName
		} else if hostname, err := os.Hostname(); err == nil {
			event.ServerName = hostname
		}
	}

	if event.Release == "" && client.Options().Release != "" {
		event.Release = client.Options().Release
	}

	if event.Dist == "" && client.Options().Dist != "" {
		event.Dist = client.Options().Dist
	}

	if event.Environment == "" && client.Options().Environment != "" {
		event.Environment = client.Options().Environment
	}

	event.Platform = "go"
	event.Sdk = SdkInfo{
		Name:         "sentry.go",
		Version:      Version,
		Integrations: client.listIntegrations(),
		Packages: []SdkPackage{{
			Name:    "sentry-go",
			Version: Version,
		}},
	}

	event = scope.ApplyToEvent(event, hint)

	for _, processor := range client.eventProcessors {
		id := event.EventID
		event = processor(event, hint)
		if event == nil {
			Logger.Printf("Event dropped by one of the Client EventProcessors: %s\n", id)
			return nil
		}
	}

	for _, processor := range globalEventProcessors {
		id := event.EventID
		event = processor(event, hint)
		if event == nil {
			Logger.Printf("Event dropped by one of the Global EventProcessors: %s\n", id)
			return nil
		}
	}

	return event
}

func (client Client) listIntegrations() []string {
	integrations := make([]string, 0, len(client.integrations))
	for _, integration := range client.integrations {
		integrations = append(integrations, integration.Name())
	}
	sort.Strings(integrations)
	return integrations
}

func (client Client) integrationAlreadyInstalled(name string) bool {
	for _, integration := range client.integrations {
		if integration.Name() == name {
			return true
		}
	}
	return false
}
