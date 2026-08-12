// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"strings"
	"time"

	audit_model "gitea.dev/models/audit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
)

// RecordParams describes an audit event. Callers (or domain-specific helpers)
// supply metadata; the message is rendered from the action's template.
type RecordParams struct {
	Action   audit_model.Action
	Actor    audit_model.EntityRef
	Scope    audit_model.EntityRef
	Metadata map[string]any
}

type originContextKeyType struct{}

var originContextKey originContextKeyType

// WithOrigin returns a context that records audit events with the given origin.
func WithOrigin(ctx context.Context, origin audit_model.Origin) context.Context {
	return context.WithValue(ctx, originContextKey, origin)
}

func buildEvent(ctx context.Context, params RecordParams) *audit_model.Event {
	return &audit_model.Event{
		Action:        params.Action,
		ActorID:       params.Actor.ID,
		ActorName:     params.Actor.DisplayName(),
		ScopeType:     params.Scope.Type,
		ScopeID:       params.Scope.ID,
		ScopeName:     params.Scope.DisplayName(),
		Message:       renderMessage(params.Action, params.Actor, params.Scope, params.Metadata),
		Metadata:      audit_model.EncodeMetadata(params.Metadata),
		IPAddress:     getIPAddress(ctx),
		Origin:        getOrigin(ctx),
		TimestampUnix: timeutil.TimeStamp(time.Now().Unix()),
	}
}

func getIPAddress(ctx context.Context) string {
	req, ok := ctx.Value(httplib.RequestContextKey).(*http.Request)
	if !ok {
		return ""
	}
	return httplib.RemoteHost(req)
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

	if err := audit_model.InsertEvent(ctx, e); err != nil {
		log.Error("Error writing audit event action=%s actor=%s scope=%s/%d to database: %v", e.Action, e.ActorName, e.ScopeType, e.ScopeID, err)
	}
}

func FindEvents(ctx context.Context, opts *audit_model.EventSearchOptions) ([]*audit_model.Event, int64, error) {
	return audit_model.FindEvents(ctx, opts)
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
