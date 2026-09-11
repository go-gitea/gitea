// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"testing"

	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestConvertUserTypeRejectsNonConvertibleTarget(t *testing.T) {
	ctx, _ := contexttest.MockAPIContext(t, "POST /api/v1/admin/users/remote/convert-type")
	ctx.Doer = &user_model.User{ID: 1}
	ctx.ContextUser = &user_model.User{ID: 2, Type: user_model.UserTypeRemoteUser}
	web.SetForm(ctx, &api.ConvertUserTypeOption{UserType: "bot"})

	ConvertUserType(ctx)

	assert.Equal(t, http.StatusBadRequest, ctx.Resp.WrittenStatus())
}
