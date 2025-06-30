// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	container_module "code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
	packages_service "code.gitea.io/gitea/services/packages"
	container_service "code.gitea.io/gitea/services/packages/container"

	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

// manifestCreationInfo describes a manifest to create
type manifestCreationInfo struct {
	MediaType  string
	Owner      *user_model.User
	Creator    *user_model.User
	Image      string
	Reference  string
	IsTagged   bool
	Properties map[string]string
}

func processManifest(ctx context.Context, mci *manifestCreationInfo, buf *packages_module.HashedBuffer) (string, error) {
	var index oci.Index
	if err := json.NewDecoder(buf).Decode(&index); err != nil {
		return "", err
	}
	if index.SchemaVersion != 2 {
		return "", errUnsupported.WithMessage("Schema version is not supported")
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	if !container_module.IsMediaTypeValid(mci.MediaType) {
		mci.MediaType = index.MediaType
		if !container_module.IsMediaTypeValid(mci.MediaType) {
			return "", errManifestInvalid.WithMessage("MediaType not recognized")
		}
	}

	// .../container/manifest.go:453:createManifestBlob() [E] Error inserting package blob: Error 1062 (23000): Duplicate entry '..........' for key 'package_blob.UQE_package_blob_md5'
	releaser, err := globallock.Lock(ctx, containerGlobalLockKey(mci.Owner.ID, mci.Image, "manifest"))
	if err != nil {
		return "", err
	}
	defer releaser()

	if container_module.IsMediaTypeImageManifest(mci.MediaType) {
		return processOciImageManifest(ctx, mci, buf)
	} else if container_module.IsMediaTypeImageIndex(mci.MediaType) {
		return processOciImageIndex(ctx, mci, buf)
	}
	return "", errManifestInvalid
}

type processManifestTxRet struct {
	pv      *packages_model.PackageVersion
	pb      *packages_model.PackageBlob
	created bool
	digest  string
}

func handleCreateManifestResult(ctx context.Context, err error, mci *manifestCreationInfo, contentStore *packages_module.ContentStore, txRet *processManifestTxRet) (string, error) {
	if err != nil && txRet.created && txRet.pb != nil {
		if err := contentStore.Delete(packages_module.BlobHash256Key(txRet.pb.HashSHA256)); err != nil {
			log.Error("Error deleting package blob from content store: %v", err)
		}
		return "", err
	}
	pd, err := packages_model.GetPackageDescriptor(ctx, txRet.pv)
	if err != nil {
		log.Error("Error getting package descriptor: %v", err) // ignore this error
	} else {
		notify_service.PackageCreate(ctx, mci.Creator, pd)
	}
	return txRet.digest, nil
}

func processOciImageManifest(ctx context.Context, mci *manifestCreationInfo, buf *packages_module.HashedBuffer) (manifestDigest string, errRet error) {
	manifest, configDescriptor, metadata, err := container_service.ParseManifestMetadata(ctx, buf, mci.Owner.ID, mci.Image)
	if err != nil {
		return "", err
	}
	if _, err = buf.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	contentStore := packages_module.NewContentStore()
	var txRet processManifestTxRet
	err = db.WithTx(ctx, func(ctx context.Context) (err error) {
		blobReferences := make([]*blobReference, 0, 1+len(manifest.Layers))
		blobReferences = append(blobReferences, &blobReference{
			Digest:       manifest.Config.Digest,
			MediaType:    manifest.Config.MediaType,
			File:         configDescriptor,
			ExpectedSize: manifest.Config.Size,
		})

		for _, layer := range manifest.Layers {
			pfd, err := container_model.GetContainerBlob(ctx, &container_model.BlobSearchOptions{
				OwnerID: mci.Owner.ID,
				Image:   mci.Image,
				Digest:  string(layer.Digest),
			})
			if err != nil {
				return err
			}

			blobReferences = append(blobReferences, &blobReference{
				Digest:       layer.Digest,
				MediaType:    layer.MediaType,
				File:         pfd,
				ExpectedSize: layer.Size,
			})
		}

		pv, err := createPackageAndVersion(ctx, mci, metadata)
		if err != nil {
			return err
		}

		uploadVersion, err := packages_model.GetInternalVersionByNameAndVersion(ctx, mci.Owner.ID, packages_model.TypeContainer, mci.Image, container_module.UploadVersion)
		if err != nil && !errors.Is(err, packages_model.ErrPackageNotExist) {
			return err
		}

		for _, ref := range blobReferences {
			if _, err = createFileFromBlobReference(ctx, pv, uploadVersion, ref); err != nil {
				return err
			}
		}
		txRet.pv = pv
		txRet.pb, txRet.created, txRet.digest, err = createManifestBlob(ctx, contentStore, mci, pv, buf)
		return err
	})

	return handleCreateManifestResult(ctx, err, mci, contentStore, &txRet)
}

func processOciImageIndex(ctx context.Context, mci *manifestCreationInfo, buf *packages_module.HashedBuffer) (manifestDigest string, errRet error) {
	var index oci.Index
	if err := json.NewDecoder(buf).Decode(&index); err != nil {
		return "", err
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	contentStore := packages_module.NewContentStore()
	var txRet processManifestTxRet
	err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		metadata := &container_module.Metadata{
			Type:      container_module.TypeOCI,
			Manifests: make([]*container_module.Manifest, 0, len(index.Manifests)),
		}

		for _, manifest := range index.Manifests {
			if !container_module.IsMediaTypeImageManifest(manifest.MediaType) {
				return errManifestInvalid
			}

			platform := container_module.DefaultPlatform
			if manifest.Platform != nil {
				platform = fmt.Sprintf("%s/%s", manifest.Platform.OS, manifest.Platform.Architecture)
				if manifest.Platform.Variant != "" {
					platform = fmt.Sprintf("%s/%s", platform, manifest.Platform.Variant)
				}
			}

			pfd, err := container_model.GetContainerBlob(ctx, &container_model.BlobSearchOptions{
				OwnerID:    mci.Owner.ID,
				Image:      mci.Image,
				Digest:     string(manifest.Digest),
				IsManifest: true,
			})
			if err != nil {
				if errors.Is(err, container_model.ErrContainerBlobNotExist) {
					return errManifestBlobUnknown
				}
				return err
			}

			size, err := packages_model.CalculateFileSize(ctx, &packages_model.PackageFileSearchOptions{
				VersionID: pfd.File.VersionID,
			})
			if err != nil {
				return err
			}

			metadata.Manifests = append(metadata.Manifests, &container_module.Manifest{
				Platform: platform,
				Digest:   string(manifest.Digest),
				Size:     size,
			})
		}

		pv, err := createPackageAndVersion(ctx, mci, metadata)
		if err != nil {
			return err
		}

		txRet.pv = pv
		txRet.pb, txRet.created, txRet.digest, err = createManifestBlob(ctx, contentStore, mci, pv, buf)
		return err
	})

	return handleCreateManifestResult(ctx, err, mci, contentStore, &txRet)
}

