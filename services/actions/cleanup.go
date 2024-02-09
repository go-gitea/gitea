// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

// Cleanup removes expired actions logs, data and artifacts
func Cleanup(taskCtx context.Context, olderThan time.Duration) error {
	// TODO: clean up expired actions logs

	// clean up expired artifacts
	return CleanupArtifacts(taskCtx)
}

// CleanupArtifacts removes expired artifacts and set records expired status
func CleanupArtifacts(taskCtx context.Context) error {
	artifacts, err := actions.ListNeedExpiredArtifacts(taskCtx)
	if err != nil {
		return err
	}
	log.Info("Found %d expired artifacts", len(artifacts))
	for _, artifact := range artifacts {
		if err := storage.ActionsArtifacts.Delete(artifact.StoragePath); err != nil {
			log.Error("Cannot delete artifact %d: %v", artifact.ID, err)
			continue
		}
		if err := actions.SetArtifactExpired(taskCtx, artifact.ID); err != nil {
			log.Error("Cannot set artifact %d expired: %v", artifact.ID, err)
			continue
		}
		log.Info("Artifact %d set expired", artifact.ID)
	}
	return nil
}
