// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	access_model "gitea.dev/models/perm/access"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
)

func checkComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, content string) error {
	if strings.TrimSpace(content) == "" {
		return util.NewInvalidArgumentErrorf("comment must not be empty")
	}
	if err := a.LoadRepo(ctx); err != nil {
		return err
	}
	if isBlockedFromAdvisory(ctx, a, doer) && !access_model.IsUserRepoAdmin(ctx, a.Repo, doer) {
		return user_model.ErrBlockedUser
	}
	return nil
}

// CreateComment adds a comment to the private discussion, the caller must check Permissions.CanSeeDiscussion
func CreateComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, perms advisory_model.Permissions, content string, isInternal bool) (*advisory_model.Comment, error) {
	if isInternal && !perms.CanSeeInternal {
		return nil, util.NewPermissionDeniedErrorf("only the maintainers can write internal comments")
	}
	if err := checkComment(ctx, doer, a, content); err != nil {
		return nil, err
	}
	c := &advisory_model.Comment{AdvisoryID: a.ID, PosterID: doer.ID, Poster: doer, Content: content, IsInternal: isInternal}
	if err := advisory_model.CreateComment(ctx, c); err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisoryComment(ctx, doer, a, c)
	return c, nil
}

func EditComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, c *advisory_model.Comment, content string) error {
	if c.PosterID != doer.ID {
		return util.NewPermissionDeniedErrorf("only the poster can edit a comment")
	}
	if err := checkComment(ctx, doer, a, content); err != nil {
		return err
	}
	c.Content = content
	return advisory_model.UpdateCommentContent(ctx, c)
}

func DeleteComment(ctx context.Context, doer *user_model.User, perms advisory_model.Permissions, c *advisory_model.Comment) error {
	if c.PosterID != doer.ID && !perms.CanManage {
		return util.NewPermissionDeniedErrorf("only the poster and repository admins can delete a comment")
	}
	return advisory_model.DeleteComment(ctx, c)
}
