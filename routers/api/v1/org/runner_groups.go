// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/services/context"
)

func ListRunnerGroups(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/runner-groups organization getOrgRunnerGroups
	// ---
	// summary: List the runner groups an organization can use, including inherited global groups
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: visible_to_repository
	//   in: query
	//   description: only return the groups that can run jobs of this repository of the organization
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunnerGroupList"
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListRunnerGroups(ctx, ctx.Org.Organization.ID)
}

func GetRunnerGroup(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/runner-groups/{runner_group_id} organization getOrgRunnerGroup
	// ---
	// summary: Get a runner group an organization can use
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: runner_group_id
	//   in: path
	//   description: id of the runner group, 0 for the default group of ungrouped runners
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunnerGroup"
	//   default:
	//     "$ref": "#/responses/error"
	shared.GetRunnerGroup(ctx, ctx.Org.Organization.ID)
}

func ListRunnerGroupRunners(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/actions/runner-groups/{runner_group_id}/runners organization getOrgRunnerGroupRunners
	// ---
	// summary: List the runners in a runner group an organization can use
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: runner_group_id
	//   in: path
	//   description: id of the runner group, 0 for the default group of ungrouped runners
	//   type: integer
	//   format: int64
	//   required: true
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
	//     "$ref": "#/responses/RunnerList"
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListRunnerGroupRunners(ctx, ctx.Org.Organization.ID)
}
