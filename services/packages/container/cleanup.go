// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"context"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
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

	versions := make(map[int64]struct{})
	for _, pf := range pfs {
		versions[pf.VersionID] = struct{}{}

		if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeFile, pf.ID); err != nil {
			return err
		}
		if err := packages_model.DeleteFileByID(ctx, pf.ID); err != nil {
			return err
		}
	}

	for versionID := range versions {
		has, err := packages_model.HasVersionFileReferences(ctx, versionID)
		if err != nil {
			return err
		}
		if !has {
			if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeVersion, versionID); err != nil {
				return err
			}

			if err := packages_model.DeleteVersionByID(ctx, versionID); err != nil {
				return err
			}
		}
	}

	return nil
}
