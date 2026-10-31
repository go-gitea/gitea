// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"errors"
	"fmt"
	"math"

	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type Subject string

const (
	Repository   Subject = "repository"
	User         Subject = "user"
	Organization Subject = "organization"
)

type Policy struct {
	ID         int64   `xorm:"pk autoincr" json:"-"`
	Subject    Subject `xorm:"VARCHAR(20) UNIQUE(subject) NOT NULL" json:"subject"`
	SubjectID  int64   `xorm:"UNIQUE(subject) NOT NULL" json:"subject_id"`
	GitLimit   *int64  `json:"git_limit"`
	LFSLimit   *int64  `json:"lfs_limit"`
	TotalLimit *int64  `json:"total_limit"`
}

type Usage struct {
	RepoID      int64              `xorm:"pk" json:"repo_id"`
	OwnerID     int64              `xorm:"INDEX NOT NULL" json:"owner_id"`
	GitBytes    int64              `xorm:"NOT NULL" json:"git_bytes"`
	LFSBytes    int64              `xorm:"NOT NULL" json:"lfs_bytes"`
	Updated     timeutil.TimeStamp `xorm:"updated" json:"updated"`
	GCRequested timeutil.TimeStamp `json:"-"`
}

type Operation struct {
	OwnerID       int64              `xorm:"pk" json:"owner_id"`
	RepoID        int64              `xorm:"INDEX NOT NULL" json:"repo_id"`
	Token         string             `xorm:"VARCHAR(64) UNIQUE NOT NULL" json:"-"`
	GitReserved   int64              `xorm:"NOT NULL" json:"git_reserved"`
	LFSReserved   int64              `xorm:"NOT NULL" json:"lfs_reserved"`
	Created       timeutil.TimeStamp `xorm:"created" json:"created"`
	NeedsRecovery bool               `xorm:"NOT NULL DEFAULT false" json:"needs_recovery"`
	Children      int64              `xorm:"NOT NULL DEFAULT 0" json:"-"`
	Closing       bool               `xorm:"NOT NULL DEFAULT false" json:"-"`
}

func (*Policy) TableName() string    { return "quota_policy" }
func (*Usage) TableName() string     { return "quota_usage" }
func (*Operation) TableName() string { return "quota_operation" }

func init() {
	db.RegisterModel(new(Policy))
	db.RegisterModel(new(Usage))
	db.RegisterModel(new(Operation))
}

var (
	ErrBusy      = util.NewAlreadyExistErrorf("storage quota capacity is reserved by another operation; retry later")
	ErrUncertain = errors.New("storage quota operation requires reconciliation")
)

type ExceededError struct {
	Resource string
	Used     int64
	Limit    int64
}

func (e ExceededError) Error() string {
	return fmt.Sprintf("storage quota exceeded for %s: %d bytes used, limit %d bytes", e.Resource, e.Used, e.Limit)
}

func (e ExceededError) Unwrap() error { return util.ErrUnprocessableContent }

func GetPolicy(ctx context.Context, subject Subject, id int64) (*Policy, error) {
	p, _, err := db.Get[Policy](ctx, builder.Eq{"subject": subject, "subject_id": id})
	if p == nil {
		p = &Policy{Subject: subject, SubjectID: id}
	}
	return p, err
}

func SetPolicy(ctx context.Context, p *Policy) error {
	if p.SubjectID <= 0 || (p.Subject != Repository && p.Subject != User && p.Subject != Organization) {
		return util.NewInvalidArgumentErrorf("invalid quota subject")
	}
	for _, limit := range []*int64{p.GitLimit, p.LFSLimit, p.TotalLimit} {
		if limit != nil && *limit < -1 {
			return util.NewInvalidArgumentErrorf("quota limits must be -1 or nonnegative")
		}
	}
	if p.Subject == Repository && p.TotalLimit != nil || p.Subject != Repository && (p.GitLimit != nil || p.LFSLimit != nil) {
		return util.NewInvalidArgumentErrorf("limit does not apply to this quota subject")
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("subject=? AND subject_id=?", p.Subject, p.SubjectID).Delete(new(Policy)); err != nil {
			return err
		}
		p.ID = 0
		if p.GitLimit == nil && p.LFSLimit == nil && p.TotalLimit == nil {
			return nil
		}
		return db.Insert(ctx, p)
	})
}

func Limit(override *int64, fallback int64) int64 {
	if override != nil {
		return *override
	}
	return fallback
}

func (p *Policy) Limits() (git, lfs, total int64) {
	if p.Subject == Repository {
		return Limit(p.GitLimit, setting.Quota.DefaultRepoGitLimit), Limit(p.LFSLimit, setting.Quota.DefaultRepoLFSLimit), -1
	}
	fallback := setting.Quota.DefaultUserLimit
	if p.Subject == Organization {
		fallback = setting.Quota.DefaultOrgLimit
	}
	return -1, -1, Limit(p.TotalLimit, fallback)
}

func Remaining(limit, used int64) int64 {
	if limit < 0 {
		return math.MaxInt64
	}
	return max(0, limit-used)
}

