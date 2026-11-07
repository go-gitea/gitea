// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"net/http"

	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
)

func GetManagedSSHKey(ctx *context.APIContext, owner *user_model.User) {
	keypair, err := user_model.GetSSHKeypairByOwner(ctx, owner.ID)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, &api.ManagedSSHKey{PublicKey: keypair.PublicKey, Fingerprint: keypair.Fingerprint})
}

func RegenerateManagedSSHKey(ctx *context.APIContext, owner *user_model.User) {
	keypair, err := user_model.CreateSSHKeypair(ctx, owner.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	audit.RecordScoped(ctx, owner, nil, audit.ManagedSSHKeyGenerate, "fingerprint", keypair.Fingerprint)
	ctx.JSON(http.StatusOK, &api.ManagedSSHKey{PublicKey: keypair.PublicKey, Fingerprint: keypair.Fingerprint})
}
