// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/context"

	"github.com/markbates/goth"
)

func oauth2SignInSync(ctx *context.Context, authSourceID int64, u *user_model.User, gothUser goth.User) {
	oauth2UpdateAvatarIfNeed(ctx, gothUser.AvatarURL, u)

	authSource, err := auth.GetSourceByID(ctx, authSourceID)
	if err != nil {
		ctx.ServerError("GetSourceByID", err)
		return
	}
	oauth2Source, _ := authSource.Cfg.(*oauth2.Source)
	if !authSource.IsOAuth2() || oauth2Source == nil {
		ctx.ServerError("oauth2SignInSync", fmt.Errorf("source %s is not an OAuth2 source", gothUser.Provider))
		return
	}

	// sync full name
	fullNameKey := util.IfZero(oauth2Source.FullNameClaimName, "name")
	fullName, _ := gothUser.RawData[fullNameKey].(string)
	fullName = util.IfZero(fullName, gothUser.Name)

	// need to update if the user has no full name set
	shouldUpdateFullName := u.FullName == ""
	// force to update if the attribute is set
	shouldUpdateFullName = shouldUpdateFullName || oauth2Source.FullNameClaimName != ""
	// only update if the full name is different
	shouldUpdateFullName = shouldUpdateFullName && u.FullName != fullName
	if shouldUpdateFullName {
		u.FullName = fullName
		if err := user_model.UpdateUserCols(ctx, u, "full_name"); err != nil {
			log.Error("Unable to sync OAuth2 user full name %s: %v", gothUser.Provider, err)
		}
	}

	err = oauth2UpdateSSHPubIfNeed(ctx, authSource, &gothUser, u)
	if err != nil {
		log.Error("Unable to sync OAuth2 SSH public key %s: %v", gothUser.Provider, err)
	}
}

func oauth2SyncGetSSHKeys(source *oauth2.Source, gothUser *goth.User) ([]string, error) {
	value, exists := gothUser.RawData[source.SSHPublicKeyClaimName]
	if !exists {
		return []string{}, nil
	}
	rawSlice, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid SSH public key value type: %T", value)
	}

	sshKeys := make([]string, 0, len(rawSlice))
	for _, v := range rawSlice {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid SSH public key value item type: %T", v)
		}
		sshKeys = append(sshKeys, str)
	}
	return sshKeys, nil
}

func oauth2UpdateSSHPubIfNeed(ctx *context.Context, authSource *auth.Source, gothUser *goth.User, user *user_model.User) error {
	oauth2Source, _ := authSource.Cfg.(*oauth2.Source)
	if oauth2Source == nil || oauth2Source.SSHPublicKeyClaimName == "" {
		return nil
	}
	sshKeys, err := oauth2SyncGetSSHKeys(oauth2Source, gothUser)
	if err != nil {
		return err
	}
	if !asymkey_model.SynchronizePublicKeys(ctx, user, authSource, sshKeys) {
		return nil
	}
	return asymkey_service.RewriteAllPublicKeys(ctx)
}
