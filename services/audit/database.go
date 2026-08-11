// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	audit_model "gitea.dev/models/audit"
)

func writeToDatabase(ctx context.Context, e *audit_model.Event) error {
	_, err := audit_model.InsertEvent(ctx, e)
	return err
}
