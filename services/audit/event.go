// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	audit_model "gitea.dev/models/audit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// Event is the transport representation of an audit log entry.
type Event struct {
	Action    audit_model.Action `json:"action"`
	Actor     EntityRef          `json:"actor"`
	Scope     EntityRef          `json:"scope"`
	Message   string             `json:"message"`
	Metadata  map[string]any     `json:"metadata,omitempty"`
	Time      time.Time          `json:"time"`
	IPAddress string             `json:"ip_address"`
	Origin    audit_model.Origin `json:"origin"`
}

// RecordParams describes an audit event. Callers (or domain-specific helpers)
// supply metadata; the message is rendered from the action's template.
type RecordParams struct {
	Action   audit_model.Action
	Actor    EntityRef
	Scope    EntityRef
	Metadata map[string]any
}

type originContextKeyType struct{}

var originContextKey originContextKeyType

// WithOrigin returns a context that records audit events with the given origin.
func WithOrigin(ctx context.Context, origin audit_model.Origin) context.Context {
	return context.WithValue(ctx, originContextKey, origin)
}

func (r EntityRef) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	if r.Type == audit_model.ScopeSystem {
		return "System"
	}
	return ""
}

func (r EntityRef) HomeLink() string {
	switch r.Type {
	case audit_model.ScopeUser, audit_model.ScopeOrganization:
		if r.Name == "" {
			return ""
		}
		return setting.AppSubURL + "/" + url.PathEscape(r.Name)
	case audit_model.ScopeRepository:
		if r.Name == "" {
			return ""
		}
		return setting.AppSubURL + "/" + util.PathEscapeSegments(r.Name)
	default:
		return ""
	}
}

func (r EntityRef) HasLink() bool {
	return r.HomeLink() != "" && r.ID > 0
}

func buildEvent(ctx context.Context, params RecordParams) *Event {
	return &Event{
		Action:    params.Action,
		Actor:     params.Actor,
		Scope:     params.Scope,
		Message:   renderMessage(params.Action, params.Actor, params.Scope, params.Metadata),
		Metadata:  params.Metadata,
		Time:      time.Now(),
		IPAddress: getIPAddress(ctx),
		Origin:    getOrigin(ctx),
	}
}

func getIPAddress(ctx context.Context) string {
	req, ok := ctx.Value(httplib.RequestContextKey).(*http.Request)
	if !ok || req == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func getOrigin(ctx context.Context) audit_model.Origin {
	if origin, ok := ctx.Value(originContextKey).(audit_model.Origin); ok && origin != "" {
		return origin
	}

	req, ok := ctx.Value(httplib.RequestContextKey).(*http.Request)
	if !ok || req == nil {
		return audit_model.OriginSystem
	}
	if req.URL == nil {
		return audit_model.OriginUI
	}

	requestPath := strings.TrimPrefix(req.URL.Path, setting.AppSubURL)
	if requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") {
		return audit_model.OriginAPI
	}
	return audit_model.OriginUI
}

// Record writes an audit event for an action against a scope entity. The actor
// is the signed-in user of the surrounding request, or whoever audit.WithDoer
// named for a background context.
//
// The scope is the affected entity and may be a *user.User,
// *organization.Organization, *repo.Repository, an EntityRef, or nil for an
// instance-wide/system event. Metadata is supplied as alternating
// string-key/value pairs and fills the placeholders of the action's message
// template, so every key a template names must be passed here.
//
//	audit.Record(ctx, audit_model.RepositoryArchive, repo)
//	audit.Record(ctx, audit_model.RepositoryDeployKeyAdd, repo, "deploy_key", key.Name)
func Record(ctx context.Context, action audit_model.Action, scope any, metadata ...any) {
	RecordAs(ctx, doerFromContext(ctx), action, scope, metadata...)
}

// RecordAs is Record with an explicit actor, for the few call sites where the
// acting user is not the one the context resolves to.
func RecordAs(ctx context.Context, doer *user_model.User, action audit_model.Action, scope any, metadata ...any) {
	writeEvent(ctx, RecordParams{
		Action:   action,
		Actor:    actorRef(doer),
		Scope:    scopeRef(scope),
		Metadata: metaPairs(metadata...),
	})
}

// writeEvent persists an audit event when audit logging is enabled.
func writeEvent(ctx context.Context, params RecordParams) {
	if !setting.Audit.Enabled {
		return
	}

	e := buildEvent(ctx, params)

	if err := writeToDatabase(ctx, e); err != nil {
		log.Error("Error writing audit event %+v to database: %v", e, err)
	}
}

func FindEvents(ctx context.Context, opts *audit_model.EventSearchOptions) ([]*Event, int64, error) {
	events, total, err := audit_model.FindEvents(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*Event, 0, len(events))
	for _, e := range events {
		out = append(out, fromDatabaseEvent(e))
	}
	return out, total, nil
}

func fromDatabaseEvent(e *audit_model.Event) *Event {
	return &Event{
		Action: e.Action,
		Actor: EntityRef{
			Type: audit_model.ScopeUser,
			ID:   e.ActorID,
			Name: e.ActorName,
		},
		Scope: EntityRef{
			Type: e.ScopeType,
			ID:   e.ScopeID,
			Name: e.ScopeName,
		},
		Message:   e.Message,
		Metadata:  decodeMetadata(e.Metadata),
		Time:      e.TimestampUnix.AsTime(),
		IPAddress: e.IPAddress,
		Origin:    e.Origin,
	}
}

func encodeMetadata(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Error("Failed to encode audit metadata: %v", err)
		return ""
	}
	return string(b)
}

func decodeMetadata(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		log.Error("Failed to decode audit metadata: %v", err)
		return nil
	}
	return m
}

// metaPairs builds caller-defined metadata from alternating string-key/value
// pairs. Keys should be stable for log parsers. A non-string key is skipped and
// logged rather than panicking: audit recording must never crash the request
// that triggered it.
func metaPairs(pairs ...any) map[string]any {
	if len(pairs) == 0 {
		return nil
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			log.Error("audit: metadata key must be string, got %T; skipping pair", pairs[i])
			continue
		}
		m[key] = pairs[i+1]
	}
	return m
}
