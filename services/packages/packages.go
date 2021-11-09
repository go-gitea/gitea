// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	packages_models "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	packages_module "code.gitea.io/gitea/modules/packages"
)

// PackageInfo describes a package
type PackageInfo struct {
	Owner       *models.User
	PackageType packages_models.Type
	Name        string
	Version     string
}

// PackageCreationInfo describes a package to create
type PackageCreationInfo struct {
	PackageInfo
	SemverCompatible bool
	Creator          *models.User
	Metadata         interface{}
	Properties       map[string]string
}

// PackageFileInfo describes a package file
type PackageFileInfo struct {
	Filename string
	Data     *packages_module.HashedBuffer
	IsLead   bool
}

// CreatePackageAndAddFile creates a package with a file. If the same package exists already, ErrDuplicatePackageVersion is returned
func CreatePackageAndAddFile(pvci *PackageCreationInfo, pfi *PackageFileInfo) (*packages_models.PackageVersion, *packages_models.PackageFile, error) {
	return createPackageAndAddFile(pvci, pfi, false)
}

// CreatePackageOrAddFileToExisting creates a package with a file or adds the file if the package exists already
func CreatePackageOrAddFileToExisting(pvci *PackageCreationInfo, pfi *PackageFileInfo) (*packages_models.PackageVersion, *packages_models.PackageFile, error) {
	return createPackageAndAddFile(pvci, pfi, true)
}

func createPackageAndAddFile(pvci *PackageCreationInfo, pfi *PackageFileInfo, allowDuplicate bool) (*packages_models.PackageVersion, *packages_models.PackageFile, error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, nil, err
	}
	defer committer.Close()

	pv, created, err := createPackageAndVersion(ctx, pvci, allowDuplicate)
	if err != nil {
		return nil, nil, err
	}

	pf, blobCreated, err := addFileToPackageVersion(ctx, pv, pfi)
	removeBlob := false
	defer func() {
		if blobCreated && removeBlob {
			contentStore := packages_module.NewContentStore()
			if err := contentStore.Delete(pf.BlobID); err != nil {
				log.Error("Error deleting package blob from content store: %v", err)
			}
		}
	}()
	if err != nil {
		removeBlob = true
		return nil, nil, err
	}

	if err := committer.Commit(); err != nil {
		removeBlob = true
		return nil, nil, err
	}

	if created {
		pd, err := packages_models.GetPackageDescriptor(pv)
		if err != nil {
			return nil, nil, err
		}

		notification.NotifyPackageCreate(pvci.Creator, pd)
	}

	return pv, pf, nil
}

func createPackageAndVersion(ctx context.Context, pvci *PackageCreationInfo, allowDuplicate bool) (*packages_models.PackageVersion, bool, error) {
	log.Trace("Creating package: %v, %v, %v, %s, %s, %+v, %v", pvci.Creator.ID, pvci.Owner.ID, pvci.PackageType, pvci.Name, pvci.Version, pvci.Properties, allowDuplicate)

	p := &packages_models.Package{
		OwnerID:          pvci.Owner.ID,
		Type:             pvci.PackageType,
		Name:             pvci.Name,
		LowerName:        strings.ToLower(pvci.Name),
		SemverCompatible: pvci.SemverCompatible,
	}
	var err error
	if p, err = packages_models.TryInsertPackage(ctx, p); err != nil {
		if err != packages_models.ErrDuplicatePackage {
			log.Error("Error inserting package: %v", err)
			return nil, false, err
		}
	}

	metadataJSON, err := json.Marshal(pvci.Metadata)
	if err != nil {
		return nil, false, err
	}

	created := true
	pv := &packages_models.PackageVersion{
		PackageID:    p.ID,
		CreatorID:    pvci.Creator.ID,
		Version:      pvci.Version,
		LowerVersion: strings.ToLower(pvci.Version),
		MetadataJSON: string(metadataJSON),
	}
	if pv, err = packages_models.GetOrInsertVersion(ctx, pv); err != nil {
		if err == packages_models.ErrDuplicatePackageVersion {
			created = false
		}
		if err != packages_models.ErrDuplicatePackageVersion || !allowDuplicate {
			log.Error("Error inserting package: %v", err)
			return nil, false, err
		}
	}

	if err := packages_models.SetVersionProperties(ctx, pv.ID, pvci.Properties); err != nil {
		log.Error("Error setting package version properties: %v", err)
		return nil, false, err
	}

	return pv, created, nil
}

