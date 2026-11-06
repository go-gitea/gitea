// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"context"
	"strconv"
	"strings"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"

	"github.com/markbates/goth"
	"xorm.io/builder"
)

func toExternalLoginUser(authSourceID int64, user *user_model.User, gothUser goth.User) *user_model.ExternalLoginUser {
	return &user_model.ExternalLoginUser{
		ExternalID:        gothUser.UserID,
		UserID:            user.ID,
		LoginSourceID:     authSourceID,
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
	}
}

// LinkAccountToUser link the gothUser to the user
func LinkAccountToUser(ctx context.Context, authSourceID int64, user *user_model.User, gothUser goth.User) error {
	externalLoginUser := toExternalLoginUser(authSourceID, user, gothUser)

	if err := user_model.LinkExternalToUser(ctx, user, externalLoginUser); err != nil {
		return err
	}

	audit.RecordAs(ctx, user, audit_model.UserExternalLoginAdd, user,
		"external_id", externalLoginUser.ExternalID, "provider", externalLoginUser.Provider)

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
func EnsureLinkExternalToUser(ctx context.Context, authSourceID int64, user *user_model.User, gothUser goth.User) error {
	externalLoginUser := toExternalLoginUser(authSourceID, user, gothUser)
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

func LinkMigratedAccount(ctx context.Context, user *user_model.User, tp structs.GitServiceType, externalID int64) error {
	if !user.IsIndividual() && !user.IsTypeBot() {
		return util.ErrorWrap(util.ErrUnprocessableContent, "the user must be an individual or a bot")
	}
	externalUserID := strconv.FormatInt(externalID, 10)
	return db.WithTx(ctx, func(ctx context.Context) error {
		link, has, err := db.Get[user_model.ExternalAccountLink](ctx, builder.Eq{"service_type": tp, "external_id": externalID})
		if err != nil {
			return err
		}
		loginOfOther, err := db.Exist[user_model.ExternalLoginUser](ctx,
			builder.Eq{"provider": tp.Name(), "external_id": externalUserID}.And(builder.Neq{"user_id": user.ID}))
		if err != nil {
			return err
		}
		if loginOfOther || (has && link.UserID != user.ID) {
			return util.NewAlreadyExistErrorf("the external account is linked to another user")
		}
		if !has {
			if err := db.Insert(ctx, &user_model.ExternalAccountLink{ServiceType: tp, ExternalID: externalID, UserID: user.ID}); err != nil {
				return err
			}
			audit.Record(ctx, audit_model.UserExternalAccountLink, user, "service", tp.Name(), "external_id", externalID)
		}
		return UpdateMigrationsByType(ctx, tp, externalUserID, user.ID)
	})
}

func UnlinkMigratedAccount(ctx context.Context, user *user_model.User, tp structs.GitServiceType, externalID int64) error {
	deleted, err := db.DeleteByBean(ctx, &user_model.ExternalAccountLink{ServiceType: tp, ExternalID: externalID, UserID: user.ID})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return util.ErrNotExist
	}
	audit.Record(ctx, audit_model.UserExternalAccountUnlink, user, "service", tp.Name(), "external_id", externalID)
	return nil
}
