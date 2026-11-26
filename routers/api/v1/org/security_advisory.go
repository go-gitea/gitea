// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/unit"
	"gitea.dev/routers/api/v1/repo"
	"gitea.dev/services/context"
)

func ListSecurityAdvisories(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/security-advisories organization orgListSecurityAdvisories
	// ---
	// summary: List security advisories across an organization's repositories
	// description: Only organization owners can list them.
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryAdvisoryList"
	//   default:
	//     "$ref": "#/responses/error"
	if unit.TypeSecurityAdvisories.UnitGlobalDisabled() {
		ctx.APIErrorNotFound()
		return
	}
	repo.ListAdvisories(ctx, nil, ctx.Org.Organization.ID, advisory_model.Viewer{Doer: ctx.Doer, IsRepoAdmin: true})
}
