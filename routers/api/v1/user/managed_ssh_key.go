// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/services/context"
)

// GetManagedSSHKey gets the managed SSH key of the authenticated user
func GetManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation GET /user/managed-ssh-key user userGetManagedSSHKey
	// ---
	// summary: Get the managed SSH key that authenticates the user's SSH migrations and mirrors
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/ManagedSSHKey"
	//   default:
	//     "$ref": "#/responses/error"

	shared.GetManagedSSHKey(ctx, ctx.Doer)
}

// RegenerateManagedSSHKey generates a new managed SSH key for the authenticated user
func RegenerateManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation POST /user/managed-ssh-key/regenerate user userRegenerateManagedSSHKey
	// ---
	// summary: Generate a new managed SSH key for the user, replacing the current one
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/ManagedSSHKey"
	//   default:
	//     "$ref": "#/responses/error"

	shared.RegenerateManagedSSHKey(ctx, ctx.Doer)
}