func Check(resource string, limit, used, growth int64) error {
	if used < 0 || growth < 0 {
		return errors.New("invalid quota accounting")
	}
	if growth > Remaining(limit, used) {
		return ExceededError{Resource: resource, Used: used, Limit: limit}
	}
	return nil
}

func Acquire(ctx context.Context, op *Operation) error {
	if op.OwnerID <= 0 || op.RepoID <= 0 || op.Token == "" {
		return errors.New("invalid quota operation")
	}

	exists, err := db.GetEngine(ctx).ID(op.OwnerID).Exist(new(Operation))
	if err != nil {
		return err
	}
	if exists {
		return ErrBusy
	}
	if err := db.Insert(ctx, op); err != nil {
		exists, readErr := db.GetEngine(ctx).ID(op.OwnerID).Exist(new(Operation))
		if readErr == nil && exists {
			return ErrBusy
		}
		return err
	}
	return nil
}

func GetOperation(ctx context.Context, ownerID int64, token string) (*Operation, error) {
	op, has, err := db.Get[Operation](ctx, builder.Eq{"owner_id": ownerID, "token": token})
	if err != nil {
		return nil, err
	}
	if !has || op.NeedsRecovery || op.Closing {
		return nil, ErrUncertain
	}
	return op, nil
}

func MarkRecovery(ctx context.Context, op *Operation) error {
	_, err := db.GetEngine(ctx).Where("owner_id=? AND token=?", op.OwnerID, op.Token).
		Cols("needs_recovery").Update(&Operation{NeedsRecovery: true})
	return err
}

func Release(ctx context.Context, op *Operation) error {
	n, err := db.GetEngine(ctx).Where("owner_id=? AND token=? AND children=0", op.OwnerID, op.Token).Delete(new(Operation))
	if err == nil && n != 1 {
		return ErrUncertain
	}
	return err
}

func AttachChild(ctx context.Context, op *Operation) error {
	n, err := db.GetEngine(ctx).Where("owner_id=? AND token=? AND closing=? AND needs_recovery=?", op.OwnerID, op.Token, false, false).
		Incr("children").Update(new(Operation))
	if err == nil && n != 1 {
		return ErrUncertain
	}
	return err
}

func DetachChild(ctx context.Context, op *Operation) error {
	n, err := db.GetEngine(ctx).Where("owner_id=? AND token=? AND children>0", op.OwnerID, op.Token).
		Decr("children").Update(new(Operation))
	if err == nil && n != 1 {
		return ErrUncertain
	}
	return err
}

func BeginClose(ctx context.Context, op *Operation) (bool, error) {
	n, err := db.GetEngine(ctx).Where("owner_id=? AND token=? AND children=0 AND closing=? AND needs_recovery=?", op.OwnerID, op.Token, false, false).
		Cols("closing").Update(&Operation{Closing: true})
	if err != nil {
		return false, err
	}
	if n == 1 {
		return true, nil
	}
	_, err = GetOperation(ctx, op.OwnerID, op.Token)
	return false, err
}

// ResetChildrenOffline requires all writers to have stopped.
func ResetChildrenOffline(ctx context.Context, op *Operation) error {
	_, err := db.GetEngine(ctx).Where("owner_id=? AND token=?", op.OwnerID, op.Token).Cols("children").Update(&Operation{Children: 0})
	return err
}

func Reserve(ctx context.Context, op *Operation) error {
	n, err := db.GetEngine(ctx).Where("owner_id=? AND token=?", op.OwnerID, op.Token).
		Cols("git_reserved", "lfs_reserved").Update(op)
	if err == nil && n != 1 {
		return ErrUncertain
	}
	return err
}

func StoreUsage(ctx context.Context, usage *Usage) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		previous, found, err := GetUsage(ctx, usage.RepoID)
		if err != nil {
			return err
		}
		if found {
			usage.GCRequested = previous.GCRequested
		}
		if _, err := db.GetEngine(ctx).ID(usage.RepoID).Delete(new(Usage)); err != nil {
			return err
		}
		return db.Insert(ctx, usage)
	})
}

func RequestGC(ctx context.Context, repoID int64) (bool, error) {
	now := timeutil.TimeStampNow()
	n, err := db.GetEngine(ctx).Where("repo_id=? AND gc_requested<=?", repoID, now-3600).
		Cols("gc_requested").Update(&Usage{GCRequested: now})
	return n == 1, err
}

func GetUsage(ctx context.Context, repoID int64) (*Usage, bool, error) {
	return db.Get[Usage](ctx, builder.Eq{"repo_id": repoID})
}

func OwnerUsage(ctx context.Context, ownerID int64) (int64, error) {
	var result struct{ Total int64 }
	_, err := db.GetEngine(ctx).Table("quota_usage").
		Join("INNER", "repository", "repository.id=quota_usage.repo_id").
		Where("repository.owner_id=? AND quota_usage.updated<>0", ownerID).
		Select("COALESCE(SUM(quota_usage.git_bytes + quota_usage.lfs_bytes), 0) AS total").Get(&result)
	return result.Total, err
}

func InvalidateUsage(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).ID(repoID).NoAutoTime().Cols("updated").Update(&Usage{Updated: 0})
	return err
}
