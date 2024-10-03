// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	container_module "code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
)

var uploadVersionMutex sync.Mutex

// saveAsPackageBlob creates a package blob from an upload
// The uploaded blob gets stored in a special upload version to link them to the package/image
func saveAsPackageBlob(ctx context.Context, hsr packages_module.HashedSizeReader, pci *packages_service.PackageCreationInfo) (*packages_model.PackageBlob, error) { //nolint:unparam
	pb := packages_service.NewPackageBlob(hsr)

	exists := false

	contentStore := packages_module.NewContentStore()

	uploadVersion, err := getOrCreateUploadVersion(ctx, &pci.PackageInfo)
	if err != nil {
		return nil, err
	}

	err = db.WithTx(ctx, func(ctx context.Context) error {
		if err := packages_service.CheckSizeQuotaExceeded(ctx, pci.Creator, pci.Owner, packages_model.TypeContainer, hsr.Size()); err != nil {
			return err
		}

		pb, exists, err = packages_model.GetOrInsertBlob(ctx, pb)
		if err != nil {
			log.Error("Error inserting package blob: %v", err)
			return err
		}
		// FIXME: Workaround to be removed in v1.20
		// https://github.com/go-gitea/gitea/issues/19586
		if exists {
			err = contentStore.Has(packages_module.BlobHash256Key(pb.HashSHA256))
			if err != nil && (errors.Is(err, util.ErrNotExist) || errors.Is(err, os.ErrNotExist)) {
				log.Debug("Package registry inconsistent: blob %s does not exist on file system", pb.HashSHA256)
				exists = false
			}
		}
		if !exists {
			if err := contentStore.Save(packages_module.BlobHash256Key(pb.HashSHA256), hsr, hsr.Size()); err != nil {
				log.Error("Error saving package blob in content store: %v", err)
				return err
			}
		}

		return createFileForBlob(ctx, uploadVersion, pb)
	})
	if err != nil {
		if !exists {
			if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
				log.Error("Error deleting package blob from content store: %v", err)
			}
		}
		return nil, err
	}

	return pb, nil
}

// mountBlob mounts the specific blob to a different package
func mountBlob(ctx context.Context, pi *packages_service.PackageInfo, pb *packages_model.PackageBlob) error {
	uploadVersion, err := getOrCreateUploadVersion(ctx, pi)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		return createFileForBlob(ctx, uploadVersion, pb)
	})
}

func getOrCreateUploadVersion(ctx context.Context, pi *packages_service.PackageInfo) (*packages_model.PackageVersion, error) {
	var uploadVersion *packages_model.PackageVersion

	// FIXME: Replace usage of mutex with database transaction
	// https://github.com/go-gitea/gitea/pull/21862
	uploadVersionMutex.Lock()
	err := db.WithTx(ctx, func(ctx context.Context) error {
		created := true
		p := &packages_model.Package{
			OwnerID:   pi.Owner.ID,
			Type:      packages_model.TypeContainer,
			Name:      strings.ToLower(pi.Name),
			LowerName: strings.ToLower(pi.Name),
		}
		var err error
		if p, err = packages_model.TryInsertPackage(ctx, p); err != nil {
			if err == packages_model.ErrDuplicatePackage {
				created = false
			} else {
				log.Error("Error inserting package: %v", err)
				return err
			}
		}

		if created {
			if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, strings.ToLower(pi.Owner.LowerName+"/"+pi.Name)); err != nil {
				log.Error("Error setting package property: %v", err)
				return err
			}
		}

		pv := &packages_model.PackageVersion{
			PackageID:    p.ID,
			CreatorID:    pi.Owner.ID,
			Version:      container_model.UploadVersion,
			LowerVersion: container_model.UploadVersion,
			IsInternal:   true,
			MetadataJSON: "null",
		}
		if pv, err = packages_model.GetOrInsertVersion(ctx, pv); err != nil {
			if err != packages_model.ErrDuplicatePackageVersion {
				log.Error("Error inserting package: %v", err)
				return err
			}
		}

		uploadVersion = pv

		return nil
	})
	uploadVersionMutex.Unlock()

	return uploadVersion, err
}

func createFileForBlob(ctx context.Context, pv *packages_model.PackageVersion, pb *packages_model.PackageBlob) error {
	filename := strings.ToLower(fmt.Sprintf("sha256_%s", pb.HashSHA256))

	pf := &packages_model.PackageFile{
		VersionID:    pv.ID,
		BlobID:       pb.ID,
		Name:         filename,
		LowerName:    filename,
		CompositeKey: packages_model.EmptyFileKey,
	}
	var err error
	if pf, err = packages_model.TryInsertFile(ctx, pf); err != nil {
		if err == packages_model.ErrDuplicatePackageFile {
			return nil
		}
		log.Error("Error inserting package file: %v", err)
		return err
	}

	if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeFile, pf.ID, container_module.PropertyDigest, digestFromPackageBlob(pb)); err != nil {
		log.Error("Error setting package file property: %v", err)
		return err
	}

	return nil
}

func deleteBlob(ctx context.Context, ownerID int64, image, digest string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		pfds, err := container_model.GetContainerBlobs(ctx, &container_model.BlobSearchOptions{
			OwnerID: ownerID,
			Image:   image,
			Digest:  digest,
		})
		if err != nil {
			return err
		}

		for _, file := range pfds {
			if err := packages_service.DeletePackageFile(ctx, file.File); err != nil {
				return err
			}
		}
		return nil
	})
}

func digestFromHashSummer(h packages_module.HashSummer) string {
	_, _, hashSHA256, _ := h.Sums()
	return "sha256:" + hex.EncodeToString(hashSHA256)
}

func digestFromPackageBlob(pb *packages_model.PackageBlob) string {
	return "sha256:" + pb.HashSHA256
}
