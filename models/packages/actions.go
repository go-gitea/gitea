// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/util"
)

type actionsPackageAccessKey struct{}

type actionsPackageAccess struct {
	ownerID int64
	repoID  int64
	mode    perm.AccessMode
}

func SetActionsPackageAccess(ctx reqctx.RequestContext, ownerID, repoID int64, mode perm.AccessMode) {
	ctx.SetContextValue(actionsPackageAccessKey{}, &actionsPackageAccess{ownerID: ownerID, repoID: repoID, mode: mode})
}

func checkActionsPackageAccess(ctx context.Context, packageID int64) error {
	access, ok := ctx.Value(actionsPackageAccessKey{}).(*actionsPackageAccess)
	if !ok {
		return nil
	}
	if access.mode < perm.AccessModeWrite || access.repoID <= 0 {
		return util.ErrPermissionDenied
	}
	pkg, err := GetPackageByID(ctx, packageID)
	if err != nil {
		return err
	}
	if pkg.OwnerID != access.ownerID || (!pkg.IsInternal && pkg.RepoID != access.repoID) {
		return util.ErrPermissionDenied
	}
	return nil
}

func assignActionsPackage(ctx context.Context, pkg *Package) error {
	access, ok := ctx.Value(actionsPackageAccessKey{}).(*actionsPackageAccess)
	if !ok {
		return nil
	}
	if access.mode < perm.AccessModeWrite || access.repoID <= 0 || pkg.OwnerID != access.ownerID {
		return util.ErrPermissionDenied
	}
	if pkg.IsInternal {
		return nil
	}
	if pkg.RepoID != 0 && pkg.RepoID != access.repoID {
		return util.ErrPermissionDenied
	}
	if pkg.ID == 0 {
		pkg.RepoID = access.repoID
		return nil
	}
	if pkg.RepoID == 0 {
		if _, err := db.GetEngine(ctx).ID(pkg.ID).Where("repo_id = ?", 0).Cols("repo_id").Update(&Package{RepoID: access.repoID}); err != nil {
			return err
		}
	}
	if err := checkActionsPackageAccess(ctx, pkg.ID); err != nil {
		return err
	}
	pkg.RepoID = access.repoID
	return nil
}

func CheckActionsPackageWriteAccess(ctx context.Context, packageID int64) error {
	return checkActionsPackageAccess(ctx, packageID)
}

func checkActionsVersionAccess(ctx context.Context, versionID int64) error {
	if ctx.Value(actionsPackageAccessKey{}) == nil {
		return nil
	}
	version, err := GetVersionByID(ctx, versionID)
	if err != nil {
		return err
	}
	return checkActionsPackageAccess(ctx, version.PackageID)
}

func checkActionsFileAccess(ctx context.Context, fileID int64) error {
	if ctx.Value(actionsPackageAccessKey{}) == nil {
		return nil
	}
	file := &PackageFile{}
	found, err := db.GetEngine(ctx).ID(fileID).Get(file)
	if err != nil {
		return err
	}
	if !found {
		return ErrPackageFileNotExist
	}
	return checkActionsVersionAccess(ctx, file.VersionID)
}

func checkActionsPropertyAccess(ctx context.Context, refType PropertyType, refID int64) error {
	if ctx.Value(actionsPackageAccessKey{}) == nil {
		return nil
	}
	switch refType {
	case PropertyTypePackage:
		return checkActionsPackageAccess(ctx, refID)
	case PropertyTypeVersion:
		return checkActionsVersionAccess(ctx, refID)
	case PropertyTypeFile:
		return checkActionsFileAccess(ctx, refID)
	default:
		return util.ErrPermissionDenied
	}
}

func checkActionsPropertyIDAccess(ctx context.Context, propertyID int64) error {
	if ctx.Value(actionsPackageAccessKey{}) == nil {
		return nil
	}
	property := &PackageProperty{}
	found, err := db.GetEngine(ctx).ID(propertyID).Get(property)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return checkActionsPropertyAccess(ctx, property.RefType, property.RefID)
}
