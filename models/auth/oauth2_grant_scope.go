// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"gitea.dev/models/db"
)

// SetScope replaces a grant's scope.
//
// A grant's scope is otherwise fixed at creation, which makes asking for a
// wider one an error rather than a question: an application that adds a scope
// gets "a grant exists with different scope" for every user who authorized it
// before, and the user is never shown the choice. RFC 6749 handles scope
// escalation by asking again, which is what this makes possible.
func (grant *OAuth2Grant) SetScope(ctx context.Context, scope string) error {
	grant.Scope = scope
	_, err := db.GetEngine(ctx).ID(grant.ID).Cols("scope").Update(grant)
	return err
}
