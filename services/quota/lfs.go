// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"errors"
	"math"
	"path/filepath"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/util"
)

func NewContentStore(ctx context.Context) *lfs.ContentStore {
	store := lfs.NewContentStore()
	if op, ok := ctx.Value(contextKey{}).(*Operation); ok {
		store.TemporaryPath = filepath.Join(filepath.Dir(gitrepo.RepoLocalPath(op.Repo)), ".gitea-quota-"+op.Token+"-lfs")
	}
	return store
}

func AssociateLFS(ctx context.Context, repoID int64, p lfs.Pointer) (*git_model.LFSMetaObject, error) {
	if !p.IsValid() {
		return nil, util.NewInvalidArgumentErrorf("invalid LFS object")
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, err
	}
	ctx, op, err := Begin(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer op.Finish()
	if op != nil {
		op.lfsMu.Lock()
		defer op.lfsMu.Unlock()
	}
	meta, err := git_model.GetLFSMetaObjectByOid(ctx, repoID, p.Oid)
	if err == nil {
		if meta.Size != p.Size {
			return nil, lfs.ErrSizeMismatch
		}
		return meta, nil
	}
	if !errors.Is(err, git_model.ErrLFSObjectNotExist) {
		return nil, err
	}
	if err = op.Check(ctx, 0, p.Size); err != nil {
		return nil, err
	}
	meta, err = git_model.NewLFSMetaObject(ctx, repoID, p)
	if err == nil {
		err = op.Refresh(ctx)
	}
	return meta, err
}

func CopyLFS(ctx context.Context, dest, source *repo_model.Repository) error {
	ctx, op, err := Begin(ctx, dest)
	if err != nil {
		return err
	}
	defer op.Finish()
	objects, err := git_model.GetLFSMetaObjects(ctx, source.ID, 0, 0)
	if err != nil {
		return err
	}
	for _, object := range objects {
		if _, err = AssociateLFS(ctx, dest.ID, object.Pointer); err != nil {
			return err
		}
	}
	return nil
}

func AutoAssociateLFS(ctx context.Context, metas []*git_model.LFSMetaObject, user *user_model.User, repoID int64) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	ctx, op, err := Begin(ctx, repo)
	if err != nil {
		return err
	}
	defer op.Finish()
	var growth int64
	seen := make(map[string]bool)
	for _, meta := range metas {
		if !meta.Pointer.IsValid() {
			return util.NewInvalidArgumentErrorf("invalid LFS object")
		}
		if seen[meta.Oid] {
			continue
		}
		seen[meta.Oid] = true
		info, err := lfs.NewContentStore().Stat(meta.Pointer)
		if err != nil {
			return err
		}
		meta.Size = info.Size()
		_, err = git_model.GetLFSMetaObjectByOid(ctx, repoID, meta.Oid)
		if err == nil {
			continue
		}
		if !errors.Is(err, git_model.ErrLFSObjectNotExist) {
			return err
		}
		if meta.Size < 0 || meta.Size > math.MaxInt64-growth {
			return lfs.ErrSizeMismatch
		}
		growth += meta.Size
	}
	if err = op.Check(ctx, 0, growth); err != nil {
		return err
	}
	if err = git_model.LFSAutoAssociate(ctx, metas, user, repoID); err != nil {
		return err
	}
	return op.Refresh(ctx)
}