// AddFileToExistingPackage adds a file to an existing package. If the package does not exist, ErrPackageNotExist is returned
func AddFileToExistingPackage(pvi *PackageInfo, pfi *PackageFileInfo) (*packages_models.PackageVersion, *packages_models.PackageFile, error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, nil, err
	}
	defer committer.Close()

	pv, err := packages_models.GetVersionByNameAndVersion(ctx, pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version)
	if err != nil {
		return nil, nil, err
	}

	pf, blobCreated, err := addFileToPackageVersion(ctx, pv, pfi)
	removeBlob := false
	defer func() {
		if blobCreated && removeBlob {
			contentStore := packages_module.NewContentStore()
			if err := contentStore.Delete(pf.BlobID); err != nil {
				log.Error("Error deleting package blob from content store: %v", err)
			}
		}
	}()
	if err != nil {
		removeBlob = true
		return nil, nil, err
	}

	if err := committer.Commit(); err != nil {
		return nil, nil, err
	}

	return pv, pf, nil
}

func addFileToPackageVersion(ctx context.Context, pv *packages_models.PackageVersion, pfi *PackageFileInfo) (*packages_models.PackageFile, bool, error) {
	log.Trace("Adding package file: %v, %s", pv.ID, pfi.Filename)

	hashMD5, hashSHA1, hashSHA256, hashSHA512 := pfi.Data.Sums()
	pb := &packages_models.PackageBlob{
		Size:       pfi.Data.Size(),
		HashMD5:    fmt.Sprintf("%x", hashMD5),
		HashSHA1:   fmt.Sprintf("%x", hashSHA1),
		HashSHA256: fmt.Sprintf("%x", hashSHA256),
		HashSHA512: fmt.Sprintf("%x", hashSHA512),
	}
	pb, exists, err := packages_models.GetOrInsertBlob(ctx, pb)
	if err != nil {
		log.Error("Error inserting package blob: %v", err)
		return nil, false, err
	}
	if !exists {
		contentStore := packages_module.NewContentStore()
		if err := contentStore.Save(pb.ID, pfi.Data, pfi.Data.Size()); err != nil {
			log.Error("Error saving package blob in content store: %v", err)
			return nil, false, err
		}
	}

	pf := &packages_models.PackageFile{
		VersionID: pv.ID,
		BlobID:    pb.ID,
		Name:      pfi.Filename,
		LowerName: strings.ToLower(pfi.Filename),
		IsLead:    pfi.IsLead,
	}
	if pf, err = packages_models.TryInsertFile(ctx, pf); err != nil {
		if err != packages_models.ErrDuplicatePackageFile {
			log.Error("Error inserting package file: %v", err)
		}
		return nil, !exists, err
	}

	return pf, !exists, nil
}

// DeletePackageVersionByNameAndVersion deletes a package version and all associated files
func DeletePackageVersionByNameAndVersion(doer *models.User, pvi *PackageInfo) error {
	return deletePackageVersion(
		doer,
		pvi.Owner,
		func(ctx context.Context) (*packages_models.PackageVersion, error) {
			return packages_models.GetVersionByNameAndVersion(ctx, pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version)
		},
	)
}

// DeleteVersionByID deletes a package version and all associated files
func DeleteVersionByID(doer *models.User, owner *models.User, versionID int64) error {
	return deletePackageVersion(
		doer,
		owner,
		func(ctx context.Context) (*packages_models.PackageVersion, error) {
			return packages_models.GetVersionByID(ctx, versionID)
		},
	)
}

