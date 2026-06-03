// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	ssh_module "gitea.dev/modules/ssh"
	"gitea.dev/services/context"
)

// GetManagedSSHKey gets the SSH public key for user mirroring
func GetManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation GET /user/managed-ssh-key user userGetManagedSSHKey
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

	keypair, err := ssh_module.GetOrCreateSSHKeypairForUser(ctx, ctx.Doer.ID)
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

// RegenerateManagedSSHKey regenerates the SSH keypair for user mirroring
func RegenerateManagedSSHKey(ctx *context.APIContext) {
	// swagger:operation POST /user/managed-ssh-key/regenerate user userRegenerateManagedSSHKey
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

	keypair, err := repo_model.RegenerateUserSSHKeypair(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"public_key":  keypair.PublicKey,
		"fingerprint": keypair.Fingerprint,
	})
}
