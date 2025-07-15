// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/services/context"
	mirror_service "code.gitea.io/gitea/services/mirror"
)

// GetMirrorSSHKey gets the SSH public key for user mirroring
func GetMirrorSSHKey(ctx *context.APIContext) {
	// swagger:operation GET /user/mirror-ssh-key user userGetMirrorSSHKey
	// ---
	// summary: Get SSH public key for user mirroring
	// produces:
	// - application/json
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	keypair, err := mirror_service.GetOrCreateSSHKeypairForUser(ctx, ctx.Doer.ID)
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

// RegenerateMirrorSSHKey regenerates the SSH keypair for user mirroring
func RegenerateMirrorSSHKey(ctx *context.APIContext) {
	// swagger:operation POST /user/mirror-ssh-key/regenerate user userRegenerateMirrorSSHKey
	// ---
	// summary: Regenerate SSH keypair for user mirroring
	// produces:
	// - application/json
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
	//   "500":
	//     "$ref": "#/responses/internalServerError"

	keypair, err := mirror_service.RegenerateSSHKeypairForUser(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"public_key":  keypair.PublicKey,
		"fingerprint": keypair.Fingerprint,
	})
}
