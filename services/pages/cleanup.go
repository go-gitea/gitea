// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/log"
	pages_content "gitea.dev/modules/pages"

	"xorm.io/builder"
)

// CleanupVersions applies each repository's automatic Pages retention setting.
func CleanupVersions(ctx context.Context) error {
	return db.Iterate[pages_model.Site](ctx, builder.Eq{"deleted": false}, func(ctx context.Context, site *pages_model.Site) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := cleanupRepositoryVersions(ctx, site.RepoID); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Error("Unable to clean Pages versions for repo %d: %v", site.RepoID, err)
		}
		return nil
	})
}

func cleanupRepositoryVersions(ctx context.Context, repoID int64) error {
	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return fmt.Errorf("cleanupRepositoryVersions: lock site: %w", err)
	}
	defer release()

	removed := make([]*pages_model.Version, 0)
	err = db.WithTx(ctx, func(ctx context.Context) error {
		site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
		if err != nil || !has || site.Deleted {
			return err
		}
		if !pages_model.IsValidVersionRetentionCount(site.VersionRetentionCount) {
			return fmt.Errorf("cleanupRepositoryVersions: invalid version retention count %d", site.VersionRetentionCount)
		}
		versions, _, err := pages_model.FindVersions(ctx, &pages_model.FindVersionOptions{
			ListOptions: db.ListOptionsAll,
			RepoID:      repoID,
		})
		if err != nil {
			return err
		}
		removeIDs := make([]int64, 0, max(0, len(versions)-site.VersionRetentionCount))
		for index, version := range versions {
			if index < site.VersionRetentionCount || version.ID == site.CurrentVersionID || version.KeepForever {
				continue
			}
			removed = append(removed, version)
			removeIDs = append(removeIDs, version.ID)
		}
		return pages_model.DeleteVersionsByIDs(ctx, removeIDs)
	})
	if err != nil {
		return fmt.Errorf("cleanupRepositoryVersions: remove records: %w", err)
	}
	store := pages_content.NewContentStore()
	for _, version := range removed {
		if err := deleteUnreferencedContent(ctx, store, repoID, version.TarGzSHA256); err != nil {
			log.Error("Unable to remove expired Pages object for repo %d version %d: %v", repoID, version.ID, err)
		}
	}
	return nil
}