func createPackageAndVersion(ctx context.Context, mci *manifestCreationInfo, metadata *container_module.Metadata) (*packages_model.PackageVersion, error) {
	created := true
	p := &packages_model.Package{
		OwnerID:   mci.Owner.ID,
		Type:      packages_model.TypeContainer,
		Name:      strings.ToLower(mci.Image),
		LowerName: strings.ToLower(mci.Image),
	}
	var err error
	if p, err = packages_model.TryInsertPackage(ctx, p); err != nil {
		if !errors.Is(err, packages_model.ErrDuplicatePackage) {
			log.Error("Error inserting package: %v", err)
			return nil, err
		}
		created = false
	}

	if created {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, strings.ToLower(mci.Owner.LowerName+"/"+mci.Image)); err != nil {
			log.Error("Error setting package property: %v", err)
			return nil, err
		}
	}

	metadata.IsTagged = mci.IsTagged

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	_pv := &packages_model.PackageVersion{
		PackageID:    p.ID,
		CreatorID:    mci.Creator.ID,
		Version:      strings.ToLower(mci.Reference),
		LowerVersion: strings.ToLower(mci.Reference),
		MetadataJSON: string(metadataJSON),
	}
	pv, err := packages_model.GetOrInsertVersion(ctx, _pv)
	if err != nil {
		if !errors.Is(err, packages_model.ErrDuplicatePackageVersion) {
			log.Error("Error inserting package: %v", err)
			return nil, err
		}

		if container_module.IsMediaTypeImageIndex(mci.MediaType) {
			if pv.CreatedUnix.AsTime().Before(time.Now().Add(-24 * time.Hour)) {
				if err = packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
					return nil, err
				}
				// keep download count on overwriting
				_pv.DownloadCount = pv.DownloadCount
				if pv, err = packages_model.GetOrInsertVersion(ctx, _pv); err != nil {
					if !errors.Is(err, packages_model.ErrDuplicatePackageVersion) {
						log.Error("Error inserting package: %v", err)
						return nil, err
					}
				}
			} else {
				err = packages_model.UpdateVersion(ctx, &packages_model.PackageVersion{ID: pv.ID, MetadataJSON: _pv.MetadataJSON})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if err := packages_service.CheckCountQuotaExceeded(ctx, mci.Creator, mci.Owner); err != nil {
		return nil, err
	}

	if mci.IsTagged {
		if err = packages_model.InsertOrUpdateProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestTagged, ""); err != nil {
			return nil, err
		}
	} else {
		if err = packages_model.DeletePropertiesByName(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestTagged); err != nil {
			return nil, err
		}
	}

	if err = packages_model.DeletePropertiesByName(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestReference); err != nil {
		return nil, err
	}
	for _, manifest := range metadata.Manifests {
		if _, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestReference, manifest.Digest); err != nil {
			return nil, err
		}
	}

	return pv, nil
}

