// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	container_module "code.gitea.io/gitea/modules/packages/container"
	packages_service "code.gitea.io/gitea/services/packages"
)

// saveAsPackageBlob creates a package blob from an upload
// The uploaded blob gets stored in a special upload version to link them to the package/image
func saveAsPackageBlob(hsr packages_module.HashedSizeReader, pi *packages_service.PackageInfo) (*packages_model.PackageBlob, error) {
	pb := packages_service.NewPackageBlob(hsr)

	exists := false

	contentStore := packages_module.NewContentStore()

	err := db.WithTx(db.DefaultContext, func(ctx context.Context) error {
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

		pb, exists, err = packages_model.GetOrInsertBlob(ctx, pb)
		if err != nil {
			log.Error("Error inserting package blob: %v", err)
			return err
		}
		if !exists {
			if err := contentStore.Save(packages_module.BlobHash256Key(pb.HashSHA256), hsr, hsr.Size()); err != nil {
				log.Error("Error saving package blob in content store: %v", err)
				return err
			}
		}

		filename := strings.ToLower(fmt.Sprintf("sha256_%s", pb.HashSHA256))

		pf := &packages_model.PackageFile{
			VersionID:    pv.ID,
			BlobID:       pb.ID,
			Name:         filename,
			LowerName:    filename,
			CompositeKey: packages_model.EmptyFileKey,
		}
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

func deleteBlob(ownerID int64, image, digest string) error {
	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
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
