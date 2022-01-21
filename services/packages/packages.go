// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	packages_module "code.gitea.io/gitea/modules/packages"
)

// PackageInfo describes a package
type PackageInfo struct {
	Owner        *user_model.User
	PackageType  packages_model.Type
	Name         string
	Version      string
	CompositeKey string
}

// PackageCreationInfo describes a package to create
type PackageCreationInfo struct {
	PackageInfo
	SemverCompatible bool
	Creator          *user_model.User
	Metadata         interface{}
	Properties       map[string]string
}

// PackageFileInfo describes a package file
type PackageFileInfo struct {
	Filename     string
	CompositeKey string
}

// PackageFileCreationInfo describes a package file to create
type PackageFileCreationInfo struct {
	PackageFileInfo
	Data   *packages_module.HashedBuffer
	IsLead bool
}

// CreatePackageAndAddFile creates a package with a file. If the same package exists already, ErrDuplicatePackageVersion is returned
func CreatePackageAndAddFile(pvci *PackageCreationInfo, pfci *PackageFileCreationInfo) (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
	return createPackageAndAddFile(pvci, pfci, false)
}

// CreatePackageOrAddFileToExisting creates a package with a file or adds the file if the package exists already
func CreatePackageOrAddFileToExisting(pvci *PackageCreationInfo, pfci *PackageFileCreationInfo) (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
	return createPackageAndAddFile(pvci, pfci, true)
}

func createPackageAndAddFile(pvci *PackageCreationInfo, pfci *PackageFileCreationInfo, allowDuplicate bool) (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, nil, err
	}
	defer committer.Close()

	pv, created, err := createPackageAndVersion(ctx, pvci, allowDuplicate)
	if err != nil {
		return nil, nil, err
	}

	pf, pb, blobCreated, err := addFileToPackageVersion(ctx, pv, pfci)
	removeBlob := false
	defer func() {
		if blobCreated && removeBlob {
			contentStore := packages_module.NewContentStore()
			if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
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
		pd, err := packages_model.GetPackageDescriptor(pv)
		if err != nil {
			return nil, nil, err
		}

		notification.NotifyPackageCreate(pvci.Creator, pd)
	}

	return pv, pf, nil
}

func createPackageAndVersion(ctx context.Context, pvci *PackageCreationInfo, allowDuplicate bool) (*packages_model.PackageVersion, bool, error) {
	log.Trace("Creating package: %v, %v, %v, %s, %s, %+v, %v", pvci.Creator.ID, pvci.Owner.ID, pvci.PackageType, pvci.Name, pvci.Version, pvci.Properties, allowDuplicate)

	p := &packages_model.Package{
		OwnerID:          pvci.Owner.ID,
		Type:             pvci.PackageType,
		Name:             pvci.Name,
		LowerName:        strings.ToLower(pvci.Name),
		SemverCompatible: pvci.SemverCompatible,
	}
	var err error
	if p, err = packages_model.TryInsertPackage(ctx, p); err != nil {
		if err != packages_model.ErrDuplicatePackage {
			log.Error("Error inserting package: %v", err)
			return nil, false, err
		}
	}

	metadataJSON, err := json.Marshal(pvci.Metadata)
	if err != nil {
		return nil, false, err
	}

	created := true
	pv := &packages_model.PackageVersion{
		PackageID:    p.ID,
		CreatorID:    pvci.Creator.ID,
		Version:      pvci.Version,
		LowerVersion: strings.ToLower(pvci.Version),
		MetadataJSON: string(metadataJSON),
	}
	if pv, err = packages_model.GetOrInsertVersion(ctx, pv); err != nil {
		if err == packages_model.ErrDuplicatePackageVersion {
			created = false
		}
		if err != packages_model.ErrDuplicatePackageVersion || !allowDuplicate {
			log.Error("Error inserting package: %v", err)
			return nil, false, err
		}
	}

	for name, value := range pvci.Properties {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, name, value); err != nil {
			log.Error("Error setting package version property: %v", err)
			return nil, false, err
		}
	}

	return pv, created, nil
}

// AddFileToExistingPackage adds a file to an existing package. If the package does not exist, ErrPackageNotExist is returned
func AddFileToExistingPackage(pvi *PackageInfo, pfci *PackageFileCreationInfo) (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, nil, err
	}
	defer committer.Close()

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version, pvi.CompositeKey)
	if err != nil {
		return nil, nil, err
	}

	pf, pb, blobCreated, err := addFileToPackageVersion(ctx, pv, pfci)
	removeBlob := false
	defer func() {
		if blobCreated && removeBlob {
			contentStore := packages_module.NewContentStore()
			if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
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

	return pv, pf, nil
}

func addFileToPackageVersion(ctx context.Context, pv *packages_model.PackageVersion, pfci *PackageFileCreationInfo) (*packages_model.PackageFile, *packages_model.PackageBlob, bool, error) {
	log.Trace("Adding package file: %v, %s", pv.ID, pfci.Filename)

	hashMD5, hashSHA1, hashSHA256, hashSHA512 := pfci.Data.Sums()

	blobKey := fmt.Sprintf("%x", hashSHA256)

	pb := &packages_model.PackageBlob{
		Size:       pfci.Data.Size(),
		HashMD5:    fmt.Sprintf("%x", hashMD5),
		HashSHA1:   fmt.Sprintf("%x", hashSHA1),
		HashSHA256: blobKey,
		HashSHA512: fmt.Sprintf("%x", hashSHA512),
	}
	pb, exists, err := packages_model.GetOrInsertBlob(ctx, pb)
	if err != nil {
		log.Error("Error inserting package blob: %v", err)
		return nil, nil, false, err
	}
	if !exists {
		contentStore := packages_module.NewContentStore()
		if err := contentStore.Save(packages_module.BlobHash256Key(blobKey), pfci.Data, pfci.Data.Size()); err != nil {
			log.Error("Error saving package blob in content store: %v", err)
			return nil, nil, false, err
		}
	}

	pf := &packages_model.PackageFile{
		VersionID:    pv.ID,
		BlobID:       pb.ID,
		Name:         pfci.Filename,
		LowerName:    strings.ToLower(pfci.Filename),
		CompositeKey: pfci.CompositeKey,
		IsLead:       pfci.IsLead,
	}
	if pf, err = packages_model.TryInsertFile(ctx, pf); err != nil {
		if err != packages_model.ErrDuplicatePackageFile {
			log.Error("Error inserting package file: %v", err)
		}
		return nil, nil, !exists, err
	}

	return pf, pb, !exists, nil
}

// DeletePackageVersionByNameAndVersion deletes a package version and all associated files
func DeletePackageVersionByNameAndVersion(doer *user_model.User, pvi *PackageInfo) error {
	pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version, pvi.CompositeKey)
	if err != nil {
		return err
	}

	return DeletePackageVersion(doer, pv)
}

