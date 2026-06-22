// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

/*
GitHub, GitLab, Gogs: *.wiki.git
BitBucket: *.git/wiki
*/
var commonWikiURLSuffixes = []string{".wiki.git", ".git/wiki"}

// WikiRemoteURL returns accessible repository URL for wiki if exists.
// Otherwise, it returns an empty string.
func WikiRemoteURL(ctx context.Context, remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	for _, suffix := range commonWikiURLSuffixes {
		wikiURL := remote + suffix
		if git.IsRepoURLAccessible(ctx, wikiURL) {
			return wikiURL
		}
	}
	return ""
}

// SyncRepoTags synchronizes releases table with repository tags
func SyncRepoTags(ctx context.Context, repoID int64) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	_, err = SyncReleasesWithTags(ctx, repo, gitRepo)
	return err
}

// StoreMissingLfsObjectsInRepository downloads missing LFS objects using a full
// repository scan. Used for initial mirror sync and migrations.
func StoreMissingLfsObjectsInRepository(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, lfsClient lfs.Client) error {
	contentStore := lfs.NewContentStore()

	pointerChan := make(chan lfs.PointerBlob)
	errChan := make(chan error, 1)
	go func() {
		errChan <- lfs.SearchPointerBlobs(ctx, gitRepo, pointerChan)
	}()

	if err := processLFSPointerChan(ctx, repo, contentStore, lfsClient, pointerChan); err != nil {
		return err
	}

	err, has := <-errChan
	if has {
		log.Error("Repo[%-v]: Error enumerating LFS objects for repository: %v", repo, err)
		return err
	}

	return nil
}

// SyncMirrorLfsObjects performs an incremental LFS sync for a pull mirror. It
// scans only objects reachable from current refs but not from lastRefs (the ref
// tips recorded after the previous successful sync), then retries any previously
// discovered pointers whose content was unavailable upstream.
// Returns the current ref tips to be stored for the next sync.
func SyncMirrorLfsObjects(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, lfsClient lfs.Client, lastRefs []string) (currentRefs []string, err error) {
	contentStore := lfs.NewContentStore()

	currentRefs, err = getCurrentRefSHAs(ctx, gitRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to get current refs: %w", err)
	}

	if len(lastRefs) == 0 {
		log.Trace("Repo[%-v]: No previous LFS ref state, performing full scan", repo)
		if err := StoreMissingLfsObjectsInRepository(ctx, repo, gitRepo, lfsClient); err != nil {
			return nil, err
		}
		return currentRefs, nil
	}

	// Incremental scan: only look at objects new since last sync
	pointerChan := make(chan lfs.PointerBlob)
	errChan := make(chan error, 1)
	go lfs.SearchPointerBlobsInRange(ctx, gitRepo, currentRefs, lastRefs, pointerChan, errChan)

	if err := processLFSPointerChan(ctx, repo, contentStore, lfsClient, pointerChan); err != nil {
		return nil, err
	}

	scanErr, has := <-errChan
	if has {
		log.Error("Repo[%-v]: Error in incremental LFS scan: %v", repo, scanErr)
		return nil, scanErr
	}

	// Retry previously failed (pending) objects
	if err := retryPendingLFSObjects(ctx, repo, contentStore, lfsClient); err != nil {
		return nil, err
	}

	return currentRefs, nil
}

