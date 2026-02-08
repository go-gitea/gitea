// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/auth"
)

var _ auth.Method = &Auth{}

type Auth struct {
	basicAuth auth.Basic
}

func (a *Auth) Name() string {
	return "nuget"
}

func (a *Auth) Verify(req *http.Request, w http.ResponseWriter, store auth.DataStore, sess auth.SessionStore) (*user_model.User, error) {
	// ref: https://docs.microsoft.com/en-us/nuget/api/package-publish-resource#request-parameters
	return a.basicAuth.VerifyAuthToken(req, w, store, sess, req.Header.Get("X-NuGet-ApiKey"))
}
