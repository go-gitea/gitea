// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace // import "go.opentelemetry.io/otel/trace"

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
)

const (
	// FlagsSampled is a bitmask with the sampled bit set. A SpanContext
	// with the sampling bit set means the span is sampled.
	FlagsSampled = byte(0x01)
	// FlagsDeferred is a bitmask with the deferred bit set. A SpanContext
	// with the deferred bit set means the sampling decision has been
	// defered to the receiver.
	FlagsDeferred = byte(0x02)
	// FlagsDebug is a bitmask with the debug bit set.
	FlagsDebug = byte(0x04)

	errInvalidHexID errorConst = "trace-id and span-id can only contain [0-9a-f] characters, all lowercase"

	errInvalidTraceIDLength errorConst = "hex encoded trace-id must have length equals to 32"
	errNilTraceID           errorConst = "trace-id can't be all zero"

	errInvalidSpanIDLength errorConst = "hex encoded span-id must have length equals to 16"
	errNilSpanID           errorConst = "span-id can't be all zero"

	// based on the W3C Trace Context specification, see https://www.w3.org/TR/trace-context-1/#tracestate-header
	traceStateKeyFormat                      = `[a-z][_0-9a-z\-\*\/]{0,255}`
	traceStateKeyFormatWithMultiTenantVendor = `[a-z][_0-9a-z\-\*\/]{0,240}@[a-z][_0-9a-z\-\*\/]{0,13}`
	traceStateValueFormat                    = `[\x20-\x2b\x2d-\x3c\x3e-\x7e]{0,255}[\x21-\x2b\x2d-\x3c\x3e-\x7e]`

	traceStateMaxListMembers = 32

	errInvalidTraceStateKeyValue errorConst = "provided key or value is not valid according to the" +
		" W3C Trace Context specification"
	errInvalidTraceStateMembersNumber errorConst = "trace state would exceed the maximum limit of members (32)"
	errInvalidTraceStateDuplicate     errorConst = "trace state key/value pairs with duplicate keys provided"
)

type errorConst string

func (e errorConst) Error() string {
	return string(e)
}

// TraceID is a unique identity of a trace.
// nolint:golint
type TraceID [16]byte

var nilTraceID TraceID
var _ json.Marshaler = nilTraceID

// IsValid checks whether the trace TraceID is valid. A valid trace ID does
// not consist of zeros only.
func (t TraceID) IsValid() bool {
	return !bytes.Equal(t[:], nilTraceID[:])
}

