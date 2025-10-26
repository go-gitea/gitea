// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/gitrepo"
)

func CreateOrUpdateProtectedBranch(ctx context.Context, repo *repo_model.Repository,
	protectBranch *git_model.ProtectedBranch, whitelistOptions git_model.WhitelistOptions,
) error {
	var err error
	if repo != nil {
		err = git_model.UpdateProtectBranch(ctx, repo, protectBranch, whitelistOptions)
	} else {
		org, err := organization.GetOrgByID(ctx, protectBranch.OwnerID)
		if err == nil {
			err = git_model.UpdateOrgProtectBranch(ctx, org, protectBranch, whitelistOptions)
		}
	}
	if err != nil {
		return err
	}

	if repo != nil {
		isPlainRule := !git_model.IsRuleNameSpecial(protectBranch.RuleName)
		var isBranchExist bool
		if isPlainRule {
			// TODO: read the database directly to check if the branch exists
			isBranchExist = gitrepo.IsBranchExist(ctx, repo, protectBranch.RuleName)
		}

		if isBranchExist {
			if err := CheckPRsForBaseBranch(ctx, repo, protectBranch.RuleName); err != nil {
				return err
			}
		} else {
			if !isPlainRule {
				// FIXME: since we only need to recheck files protected rules, we could improve this
				matchedBranches, err := git_model.FindAllMatchedBranches(ctx, repo.ID, protectBranch.RuleName)
				if err != nil {
					return err
				}
				for _, branchName := range matchedBranches {
					if err = CheckPRsForBaseBranch(ctx, repo, branchName); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
