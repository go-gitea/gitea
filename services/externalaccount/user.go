// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"

	"github.com/markbates/goth"
)

func toExternalLoginUser(user *user_model.User, gothUser goth.User, authType auth.Type) (*user_model.ExternalLoginUser, error) {
	authSource, err := auth.GetActiveAuthSourceByName(gothUser.Provider, authType)
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
func LinkAccountToUser(ctx context.Context, user *user_model.User, gothUser goth.User, authType auth.Type) error {
	externalLoginUser, err := toExternalLoginUser(user, gothUser, authType)
	if err != nil {
		return err
	}

	if err := user_model.LinkExternalToUser(user, externalLoginUser); err != nil {
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

// UpdateExternalUser updates external user's information
func UpdateExternalUser(user *user_model.User, gothUser goth.User, authType auth.Type) error {
	externalLoginUser, err := toExternalLoginUser(user, gothUser, authType)
	if err != nil {
		return err
	}

	return user_model.UpdateExternalUserByExternalID(externalLoginUser)
}

// UpdateMigrationsByType updates all migrated repositories' posterid from gitServiceType to replace originalAuthorID to posterID
func UpdateMigrationsByType(ctx context.Context, tp structs.GitServiceType, externalUserID string, userID int64) error {
	if err := issues_model.UpdateIssuesMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := issues_model.UpdateCommentsMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := repo_model.UpdateReleasesMigrationsByType(ctx, tp, externalUserID, userID); err != nil {
		return err
	}

	if err := issues_model.UpdateReactionsMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}
	return issues_model.UpdateReviewsMigrationsByType(tp, externalUserID, userID)
}
