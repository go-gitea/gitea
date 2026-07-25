// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/modules/timeutil"
)

func writeToDatabase(ctx context.Context, e *Event) error {
	_, err := audit_model.InsertEvent(ctx, &audit_model.Event{
		Action:        e.Action,
		ActorID:       e.Actor.ID,
		ActorName:     e.Actor.DisplayName(),
		ScopeType:     e.Scope.Type,
		ScopeID:       e.Scope.ID,
		ScopeName:     e.Scope.DisplayName(),
		Message:       e.Message,
		Metadata:      encodeMetadata(e.Metadata),
		IPAddress:     e.IPAddress,
		TimestampUnix: timeutil.TimeStamp(e.Time.Unix()),
	})
	return err
}
