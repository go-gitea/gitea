// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

const EnvOperation = "GITEA_QUOTA_OPERATION"

type contextKey struct{}

type Operation struct {
	*quota_model.Operation
	Repo          *repo_model.Repository
	GitLimit      int64
	LFSLimit      int64
	AccountLimit  int64
	GitBefore     int64
	LFSBefore     int64
	AccountBefore int64
	owned         bool
	cleanupFailed *bool
	lfsMu         *sync.Mutex
	finishCtx     context.Context
}

func Bind(ctx interface {
	context.Context
	SetContextValue(any, any)
}, repo *repo_model.Repository, token string,
) (release func(), err error) {
	if token == "" {
		return func() {}, nil
	}
	stored, err := quota_model.GetOperation(ctx, repo.OwnerID, token)
	if err != nil {
		return nil, err
	}
	if stored.RepoID != repo.ID {
		return nil, quota_model.ErrUncertain
	}
	if err = quota_model.AttachChild(ctx, stored); err != nil {
		return nil, err
	}
	release = func() {
		detachCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := quota_model.DetachChild(detachCtx, stored); err != nil {
			log.Error("Detaching quota operation %s: %v", token, err)
		}
	}
	detach := release
	defer func() {
		if err != nil {
			detach()
		}
	}()
	usage, has, err := quota_model.GetUsage(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	if !has || usage.Updated == 0 {
		err = quota_model.ErrUncertain
		return nil, err
	}
	gitLimit, lfsLimit, accountLimit, err := EffectiveLimits(ctx, repo)
	if err != nil {
		return nil, err
	}
	accountBefore, err := quota_model.OwnerUsage(ctx, repo.OwnerID)
	if err != nil {
		return nil, err
	}
	ctx.SetContextValue(contextKey{}, &Operation{
		Operation: stored, Repo: repo,
		GitLimit: gitLimit, LFSLimit: lfsLimit, AccountLimit: accountLimit,
		GitBefore: usage.GitBytes, LFSBefore: usage.LFSBytes, AccountBefore: accountBefore, cleanupFailed: new(bool), lfsMu: new(sync.Mutex),
	})
	return release, nil
}

func Subject(owner *user_model.User) quota_model.Subject {
	if owner.IsOrganization() {
		return quota_model.Organization
	}
	return quota_model.User
}

func EffectiveLimits(ctx context.Context, repo *repo_model.Repository) (git, lfs, account int64, err error) {
	if err = repo.LoadOwner(ctx); err != nil {
		return
	}
	rp, err := quota_model.GetPolicy(ctx, quota_model.Repository, repo.ID)
	if err != nil {
		return 0, 0, 0, err
	}
	op, err := quota_model.GetPolicy(ctx, Subject(repo.Owner), repo.OwnerID)
	if err != nil {
		return 0, 0, 0, err
	}
	git, lfs, _ = rp.Limits()
	_, _, account = op.Limits()
	return
}

func ObjectBytes(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "info" || isQuarantineName(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			if info.Size() > math.MaxInt64-size {
				return errors.New("object storage size overflow")
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func Measure(ctx context.Context, repo *repo_model.Repository) (*quota_model.Usage, error) {
	usage := &quota_model.Usage{RepoID: repo.ID, OwnerID: repo.OwnerID}
	for _, storageRepo := range []gitrepo.RepositoryFacade{repo.CodeStorageRepo(), repo.WikiStorageRepo()} {
		size, err := ObjectBytes(filepath.Join(gitrepo.RepoLocalPath(storageRepo), "objects"))
		if err != nil {
			return nil, err
		}
		if size > math.MaxInt64-usage.GitBytes {
			return nil, errors.New("repository storage size overflow")
		}
		usage.GitBytes += size
	}
	var err error
	usage.LFSBytes, err = git_model.GetRepoLFSSize(ctx, repo.ID)
	return usage, err
}

func Reconcile(ctx context.Context, repo *repo_model.Repository) (*quota_model.Usage, error) {
	usage, err := Measure(ctx, repo)
	if err != nil {
		return nil, err
	}
	return usage, quota_model.StoreUsage(ctx, usage)
}

func Begin(ctx context.Context, repo *repo_model.Repository) (context.Context, *Operation, error) {
	if op, ok := ctx.Value(contextKey{}).(*Operation); ok && op.Repo.ID == repo.ID {
		if _, err := quota_model.GetOperation(ctx, op.OwnerID, op.Token); err == nil {
			nested := *op
			nested.owned = false
			return ctx, &nested, nil
		} else if !errors.Is(err, quota_model.ErrUncertain) {
			return ctx, nil, err
		}
	}
	gitLimit, lfsLimit, accountLimit, err := EffectiveLimits(ctx, repo)
	if err != nil {
		return ctx, nil, err
	}
	if gitLimit < 0 && lfsLimit < 0 && accountLimit < 0 {
		enabled, err := Enabled(ctx)
		if err != nil {
			return ctx, nil, err
		}
		if !enabled {
			_, err = db.GetEngine(ctx).ID(repo.ID).Delete(new(quota_model.Usage))
			return ctx, nil, err
		}
	}
	op := &Operation{
		Operation: &quota_model.Operation{OwnerID: repo.OwnerID, RepoID: repo.ID, Token: rand.Text()},
		Repo:      repo, GitLimit: gitLimit, LFSLimit: lfsLimit, AccountLimit: accountLimit, owned: true,
		cleanupFailed: new(bool),
		lfsMu:         new(sync.Mutex),
		finishCtx:     ctx,
	}
	if err = quota_model.Acquire(ctx, op.Operation); err != nil {
		return ctx, nil, err
	}
	success := false
	defer func() {
		if !success {
			if releaseErr := quota_model.Release(context.WithoutCancel(ctx), op.Operation); releaseErr != nil {
				log.Error("Releasing quota reservation: %v", releaseErr)
			}
		}
	}()
	if accountLimit >= 0 {
		var repos []*repo_model.Repository
		if err = db.GetEngine(ctx).Where("owner_id=?", repo.OwnerID).Find(&repos); err != nil {
			return ctx, nil, err
		}
		for _, ownedRepo := range repos {
			cached, found, readErr := quota_model.GetUsage(ctx, ownedRepo.ID)
			if readErr != nil {
				return ctx, nil, readErr
			}
			if !found || cached.Updated == 0 {
				if _, err = Reconcile(ctx, ownedRepo); err != nil {
					return ctx, nil, err
				}
			}
		}
	}
	usage, err := Reconcile(ctx, repo)
	if err != nil {
		return ctx, nil, err
	}
	op.GitBefore, op.LFSBefore = usage.GitBytes, usage.LFSBytes
	op.AccountBefore, err = quota_model.OwnerUsage(ctx, repo.OwnerID)
	if err != nil {
		return ctx, nil, err
	}
	accountRemaining := quota_model.Remaining(accountLimit, op.AccountBefore)
	op.GitReserved = min(quota_model.Remaining(gitLimit, usage.GitBytes), accountRemaining)
	op.LFSReserved = min(quota_model.Remaining(lfsLimit, usage.LFSBytes), accountRemaining)
	if err = quota_model.Reserve(ctx, op.Operation); err != nil {
		return ctx, nil, err
	}
	success = true
	return context.WithValue(ctx, contextKey{}, op), op, nil
}

func Enabled(ctx context.Context) (bool, error) {
	if setting.Quota.DefaultRepoGitLimit >= 0 || setting.Quota.DefaultRepoLFSLimit >= 0 || setting.Quota.DefaultUserLimit >= 0 || setting.Quota.DefaultOrgLimit >= 0 {
		return true, nil
	}
	return db.GetEngine(ctx).Exist(new(quota_model.Policy))
}

func (op *Operation) Environment(env []string) []string {
	if op == nil {
		return env
	}
	count, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "GIT_CONFIG_COUNT="); ok {
			count, _ = strconv.Atoi(value)
		}
	}
	return append(env, EnvOperation+"="+op.Token,
		fmt.Sprintf("GIT_CONFIG_COUNT=%d", count+2), fmt.Sprintf("GIT_CONFIG_KEY_%d=receive.maxInputSize", count), fmt.Sprintf("GIT_CONFIG_VALUE_%d=%d", count, max(1024, op.GitReserved)),
		fmt.Sprintf("GIT_CONFIG_KEY_%d=receive.autogc", count+1), fmt.Sprintf("GIT_CONFIG_VALUE_%d=false", count+1))
}

func Complete(ctx context.Context, repo *repo_model.Repository, token string) error {
	if token == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	op, err := quota_model.GetOperation(ctx, repo.OwnerID, token)
	if err != nil {
		return err
	}
	if op.RepoID != repo.ID {
		return quota_model.ErrUncertain
	}
	if err = closeOperation(ctx, op); err != nil {
		return err
	}
	if err = cleanupIncoming(repo); err != nil {
		return err
	}
	if err = cleanupStaging(repo, token); err != nil {
		return err
	}
	if _, err = Reconcile(ctx, repo); err != nil {
		return err
	}
	return quota_model.Release(ctx, op)
}

func (op *Operation) Check(ctx context.Context, gitGrowth, lfsGrowth int64) error {
	if op == nil {
		return nil
	}
	usage, err := Measure(ctx, op.Repo)
	if err != nil {
		return err
	}
	if err = quota_model.Check("repository Git", op.GitLimit, op.GitBefore, max(0, usage.GitBytes-op.GitBefore)+gitGrowth); err != nil {
		return err
	}
	if err = quota_model.Check("repository LFS", op.LFSLimit, op.LFSBefore, max(0, usage.LFSBytes-op.LFSBefore)+lfsGrowth); err != nil {
		return err
	}
	return quota_model.Check("account", op.AccountLimit, op.AccountBefore,
		max(0, usage.GitBytes-op.GitBefore)+max(0, usage.LFSBytes-op.LFSBefore)+gitGrowth+lfsGrowth)
}

func (op *Operation) Refresh(ctx context.Context) error {
	if op == nil {
		return nil
	}
	usage, err := Measure(ctx, op.Repo)
	if err != nil {
		return err
	}
	remaining := quota_model.Remaining(op.AccountLimit, op.AccountBefore)
	gitGrowth, lfsGrowth := max(0, usage.GitBytes-op.GitBefore), max(0, usage.LFSBytes-op.LFSBefore)
	if gitGrowth > remaining || lfsGrowth > remaining-gitGrowth {
		return quota_model.ExceededError{Resource: "account", Used: op.AccountBefore, Limit: op.AccountLimit}
	}
	remaining -= gitGrowth + lfsGrowth
	op.GitReserved = min(quota_model.Remaining(op.GitLimit, usage.GitBytes), remaining)
	op.LFSReserved = min(quota_model.Remaining(op.LFSLimit, usage.LFSBytes), remaining)
	return quota_model.Reserve(ctx, op.Operation)
}

func (op *Operation) Finish() {
	if op == nil || !op.owned {
		return
	}
	if *op.cleanupFailed {
		log.Error("Quota operation %s requires staging cleanup", op.Token)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(op.finishCtx), 30*time.Second)
	defer cancel()
	if _, err := quota_model.GetOperation(ctx, op.OwnerID, op.Token); err != nil {
		log.Error("Quota operation %s requires recovery: %v", op.Token, err)
		return
	}
	if err := closeOperation(ctx, op.Operation); err != nil {
		log.Error("Quota operation %s requires recovery: %v", op.Token, err)
		return
	}
	if err := cleanupIncoming(op.Repo); err != nil {
		log.Error("Quota operation %s requires quarantine cleanup: %v", op.Token, err)
		return
	}
	if err := cleanupStaging(op.Repo, op.Token); err != nil {
		log.Error("Quota operation %s requires staging cleanup: %v", op.Token, err)
		return
	}
	if _, err := Reconcile(ctx, op.Repo); err != nil {
		log.Error("Quota operation %s requires reconciliation: %v", op.Token, err)
		return
	}
	if err := quota_model.Release(ctx, op.Operation); err != nil {
		log.Error("Releasing quota operation %s: %v", op.Token, err)
	}
}

func closeOperation(ctx context.Context, op *quota_model.Operation) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		closed, err := quota_model.BeginClose(ctx, op)
		if err != nil || closed {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func cleanupStaging(repo *repo_model.Repository, token string) error {
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(gitrepo.RepoLocalPath(repo)), ".gitea-quota-"+token+"-*"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func cleanupIncoming(repo *repo_model.Repository) error {
	for _, storageRepo := range []gitrepo.RepositoryFacade{repo.CodeStorageRepo(), repo.WikiStorageRepo()} {
		root := filepath.Join(gitrepo.RepoLocalPath(storageRepo), "objects")
		entries, err := os.ReadDir(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if isQuarantineName(entry.Name()) {
				if err = os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func CheckReceive(ctx context.Context, repo *repo_model.Repository, token, quarantine string) error {
	gitLimit, _, accountLimit, err := EffectiveLimits(ctx, repo)
	if err != nil {
		return err
	}
	if gitLimit < 0 && accountLimit < 0 {
		return nil
	}
	op, err := quota_model.GetOperation(ctx, repo.OwnerID, token)
	if err != nil {
		return err
	}
	if op.RepoID != repo.ID {
		return quota_model.ErrUncertain
	}
	if quarantine == "" {
		return nil
	}
	path := filepath.Clean(quarantine)
	valid := false
	for _, storageRepo := range []gitrepo.RepositoryFacade{repo.CodeStorageRepo(), repo.WikiStorageRepo()} {
		root := filepath.Join(gitrepo.RepoLocalPath(storageRepo), "objects")
		if filepath.Dir(path) == root && isQuarantineName(filepath.Base(path)) {
			valid = true
		}
	}
	if !valid {
		return errors.New("invalid Git quarantine path")
	}
	// The root itself is a quarantine directory, so traverse its children individually.
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	var size int64
	for _, entry := range entries {
		n, err := ObjectBytes(filepath.Join(path, entry.Name()))
		if err != nil {
			return err
		}
		if n > op.GitReserved-size {
			return quota_model.ExceededError{Resource: "incoming Git objects", Used: size + n, Limit: op.GitReserved}
		}
		size += n
	}
	return nil
}

func isQuarantineName(name string) bool {
	return strings.HasPrefix(name, "tmp_objdir-incoming-") || strings.HasPrefix(name, "incoming-")
}
