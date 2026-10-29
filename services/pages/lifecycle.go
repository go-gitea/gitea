// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/timeutil"
)

// LockLifecycleState reserves the next Pages state timestamp until the
// caller's surrounding database transaction has committed. Repository
// changes and their Pages notification therefore become visible in the same
// order that Pages instances observe them.
func LockLifecycleState(ctx context.Context) (func(), error) {
	stateClock.Lock()
	if _, err := nextStateTime(ctx); err != nil {
		stateClock.Unlock()
		return nil, err
	}
	return stateClock.Unlock, nil
}

func lockedLifecycleStateTime() int64 {
	if stateClock.initialized {
		return stateClock.last
	}
	return nextLifecycleTime(0)
}

func touchOwnerRepositoriesAt(ctx context.Context, ownerID, updated int64) error {
	_, err := db.Exec(ctx, "UPDATE pages_site SET pages_updated_unix_nano = CASE WHEN pages_updated_unix_nano >= ? THEN pages_updated_unix_nano + 1 ELSE ? END WHERE deleted = ? AND repo_id IN (SELECT id FROM repository WHERE owner_id = ?)", updated, updated, false, ownerID)
	if err != nil {
		return fmt.Errorf("touchOwnerRepositoriesAt: owner %d: %w", ownerID, err)
	}
	return nil
}

// nextLifecycleTime returns a value that is newer than the row being changed.
// Pages instances use this value to notice repository lifecycle changes even
// when the change happens outside the Pages settings handlers.
func nextLifecycleTime(previous int64) int64 {
	now := int64(timeutil.TimeStampNanoNow())
	if now <= previous {
		return previous + 1
	}
	return now
}

func disableRepositoryAt(ctx context.Context, repoID, updated int64) error {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("disableRepositoryAt: repository %d: %w", repoID, err)
	}
	if !has || site.Deleted || !site.Enabled {
		return nil
	}
	site.Enabled = false
	site.PagesUpdatedUnixNano = max(updated, site.PagesUpdatedUnixNano+1)
	if _, err := db.GetEngine(ctx).ID(repoID).NoAutoCondition().AllCols().Update(site); err != nil {
		return fmt.Errorf("disableRepositoryAt: repository %d: %w", repoID, err)
	}
	return nil
}

// DisableRepository closes a repository's public Pages site and advances the
// shared Pages state clock. Callers that already own a Gitea transaction should
// use DisableRepositoryInTransaction so the repository and Pages changes are
// committed together.
func DisableRepository(ctx context.Context, repoID int64) error {
	return withStateChange(ctx, func(ctx context.Context, updated int64) error {
		return disableRepositoryAt(ctx, repoID, updated)
	})
}

// DisableRepositoryInTransaction is the transaction-safe form of
// DisableRepository. It does not open a nested transaction and must be used
// while LockLifecycleState is held by the surrounding operation.
func DisableRepositoryInTransaction(ctx context.Context, repoID int64) error {
	return disableRepositoryAt(ctx, repoID, lockedLifecycleStateTime())
}

func touchRepositoryAt(ctx context.Context, repoID, updated int64) error {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("touchRepositoryAt: repository %d: %w", repoID, err)
	}
	if !has || site.Deleted {
		return nil
	}
	if site.PagesUpdatedUnixNano >= updated {
		updated = site.PagesUpdatedUnixNano + 1
	}
	if _, err := db.GetEngine(ctx).ID(repoID).Cols("pages_updated_unix_nano").Update(&pages_model.Site{PagesUpdatedUnixNano: updated}); err != nil {
		return fmt.Errorf("touchRepositoryAt: repository %d: %w", repoID, err)
	}
	return nil
}

// TouchRepositoryInTransaction announces a repository path or ownership
// change to Pages without changing its published content. The caller holds
// LockLifecycleState for the surrounding transaction.
func TouchRepositoryInTransaction(ctx context.Context, repoID int64) error {
	return touchRepositoryAt(ctx, repoID, lockedLifecycleStateTime())
}

// TouchOwnerRepositoriesInTransaction announces an owner rename to all of
// the owner's Pages sites in one SQL update. Repository IDs remain stable, so
// this is enough for each Pages instance to refresh the owner/repository path.
// The caller holds LockLifecycleState for the surrounding transaction.
func TouchOwnerRepositoriesInTransaction(ctx context.Context, ownerID int64) error {
	return touchOwnerRepositoriesAt(ctx, ownerID, lockedLifecycleStateTime())
}

// markRepositoryDeletedAt records repository deletion before the repository row
// is removed. The compact row remains permanently so returning instances can
// observe the deletion by repository ID without relying on repository names.
//
// The returned digest is the object that was current before deletion. The
// caller may remove that object after its surrounding repository transaction
// commits.
func markRepositoryDeletedAt(ctx context.Context, repoID, updated int64) (string, error) {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return "", fmt.Errorf("markRepositoryDeletedAt: repository %d: %w", repoID, err)
	}
	if !has || site.Deleted {
		return "", nil
	}
	oldDigest := site.TarGzSHA256
	site.Deleted = true
	site.Enabled = false
	site.SourceType = ""
	site.SourceBranch = ""
	site.SourcePath = ""
	site.SourceCommitID = ""
	site.SourceNextCheckUnix = 0
	site.SourceErrorCode = ""
	site.TarGzSHA256 = ""
	site.PagesUpdatedUnixNano = max(updated, site.PagesUpdatedUnixNano+1)
	if _, err := db.GetEngine(ctx).ID(repoID).NoAutoCondition().AllCols().Update(site); err != nil {
		return "", fmt.Errorf("markRepositoryDeletedAt: repository %d: %w", repoID, err)
	}
	// Pending domains were never served, so only active bindings need an
	// acknowledgement before their host reservation is released.
	if _, err := db.GetEngine(ctx).Where("repo_id = ? AND state = ?", repoID, pages_model.DomainStatePending).Delete(new(pages_model.Domain)); err != nil {
		return "", fmt.Errorf("markRepositoryDeletedAt: remove pending domain for repository %d: %w", repoID, err)
	}
	if _, err := db.GetEngine(ctx).Where("repo_id = ? AND state = ?", repoID, pages_model.DomainStateActive).
		Cols("state").Update(&pages_model.Domain{State: pages_model.DomainStateReleasing}); err != nil {
		return "", fmt.Errorf("markRepositoryDeletedAt: release domains for repository %d: %w", repoID, err)
	}
	domains, err := pages_model.ListDomainsByRepoID(ctx, repoID)
	if err != nil {
		return "", fmt.Errorf("markRepositoryDeletedAt: list domains for repository %d: %w", repoID, err)
	}
	if len(domains) == 0 {
		if _, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(new(pages_model.InstanceResult)); err != nil {
			return "", fmt.Errorf("markRepositoryDeletedAt: remove results for repository %d: %w", repoID, err)
		}
	}
	return oldDigest, nil
}

// MarkRepositoryDeletedInTransaction is the transaction-safe form of
// repository deletion. It does not open a nested transaction and must be used
// while LockLifecycleState is held by the surrounding operation.
func MarkRepositoryDeletedInTransaction(ctx context.Context, repoID int64) (string, error) {
	return markRepositoryDeletedAt(ctx, repoID, lockedLifecycleStateTime())
}
