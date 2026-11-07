// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	actions_model "gitea.dev/models/actions"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/perm"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	context_service "gitea.dev/services/context"
)

// AssignActionsAccess checks task identity on every OCI request, including registry Bearer tokens.
func AssignActionsAccess(ctx *context_service.Context) {
	taskID, ok := user_model.GetActionsUserTaskID(ctx.Doer)
	if !ok {
		return
	}
	mode, repoID, err := actions_model.GetActionsPackageAccessMode(ctx, taskID, ctx.Package.Owner.ID)
	if err != nil {
		apiError(ctx, http.StatusForbidden, err)
		return
	}
	ctx.Package.AccessMode = mode
	packages_model.SetActionsPackageAccess(ctx, ctx.Package.Owner.ID, repoID, mode)
	if mode < perm.AccessModeRead {
		APIUnauthorizedError(ctx)
		return
	}
	pkg, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeContainer, ctx.PathParam("image"))
	if err != nil && !errors.Is(err, packages_model.ErrPackageNotExist) {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if pkg != nil && pkg.RepoID != 0 && pkg.RepoID != repoID {
		APIUnauthorizedError(ctx)
		return
	}
	if upload := ctx.PathParam("uuid"); upload != "" {
		unsigned, signature, found := strings.Cut(upload, ".")
		if !found || !hmac.Equal([]byte(signature), []byte(signTaskUpload(taskID, ctx.Package.Owner.ID, ctx.PathParam("image"), unsigned))) {
			APIUnauthorizedError(ctx)
			return
		}
		ctx.SetPathParam("uuid", unsigned)
	}
}

func signTaskUpload(taskID, ownerID int64, image, uuid string) string {
	mac := hmac.New(sha256.New, setting.GetGeneralTokenSigningSecret())
	_, _ = fmt.Fprintf(mac, "actions-oci-upload:%d:%d:%s:%s", taskID, ownerID, image, uuid)
	return hex.EncodeToString(mac.Sum(nil))
}

func uploadLocationID(ctx *context_service.Context, uuid string) string {
	if taskID, ok := user_model.GetActionsUserTaskID(ctx.Doer); ok {
		return uuid + "." + signTaskUpload(taskID, ctx.Package.Owner.ID, ctx.PathParam("image"), uuid)
	}
	return uuid
}

func linkTaskPackage(ctx context.Context, pkg *packages_model.Package, doer *user_model.User) error {
	taskID, ok := user_model.GetActionsUserTaskID(doer)
	if !ok {
		return nil
	}
	mode, repoID, err := actions_model.GetActionsPackageAccessMode(ctx, taskID, pkg.OwnerID)
	if err != nil {
		return err
	}
	if mode < perm.AccessModeWrite || (pkg.RepoID != 0 && pkg.RepoID != repoID) {
		return util.ErrPermissionDenied
	}
	return packages_model.SetRepositoryLink(ctx, pkg.ID, repoID)
}
