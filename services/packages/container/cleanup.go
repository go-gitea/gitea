// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"context"
	"strings"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	user_model "code.gitea.io/gitea/models/user"
	container_module "code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/util"
)

// Cleanup removes expired container data
func Cleanup(ctx context.Context, olderThan time.Duration) error {
	if err := cleanupExpiredBlobUploads(ctx, olderThan); err != nil {
		return err
	}
	return cleanupExpiredUploadedBlobs(ctx, olderThan)
}

// cleanupExpiredBlobUploads removes expired blob uploads
func cleanupExpiredBlobUploads(ctx context.Context, olderThan time.Duration) error {
	pbus, err := packages_model.FindExpiredBlobUploads(ctx, olderThan)
	if err != nil {
		return err
	}

	for _, pbu := range pbus {
		if err := RemoveBlobUploadByID(ctx, pbu.ID); err != nil {
			return err
		}
	}

	return nil
}

// cleanupExpiredUploadedBlobs removes expired uploaded blobs not referenced by a manifest
func cleanupExpiredUploadedBlobs(ctx context.Context, olderThan time.Duration) error {
	pfs, err := container_model.SearchExpiredUploadedBlobs(ctx, olderThan)
	if err != nil {
		return err
	}

	for _, pf := range pfs {
		if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeFile, pf.ID); err != nil {
			return err
		}
		if err := packages_model.DeleteFileByID(ctx, pf.ID); err != nil {
			return err
		}
	}

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		Type: packages_model.TypeContainer,
		Version: packages_model.SearchValue{
			ExactMatch: true,
			Value:      container_model.UploadVersion,
		},
		IsInternal: true,
		HasFiles:   util.OptionalBoolFalse,
	})
	if err != nil {
		return err
	}

	for _, pv := range pvs {
		if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeVersion, pv.ID); err != nil {
			return err
		}

		if err := packages_model.DeleteVersionByID(ctx, pv.ID); err != nil {
			return err
		}
	}

	return nil
}

// UpdateRepositoryNames updates the repository name property for all packages of the specific owner
func UpdateRepositoryNames(ctx context.Context, owner *user_model.User, newOwnerName string) error {
	ps, err := packages_model.GetPackagesByType(ctx, owner.ID, packages_model.TypeContainer)
	if err != nil {
		return err
	}

	newOwnerName = strings.ToLower(newOwnerName)

	for _, p := range ps {
		if err := packages_model.DeletePropertyByName(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository); err != nil {
			return err
		}

		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, newOwnerName+"/"+p.LowerName); err != nil {
			return err
		}
	}

	return nil
}
