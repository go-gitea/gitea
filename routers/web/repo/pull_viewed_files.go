// Copyright 2022 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

const headCommitKey = "_headCommitID"

func UpdateViewedFiles(ctx *context.Context) {
	pull := checkPullInfo(ctx).PullRequest
	if ctx.Written() {
		return
	}
	updatedFiles := make(map[string]bool, len(ctx.Req.Form))
	headCommitID := ""

	// Collect all files and their viewed state
	for file, values := range ctx.Req.Form {
		for _, viewedString := range values {
			viewed, err := strconv.ParseBool(viewedString)

			// Ignore fields that do not parse as a boolean, i.e. the CSRF token
			if err != nil {

				// Prevent invalid reviews by specifically supplying the commit the user viewed the file under
				if file == headCommitKey {
					headCommitID = viewedString
				}
				continue
			}
			updatedFiles[file] = viewed
		}
	}

	// No head commit ID was supplied - expect the review to have been now
	if headCommitID == "" {
		headCommitID = pull.HeadCommitID
	}

	if err := models.UpdateReview(ctx.User.ID, pull.ID, headCommitID, updatedFiles); err != nil {
		ctx.ServerError("UpdateReview", err)
	}
}