// DeletePackageVersion deletes the package version and all associated files
func DeletePackageVersion(doer *user_model.User, pv *packages_model.PackageVersion) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	pd, err := packages_model.GetPackageDescriptorCtx(ctx, pv)
	if err != nil {
		return err
	}

	log.Trace("Deleting package: %v", pv.ID)

	if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeVersion, pv.ID); err != nil {
		return err
	}

	if err := packages_model.DeleteFilesByVersionID(ctx, pv.ID); err != nil {
		return err
	}

	if err := packages_model.DeleteVersionByID(ctx, pv.ID); err != nil {
		return err
	}

	if err := packages_model.DeletePackageByIDIfUnreferenced(ctx, pv.PackageID); err != nil {
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

	pbs, err := packages_model.GetUnreferencedBlobs(ctx)
	if err != nil {
		return err
	}

	for _, pb := range pbs {
		if err := packages_model.DeleteBlobByID(ctx, pb.ID); err != nil {
			return err
		}
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	contentStore := packages_module.NewContentStore()
	for _, pb := range pbs {
		if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
			log.Error("Error deleting package blob [%v]: %v", pb.ID, err)
		}
	}

	return nil
}

// GetFileStreamByPackageNameAndVersion returns the content of the specific package file
func GetFileStreamByPackageNameAndVersion(pvi *PackageInfo, pfi *PackageFileInfo) (io.ReadCloser, *packages_model.PackageFile, error) {
	log.Trace("Getting package file stream: %v, %v, %s, %s, %s, %s", pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version, pfi.Filename, pfi.CompositeKey)

	pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, pvi.Owner.ID, pvi.PackageType, pvi.Name, pvi.Version, pvi.CompositeKey)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			return nil, nil, err
		}
		log.Error("Error getting package: %v", err)
		return nil, nil, err
	}

	return GetFileStreamByPackageVersion(pv, pfi)
}

// GetFileStreamByPackageVersionAndFileID returns the content of the specific package file
func GetFileStreamByPackageVersionAndFileID(owner *user_model.User, versionID, fileID int64) (io.ReadCloser, *packages_model.PackageFile, error) {
	log.Trace("Getting package file stream: %v, %v, %v", owner.ID, versionID, fileID)

	pv, err := packages_model.GetVersionByID(db.DefaultContext, versionID)
	if err != nil {
		if err == packages_model.ErrPackageVersionNotExist {
			return nil, nil, packages_model.ErrPackageNotExist
		}
		log.Error("Error getting package version: %v", err)
		return nil, nil, err
	}

	p, err := packages_model.GetPackageByID(db.DefaultContext, pv.PackageID)
	if err != nil {
		log.Error("Error getting package: %v", err)
		return nil, nil, err
	}

	if p.OwnerID != owner.ID {
		return nil, nil, packages_model.ErrPackageNotExist
	}

	pf, err := packages_model.GetFileForVersionByID(db.DefaultContext, versionID, fileID)
	if err != nil {
		log.Error("Error getting file: %v", err)
		return nil, nil, err
	}

	return GetPackageFileStream(pv, pf)
}

// GetFileStreamByPackageVersion returns the content of the specific package file
func GetFileStreamByPackageVersion(pv *packages_model.PackageVersion, pfi *PackageFileInfo) (io.ReadCloser, *packages_model.PackageFile, error) {
	pf, err := packages_model.GetFileForVersionByName(db.DefaultContext, pv.ID, pfi.Filename, pfi.CompositeKey)
	if err != nil {
		return nil, nil, err
	}

	return GetPackageFileStream(pv, pf)
}

// GetPackageFileStream returns the cotent of the specific package file
func GetPackageFileStream(pv *packages_model.PackageVersion, pf *packages_model.PackageFile) (io.ReadCloser, *packages_model.PackageFile, error) {
	pb, err := packages_model.GetBlobByID(db.DefaultContext, pf.BlobID)
	if err != nil {
		return nil, nil, err
	}

	s, err := packages_module.NewContentStore().Get(packages_module.BlobHash256Key(pb.HashSHA256))
	if err == nil {
		if pf.IsLead {
			if err := packages_model.IncrementDownloadCounter(pv.ID); err != nil {
				log.Error("Error incrementing download counter: %v", err)
			}
		}
	}
	return s, pf, err
}
