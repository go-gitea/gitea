// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	access_model "gitea.dev/models/perm/access"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
)

func checkComment(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, content string) error {
	if strings.TrimSpace(content) == "" {
		return util.NewInvalidArgumentErrorf("comment must not be empty")
	}
	if err := advisory.LoadRepo(ctx); err != nil {
		return err
	}
	if isBlockedFromAdvisory(ctx, advisory, doer) && !access_model.IsUserRepoAdmin(ctx, advisory.Repo, doer) {
		return user_model.ErrBlockedUser
	}
	return nil
}

func CreateComment(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, perms advisory_model.Permissions, content string, isConfidential bool) (*advisory_model.Comment, error) {
	if !perms.CanComment {
		return nil, util.NewPermissionDeniedErrorf("you can only follow this advisory")
	}
	if isConfidential && !perms.CanSeeConfidential {
		return nil, util.NewPermissionDeniedErrorf("only the maintainers can write confidential comments")
	}
	if err := checkComment(ctx, doer, advisory, content); err != nil {
		return nil, err
	}
	comment := &advisory_model.Comment{AdvisoryID: advisory.ID, PosterID: doer.ID, Poster: doer, Content: content, IsConfidential: isConfidential}
	if err := db.Insert(ctx, comment); err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisoryComment(ctx, doer, advisory, comment)
	return comment, nil
}

func EditComment(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, comment *advisory_model.Comment, content string) error {
	if comment.PosterID != doer.ID {
		return util.NewPermissionDeniedErrorf("only the poster can edit a comment")
	}
	if err := checkComment(ctx, doer, advisory, content); err != nil {
		return err
	}
	comment.Content = content
	return advisory_model.UpdateCommentContent(ctx, comment)
}

func DeleteComment(ctx context.Context, doer *user_model.User, perms advisory_model.Permissions, comment *advisory_model.Comment) error {
	if comment.PosterID != doer.ID && !perms.CanManage {
		return util.NewPermissionDeniedErrorf("only the poster and repository admins can delete a comment")
	}
	_, err := db.DeleteByID[advisory_model.Comment](ctx, comment.ID)
	return err
}
