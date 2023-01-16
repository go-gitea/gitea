// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"sort"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"

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
		if rules[i].isPlainName {
			if !rules[j].isPlainName {
				return true
			}
		} else if rules[j].isPlainName {
			return true
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
	rules.sort()
	return rules, nil
}

// FindAllMatchedBranches find all matched branches
func FindAllMatchedBranches(ctx context.Context, gitRepo *git.Repository, ruleName string) ([]string, error) {
	// FIXME: how many should we get?
	branches, _, err := gitRepo.GetBranchNames(0, 9999999)
	if err != nil {
		return nil, err
	}
	rule := glob.MustCompile(ruleName)
	results := make([]string, 0, len(branches))
	for _, branch := range branches {
		if rule.Match(branch) {
			results = append(results, branch)
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
