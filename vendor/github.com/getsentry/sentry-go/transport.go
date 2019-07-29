package sentry

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const defaultBufferSize = 30
const defaultRetryAfter = time.Second * 60
const defaultTimeout = time.Second * 30

// Transport is used by the `Client` to deliver events to remote server.
type Transport interface {
	Flush(timeout time.Duration) bool
	Configure(options ClientOptions)
	SendEvent(event *Event)
}

func getProxyConfig(options ClientOptions) func(*http.Request) (*url.URL, error) {
	if options.HTTPSProxy != "" {
		return func(_ *http.Request) (*url.URL, error) {
			return url.Parse(options.HTTPSProxy)
		}
	} else if options.HTTPProxy != "" {
		return func(_ *http.Request) (*url.URL, error) {
			return url.Parse(options.HTTPProxy)
		}
	}

	return http.ProxyFromEnvironment
}

func getTLSConfig(options ClientOptions) *tls.Config {
	if options.CaCerts != nil {
		return &tls.Config{
			RootCAs: options.CaCerts,
		}
	}

	return nil
}

func retryAfter(now time.Time, r *http.Response) time.Duration {
	retryAfterHeader := r.Header["Retry-After"]

	if retryAfterHeader == nil {
		return defaultRetryAfter
	}

	if date, err := time.Parse(time.RFC1123, retryAfterHeader[0]); err == nil {
		return date.Sub(now)
	}

	if seconds, err := strconv.Atoi(retryAfterHeader[0]); err == nil {
		return time.Second * time.Duration(seconds)
	}

	return defaultRetryAfter
}

// ================================
// HTTPTransport
// ================================

// HTTPTransport is a default implementation of `Transport` interface used by `Client`.
type HTTPTransport struct {
	dsn       *Dsn
	client    *http.Client
	transport *http.Transport

	buffer        chan *http.Request
	disabledUntil time.Time

	wg    sync.WaitGroup
	start sync.Once

	// Size of the transport buffer. Defaults to 30.
	BufferSize int
	// HTTP Client request timeout. Defaults to 30 seconds.
	Timeout time.Duration
}

// NewHTTPTransport returns a new pre-configured instance of HTTPTransport
func NewHTTPTransport() *HTTPTransport {
	transport := HTTPTransport{
		BufferSize: defaultBufferSize,
		Timeout:    defaultTimeout,
	}
	return &transport
}

// Configure is called by the `Client` itself, providing it it's own `ClientOptions`.
func (t *HTTPTransport) Configure(options ClientOptions) {
	dsn, err := NewDsn(options.Dsn)
	if err != nil {
		Logger.Printf("%v\n", err)
		return
	}

	t.dsn = dsn
	t.buffer = make(chan *http.Request, t.BufferSize)

	if options.HTTPTransport != nil {
		t.transport = options.HTTPTransport
	} else {
		t.transport = &http.Transport{
			Proxy:           getProxyConfig(options),
			TLSClientConfig: getTLSConfig(options),
		}
	}

	t.client = &http.Client{
		Transport: t.transport,
		Timeout:   t.Timeout,
	}

	t.start.Do(func() {
		go t.worker()
	})
}

// SendEvent assembles a new packet out of `Event` and sends it to remote server.
func (t *HTTPTransport) SendEvent(event *Event) {
	if t.dsn == nil || time.Now().Before(t.disabledUntil) {
		return
	}

	body, _ := json.Marshal(event)

	request, _ := http.NewRequest(
		http.MethodPost,
		t.dsn.StoreAPIURL().String(),
		bytes.NewBuffer(body),
	)

	for headerKey, headerValue := range t.dsn.RequestHeaders() {
		request.Header.Set(headerKey, headerValue)
	}

	t.wg.Add(1)

	select {
	case t.buffer <- request:
		Logger.Printf(
			"Sending %s event [%s] to %s project: %d\n",
			event.Level,
			event.EventID,
			t.dsn.host,
			t.dsn.projectID,
		)
	default:
		t.wg.Done()
		Logger.Println("Event dropped due to transport buffer being full.")
		// worker would block, drop the packet
	}
}

