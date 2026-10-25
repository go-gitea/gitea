// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
)

func init() { git.GuardStorageMutation = GuardGitMutation }

func GuardGitMutation(ctx context.Context, target git.RepositoryFacade, mutation git.StorageMutation, run git.StorageMutationRunner) error {
	id, managed := gitrepo.ManagedID(target)
	if !managed {
		return run(ctx, target, nil)
	}
	id = strings.TrimPrefix(strings.TrimPrefix(id, "repo-"), "wiki-")
	repoID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("GuardGitMutation: invalid managed repository: %w", err)
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	ctx, op, err := Begin(ctx, repo)
	if err != nil {
		return err
	}
	defer op.Finish()
	if op == nil {
		return run(ctx, target, nil)
	}
	if err = op.Refresh(ctx); err != nil {
		return err
	}
	if mutation == git.StoragePush {
		return run(ctx, target, op.Environment(os.Environ()))
	}
	return op.stageGit(ctx, target, mutation, run)
}

func (op *Operation) stageGit(ctx context.Context, target git.RepositoryFacade, mutation git.StorageMutation, run git.StorageMutationRunner) (retErr error) {
	livePath := gitrepo.RepoLocalPath(target)
	if err := os.MkdirAll(filepath.Dir(livePath), 0o755); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(filepath.Dir(livePath), ".gitea-quota-"+op.Token+"-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			*op.cleanupFailed = true
			retErr = errors.Join(retErr, quota_model.MarkRecovery(context.WithoutCancel(ctx), op.Operation))
			retErr = errors.Join(retErr, err)
		}
	}()
	stage := gitrepo.RepositoryUnmanaged(filepath.Join(dir, "repo.git"))
	var baseRefs map[string]string
	if mutation == git.StorageFetch {
		if err := git.CloneRepoToLocal(ctx, target, gitrepo.RepoLocalPath(stage), git.CloneRepoOptions{Mirror: true, Shared: true}); err != nil {
			return err
		}
		config, err := os.ReadFile(filepath.Join(livePath, "config"))
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(gitrepo.RepoLocalPath(stage), "config"), config, 0o600); err != nil {
			return err
		}
		baseRefs, err = refSnapshot(ctx, stage)
		if err != nil {
			return err
		}
	}
	stageCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-stageCtx.Done():
				return
			case <-ticker.C:
				size, err := ObjectBytes(filepath.Join(gitrepo.RepoLocalPath(stage), "objects"))
				if err == nil {
					err = op.Check(ctx, size, 0)
				}
				if err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	err = run(stageCtx, stage, op.Environment(os.Environ()))
	close(stop)
	<-done
	if cause := context.Cause(stageCtx); cause != nil {
		return cause
	}
	if err != nil {
		return err
	}
	size, err := ObjectBytes(filepath.Join(gitrepo.RepoLocalPath(stage), "objects"))
	if err != nil {
		return err
	}
	if err = op.Check(ctx, size, 0); err != nil {
		return err
	}
	if mutation == git.StorageClone {
		if entries, err := os.ReadDir(livePath); err == nil {
			if len(entries) != 0 {
				return errors.New("clone destination is not empty")
			}
			if err = os.Remove(livePath); err != nil {
				return err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return os.Rename(gitrepo.RepoLocalPath(stage), livePath)
	}
	return op.promoteFetch(ctx, target, stage, baseRefs)
}

func refSnapshot(ctx context.Context, repo git.RepositoryFacade) (map[string]string, error) {
	output, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(refname) %(objectname)").WithRepo(repo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	refs := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if ref, oid, ok := strings.Cut(line, " "); ok {
			refs[ref] = oid
		}
	}
	return refs, nil
}

func (op *Operation) promoteFetch(ctx context.Context, target, stage git.RepositoryFacade, oldRefs map[string]string) (retErr error) {
	newRefs, err := refSnapshot(ctx, stage)
	if err != nil {
		return err
	}
	var installed []string
	publishing := false
	defer func() {
		if retErr != nil && !publishing {
			for _, path := range installed {
				if err := os.Remove(path); err != nil {
					*op.cleanupFailed = true
					retErr = errors.Join(retErr, err, quota_model.MarkRecovery(context.WithoutCancel(ctx), op.Operation))
				}
			}
		}
	}()
	objects := filepath.Join(gitrepo.RepoLocalPath(stage), "objects")
	err = filepath.WalkDir(objects, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "info" {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("unexpected staged Git object type")
		}
		rel, err := filepath.Rel(objects, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(gitrepo.RepoLocalPath(target), "objects", rel)
		if err = os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err = os.Link(path, dest); err == nil {
			installed = append(installed, dest)
			return nil
		} else if errors.Is(err, fs.ErrExist) {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			return nil
		}
		if err != nil {
			return err
		}
		installed = append(installed, dest)
		_, err = io.Copy(out, src)
		return errors.Join(err, out.Close())
	})
	if err != nil {
		return err
	}
	var commands strings.Builder
	commands.WriteString("start\n")
	for ref, oid := range newRefs {
		if oldRefs[ref] == oid {
			continue
		}
		if old := oldRefs[ref]; old != "" {
			fmt.Fprintf(&commands, "update %s %s %s\n", ref, oid, old)
		} else {
			fmt.Fprintf(&commands, "create %s %s\n", ref, oid)
		}
	}
	for ref, oid := range oldRefs {
		if _, exists := newRefs[ref]; !exists {
			fmt.Fprintf(&commands, "delete %s %s\n", ref, oid)
		}
	}
	commands.WriteString("prepare\ncommit\n")
	publishing = true
	return gitcmd.NewCommand("update-ref", "--stdin").WithRepo(target).WithStdinBytes([]byte(commands.String())).Run(ctx)
}

func Fetch(ctx context.Context, repo git.RepositoryFacade, run git.StorageMutationRunner) error {
	return GuardGitMutation(ctx, repo, git.StorageFetch, run)
}

func IsQuotaError(err error) bool {
	var exceeded quota_model.ExceededError
	return errors.As(err, &exceeded) || errors.Is(err, quota_model.ErrBusy) || errors.Is(err, quota_model.ErrUncertain)
}
