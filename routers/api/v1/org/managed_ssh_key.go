// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/services/context"
)

// GetManagedSSHKey gets the managed SSH key of an organization
func GetManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/managed-ssh-key organization orgGetManagedSSHKey
	// ---
	// summary: Get the managed SSH key that authenticates the organization's SSH migrations and mirrors
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
	//     "$ref": "#/responses/ManagedSSHKey"
	//   default:
	//     "$ref": "#/responses/error"

	shared.GetManagedSSHKey(ctx, ctx.Org.Organization.AsUser())
}

// RegenerateManagedSSHKey generates a new managed SSH key for an organization
func RegenerateManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/managed-ssh-key/regenerate organization orgRegenerateManagedSSHKey
	// ---
	// summary: Generate a new managed SSH key for the organization, replacing the current one
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
	//     "$ref": "#/responses/ManagedSSHKey"
	//   default:
	//     "$ref": "#/responses/error"

	shared.RegenerateManagedSSHKey(ctx, ctx.Org.Organization.AsUser())
}
