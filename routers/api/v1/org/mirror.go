// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	ssh_module "gitea.dev/modules/ssh"
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
	//     description: SSH public key
	//     schema:
	//       type: object
	//       properties:
	//         public_key:
	//           type: string
	//         fingerprint:
	//           type: string
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	keypair, err := ssh_module.GetOrCreateSSHKeypairForOrg(ctx, ctx.Org.Organization.ID)
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.APIError(http.StatusNotFound, "SSH keypair not found")
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"public_key":  keypair.PublicKey,
		"fingerprint": keypair.Fingerprint,
	})
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
	//     description: New SSH public key
	//     schema:
	//       type: object
	//       properties:
	//         public_key:
	//           type: string
	//         fingerprint:
	//           type: string
	//   "403":
	//     "$ref": "#/responses/forbidden"

	keypair, err := repo_model.RegenerateUserSSHKeypair(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"public_key":  keypair.PublicKey,
		"fingerprint": keypair.Fingerprint,
	})
}
