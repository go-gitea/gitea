// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"net/http"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/auth"
)

type Auth struct{}

func (a *Auth) Name() string {
	return "nuget"
}

// https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#request-parameters
func (a *Auth) Verify(req *http.Request, w http.ResponseWriter, store auth.DataStore, sess auth.SessionStore) *user_model.User {
	token, err := models.GetAccessTokenBySHA(req.Header.Get("X-NuGet-ApiKey"))
	if err != nil {
		if !(models.IsErrAccessTokenNotExist(err) || models.IsErrAccessTokenEmpty(err)) {
			log.Error("GetAccessTokenBySHA: %v", err)
		}
		return nil
	}

	u, err := user_model.GetUserByID(token.UID)
	if err != nil {
		log.Error("GetUserByID:  %v", err)
		return nil
	}

	token.UpdatedUnix = timeutil.TimeStampNow()
	if err := models.UpdateAccessToken(token); err != nil {
		log.Error("UpdateAccessToken:  %v", err)
	}

	return u
}
