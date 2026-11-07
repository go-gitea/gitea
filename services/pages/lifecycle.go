// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
)

// LockLifecycleState reserves the next Pages state timestamp until the
// caller's surrounding database transaction has committed. Repository
// changes and their Pages notification therefore become visible in the same
// order that Pages instances observe them.
func LockLifecycleState(ctx context.Context) (int64, func(), error) {
	stateClock.Lock()
	updated, err := nextStateTime(ctx)
	if err != nil {
		stateClock.Unlock()
		return 0, nil, err
	}
	return updated, stateClock.Unlock, nil
}

// TouchOwnerRepositoriesInTransaction announces an owner rename to all of
// the owner's Pages sites in one SQL update. Repository IDs remain stable, so
// this is enough for each Pages instance to refresh the owner/repository path.
// The caller holds LockLifecycleState for the surrounding transaction.
func TouchOwnerRepositoriesInTransaction(ctx context.Context, ownerID, updated int64) error {
	_, err := db.Exec(ctx, "UPDATE pages_site SET pages_updated_unix_nano = CASE WHEN pages_updated_unix_nano >= ? THEN pages_updated_unix_nano + 1 ELSE ? END WHERE deleted = ? AND repo_id IN (SELECT id FROM repository WHERE owner_id = ?)", updated, updated, false, ownerID)
	if err != nil {
		return fmt.Errorf("TouchOwnerRepositoriesInTransaction: owner %d: %w", ownerID, err)
	}
	return nil
}

// DisableRepositoryInTransaction closes a repository's public Pages site in
// the caller's transaction using the timestamp reserved by LockLifecycleState.
func DisableRepositoryInTransaction(ctx context.Context, repoID, updated int64) error {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("DisableRepositoryInTransaction: repository %d: %w", repoID, err)
	}
	if !has || site.Deleted || !site.Enabled {
		return nil
	}
	site.Enabled = false
	site.PagesUpdatedUnixNano = max(updated, site.PagesUpdatedUnixNano+1)
	if _, err := db.GetEngine(ctx).ID(repoID).NoAutoCondition().AllCols().Update(site); err != nil {
		return fmt.Errorf("DisableRepositoryInTransaction: repository %d: %w", repoID, err)
	}
	return nil
}

// TouchRepositoryInTransaction announces a repository path or ownership
// change to Pages without changing its published content. The caller holds
// LockLifecycleState for the surrounding transaction.
func TouchRepositoryInTransaction(ctx context.Context, repoID, updated int64) error {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("TouchRepositoryInTransaction: repository %d: %w", repoID, err)
	}
	if !has || site.Deleted {
		return nil
	}
	if site.PagesUpdatedUnixNano >= updated {
		updated = site.PagesUpdatedUnixNano + 1
	}
	if _, err := db.GetEngine(ctx).ID(repoID).Cols("pages_updated_unix_nano").Update(&pages_model.Site{PagesUpdatedUnixNano: updated}); err != nil {
		return fmt.Errorf("TouchRepositoryInTransaction: repository %d: %w", repoID, err)
	}
	return nil
}

// MarkRepositoryDeletedInTransaction records repository deletion before the repository row
// is removed. The compact row remains permanently so returning instances can
// observe the deletion by repository ID without relying on repository names.
//
// The returned digests identify every retained Pages object. The caller may
// remove those objects after its surrounding repository transaction
// commits. The caller holds LockLifecycleState for the surrounding transaction.
func MarkRepositoryDeletedInTransaction(ctx context.Context, repoID, updated int64) ([]string, error) {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: repository %d: %w", repoID, err)
	}
	if !has || site.Deleted {
		return nil, nil
	}
	versions, _, err := pages_model.FindVersions(ctx, &pages_model.FindVersionOptions{ListOptions: db.ListOptionsAll, RepoID: repoID})
	if err != nil {
		return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: list versions for repository %d: %w", repoID, err)
	}
	digests := make([]string, 0, len(versions))
	for _, version := range versions {
		digests = append(digests, version.TarGzSHA256)
	}
	site.Deleted = true
	site.Enabled = false
	site.SourceType = ""
	site.SourceBranch = ""
	site.SourcePath = ""
	site.SourceCommitID = ""
	site.SourceNextCheckUnix = 0
	site.SourceErrorCode = ""
	site.CurrentVersionID = 0
	site.PagesUpdatedUnixNano = max(updated, site.PagesUpdatedUnixNano+1)
	if _, err := db.GetEngine(ctx).ID(repoID).NoAutoCondition().AllCols().Update(site); err != nil {
		return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: repository %d: %w", repoID, err)
	}
	if _, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(new(pages_model.Version)); err != nil {
		return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: remove versions for repository %d: %w", repoID, err)
	}
	// Pending domains were never served, so only active bindings need an
	// acknowledgement before their host reservation is released.
	if _, err := db.GetEngine(ctx).Where("repo_id = ? AND state = ?", repoID, pages_model.DomainStatePending).Delete(new(pages_model.Domain)); err != nil {
		return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: remove pending domain for repository %d: %w", repoID, err)
	}
	if _, err := db.GetEngine(ctx).Where("repo_id = ? AND state = ?", repoID, pages_model.DomainStateActive).
		Cols("state").Update(&pages_model.Domain{State: pages_model.DomainStateReleasing}); err != nil {
		return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: release domains for repository %d: %w", repoID, err)
	}
	domains, err := pages_model.ListDomainsByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: list domains for repository %d: %w", repoID, err)
	}
	if len(domains) == 0 {
		if _, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(new(pages_model.InstanceResult)); err != nil {
			return nil, fmt.Errorf("MarkRepositoryDeletedInTransaction: remove results for repository %d: %w", repoID, err)
		}
	}
	return digests, nil
}
