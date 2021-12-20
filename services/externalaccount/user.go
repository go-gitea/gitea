// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package externalaccount

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/login"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"

	"github.com/markbates/goth"
)

func toExternalLoginUser(user *user_model.User, gothUser goth.User) (*user_model.ExternalLoginUser, error) {
	loginSource, err := login.GetActiveOAuth2LoginSourceByName(gothUser.Provider)
	if err != nil {
		return nil, err
	}
	return &user_model.ExternalLoginUser{
		ExternalID:        gothUser.UserID,
		UserID:            user.ID,
		LoginSourceID:     loginSource.ID,
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
func LinkAccountToUser(user *user_model.User, gothUser goth.User) error {
	externalLoginUser, err := toExternalLoginUser(user, gothUser)
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
		return models.UpdateMigrationsByType(tp, externalID, user.ID)
	}

	return nil
}

// UpdateExternalUser updates external user's information
func UpdateExternalUser(user *user_model.User, gothUser goth.User) error {
	externalLoginUser, err := toExternalLoginUser(user, gothUser)
	if err != nil {
		return err
	}

	return user_model.UpdateExternalUserByExternalID(externalLoginUser)
}
