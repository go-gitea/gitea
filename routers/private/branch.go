// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"code.gitea.io/gitea/models"

	macaron "gopkg.in/macaron.v1"
)

// GetProtectedBranchBy get protected branch information
func GetProtectedBranchBy(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":id")
	branchName := ctx.Params("*")
	protectBranch, err := models.GetProtectedBranchBy(repoID, branchName)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	} else if protectBranch != nil {
		ctx.JSON(200, protectBranch)
	} else {
		ctx.JSON(200, &models.ProtectedBranch{
			ID: 0,
		})
	}
}

// CanUserPush returns if user push
func CanUserPush(ctx *macaron.Context) {
	pbID := ctx.ParamsInt64(":pbid")
	userID := ctx.ParamsInt64(":userid")

	protectBranch, err := models.GetProtectedBranchByID(pbID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	} else if protectBranch != nil {
		ctx.JSON(200, map[string]interface{}{
			"can_push": protectBranch.CanUserPush(userID),
		})
	} else {
		ctx.JSON(200, map[string]interface{}{
			"can_push": false,
		})
	}
}

// HasEnoughApprovals return if PR has enough approvals
func HasEnoughApprovals(ctx *macaron.Context) {
	pbID := ctx.ParamsInt64(":pbid")
	prID := ctx.ParamsInt64(":prid")

	protectBranch, err := models.GetProtectedBranchByID(pbID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	} else if prID > 0 && protectBranch != nil {
		pr, err := models.GetPullRequestByID(prID)
		if err == nil {
			err = pr.LoadAttributes()
		}
		if err == nil {
			err = pr.LoadIssue()
		}
		if err != nil {
			ctx.JSON(500, map[string]interface{}{
				"err": err.Error(),
			})
			return
		}
		ctx.JSON(200, map[string]interface{}{
			"can_push": protectBranch.HasEnoughApprovals(pr),
		})
	} else {
		ctx.JSON(200, map[string]interface{}{
			"can_push": false,
		})
	}
}