type blobReference struct {
	Digest       digest.Digest
	MediaType    string
	Name         string
	File         *packages_model.PackageFileDescriptor
	ExpectedSize int64
	IsLead       bool
}

func createFileFromBlobReference(ctx context.Context, pv, uploadVersion *packages_model.PackageVersion, ref *blobReference) (*packages_model.PackageFile, error) {
	if ref.File.Blob.Size != ref.ExpectedSize {
		return nil, errSizeInvalid
	}

	if ref.Name == "" {
		ref.Name = strings.ToLower("sha256_" + ref.File.Blob.HashSHA256)
	}

	pf := &packages_model.PackageFile{
		VersionID:    pv.ID,
		BlobID:       ref.File.Blob.ID,
		Name:         ref.Name,
		LowerName:    ref.Name,
		CompositeKey: string(ref.Digest),
		IsLead:       ref.IsLead,
	}
	var err error
	if pf, err = packages_model.TryInsertFile(ctx, pf); err != nil {
		if errors.Is(err, packages_model.ErrDuplicatePackageFile) {
			// Skip this blob because the manifest contains the same filesystem layer multiple times.
			return pf, nil
		}
		log.Error("Error inserting package file: %v", err)
		return nil, err
	}

	props := map[string]string{
		container_module.PropertyMediaType: ref.MediaType,
		container_module.PropertyDigest:    string(ref.Digest),
	}
	for name, value := range props {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeFile, pf.ID, name, value); err != nil {
			log.Error("Error setting package file property: %v", err)
			return nil, err
		}
	}

	// Remove the ref file (old file) from the blob upload version
	if uploadVersion != nil && ref.File.File != nil && uploadVersion.ID == ref.File.File.VersionID {
		if err := packages_service.DeletePackageFile(ctx, ref.File.File); err != nil {
			return nil, err
		}
	}

	return pf, nil
}

func createManifestBlob(ctx context.Context, contentStore *packages_module.ContentStore, mci *manifestCreationInfo, pv *packages_model.PackageVersion, buf *packages_module.HashedBuffer) (_ *packages_model.PackageBlob, created bool, manifestDigest string, _ error) {
	pb, exists, err := packages_model.GetOrInsertBlob(ctx, packages_service.NewPackageBlob(buf))
	if err != nil {
		log.Error("Error inserting package blob: %v", err)
		return nil, false, "", err
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
		if err := contentStore.Save(packages_module.BlobHash256Key(pb.HashSHA256), buf, buf.Size()); err != nil {
			log.Error("Error saving package blob in content store: %v", err)
			return nil, false, "", err
		}
	}

	manifestDigest = digestFromHashSummer(buf)
	pf, err := createFileFromBlobReference(ctx, pv, nil, &blobReference{
		Digest:       digest.Digest(manifestDigest),
		MediaType:    mci.MediaType,
		Name:         container_module.ManifestFilename,
		File:         &packages_model.PackageFileDescriptor{Blob: pb},
		ExpectedSize: pb.Size,
		IsLead:       true,
	})
	if err != nil {
		return nil, false, "", err
	}

	oldManifestFiles, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:     mci.Owner.ID,
		PackageType: packages_model.TypeContainer,
		VersionID:   pv.ID,
		Query:       container_module.ManifestFilename,
	})
	if err != nil {
		return nil, false, "", err
	}
	for _, oldManifestFile := range oldManifestFiles {
		if oldManifestFile.ID != pf.ID && oldManifestFile.IsLead {
			err = packages_model.UpdateFile(ctx, &packages_model.PackageFile{ID: oldManifestFile.ID, IsLead: false}, []string{"is_lead"})
			if err != nil {
				return nil, false, "", err
			}
		}
	}
	return pb, !exists, manifestDigest, err
}
