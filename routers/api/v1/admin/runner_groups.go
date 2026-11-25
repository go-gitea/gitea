// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/services/context"
)

func ListRunnerGroups(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runner-groups admin getAdminRunnerGroups
	// ---
	// summary: List the global runner groups
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunnerGroupList"
	//   default:
	//     "$ref": "#/responses/error"
	shared.ListRunnerGroups(ctx, 0)
}

func GetRunnerGroup(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runner-groups/{runner_group_id} admin getAdminRunnerGroup
	// ---
	// summary: Get a global runner group
	// produces:
	// - application/json
	// parameters:
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
	shared.GetRunnerGroup(ctx, 0)
}

func ListRunnerGroupRunners(ctx *context.APIContext) {
	// swagger:operation GET /admin/actions/runner-groups/{runner_group_id}/runners admin getAdminRunnerGroupRunners
	// ---
	// summary: List the runners in a global runner group
	// produces:
	// - application/json
	// parameters:
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
	shared.ListRunnerGroupRunners(ctx, 0)
}
