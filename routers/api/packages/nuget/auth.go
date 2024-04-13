// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"net/http"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/auth"
)

var _ auth.Method = &Auth{}

type Auth struct{}

func (a *Auth) Name() string {
	return "nuget"
}

// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#request-parameters
func (a *Auth) Verify(req *http.Request, w http.ResponseWriter, store auth.DataStore, sess auth.SessionStore) (*user_model.User, error) {
	token, err := auth_model.GetAccessTokenBySHA(req.Context(), req.Header.Get("X-NuGet-ApiKey"))
	if err != nil {
		if !(auth_model.IsErrAccessTokenNotExist(err) || auth_model.IsErrAccessTokenEmpty(err)) {
			log.Error("GetAccessTokenBySHA: %v", err)
			return nil, err
		}
		return nil, nil
	}

	u, err := user_model.GetUserByID(req.Context(), token.UID)
	if err != nil {
		log.Error("GetUserByID:  %v", err)
		return nil, err
	}

	token.UpdatedUnix = timeutil.TimeStampNow()
	if err := auth_model.UpdateAccessToken(req.Context(), token); err != nil {
		log.Error("UpdateAccessToken:  %v", err)
	}

	return u, nil
}