// Flush notifies when all the buffered events have been sent by returning `true`
// or `false` if timeout was reached.
func (t *HTTPTransport) Flush(timeout time.Duration) bool {
	c := make(chan struct{})

	go func() {
		t.wg.Wait()
		close(c)
	}()

	select {
	case <-c:
		Logger.Println("Buffer flushed successfully.")
		return true
	case <-time.After(timeout):
		Logger.Println("Buffer flushing reached the timeout.")
		return false
	}
}

func (t *HTTPTransport) worker() {
	for request := range t.buffer {
		if time.Now().Before(t.disabledUntil) {
			t.wg.Done()
			continue
		}

		response, err := t.client.Do(request)

		if err != nil {
			Logger.Printf("There was an issue with sending an event: %v", err)
		}

		if response != nil && response.StatusCode == http.StatusTooManyRequests {
			t.disabledUntil = time.Now().Add(retryAfter(time.Now(), response))
			Logger.Printf("Too many requests, backing off till: %s\n", t.disabledUntil)
		}

		t.wg.Done()
	}
}

// ================================
// HTTPSyncTransport
// ================================

// HTTPSyncTransport is an implementation of `Transport` interface which blocks after each captured event.
type HTTPSyncTransport struct {
	dsn           *Dsn
	client        *http.Client
	transport     *http.Transport
	disabledUntil time.Time

	// HTTP Client request timeout. Defaults to 30 seconds.
	Timeout time.Duration
}

// NewHTTPSyncTransport returns a new pre-configured instance of HTTPSyncTransport
func NewHTTPSyncTransport() *HTTPSyncTransport {
	transport := HTTPSyncTransport{
		Timeout: defaultTimeout,
	}

	return &transport
}

// Configure is called by the `Client` itself, providing it it's own `ClientOptions`.
func (t *HTTPSyncTransport) Configure(options ClientOptions) {
	dsn, err := NewDsn(options.Dsn)
	if err != nil {
		Logger.Printf("%v\n", err)
		return
	}
	t.dsn = dsn

	if options.HTTPTransport != nil {
		t.transport = options.HTTPTransport
	} else {
		t.transport = &http.Transport{
			Proxy:           getProxyConfig(options),
			TLSClientConfig: getTLSConfig(options),
		}
	}

	t.client = &http.Client{
		Transport: t.transport,
		Timeout:   t.Timeout,
	}
}

// SendEvent assembles a new packet out of `Event` and sends it to remote server.
func (t *HTTPSyncTransport) SendEvent(event *Event) {
	if t.dsn == nil || time.Now().Before(t.disabledUntil) {
		return
	}

	body, _ := json.Marshal(event)

	request, _ := http.NewRequest(
		http.MethodPost,
		t.dsn.StoreAPIURL().String(),
		bytes.NewBuffer(body),
	)

	for headerKey, headerValue := range t.dsn.RequestHeaders() {
		request.Header.Set(headerKey, headerValue)
	}

	Logger.Printf(
		"Sending %s event [%s] to %s project: %d\n",
		event.Level,
		event.EventID,
		t.dsn.host,
		t.dsn.projectID,
	)

	response, err := t.client.Do(request)

	if err != nil {
		Logger.Printf("There was an issue with sending an event: %v", err)
	}

	if response != nil && response.StatusCode == http.StatusTooManyRequests {
		t.disabledUntil = time.Now().Add(retryAfter(time.Now(), response))
		Logger.Printf("Too many requests, backing off till: %s\n", t.disabledUntil)
	}
}

// Flush notifies when all the buffered events have been sent by returning `true`
// or `false` if timeout was reached. No-op for HTTPSyncTransport.
func (t *HTTPSyncTransport) Flush(_ time.Duration) bool {
	return true
}

// ================================
// noopTransport
// ================================

// noopTransport is an implementation of `Transport` interface which drops all the events.
// Only used internally when an empty DSN is provided, which effectively disables the SDK.
type noopTransport struct{}

func (t *noopTransport) Configure(options ClientOptions) {
	Logger.Println("Sentry client initialized with an empty DSN. Using noopTransport. No events will be delivered.")
}

func (t *noopTransport) SendEvent(event *Event) {
	Logger.Println("Event dropped due to noopTransport usage.")
}

func (t *noopTransport) Flush(_ time.Duration) bool {
	return true
}
