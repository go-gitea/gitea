// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/web/middleware"
)

type doerContextKeyType struct{}

var doerContextKey doerContextKeyType

type impersonatorContextKeyType struct{}

var impersonatorContextKey impersonatorContextKeyType

// WithImpersonator returns a context that records audit events as performed by
// the doer on behalf of the given admin.
func WithImpersonator(ctx context.Context, impersonator *user_model.User) context.Context {
	return context.WithValue(ctx, impersonatorContextKey, impersonator)
}

// WithDoer returns a context that records audit events as the given user.
//
// Web and API requests need this only in unusual cases: the signed-in user is
// already published to the request data store by routers/common.AuthShared and
// is picked up automatically. Use it at entry points that have no signed-in
// user - the CLI, cron tasks, authentication source syncs and git hooks - before
// calling into services that record audit events themselves.
func WithDoer(ctx context.Context, doer *user_model.User) context.Context {
	return context.WithValue(ctx, doerContextKey, doer)
}

// doerFromContext resolves the actor of an audit event: an explicit WithDoer
// value wins over the signed-in user of the surrounding request. Returns nil
// when neither is available, which Record turns into an unknown actor.
func doerFromContext(ctx context.Context) *user_model.User {
	if doer, ok := ctx.Value(doerContextKey).(*user_model.User); ok && doer != nil {
		return doer
	}
	if data := middleware.GetContextData(ctx); data != nil {
		if doer, ok := data[middleware.ContextDataKeySignedUser].(*user_model.User); ok {
			return doer
		}
	}
	return nil
}

// credentialFromContext returns the credential the surrounding request
// authenticated with. It is dropped when the event is recorded for someone
// other than the signed-in user, so an explicit actor is never tied to a
// credential that is not theirs.
func credentialFromContext(ctx context.Context, doer *user_model.User) string {
	data := middleware.GetContextData(ctx)
	if data == nil {
		return ""
	}
	signedUser, _ := data[middleware.ContextDataKeySignedUser].(*user_model.User)
	if signedUser == nil || signedUser.ID != doer.ID {
		return ""
	}
	credential, _ := data[middleware.ContextDataKeyAuthCredential].(string)
	return credential
}

// ImpersonatorFromContext resolves the admin acting as the doer, so an event
// recorded during an impersonated session cannot be pinned on the impersonated
// user alone. Returns nil for ordinary sessions.
func ImpersonatorFromContext(ctx context.Context) *user_model.User {
	if impersonator, ok := ctx.Value(impersonatorContextKey).(*user_model.User); ok && impersonator != nil {
		return impersonator
	}
	if data := middleware.GetContextData(ctx); data != nil {
		if impersonator, ok := data[middleware.ContextDataKeyImpersonator].(*user_model.User); ok {
			return impersonator
		}
	}
	return nil
}
