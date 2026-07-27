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
