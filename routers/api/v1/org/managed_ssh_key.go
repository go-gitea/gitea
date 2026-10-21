// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"

	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

// GetManagedSSHKey gets the SSH public key for organization mirroring
func GetManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/managed-ssh-key organization orgGetManagedSSHKey
	// ---
	// summary: Get SSH public key for organization mirroring
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// a GET must not create anything, so look the keypair up read-only
	keypair, err := user_model.GetSSHKeypairByOwner(ctx, ctx.Org.Organization.ID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, "SSH keypair not found")
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, &api.ManagedSSHKey{PublicKey: keypair.PublicKey, Fingerprint: keypair.Fingerprint})
}

// RegenerateManagedSSHKey regenerates the SSH keypair for organization mirroring
func RegenerateManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/managed-ssh-key/regenerate organization orgRegenerateManagedSSHKey
	// ---
	// summary: Regenerate SSH keypair for organization mirroring
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
	//   "403":
	//     "$ref": "#/responses/forbidden"

	keypair, err := user_model.CreateSSHKeypair(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, &api.ManagedSSHKey{PublicKey: keypair.PublicKey, Fingerprint: keypair.Fingerprint})
}
