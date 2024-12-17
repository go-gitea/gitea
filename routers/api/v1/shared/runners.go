// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"errors"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

// RegistrationToken is response related to registration token
// swagger:response RegistrationToken
type RegistrationToken struct {
	Token string `json:"token"`
}

func GetRegistrationToken(ctx *context.APIContext, ownerID, repoID int64) {
	putToken := ctx.FormString("put-token")
	var token *actions_model.ActionRunnerToken
	var err error
	if putToken == "" {
		token, err = actions_model.GetLatestRunnerToken(ctx, ownerID, repoID)
	}
	if putToken != "" || errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, ownerID, repoID, putToken)
	}
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusOK, RegistrationToken{Token: token.Token})
}
