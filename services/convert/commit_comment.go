// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"strconv"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

func ToCommitComment(ctx context.Context, repo *repo_model.Repository, comment *issues_model.Comment) *api.CommitComment {
	result := &api.CommitComment{
		ID:          comment.ID,
		HTMLURL:     repo.HTMLURL(ctx) + "/commit/" + comment.CommitComment.CommitSHA + "#" + comment.HashTag(),
		URL:         repo.APIURL(ctx) + "/comments/" + strconv.FormatInt(comment.ID, 10),
		Body:        comment.Content,
		User:        ToAuthor(ctx, comment.Poster, comment.OriginalAuthor, nil),
		CommitID:    comment.CommitComment.CommitSHA,
		Attachments: ToAPIAttachments(ctx, repo, comment.Attachments),
		Created:     comment.CreatedUnix.AsTime(),
		Updated:     comment.UpdatedUnix.AsTime(),
	}
	if comment.TreePath != "" {
		result.Path = &comment.TreePath
		result.Line = new(int64(comment.UnsignedLine()))
		result.Side = util.Iif(comment.Line < 0, "old", "new")
	}
	return result
}
