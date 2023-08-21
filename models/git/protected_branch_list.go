// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"sort"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
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

func (rules ProtectedBranchRules) sort() {
	sort.Slice(rules, func(i, j int) bool {
		rules[i].loadGlob()
		rules[j].loadGlob()
		if rules[i].isPlainName != rules[j].isPlainName {
			return rules[i].isPlainName // plain name comes first, so plain name means "less"
		}
		return rules[i].CreatedUnix < rules[j].CreatedUnix
	})
}

// FindRepoProtectedBranchRules load all repository's protected rules
func FindRepoProtectedBranchRules(ctx context.Context, repoID int64) (ProtectedBranchRules, error) {
	var rules ProtectedBranchRules
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Asc("created_unix").Find(&rules)
	if err != nil {
		return nil, err
	}
	rules.sort() // to make non-glob rules have higher priority, and for same glob/non-glob rules, first created rules have higher priority
	return rules, nil
}

// FindAllMatchedBranches find all matched branches
func FindAllMatchedBranches(ctx context.Context, repoID int64, ruleName string) ([]string, error) {
	results := make([]string, 0, 10)
	for page := 1; ; page++ {
		brancheNames, err := FindBranchNames(ctx, FindBranchOptions{
			ListOptions: db.ListOptions{
				PageSize: 100,
				Page:     page,
			},
			RepoID:          repoID,
			IsDeletedBranch: util.OptionalBoolFalse,
		})
		if err != nil {
			return nil, err
		}
		rule := glob.MustCompile(ruleName)

		for _, branch := range brancheNames {
			if rule.Match(branch) {
				results = append(results, branch)
			}
		}
		if len(brancheNames) < 100 {
			break
		}
	}

	return results, nil
}

// GetFirstMatchProtectedBranchRule returns the first matched rules
func GetFirstMatchProtectedBranchRule(ctx context.Context, repoID int64, branchName string) (*ProtectedBranch, error) {
	rules, err := FindRepoProtectedBranchRules(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return rules.GetFirstMatched(branchName), nil
}

// IsBranchProtected checks if branch is protected
func IsBranchProtected(ctx context.Context, repoID int64, branchName string) (bool, error) {
	rule, err := GetFirstMatchProtectedBranchRule(ctx, repoID, branchName)
	if err != nil {
		return false, err
	}
	return rule != nil, nil
}