func deletePackageVersion(doer *models.User, owner *models.User, cb func(context.Context) (*packages_models.PackageVersion, error)) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	pv, err := cb(ctx)
	if err != nil {
		if err != packages_models.ErrPackageNotExist {
			log.Error("Error getting package: %v", err)
		}
		return err
	}

	pd, err := packages_models.GetPackageDescriptorCtx(ctx, pv)
	if err != nil {
		return err
	}

	log.Trace("Deleting package: %v, %v", owner.ID, pv.ID)

	if err := packages_models.DeleteVersionPropertiesByVersionID(ctx, pv.ID); err != nil {
		return err
	}

	if err := packages_models.DeleteFilesByVersionID(ctx, pv.ID); err != nil {
		return err
	}

	if err := packages_models.DeleteVersionByID(ctx, pv.ID); err != nil {
		return err
	}

	if err := packages_models.DeletePackageByIDIfUnreferenced(ctx, pv.PackageID); err != nil {
		return err
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	notification.NotifyPackageDelete(doer, pd)

	return DeleteUnreferencedBlobs()
}

// DeleteUnreferencedBlobs deletes all unreferenced package blobs
func DeleteUnreferencedBlobs() error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	pbs, err := packages_models.GetUnreferencedBlobs(ctx)
	if err != nil {
		return err
	}

	for _, pb := range pbs {
		if err := packages_models.DeleteBlobByID(ctx, pb.ID); err != nil {
			return err
		}
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	contentStore := packages_module.NewContentStore()
	for _, pb := range pbs {
		if err := contentStore.Delete(pb.ID); err != nil {
			log.Error("Error deleting package blob [%v]: %v", pb.ID, err)
		}
	}

	return nil
}

// GetFileStreamByPackageNameAndVersion returns the content of the specific package file
func GetFileStreamByPackageNameAndVersion(pvi *PackageInfo, filename string) (io.ReadCloser, *packages_models.PackageFile, error) {
	log.Trace("Getting package file stream: %v, %v, %s, %s, %s", pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version, filename)

	pv, err := packages_models.GetVersionByNameAndVersion(db.DefaultContext, pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version)
	if err != nil {
		if err == packages_models.ErrPackageNotExist {
			return nil, nil, err
		}
		log.Error("Error getting package: %v", err)
		return nil, nil, err
	}

	return GetPackageFileStream(pv, filename)
}

// GetFileStreamByPackageVersionID returns the content of the specific package file
func GetFileStreamByPackageVersionID(owner *models.User, versionID int64, filename string) (io.ReadCloser, *packages_models.PackageFile, error) {
	log.Trace("Getting package file stream: %v, %v, %s", owner.ID, versionID, filename)

	pv, err := packages_models.GetVersionByID(db.DefaultContext, versionID)
	if err != nil {
		if err == packages_models.ErrPackageVersionNotExist {
			return nil, nil, packages_models.ErrPackageNotExist
		}
		log.Error("Error getting package version: %v", err)
		return nil, nil, err
	}

	p, err := packages_models.GetPackageByID(db.DefaultContext, pv.PackageID)
	if err != nil {
		log.Error("Error getting package: %v", err)
		return nil, nil, err
	}

	if p.OwnerID != owner.ID {
		return nil, nil, packages_models.ErrPackageNotExist
	}

	return GetPackageFileStream(pv, filename)
}

// GetPackageFileStream returns the cotent of the specific package file
func GetPackageFileStream(pv *packages_models.PackageVersion, filename string) (io.ReadCloser, *packages_models.PackageFile, error) {
	pf, err := packages_models.GetFileForVersionByName(db.DefaultContext, pv.ID, filename)
	if err != nil {
		return nil, nil, err
	}

	s, err := packages_module.NewContentStore().Get(pf.BlobID)
	if err == nil {
		if pf.IsLead {
			if err := packages_models.IncrementDownloadCounter(pv.ID); err != nil {
				log.Error("Error incrementing download counter: %v", err)
			}
		}
	}
	return s, pf, err
}