// processLFSPointerChan processes discovered LFS pointers: records them as
// pending in lfs_mirror_pending, then attempts to download their content from
// the upstream LFS server. Successfully downloaded objects are promoted to
// lfs_meta_object and removed from the pending table.
func processLFSPointerChan(ctx context.Context, repo *repo_model.Repository, contentStore *lfs.ContentStore, lfsClient lfs.Client, pointerChan <-chan lfs.PointerBlob) error {
	downloadObjects := func(pointers []lfs.Pointer) error {
		err := lfsClient.Download(ctx, pointers, func(p lfs.Pointer, content io.ReadCloser, objectError error) error {
			if errors.Is(objectError, lfs.ErrObjectNotExist) {
				log.Warn("Repo[%-v]: Upstream missing LFS object %-v: %v", repo, p, objectError)
				return nil
			}

			if objectError != nil {
				return objectError
			}

			defer content.Close()

			if err := contentStore.Put(p, content); err != nil {
				log.Error("Repo[%-v]: Error storing content for LFS meta object %-v: %v", repo, p, err)
				return err
			}
			if _, err := git_model.NewLFSMetaObject(ctx, repo.ID, p); err != nil {
				log.Error("Repo[%-v]: Error creating LFS meta object %-v: %v", repo, p, err)
				return err
			}
			if err := git_model.RemoveLFSMirrorPending(ctx, repo.ID, p.Oid); err != nil {
				log.Error("Repo[%-v]: Error removing pending LFS object %-v: %v", repo, p, err)
				return err
			}
			return nil
		})
		// err may be a context-cancellation error if the parent context was
		// killed; nil on the happy path, a real error otherwise. We deliberately
		// return it unmodified so a cancelled/failed sync does not advance the
		// mirror's LFSLastRefs watermark (see runSync).
		return err
	}

	var batch []lfs.Pointer
	for pointerBlob := range pointerChan {
		// Record as pending if not already tracked
		isNew, err := git_model.AddLFSMirrorPending(ctx, repo.ID, pointerBlob)
		if err != nil {
			log.Error("Repo[%-v]: Error recording pending LFS object %-v: %v", repo, pointerBlob.Pointer, err)
			return err
		}
		if !isNew {
			continue
		}

		// Check if content already exists in the local store
		exist, err := contentStore.Exists(pointerBlob.Pointer)
		if err != nil {
			log.Error("Repo[%-v]: Error checking if LFS object %-v exists: %v", repo, pointerBlob.Pointer, err)
			return err
		}

		if exist {
			log.Trace("Repo[%-v]: LFS object %-v already present; creating meta object", repo, pointerBlob.Pointer)
			if _, err := git_model.NewLFSMetaObject(ctx, repo.ID, pointerBlob.Pointer); err != nil {
				log.Error("Repo[%-v]: Error creating LFS meta object %-v: %v", repo, pointerBlob.Pointer, err)
				return err
			}
			if err := git_model.RemoveLFSMirrorPending(ctx, repo.ID, pointerBlob.Oid); err != nil {
				log.Error("Repo[%-v]: Error removing pending LFS object %-v: %v", repo, pointerBlob.Pointer, err)
				return err
			}
		} else {
			if setting.LFS.MaxFileSize > 0 && pointerBlob.Size > setting.LFS.MaxFileSize {
				log.Info("Repo[%-v]: LFS object %-v download denied because of LFS_MAX_FILE_SIZE=%d < size %d", repo, pointerBlob.Pointer, setting.LFS.MaxFileSize, pointerBlob.Size)
				continue
			}

			batch = append(batch, pointerBlob.Pointer)
			if len(batch) >= lfsClient.BatchSize() {
				if err := downloadObjects(batch); err != nil {
					return err
				}
				batch = nil
			}
		}
	}
	if len(batch) > 0 {
		if err := downloadObjects(batch); err != nil {
			return err
		}
	}
	return nil
}

