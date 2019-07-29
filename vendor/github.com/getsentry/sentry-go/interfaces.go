package sentry

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

// Protocol Docs (kinda)
// https://github.com/getsentry/rust-sentry-types/blob/master/src/protocol/v7.rs

// Level marks the severity of the event
type Level string

const (
	LevelDebug   Level = "debug"
	LevelInfo    Level = "info"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
	LevelFatal   Level = "fatal"
)

// https://docs.sentry.io/development/sdk-dev/interfaces/sdk/
type SdkInfo struct {
	Name         string       `json:"name,omitempty"`
	Version      string       `json:"version,omitempty"`
	Integrations []string     `json:"integrations,omitempty"`
	Packages     []SdkPackage `json:"packages,omitempty"`
}

type SdkPackage struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// TODO: This type could be more useful, as map of interface{} is too generic
// and requires a lot of type assertions in beforeBreadcrumb calls
// plus it could just be `map[string]interface{}` then
type BreadcrumbHint map[string]interface{}

// https://docs.sentry.io/development/sdk-dev/interfaces/breadcrumbs/
type Breadcrumb struct {
	Category  string                 `json:"category,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Level     Level                  `json:"level,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	Type      string                 `json:"type,omitempty"`
}

// https://docs.sentry.io/development/sdk-dev/interfaces/user/
type User struct {
	Email     string `json:"email,omitempty"`
	ID        string `json:"id,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	Username  string `json:"username,omitempty"`
}

// https://docs.sentry.io/development/sdk-dev/interfaces/http/
type Request struct {
	URL         string            `json:"url,omitempty"`
	Method      string            `json:"method,omitempty"`
	Data        string            `json:"data,omitempty"`
	QueryString string            `json:"query_string,omitempty"`
	Cookies     string            `json:"cookies,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

func (r Request) FromHTTPRequest(request *http.Request) Request {
	// Method
	r.Method = request.Method

	// URL
	protocol := schemeHTTP
	if request.TLS != nil || request.Header.Get("X-Forwarded-Proto") == "https" {
		protocol = schemeHTTPS
	}
	r.URL = fmt.Sprintf("%s://%s%s", protocol, request.Host, request.URL.Path)

	// Headers
	headers := make(map[string]string, len(request.Header))
	for k, v := range request.Header {
		headers[k] = strings.Join(v, ",")
	}
	headers["Host"] = request.Host
	r.Headers = headers

	// Cookies
	r.Cookies = request.Header.Get("Cookie")

	// Env
	if addr, port, err := net.SplitHostPort(request.RemoteAddr); err == nil {
		r.Env = map[string]string{"REMOTE_ADDR": addr, "REMOTE_PORT": port}
	}

	// QueryString
	r.QueryString = request.URL.RawQuery

	// Body
	if request.GetBody != nil {
		if bodyCopy, err := request.GetBody(); err == nil && bodyCopy != nil {
			body, err := ioutil.ReadAll(bodyCopy)
			if err == nil {
				r.Data = string(body)
			}
		}
	}

	return r
}

// https://docs.sentry.io/development/sdk-dev/interfaces/exception/
type Exception struct {
	Type          string      `json:"type,omitempty"`
	Value         string      `json:"value,omitempty"`
	Module        string      `json:"module,omitempty"`
	Stacktrace    *Stacktrace `json:"stacktrace,omitempty"`
	RawStacktrace *Stacktrace `json:"raw_stacktrace,omitempty"`
}

type EventID string

// https://docs.sentry.io/development/sdk-dev/attributes/
type Event struct {
	Breadcrumbs []*Breadcrumb          `json:"breadcrumbs,omitempty"`
	Contexts    map[string]interface{} `json:"contexts,omitempty"`
	Dist        string                 `json:"dist,omitempty"`
	Environment string                 `json:"environment,omitempty"`
	EventID     EventID                `json:"event_id,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
	Fingerprint []string               `json:"fingerprint,omitempty"`
	Level       Level                  `json:"level,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Platform    string                 `json:"platform,omitempty"`
	Release     string                 `json:"release,omitempty"`
	Sdk         SdkInfo                `json:"sdk,omitempty"`
	ServerName  string                 `json:"server_name,omitempty"`
	Threads     []Thread               `json:"threads,omitempty"`
	Tags        map[string]string      `json:"tags,omitempty"`
	Timestamp   int64                  `json:"timestamp,omitempty"`
	Transaction string                 `json:"transaction,omitempty"`
	User        User                   `json:"user,omitempty"`
	Logger      string                 `json:"logger,omitempty"`
	Modules     map[string]string      `json:"modules,omitempty"`
	Request     Request                `json:"request,omitempty"`
	Exception   []Exception            `json:"exception,omitempty"`
}

func NewEvent() *Event {
	event := Event{
		Contexts: make(map[string]interface{}),
		Extra:    make(map[string]interface{}),
		Tags:     make(map[string]string),
		Modules:  make(map[string]string),
	}
	return &event
}

type Thread struct {
	ID            string      `json:"id,omitempty"`
	Name          string      `json:"name,omitempty"`
	Stacktrace    *Stacktrace `json:"stacktrace,omitempty"`
	RawStacktrace *Stacktrace `json:"raw_stacktrace,omitempty"`
	Crashed       bool        `json:"crashed,omitempty"`
	Current       bool        `json:"current,omitempty"`
}

type EventHint struct {
	Data               interface{}
	EventID            string
	OriginalException  error
	RecoveredException interface{}
	Context            context.Context
	Request            *http.Request
	Response           *http.Response
}
