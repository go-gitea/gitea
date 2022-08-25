// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"

	"code.gitea.io/gitea/models/organization"
)

// CheckContextUser checks context user
func CheckContextUser(ctx *context.Context, uid int64) *user_model.User {
	orgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return nil
	}

	if !ctx.Doer.IsAdmin {
		orgsAvailable := []*organization.Organization{}
		for i := 0; i < len(orgs); i++ {
			if orgs[i].CanCreateRepo() {
				orgsAvailable = append(orgsAvailable, orgs[i])
			}
		}
		ctx.Data["Orgs"] = orgsAvailable
	} else {
		ctx.Data["Orgs"] = orgs
	}

	// Not equal means current user is an organization.
	if uid == ctx.Doer.ID || uid == 0 {
		return ctx.Doer
	}

	org, err := user_model.GetUserByID(uid)
	if user_model.IsErrUserNotExist(err) {
		return ctx.Doer
	}

	if err != nil {
		ctx.ServerError("GetUserByID", fmt.Errorf("[%d]: %v", uid, err))
		return nil
	}

	// Check ownership of organization.
	if !org.IsOrganization() {
		ctx.Error(http.StatusForbidden)
		return nil
	}
	if !ctx.Doer.IsAdmin {
		canCreate, err := organization.OrgFromUser(org).CanCreateOrgRepo(ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("CanCreateOrgRepo", err)
			return nil
		} else if !canCreate {
			ctx.Error(http.StatusForbidden)
			return nil
		}
	} else {
		ctx.Data["Orgs"] = orgs
	}
	return org
}
