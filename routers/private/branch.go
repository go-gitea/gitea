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
			CanPush: true,
		})
	}
}
