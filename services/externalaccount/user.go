// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"context"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"

	"github.com/markbates/goth"
)

func toExternalLoginUser(ctx context.Context, user *user_model.User, gothUser goth.User) (*user_model.ExternalLoginUser, error) {
	authSource, err := auth.GetActiveOAuth2SourceByName(ctx, gothUser.Provider)
	if err != nil {
		return nil, err
	}
	return &user_model.ExternalLoginUser{
		ExternalID:        gothUser.UserID,
		UserID:            user.ID,
		LoginSourceID:     authSource.ID,
		RawData:           gothUser.RawData,
		Provider:          gothUser.Provider,
		Email:             gothUser.Email,
		Name:              gothUser.Name,
		FirstName:         gothUser.FirstName,
		LastName:          gothUser.LastName,
		NickName:          gothUser.NickName,
		Description:       gothUser.Description,
		AvatarURL:         gothUser.AvatarURL,
		Location:          gothUser.Location,
		AccessToken:       gothUser.AccessToken,
		AccessTokenSecret: gothUser.AccessTokenSecret,
		RefreshToken:      gothUser.RefreshToken,
		ExpiresAt:         gothUser.ExpiresAt,
	}, nil
}

// LinkAccountToUser link the gothUser to the user
func LinkAccountToUser(ctx context.Context, user *user_model.User, gothUser goth.User) error {
	externalLoginUser, err := toExternalLoginUser(ctx, user, gothUser)
	if err != nil {
		return err
	}

	if err := user_model.LinkExternalToUser(ctx, user, externalLoginUser); err != nil {
		return err
	}

	externalID := externalLoginUser.ExternalID

	var tp structs.GitServiceType
	for _, s := range structs.SupportedFullGitService {
		if strings.EqualFold(s.Name(), gothUser.Provider) {
			tp = s
			break
		}
	}

	if tp.Name() != "" {
		return UpdateMigrationsByType(ctx, tp, externalID, user.ID)
	}

	return nil
}

// EnsureLinkExternalToUser link the gothUser to the user
func EnsureLinkExternalToUser(ctx context.Context, user *user_model.User, gothUser goth.User) error {
	externalLoginUser, err := toExternalLoginUser(ctx, user, gothUser)
	if err != nil {
		return err
	}

	return user_model.EnsureLinkExternalToUser(ctx, externalLoginUser)
}

// UpdateMigrationsByType updates all migrated repositories' posterid from gitServiceType to replace originalAuthorID to posterID
func UpdateMigrationsByType(ctx context.Context, tp structs.GitServiceType, externalUserID string, userID int64) error {
	// Skip update if externalUserID is not a valid numeric ID or exceeds int64
	if _, err := strconv.ParseInt(externalUserID, 10, 64); err != nil {
		return nil
	}

	if err := issues_model.UpdateIssuesMigrationsByType(ctx, tp, externalUserID, userID); err != nil {
		return err
	}

	if err := issues_model.UpdateCommentsMigrationsByType(ctx, tp, externalUserID, userID); err != nil {
		return err
	}

	if err := repo_model.UpdateReleasesMigrationsByType(ctx, tp, externalUserID, userID); err != nil {
		return err
	}

	if err := issues_model.UpdateReactionsMigrationsByType(ctx, tp, externalUserID, userID); err != nil {
		return err
	}
	return issues_model.UpdateReviewsMigrationsByType(ctx, tp, externalUserID, userID)
}
