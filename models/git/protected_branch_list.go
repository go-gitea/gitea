// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

type ProtectedBranchRules []*ProtectedBranch

func (rules ProtectedBranchRules) GetFirstMatched(branchName string) *ProtectedBranch {
	for _, rule := range rules {
		if rule.Match(branchName) {
			return rule
		}
	}
	return nil
}

// FindRepoProtectedBranchRules load all repository's protected rules
func FindRepoProtectedBranchRules(ctx context.Context, repoID int64) (ProtectedBranchRules, error) {
	var rules []*ProtectedBranch
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Asc("created_unix").Find(&rules)
	return rules, err
}

// GetFirstMatchProtectedBranchRule returns the first matched rules
func GetFirstMatchProtectedBranchRule(ctx context.Context, repoID int64, ruleName string) (*ProtectedBranch, error) {
	rules, err := FindRepoProtectedBranchRules(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return rules.GetFirstMatched(ruleName), nil
}
