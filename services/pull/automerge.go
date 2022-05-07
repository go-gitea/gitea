// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	pull_model "code.gitea.io/gitea/models/pull"
	user_model "code.gitea.io/gitea/models/user"
)

// RemoveScheduledAutoMerge cancels a previously scheduled pull request
func RemoveScheduledAutoMerge(ctx context.Context, doer *user_model.User, pullID int64, comment bool) error {
	return db.WithTx(func(ctx context.Context) error {
		if err := pull_model.DeleteScheduledAutoMerge(ctx, pullID); err != nil {
			return err
		}

		// if pull got merged we don't need to add "auto-merge canceled comment"
		if !comment || doer == nil {
			return nil
		}

		pr, err := models.GetPullRequestByID(ctx, pullID)
		if err != nil {
			return err
		}

		_, err = models.CreateAutoMergeComment(ctx, models.CommentTypePRUnScheduledToAutoMerge, pr, doer)
		return err
	}, ctx)
}
