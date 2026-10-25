// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"

	user_model "gitea.dev/models/user"
	"gitea.dev/services/quota"
)

func registerStorageQuotaReconciliation() {
	RegisterTaskFatal("storage_quota_reconcile", &BaseConfig{
		Enabled:    true,
		RunAtStart: false,
		Schedule:   "@every 24h",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error { return quota.ReconcileAll(ctx) })
}
