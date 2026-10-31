// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"errors"

	"gitea.dev/models/db"
	quota_model "gitea.dev/models/quota"
	"gitea.dev/modules/log"
	"gitea.dev/services/quota"
)

type quotaOfflineKey struct{}

func WithQuotaRecoveryOffline(ctx context.Context, confirmed bool) context.Context {
	return context.WithValue(ctx, quotaOfflineKey{}, confirmed)
}

func init() {
	Register(&Check{Name: "storage-quota", Title: "Check storage quota reservations", Run: checkStorageQuota})
}

func checkStorageQuota(ctx context.Context, logger log.Logger, autofix bool) error {
	var operations []*quota_model.Operation
	if err := db.GetEngine(ctx).Find(&operations); err != nil {
		return err
	}
	for _, op := range operations {
		logger.Info("Account %d: repository %d, operation %s, created %s", op.OwnerID, op.RepoID, op.Token, op.Created.AsTime())
	}
	if !autofix {
		return nil
	}
	if confirmed, _ := ctx.Value(quotaOfflineKey{}).(bool); !confirmed {
		return errors.New("storage quota recovery requires --quota-recovery-offline after stopping all web, SSH, and worker processes sharing the database and storage")
	}
	return quota.RecoverOffline(ctx)
}
