// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net"
	"net/http"
	"net/url"
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
}

// RecordParams describes an audit event. Callers (or domain-specific helpers) supply metadata.
type RecordParams struct {
	Action   audit_model.Action
	Actor    EntityRef
	Scope    EntityRef
	Message  string
	Metadata map[string]any
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
		Message:   params.Message,
		Metadata:  params.Metadata,
		Time:      time.Now(),
		IPAddress: getIPAddress(ctx),
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

// Record writes an audit event for an action performed by doer against a scope
// entity. The scope is the affected entity and may be a *user.User,
// *organization.Organization, *repo.Repository, an EntityRef, or nil for an
// instance-wide/system event. Optional metadata is supplied as alternating
// string-key/value pairs.
//
//	audit.Record(ctx, audit_model.RepositoryArchive, doer, repo,
//	    fmt.Sprintf("Archived repository %s.", repo.FullName()))
func Record(ctx context.Context, action audit_model.Action, doer *user_model.User, scope any, message string, metadata ...any) {
	writeEvent(ctx, RecordParams{
		Action:   action,
		Actor:    ActorFromUser(doer),
		Scope:    scopeRef(scope),
		Message:  message,
		Metadata: metaPairs(metadata...),
	})
}

// writeEvent persists an audit event when audit logging is enabled.
//
// The database is the source of truth (it backs the in-app audit log views); the
// file sink is an optional append-only mirror for external log shipping. The two
// are written independently so a failure in one is logged but never blocks the
// other or the originating request.
func writeEvent(ctx context.Context, params RecordParams) {
	if !setting.Audit.Enabled {
		return
	}

	e := buildEvent(ctx, params)

	if err := writeToFile(e); err != nil {
		log.Error("Error writing audit event to file: %v", err)
	}
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
