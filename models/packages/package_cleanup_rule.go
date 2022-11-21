// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

var ErrPackageCleanupRuleNotExist = errors.New("Package blob does not exist")

func init() {
	db.RegisterModel(new(PackageCleanupRule))
}

// PackageCleanupRule represents a rule which describes when to clean up package versions
type PackageCleanupRule struct {
	ID                   int64              `xorm:"pk autoincr"`
	Enabled              bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	OwnerID              int64              `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	Type                 Type               `xorm:"UNIQUE(s) INDEX NOT NULL"`
	KeepCount            int                `xorm:"NOT NULL DEFAULT 0"`
	KeepPattern          string             `xorm:"NOT NULL DEFAULT ''"`
	KeepPatternMatcher   *regexp.Regexp     `xorm:"-"`
	RemoveDays           int                `xorm:"NOT NULL DEFAULT 0"`
	RemovePattern        string             `xorm:"NOT NULL DEFAULT ''"`
	RemovePatternMatcher *regexp.Regexp     `xorm:"-"`
	MatchFullName        bool               `xorm:"NOT NULL DEFAULT false"`
	CreatedUnix          timeutil.TimeStamp `xorm:"created NOT NULL DEFAULT 0"`
	UpdatedUnix          timeutil.TimeStamp `xorm:"updated NOT NULL DEFAULT 0"`
}

func (pcr *PackageCleanupRule) CompiledPattern() error {
	if pcr.KeepPatternMatcher != nil || pcr.RemovePatternMatcher != nil {
		return nil
	}

	if pcr.KeepPattern != "" {
		var err error
		pcr.KeepPatternMatcher, err = regexp.Compile(fmt.Sprintf(`(?i)\A%s\z`, pcr.KeepPattern))
		if err != nil {
			return err
		}
	}

	if pcr.RemovePattern != "" {
		var err error
		pcr.RemovePatternMatcher, err = regexp.Compile(fmt.Sprintf(`(?i)\A%s\z`, pcr.RemovePattern))
		if err != nil {
			return err
		}
	}

	return nil
}

func InsertCleanupRule(ctx context.Context, pcr *PackageCleanupRule) (*PackageCleanupRule, error) {
	return pcr, db.Insert(ctx, pcr)
}

func GetCleanupRuleByID(ctx context.Context, id int64) (*PackageCleanupRule, error) {
	pcr := &PackageCleanupRule{}

	has, err := db.GetEngine(ctx).ID(id).Get(pcr)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageCleanupRuleNotExist
	}
	return pcr, nil
}

func UpdateCleanupRule(ctx context.Context, pcr *PackageCleanupRule) error {
	_, err := db.GetEngine(ctx).ID(pcr.ID).AllCols().Update(pcr)
	return err
}

func GetCleanupRulesByOwner(ctx context.Context, ownerID int64) ([]*PackageCleanupRule, error) {
	pcrs := make([]*PackageCleanupRule, 0, 10)
	return pcrs, db.GetEngine(ctx).Where("owner_id = ?", ownerID).Find(&pcrs)
}

func DeleteCleanupRuleByID(ctx context.Context, ruleID int64) error {
	_, err := db.GetEngine(ctx).ID(ruleID).Delete(&PackageCleanupRule{})
	return err
}

func HasOwnerCleanupRuleForPackageType(ctx context.Context, ownerID int64, packageType Type) (bool, error) {
	return db.GetEngine(ctx).
		Where("owner_id = ? AND type = ?", ownerID, packageType).
		Exist(&PackageCleanupRule{})
}

func IterateEnabledCleanupRules(ctx context.Context, callback func(context.Context, *PackageCleanupRule) error) error {
	return db.Iterate(
		ctx,
		builder.Eq{"enabled": true},
		callback,
	)
}