// retryPendingLFSObjects re-attempts download of LFS objects that were
// previously discovered but whose content was unavailable upstream.
func retryPendingLFSObjects(ctx context.Context, repo *repo_model.Repository, contentStore *lfs.ContentStore, lfsClient lfs.Client) error {
	pending, err := git_model.GetLFSMirrorPendingByRepoID(ctx, repo.ID)
	if err != nil {
		log.Error("Repo[%-v]: Error getting pending LFS objects: %v", repo, err)
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	log.Trace("Repo[%-v]: Retrying %d pending LFS objects", repo, len(pending))

	downloadObjects := func(pointers []lfs.Pointer) error {
		err := lfsClient.Download(ctx, pointers, func(p lfs.Pointer, content io.ReadCloser, objectError error) error {
			if errors.Is(objectError, lfs.ErrObjectNotExist) {
				log.Trace("Repo[%-v]: Upstream still missing LFS object %-v", repo, p)
				return nil
			}
			if objectError != nil {
				return objectError
			}

			defer content.Close()

			if err := contentStore.Put(p, content); err != nil {
				log.Error("Repo[%-v]: Error storing content for LFS object %-v: %v", repo, p, err)
				return err
			}
			if _, err := git_model.NewLFSMetaObject(ctx, repo.ID, p); err != nil {
				log.Error("Repo[%-v]: Error creating LFS meta object %-v: %v", repo, p, err)
				return err
			}
			if err := git_model.RemoveLFSMirrorPending(ctx, repo.ID, p.Oid); err != nil {
				log.Error("Repo[%-v]: Error removing pending LFS object %-v: %v", repo, p, err)
				return err
			}
			return nil
		})
		// See processLFSPointerChan: return err unmodified (including any
		// context-cancellation error) so a cancelled retry does not let the
		// caller advance LFSLastRefs past objects that were never downloaded.
		return err
	}

	var batch []lfs.Pointer
	for _, obj := range pending {
		if setting.LFS.MaxFileSize > 0 && obj.Size > setting.LFS.MaxFileSize {
			continue
		}
		batch = append(batch, lfs.Pointer{Oid: obj.Oid, Size: obj.Size})
		if len(batch) >= lfsClient.BatchSize() {
			if err := downloadObjects(batch); err != nil {
				return err
			}
			batch = nil
		}
	}
	if len(batch) > 0 {
		if err := downloadObjects(batch); err != nil {
			return err
		}
	}
	return nil
}

// getCurrentRefSHAs returns the SHA of every ref in the repository.
func getCurrentRefSHAs(ctx context.Context, gitRepo *git.Repository) ([]string, error) {
	allRefs, err := gitRepo.GetRefs(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0, len(allRefs))
	for _, ref := range allRefs {
		refs = append(refs, ref.Object.String())
	}
	return refs, nil
}

// shortRelease to reduce load memory, this struct can replace repo_model.Release
type shortRelease struct {
	ID      int64
	TagName string
	Sha1    string
	IsTag   bool
}

func (shortRelease) TableName() string {
	return "release"
}

// SyncReleasesWithTags is a tag<->release table
// synchronization which overwrites all Releases from the repository tags. This
// can be relied on since a pull-mirror is always identical to its
// upstream. Hence, after each sync we want the release set to be
// identical to the upstream tag set. This is much more efficient for
// repositories like https://github.com/vim/vim (with over 13000 tags).
func SyncReleasesWithTags(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository) ([]*SyncResult, error) {
	log.Debug("SyncReleasesWithTags: in Repo[%d:%s/%s]", repo.ID, repo.OwnerName, repo.Name)
	tags, _, err := gitRepo.GetTagInfos(ctx, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("unable to GetTagInfos in pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
	}
	var added, deleted, updated int
	var syncResults []*SyncResult
	err = db.WithTx(ctx, func(ctx context.Context) error {
		dbReleases, err := db.Find[shortRelease](ctx, repo_model.FindReleasesOptions{
			RepoID:        repo.ID,
			IncludeDrafts: true,
			IncludeTags:   true,
		})
		if err != nil {
			return fmt.Errorf("unable to FindReleases in pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
		}

		dbReleasesByID := make(map[int64]*shortRelease, len(dbReleases))
		dbReleasesByTag := make(map[string]*shortRelease, len(dbReleases))
		for _, release := range dbReleases {
			dbReleasesByID[release.ID] = release
			dbReleasesByTag[release.TagName] = release
		}

		inserts, deletes, updates := calcSync(tags, dbReleases)
		syncResults = make([]*SyncResult, 0, len(inserts)+len(deletes)+len(updates))
		for _, tag := range inserts {
			syncResults = append(syncResults, &SyncResult{
				RefName:     git.RefNameFromTag(tag.Name),
				OldCommitID: "",
				NewCommitID: tag.Object.RefName(),
			})
		}
		for _, deleteID := range deletes {
			release := dbReleasesByID[deleteID]
			if release == nil {
				continue
			}
			syncResults = append(syncResults, &SyncResult{
				RefName:     git.RefNameFromTag(release.TagName),
				OldCommitID: git.RefNameFromCommit(release.Sha1),
				NewCommitID: "",
			})
		}
		for _, tag := range updates {
			release := dbReleasesByTag[tag.Name]
			var oldCommitID git.RefName
			if release != nil {
				oldCommitID = git.RefNameFromCommit(release.Sha1)
			}
			syncResults = append(syncResults, &SyncResult{
				RefName:     git.RefNameFromTag(tag.Name),
				OldCommitID: oldCommitID,
				NewCommitID: tag.Object.RefName(),
			})
		}
		//
		// make release set identical to upstream tags
		//
		for _, tag := range inserts {
			release := repo_model.Release{
				RepoID:       repo.ID,
				TagName:      tag.Name,
				LowerTagName: strings.ToLower(tag.Name),
				Sha1:         tag.Object.String(),
				// NOTE: ignored, The NumCommits value is calculated and cached on demand when the UI requires it.
				NumCommits:    -1,
				CreatedUnix:   timeutil.TimeStamp(util.IfZero(tag.CommitDate, tag.Tagger.When).Unix()),
				PublishedUnix: timeutil.TimeStamp(tag.Tagger.When.Unix()),
				IsTag:         true,
			}
			if err := db.Insert(ctx, release); err != nil {
				return fmt.Errorf("unable insert tag %s for pull-mirror Repo[%d:%s/%s]: %w", tag.Name, repo.ID, repo.OwnerName, repo.Name, err)
			}
		}

		// only delete tags releases
		if len(deletes) > 0 {
			if _, err := db.GetEngine(ctx).Where("repo_id=?", repo.ID).
				In("id", deletes).
				Delete(&repo_model.Release{}); err != nil {
				return fmt.Errorf("unable to delete tags for pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
			}
		}

		for _, tag := range updates {
			if _, err := db.GetEngine(ctx).Where("repo_id = ? AND lower_tag_name = ?", repo.ID, strings.ToLower(tag.Name)).
				Cols("sha1", "created_unix", "published_unix").
				Update(&repo_model.Release{
					Sha1:          tag.Object.String(),
					CreatedUnix:   timeutil.TimeStamp(util.IfZero(tag.CommitDate, tag.Tagger.When).Unix()),
					PublishedUnix: timeutil.TimeStamp(tag.Tagger.When.Unix()),
				}); err != nil {
				return fmt.Errorf("unable to update tag %s for pull-mirror Repo[%d:%s/%s]: %w", tag.Name, repo.ID, repo.OwnerName, repo.Name, err)
			}
		}
		added, deleted, updated = len(inserts), len(deletes), len(updates)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to rebuild release table for pull-mirror Repo[%d:%s/%s]: %w", repo.ID, repo.OwnerName, repo.Name, err)
	}

	log.Trace("SyncReleasesWithTags: %d tags added, %d tags deleted, %d tags updated", added, deleted, updated)
	return syncResults, nil
}

func calcSync(destTags []*git.Tag, dbTags []*shortRelease) ([]*git.Tag, []int64, []*git.Tag) {
	destTagMap := make(map[string]*git.Tag)
	for _, tag := range destTags {
		destTagMap[tag.Name] = tag
	}
	dbTagMap := make(map[string]*shortRelease)
	for _, rel := range dbTags {
		dbTagMap[rel.TagName] = rel
	}

	inserted := make([]*git.Tag, 0, 10)
	updated := make([]*git.Tag, 0, 10)
	for _, tag := range destTags {
		rel := dbTagMap[tag.Name]
		if rel == nil {
			inserted = append(inserted, tag)
		} else if rel.Sha1 != tag.Object.String() {
			updated = append(updated, tag)
		}
	}
	deleted := make([]int64, 0, 10)
	for _, tag := range dbTags {
		if destTagMap[tag.TagName] == nil && tag.IsTag {
			deleted = append(deleted, tag.ID)
		}
	}
	return inserted, deleted, updated
}
