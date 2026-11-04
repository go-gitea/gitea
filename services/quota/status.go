// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"crypto/rand"
	"errors"
	"math"

	"gitea.dev/models/db"
	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

func resolveSubject(ctx context.Context, subject quota_model.Subject, id int64) (ownerID int64, name string, repos []*repo_model.Repository, err error) {
	if id <= 0 {
		return 0, "", nil, util.ErrNotExist
	}
	if subject == quota_model.Repository {
		repo, err := repo_model.GetRepositoryByID(ctx, id)
		if err != nil {
			return 0, "", nil, err
		}
		return repo.OwnerID, repo.FullName(), []*repo_model.Repository{repo}, nil
	}
	if subject != quota_model.User && subject != quota_model.Organization {
		return 0, "", nil, util.ErrNotExist
	}
	user, err := user_model.GetUserByID(ctx, id)
	if err != nil {
		return 0, "", nil, err
	}
	if Subject(user) != subject {
		return 0, "", nil, util.ErrNotExist
	}
	err = db.GetEngine(ctx).Table("repository").
		Join("LEFT", "quota_usage", "repository.id=quota_usage.repo_id").
		Where("repository.owner_id=? AND (quota_usage.repo_id IS NULL OR quota_usage.updated=0)", id).
		Select("repository.*").Find(&repos)
	return id, user.Name, repos, err
}

func resource(used, limit, reserved int64) *api.StorageQuotaResource {
	if reserved == math.MaxInt64 {
		reserved = 0
	}
	threshold := limit/5*4 + (limit%5*4+4)/5
	return &api.StorageQuotaResource{Used: used, Limit: limit, Reserved: reserved, Warning: limit >= 0 && (limit == 0 || used >= threshold)}
}

func Status(ctx context.Context, subject quota_model.Subject, id int64) (*api.StorageQuota, error) {
	ownerID, name, repos, err := resolveSubject(ctx, subject, id)
	if err != nil {
		return nil, err
	}
	policy, err := quota_model.GetPolicy(ctx, subject, id)
	if err != nil {
		return nil, err
	}
	gitLimit, lfsLimit, totalLimit := policy.Limits()
	var gitBytes, lfsBytes int64
	for _, repo := range repos {
		usage, found, err := quota_model.GetUsage(ctx, repo.ID)
		if err != nil {
			return nil, err
		}
		if !found || usage.Updated == 0 {
			usage, err = Measure(ctx, repo)
			if err != nil {
				return nil, err
			}
		}
		if usage.GitBytes > math.MaxInt64-gitBytes || usage.LFSBytes > math.MaxInt64-lfsBytes {
			return nil, errors.New("storage size overflow")
		}
		gitBytes += usage.GitBytes
		lfsBytes += usage.LFSBytes
	}
	var ops []*quota_model.Operation
	if err := db.GetEngine(ctx).Where("owner_id=?", ownerID).Find(&ops); err != nil {
		return nil, err
	}
	var gitReserved, lfsReserved int64
	for _, op := range ops {
		if subject != quota_model.Repository || op.RepoID == id {
			gitReserved, lfsReserved = op.GitReserved, op.LFSReserved
		}
	}
	status := &api.StorageQuota{
		Subject: string(subject), SubjectID: id, Name: name,
		Overrides: api.StorageQuotaOptions{GitLimit: policy.GitLimit, LFSLimit: policy.LFSLimit, TotalLimit: policy.TotalLimit},
	}
	if subject == quota_model.Repository {
		status.Git, status.LFS = resource(gitBytes, gitLimit, gitReserved), resource(lfsBytes, lfsLimit, lfsReserved)
		owner, err := user_model.GetUserByID(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		accountPolicy, err := quota_model.GetPolicy(ctx, Subject(owner), ownerID)
		if err != nil {
			return nil, err
		}
		if _, _, limit := accountPolicy.Limits(); limit >= 0 {
			accountStatus, err := Status(ctx, Subject(owner), ownerID)
			if err != nil {
				return nil, err
			}
			status.Account = accountStatus.Total
		}
	} else {
		cached, err := quota_model.OwnerUsage(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		if lfsBytes > math.MaxInt64-gitBytes || cached > math.MaxInt64-gitBytes-lfsBytes {
			return nil, errors.New("storage size overflow")
		}
		status.Total = resource(cached+gitBytes+lfsBytes, totalLimit, max(gitReserved, lfsReserved))
	}
	return status, nil
}

func StatusIfLimited(ctx context.Context, subject quota_model.Subject, id int64) (*api.StorageQuota, bool, error) {
	enabled, err := Enabled(ctx)
	if err != nil || !enabled {
		return nil, false, err
	}
	if subject == quota_model.Repository {
		repo, err := repo_model.GetRepositoryByID(ctx, id)
		if err != nil {
			return nil, false, err
		}
		git, lfs, account, err := EffectiveLimits(ctx, repo)
		if err != nil {
			return nil, false, err
		}
		if git < 0 && lfs < 0 && account < 0 {
			return nil, false, nil
		}
	} else {
		policy, err := quota_model.GetPolicy(ctx, subject, id)
		if err != nil {
			return nil, false, err
		}
		if _, _, limit := policy.Limits(); limit < 0 {
			return nil, false, nil
		}
	}
	status, err := Status(ctx, subject, id)
	return status, err == nil, err
}

func UpdatePolicy(ctx context.Context, subject quota_model.Subject, id int64, options api.StorageQuotaOptions) error {
	ownerID, _, _, err := resolveSubject(ctx, subject, id)
	if err != nil {
		return err
	}
	op := &quota_model.Operation{OwnerID: ownerID, RepoID: id, Token: rand.Text()}
	if err = quota_model.Acquire(ctx, op); err != nil {
		return err
	}
	policyErr := quota_model.SetPolicy(ctx, &quota_model.Policy{
		Subject: subject, SubjectID: id,
		GitLimit: options.GitLimit, LFSLimit: options.LFSLimit, TotalLimit: options.TotalLimit,
	})
	return errors.Join(policyErr, quota_model.Release(context.WithoutCancel(ctx), op))
}