// MarshalJSON implements a custom marshal function to encode TraceID
// as a hex string.
func (t TraceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// String returns the hex string representation form of a TraceID
func (t TraceID) String() string {
	return hex.EncodeToString(t[:])
}

// SpanID is a unique identity of a span in a trace.
type SpanID [8]byte

var nilSpanID SpanID
var _ json.Marshaler = nilSpanID

// IsValid checks whether the SpanID is valid. A valid SpanID does not consist
// of zeros only.
func (s SpanID) IsValid() bool {
	return !bytes.Equal(s[:], nilSpanID[:])
}

// MarshalJSON implements a custom marshal function to encode SpanID
// as a hex string.
func (s SpanID) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// String returns the hex string representation form of a SpanID
func (s SpanID) String() string {
	return hex.EncodeToString(s[:])
}

// TraceIDFromHex returns a TraceID from a hex string if it is compliant with
// the W3C trace-context specification.  See more at
// https://www.w3.org/TR/trace-context/#trace-id
// nolint:golint
func TraceIDFromHex(h string) (TraceID, error) {
	t := TraceID{}
	if len(h) != 32 {
		return t, errInvalidTraceIDLength
	}

	if err := decodeHex(h, t[:]); err != nil {
		return t, err
	}

	if !t.IsValid() {
		return t, errNilTraceID
	}
	return t, nil
}

// SpanIDFromHex returns a SpanID from a hex string if it is compliant
// with the w3c trace-context specification.
// See more at https://www.w3.org/TR/trace-context/#parent-id
func SpanIDFromHex(h string) (SpanID, error) {
	s := SpanID{}
	if len(h) != 16 {
		return s, errInvalidSpanIDLength
	}

	if err := decodeHex(h, s[:]); err != nil {
		return s, err
	}

	if !s.IsValid() {
		return s, errNilSpanID
	}
	return s, nil
}

func decodeHex(h string, b []byte) error {
	for _, r := range h {
		switch {
		case 'a' <= r && r <= 'f':
			continue
		case '0' <= r && r <= '9':
			continue
		default:
			return errInvalidHexID
		}
	}

	decoded, err := hex.DecodeString(h)
	if err != nil {
		return err
	}

	copy(b, decoded)
	return nil
}

// TraceState provides additional vendor-specific trace identification information
// across different distributed tracing systems. It represents an immutable list consisting
// of key/value pairs. There can be a maximum of 32 entries in the list.
//
// Key and value of each list member must be valid according to the W3C Trace Context specification
// (see https://www.w3.org/TR/trace-context-1/#key and https://www.w3.org/TR/trace-context-1/#value
// respectively).
//
// Trace state must be valid according to the W3C Trace Context specification at all times. All
// mutating operations validate their input and, in case of valid parameters, return a new TraceState.
type TraceState struct { //nolint:golint
	// TODO @matej-g: Consider implementing this as label.Set, see
	// comment https://github.com/open-telemetry/opentelemetry-go/pull/1340#discussion_r540599226
	kvs []label.KeyValue
}

var _ json.Marshaler = TraceState{}
var keyFormatRegExp = regexp.MustCompile(
	`^((` + traceStateKeyFormat + `)|(` + traceStateKeyFormatWithMultiTenantVendor + `))$`,
)
var valueFormatRegExp = regexp.MustCompile(`^(` + traceStateValueFormat + `)$`)

// MarshalJSON implements a custom marshal function to encode trace state.
func (ts TraceState) MarshalJSON() ([]byte, error) {
	return json.Marshal(ts.kvs)
}

// String returns trace state as a string valid according to the
// W3C Trace Context specification.
func (ts TraceState) String() string {
	var sb strings.Builder

	for i, kv := range ts.kvs {
		sb.WriteString((string)(kv.Key))
		sb.WriteByte('=')
		sb.WriteString(kv.Value.Emit())

		if i != len(ts.kvs)-1 {
			sb.WriteByte(',')
		}
	}

	return sb.String()
}

// Get returns a value for given key from the trace state.
// If no key is found or provided key is invalid, returns an empty value.
func (ts TraceState) Get(key label.Key) label.Value {
	if !isTraceStateKeyValid(key) {
		return label.Value{}
	}

	for _, kv := range ts.kvs {
		if kv.Key == key {
			return kv.Value
		}
	}

	return label.Value{}
}

// Insert adds a new key/value, if one doesn't exists; otherwise updates the existing entry.
// The new or updated entry is always inserted at the beginning of the TraceState, i.e.
// on the left side, as per the W3C Trace Context specification requirement.
func (ts TraceState) Insert(entry label.KeyValue) (TraceState, error) {
	if !isTraceStateKeyValueValid(entry) {
		return ts, errInvalidTraceStateKeyValue
	}

	ckvs := ts.copyKVsAndDeleteEntry(entry.Key)
	if len(ckvs)+1 > traceStateMaxListMembers {
		return ts, errInvalidTraceStateMembersNumber
	}

	ckvs = append(ckvs, label.KeyValue{})
	copy(ckvs[1:], ckvs)
	ckvs[0] = entry

	return TraceState{ckvs}, nil
}

// Delete removes specified entry from the trace state.
func (ts TraceState) Delete(key label.Key) (TraceState, error) {
	if !isTraceStateKeyValid(key) {
		return ts, errInvalidTraceStateKeyValue
	}

	return TraceState{ts.copyKVsAndDeleteEntry(key)}, nil
}

// IsEmpty returns true if the TraceState does not contain any entries
func (ts TraceState) IsEmpty() bool {
	return len(ts.kvs) == 0
}

func (ts TraceState) copyKVsAndDeleteEntry(key label.Key) []label.KeyValue {
	ckvs := make([]label.KeyValue, len(ts.kvs))
	copy(ckvs, ts.kvs)
	for i, kv := range ts.kvs {
		if kv.Key == key {
			ckvs = append(ckvs[:i], ckvs[i+1:]...)
			break
		}
	}

	return ckvs
}

// TraceStateFromKeyValues is a convenience method to create a new TraceState from
// provided key/value pairs.
func TraceStateFromKeyValues(kvs ...label.KeyValue) (TraceState, error) { //nolint:golint
	if len(kvs) == 0 {
		return TraceState{}, nil
	}

	if len(kvs) > traceStateMaxListMembers {
		return TraceState{}, errInvalidTraceStateMembersNumber
	}

	km := make(map[label.Key]bool)
	for _, kv := range kvs {
		if !isTraceStateKeyValueValid(kv) {
			return TraceState{}, errInvalidTraceStateKeyValue
		}
		_, ok := km[kv.Key]
		if ok {
			return TraceState{}, errInvalidTraceStateDuplicate
		}
		km[kv.Key] = true
	}

	ckvs := make([]label.KeyValue, len(kvs))
	copy(ckvs, kvs)
	return TraceState{ckvs}, nil
}

func isTraceStateKeyValid(key label.Key) bool {
	return keyFormatRegExp.MatchString(string(key))
}

func isTraceStateKeyValueValid(kv label.KeyValue) bool {
	return isTraceStateKeyValid(kv.Key) &&
		valueFormatRegExp.MatchString(kv.Value.Emit())
}

// SpanContext contains identifying trace information about a Span.
type SpanContext struct {
	TraceID    TraceID
	SpanID     SpanID
	TraceFlags byte
	TraceState TraceState
}

// IsValid returns if the SpanContext is valid. A valid span context has a
// valid TraceID and SpanID.
func (sc SpanContext) IsValid() bool {
	return sc.HasTraceID() && sc.HasSpanID()
}

// HasTraceID checks if the SpanContext has a valid TraceID.
func (sc SpanContext) HasTraceID() bool {
	return sc.TraceID.IsValid()
}

// HasSpanID checks if the SpanContext has a valid SpanID.
func (sc SpanContext) HasSpanID() bool {
	return sc.SpanID.IsValid()
}

// IsDeferred returns if the deferred bit is set in the trace flags.
func (sc SpanContext) IsDeferred() bool {
	return sc.TraceFlags&FlagsDeferred == FlagsDeferred
}

// IsDebug returns if the debug bit is set in the trace flags.
func (sc SpanContext) IsDebug() bool {
	return sc.TraceFlags&FlagsDebug == FlagsDebug
}

// IsSampled returns if the sampling bit is set in the trace flags.
func (sc SpanContext) IsSampled() bool {
	return sc.TraceFlags&FlagsSampled == FlagsSampled
}

type traceContextKeyType int

const (
	currentSpanKey traceContextKeyType = iota
	remoteContextKey
)

// ContextWithSpan returns a copy of parent with span set to current.
func ContextWithSpan(parent context.Context, span Span) context.Context {
	return context.WithValue(parent, currentSpanKey, span)
}

// SpanFromContext returns the current span from ctx, or noop span if none set.
func SpanFromContext(ctx context.Context) Span {
	if span, ok := ctx.Value(currentSpanKey).(Span); ok {
		return span
	}
	return noopSpan{}
}

// SpanContextFromContext returns the current SpanContext from ctx, or an empty SpanContext if none set.
func SpanContextFromContext(ctx context.Context) SpanContext {
	if span := SpanFromContext(ctx); span != nil {
		return span.SpanContext()
	}
	return SpanContext{}
}

// ContextWithRemoteSpanContext returns a copy of parent with a remote set as
// the remote span context.
func ContextWithRemoteSpanContext(parent context.Context, remote SpanContext) context.Context {
	return context.WithValue(parent, remoteContextKey, remote)
}

// RemoteSpanContextFromContext returns the remote span context from ctx.
func RemoteSpanContextFromContext(ctx context.Context) SpanContext {
	if sc, ok := ctx.Value(remoteContextKey).(SpanContext); ok {
		return sc
	}
	return SpanContext{}
}

// Span is the individual component of a trace. It represents a single named
// and timed operation of a workflow that is traced. A Tracer is used to
// create a Span and it is then up to the operation the Span represents to
// properly end the Span when the operation itself ends.
type Span interface {
	// Tracer returns the Tracer that created the Span. Tracer MUST NOT be
	// nil.
	Tracer() Tracer

	// End completes the Span. The Span is considered complete and ready to be
	// delivered through the rest of the telemetry pipeline after this method
	// is called. Therefore, updates to the Span are not allowed after this
	// method has been called.
	End(options ...SpanOption)

	// AddEvent adds an event with the provided name and options.
	AddEvent(name string, options ...EventOption)

	// IsRecording returns the recording state of the Span. It will return
	// true if the Span is active and events can be recorded.
	IsRecording() bool

	// RecordError records an error as a Span event.
	RecordError(err error, options ...EventOption)

	// SpanContext returns the SpanContext of the Span. The returned
	// SpanContext is usable even after the End has been called for the Span.
	SpanContext() SpanContext

	// SetStatus sets the status of the Span in the form of a code and a
	// message. SetStatus overrides the value of previous calls to SetStatus
	// on the Span.
	SetStatus(code codes.Code, msg string)

	// SetName sets the Span name.
	SetName(name string)

	// SetAttributes sets kv as attributes of the Span. If a key from kv
	// already exists for an attribute of the Span it will be overwritten with
	// the value contained in kv.
	SetAttributes(kv ...label.KeyValue)
}

// Link is the relationship between two Spans. The relationship can be within
// the same Trace or across different Traces.
//
// For example, a Link is used in the following situations:
//
//   1. Batch Processing: A batch of operations may contain operations
//      associated with one or more traces/spans. Since there can only be one
//      parent SpanContext, a Link is used to keep reference to the
//      SpanContext of all operations in the batch.
//   2. Public Endpoint: A SpanContext for an in incoming client request on a
//      public endpoint should be considered untrusted. In such a case, a new
//      trace with its own identity and sampling decision needs to be created,
//      but this new trace needs to be related to the original trace in some
//      form. A Link is used to keep reference to the original SpanContext and
//      track the relationship.
type Link struct {
	SpanContext
	Attributes []label.KeyValue
}

// SpanKind is the role a Span plays in a Trace.
type SpanKind int

// As a convenience, these match the proto definition, see
// https://github.com/open-telemetry/opentelemetry-proto/blob/30d237e1ff3ab7aa50e0922b5bebdd93505090af/opentelemetry/proto/trace/v1/trace.proto#L101-L129
//
// The unspecified value is not a valid `SpanKind`. Use `ValidateSpanKind()`
// to coerce a span kind to a valid value.
const (
	// SpanKindUnspecified is an unspecified SpanKind and is not a valid
	// SpanKind. SpanKindUnspecified should be replaced with SpanKindInternal
	// if it is received.
	SpanKindUnspecified SpanKind = 0
	// SpanKindInternal is a SpanKind for a Span that represents an internal
	// operation within an application.
	SpanKindInternal SpanKind = 1
	// SpanKindServer is a SpanKind for a Span that represents the operation
	// of handling a request from a client.
	SpanKindServer SpanKind = 2
	// SpanKindClient is a SpanKind for a Span that represents the operation
	// of client making a request to a server.
	SpanKindClient SpanKind = 3
	// SpanKindProducer is a SpanKind for a Span that represents the operation
	// of a producer sending a message to a message broker. Unlike
	// SpanKindClient and SpanKindServer, there is often no direct
	// relationship between this kind of Span and a SpanKindConsumer kind. A
	// SpanKindProducer Span will end once the message is accepted by the
	// message broker which might not overlap with the processing of that
	// message.
	SpanKindProducer SpanKind = 4
	// SpanKindConsumer is a SpanKind for a Span that represents the operation
	// of a consumer receiving a message from a message broker. Like
	// SpanKindProducer Spans, there is often no direct relationship between
	// this Span and the Span that produced the message.
	SpanKindConsumer SpanKind = 5
)

// ValidateSpanKind returns a valid span kind value.  This will coerce
// invalid values into the default value, SpanKindInternal.
func ValidateSpanKind(spanKind SpanKind) SpanKind {
	switch spanKind {
	case SpanKindInternal,
		SpanKindServer,
		SpanKindClient,
		SpanKindProducer,
		SpanKindConsumer:
		// valid
		return spanKind
	default:
		return SpanKindInternal
	}
}

// String returns the specified name of the SpanKind in lower-case.
func (sk SpanKind) String() string {
	switch sk {
	case SpanKindInternal:
		return "internal"
	case SpanKindServer:
		return "server"
	case SpanKindClient:
		return "client"
	case SpanKindProducer:
		return "producer"
	case SpanKindConsumer:
		return "consumer"
	default:
		return "unspecified"
	}
}

// Tracer is the creator of Spans.
type Tracer interface {
	// Start creates a span.
	Start(ctx context.Context, spanName string, opts ...SpanOption) (context.Context, Span)
}

// TracerProvider provides access to instrumentation Tracers.
type TracerProvider interface {
	// Tracer creates an implementation of the Tracer interface.
	// The instrumentationName must be the name of the library providing
	// instrumentation. This name may be the same as the instrumented code
	// only if that code provides built-in instrumentation. If the
	// instrumentationName is empty, then a implementation defined default
	// name will be used instead.
	Tracer(instrumentationName string, opts ...TracerOption) Tracer
}
