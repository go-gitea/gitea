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
	// - name: state
	//   in: query
	//   description: only advisories in this state (triage, draft, published, closed, withdrawn)
	//   type: string
	// - name: q
	//   in: query
	//   description: search in the summary, description, identifier, CVE ID and package names
	//   type: string
	// - name: severity
	//   in: query
	//   description: only advisories with this severity (low, medium, high, critical)
	//   type: string
	// - name: ecosystem
	//   in: query
	//   description: only advisories affecting a package of this ecosystem
	//   type: string
	// - name: cwe
	//   in: query
	//   description: only advisories with this CWE ID
	//   type: string
	// - name: close_reason
	//   in: query
	//   description: only advisories closed for this reason (invalid, duplicate)
	//   type: string
	// - name: sort
	//   in: query
	//   description: sort by created (default), updated or published
	//   type: string
	// - name: direction
	//   in: query
	//   description: sort direction, asc or desc (default)
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
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
