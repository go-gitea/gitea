// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	api "github.com/gogits/go-gogs-client"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/routers/api/v1/convert"
	"github.com/go-gitea/gitea/routers/api/v1/user"
)

// https://github.com/gogits/go-gogs-client/wiki/Administration-Organizations#create-a-new-organization
func CreateOrg(ctx *context.APIContext, form api.CreateOrgOption) {
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	org := &models.User{
		Name:        form.UserName,
		FullName:    form.FullName,
		Description: form.Description,
		Website:     form.Website,
		Location:    form.Location,
		IsActive:    true,
		Type:        models.USER_TYPE_ORGANIZATION,
	}
	if err := models.CreateOrganization(org, u); err != nil {
		if models.IsErrUserAlreadyExist(err) ||
			models.IsErrNameReserved(err) ||
			models.IsErrNamePatternNotAllowed(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "CreateOrganization", err)
		}
		return
	}

	ctx.JSON(201, convert.ToOrganization(org))
}
