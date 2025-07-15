// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/services/context"
	mirror_service "code.gitea.io/gitea/services/mirror"
)

// GetMirrorSSHKey gets the SSH public key for organization mirroring
func GetMirrorSSHKey(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/mirror-ssh-key organization orgGetMirrorSSHKey
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

	keypair, err := mirror_service.GetOrCreateSSHKeypairForOrg(ctx, ctx.Org.Organization.ID)
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

// RegenerateMirrorSSHKey regenerates the SSH keypair for organization mirroring
func RegenerateMirrorSSHKey(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/mirror-ssh-key/regenerate organization orgRegenerateMirrorSSHKey
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
	//   "500":
	//     "$ref": "#/responses/internalServerError"

	keypair, err := mirror_service.RegenerateSSHKeypairForOrg(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"public_key":  keypair.PublicKey,
		"fingerprint": keypair.Fingerprint,
	})
}
