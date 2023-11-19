// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	audit_model "code.gitea.io/gitea/models/audit"
)

func writeToDatabase(ctx context.Context, e *Event) error {
	_, err := audit_model.InsertEvent(ctx, &audit_model.Event{
		Action:     e.Action,
		ActorID:    e.Actor.ID,
		ScopeType:  e.Scope.Type,
		ScopeID:    e.Scope.ID,
		TargetType: e.Target.Type,
		TargetID:   e.Target.ID,
		Message:    e.Message,
		IPAddress:  e.IPAddress,
	})
	return err
}
