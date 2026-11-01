// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"errors"
	"io"
	"os"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

// StoreMissingLfsObjectsInRepository downloads missing LFS objects
func StoreMissingLfsObjectsInRepository(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, lfsClient lfs.Client) error {
	ctx, op, err := Begin(ctx, repo)
	if err != nil {
		return err
	}
	defer op.Finish()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	contentStore := NewContentStore(ctx)

	pointerChan := make(chan lfs.PointerBlob)
	errChan := make(chan error, 1)
	go func() {
		errChan <- lfs.SearchPointerBlobs(ctx, gitRepo, pointerChan)
	}()

	// some servers require `ref`, send the default branch for objects from all refs like
	// `git lfs fetch --all` does on the main branch: it uses current branch as `ref`:
	// https://github.com/git-lfs/git-lfs/blob/58a763d09d5fe4b7ea85c89ec799d71c5b0dc1da/commands/commands.go#L124-L137
	// `man git-lfs-fetch` said that "--all" can fetch all objects, so the default branch should work
	ref := git.RefNameFromBranch(repo.DefaultBranch).String()
	downloadObjects := func(pointers []lfs.Pointer) error {
		err := lfsClient.Download(ctx, ref, pointers, func(p lfs.Pointer, content io.ReadCloser, objectError error) error {
			if errors.Is(objectError, lfs.ErrObjectNotExist) {
				log.Warn("Ignoring missing upstream LFS object %-v: %v", p, objectError)
				return nil
			}

			if objectError != nil {
				return objectError
			}

			defer content.Close()
			if err := op.Check(ctx, 0, p.Size); err != nil {
				return err
			}

			_, err := AssociateLFS(ctx, repo.ID, p)
			if err != nil {
				log.Error("Repo[%-v]: Error creating LFS meta object %-v: %v", repo, p, err)
				return err
			}

			if err := contentStore.Put(p, content); err != nil {
				log.Error("Repo[%-v]: Error storing content for LFS meta object %-v: %v", repo, p, err)
				if _, err2 := git_model.RemoveLFSMetaObjectByOid(ctx, repo.ID, p.Oid); err2 != nil {
					log.Error("Repo[%-v]: Error removing LFS meta object %-v: %v", repo, p, err2)
				}
				return err
			}
			return nil
		})
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
		}
		return err
	}

	var batch []lfs.Pointer
	seen := make(map[string]bool)
	for pointerBlob := range pointerChan {
		if seen[pointerBlob.Oid] {
			continue
		}
		seen[pointerBlob.Oid] = true
		meta, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, pointerBlob.Oid)
		if err != nil && err != git_model.ErrLFSObjectNotExist {
			log.Error("Repo[%-v]: Error querying LFS meta object %-v: %v", repo, pointerBlob.Pointer, err)
			return err
		}
		if meta != nil {
			log.Trace("Repo[%-v]: Skipping unknown LFS meta object %-v", repo, pointerBlob.Pointer)
			continue
		}

		log.Trace("Repo[%-v]: LFS object %-v not present in repository", repo, pointerBlob.Pointer)

		info, err := contentStore.Stat(pointerBlob.Pointer)
		if err != nil && !os.IsNotExist(err) {
			log.Error("Repo[%-v]: Error checking if LFS object %-v exists: %v", repo, pointerBlob.Pointer, err)
			return err
		}

		if err == nil {
			if info.Size() != pointerBlob.Size {
				return lfs.ErrSizeMismatch
			}
			log.Trace("Repo[%-v]: LFS object %-v already present; creating meta object", repo, pointerBlob.Pointer)
			_, err := AssociateLFS(ctx, repo.ID, pointerBlob.Pointer)
			if err != nil {
				log.Error("Repo[%-v]: Error creating LFS meta object %-v: %v", repo, pointerBlob.Pointer, err)
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

	err = <-errChan
	if err != nil {
		log.Error("Repo[%-v]: Error enumerating LFS objects for repository: %v", repo, err)
	}

	return err
}
