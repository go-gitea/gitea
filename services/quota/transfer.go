// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"slices"

	"gitea.dev/models/db"
	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
)

func BeginTransfer(ctx context.Context, repo *repo_model.Repository, dest *user_model.User) (func(), error) {
	enabled, err := Enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled || repo.OwnerID == dest.ID {
		return func() {}, nil
	}
	owners := []int64{repo.OwnerID, dest.ID}
	slices.Sort(owners)
	var locks []*quota_model.Operation
	release := func() {
		for _, lock := range locks {
			if err := quota_model.Release(context.WithoutCancel(ctx), lock); err != nil {
				log.Error("Releasing transfer quota reservation: %v", err)
			}
		}
	}
	for _, ownerID := range owners {
		op := &quota_model.Operation{OwnerID: ownerID, RepoID: repo.ID, Token: rand.Text()}
		if err = quota_model.Acquire(ctx, op); err != nil {
			release()
			return nil, err
		}
		locks = append(locks, op)
	}
	usage, err := Measure(ctx, repo)
	if err != nil {
		release()
		return nil, err
	}
	if usage.LFSBytes > math.MaxInt64-usage.GitBytes {
		release()
		return nil, errors.New("repository storage size overflow")
	}
	var repos []*repo_model.Repository
	if err = db.GetEngine(ctx).Where("owner_id=?", dest.ID).Find(&repos); err != nil {
		release()
		return nil, err
	}
	for _, owned := range repos {
		if _, err = Reconcile(ctx, owned); err != nil {
			release()
			return nil, err
		}
	}
	used, err := quota_model.OwnerUsage(ctx, dest.ID)
	if err != nil {
		release()
		return nil, err
	}
	policy, err := quota_model.GetPolicy(ctx, Subject(dest), dest.ID)
	if err != nil {
		release()
		return nil, err
	}
	_, _, limit := policy.Limits()
	if err = quota_model.Check("destination account", limit, used, usage.GitBytes+usage.LFSBytes); err != nil {
		release()
		return nil, err
	}
	return release, nil
}
