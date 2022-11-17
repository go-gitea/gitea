// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"context"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	container_module "code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/packages/container/oci"
	packages_service "code.gitea.io/gitea/services/packages"
)

// manifestCreationInfo describes a manifest to create
type manifestCreationInfo struct {
	MediaType  oci.MediaType
	Owner      *user_model.User
	Creator    *user_model.User
	Image      string
	Reference  string
	IsTagged   bool
	Properties map[string]string
}

func processManifest(mci *manifestCreationInfo, buf *packages_module.HashedBuffer) (string, error) {
	var schema oci.SchemaMediaBase
	if err := json.NewDecoder(buf).Decode(&schema); err != nil {
		return "", err
	}

	if schema.SchemaVersion != 2 {
		return "", errUnsupported.WithMessage("Schema version is not supported")
	}

	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	if !mci.MediaType.IsValid() {
		mci.MediaType = schema.MediaType
		if !mci.MediaType.IsValid() {
			return "", errManifestInvalid.WithMessage("MediaType not recognized")
		}
	}

	if mci.MediaType.IsImageManifest() {
		d, err := processImageManifest(mci, buf)
		return d, err
	} else if mci.MediaType.IsImageIndex() {
		d, err := processImageManifestIndex(mci, buf)
		return d, err
	}
	return "", errManifestInvalid
}

func processImageManifest(mci *manifestCreationInfo, buf *packages_module.HashedBuffer) (string, error) {
	manifestDigest := ""

	err := func() error {
		var manifest oci.Manifest
		if err := json.NewDecoder(buf).Decode(&manifest); err != nil {
			return err
		}

		if _, err := buf.Seek(0, io.SeekStart); err != nil {
			return err
		}

		ctx, committer, err := db.TxContext(db.DefaultContext)
		if err != nil {
			return err
		}
		defer committer.Close()

		configDescriptor, err := container_model.GetContainerBlob(ctx, &container_model.BlobSearchOptions{
			OwnerID: mci.Owner.ID,
			Image:   mci.Image,
			Digest:  string(manifest.Config.Digest),
		})
		if err != nil {
			return err
		}

		configReader, err := packages_module.NewContentStore().Get(packages_module.BlobHash256Key(configDescriptor.Blob.HashSHA256))
		if err != nil {
			return err
		}
		defer configReader.Close()

		metadata, err := container_module.ParseImageConfig(manifest.Config.MediaType, configReader)
		if err != nil {
			return err
		}

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

		uploadVersion, err := packages_model.GetInternalVersionByNameAndVersion(ctx, mci.Owner.ID, packages_model.TypeContainer, mci.Image, container_model.UploadVersion)
		if err != nil && err != packages_model.ErrPackageNotExist {
			return err
		}

		for _, ref := range blobReferences {
			if err := createFileFromBlobReference(ctx, pv, uploadVersion, ref); err != nil {
				return err
			}
		}

		pb, created, digest, err := createManifestBlob(ctx, mci, pv, buf)
		removeBlob := false
		defer func() {
			if removeBlob {
				contentStore := packages_module.NewContentStore()
				if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
					log.Error("Error deleting package blob from content store: %v", err)
				}
			}
		}()
		if err != nil {
			removeBlob = created
			return err
		}

		if err := committer.Commit(); err != nil {
			removeBlob = created
			return err
		}

		manifestDigest = digest

		return nil
	}()
	if err != nil {
		return "", err
	}

	return manifestDigest, nil
}

func processImageManifestIndex(mci *manifestCreationInfo, buf *packages_module.HashedBuffer) (string, error) {
	manifestDigest := ""

	err := func() error {
		var index oci.Index
		if err := json.NewDecoder(buf).Decode(&index); err != nil {
			return err
		}

		if _, err := buf.Seek(0, io.SeekStart); err != nil {
			return err
		}

		ctx, committer, err := db.TxContext(db.DefaultContext)
		if err != nil {
			return err
		}
		defer committer.Close()

		metadata := &container_module.Metadata{
			Type:      container_module.TypeOCI,
			MultiArch: make(map[string]string),
		}

		for _, manifest := range index.Manifests {
			if !manifest.MediaType.IsImageManifest() {
				return errManifestInvalid
			}

			platform := container_module.DefaultPlatform
			if manifest.Platform != nil {
				platform = fmt.Sprintf("%s/%s", manifest.Platform.OS, manifest.Platform.Architecture)
				if manifest.Platform.Variant != "" {
					platform = fmt.Sprintf("%s/%s", platform, manifest.Platform.Variant)
				}
			}

			_, err := container_model.GetContainerBlob(ctx, &container_model.BlobSearchOptions{
				OwnerID:    mci.Owner.ID,
				Image:      mci.Image,
				Digest:     string(manifest.Digest),
				IsManifest: true,
			})
			if err != nil {
				if err == container_model.ErrContainerBlobNotExist {
					return errManifestBlobUnknown
				}
				return err
			}

			metadata.MultiArch[platform] = string(manifest.Digest)
		}

		pv, err := createPackageAndVersion(ctx, mci, metadata)
		if err != nil {
			return err
		}

		pb, created, digest, err := createManifestBlob(ctx, mci, pv, buf)
		removeBlob := false
		defer func() {
			if removeBlob {
				contentStore := packages_module.NewContentStore()
				if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
					log.Error("Error deleting package blob from content store: %v", err)
				}
			}
		}()
		if err != nil {
			removeBlob = created
			return err
		}

		if err := committer.Commit(); err != nil {
			removeBlob = created
			return err
		}

		manifestDigest = digest

		return nil
	}()
	if err != nil {
		return "", err
	}

	return manifestDigest, nil
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
		if err == packages_model.ErrDuplicatePackage {
			created = false
		} else {
			log.Error("Error inserting package: %v", err)
			return nil, err
		}
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
	var pv *packages_model.PackageVersion
	if pv, err = packages_model.GetOrInsertVersion(ctx, _pv); err != nil {
		if err == packages_model.ErrDuplicatePackageVersion {
			if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
				return nil, err
			}

			// keep download count on overwrite
			_pv.DownloadCount = pv.DownloadCount

			if pv, err = packages_model.GetOrInsertVersion(ctx, _pv); err != nil {
				log.Error("Error inserting package: %v", err)
				return nil, err
			}
		} else {
			log.Error("Error inserting package: %v", err)
			return nil, err
		}
	}

	if mci.IsTagged {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestTagged, ""); err != nil {
			log.Error("Error setting package version property: %v", err)
			return nil, err
		}
	}
	for _, digest := range metadata.MultiArch {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestReference, digest); err != nil {
			log.Error("Error setting package version property: %v", err)
			return nil, err
		}
	}

	return pv, nil
}

type blobReference struct {
	Digest       oci.Digest
	MediaType    oci.MediaType
	Name         string
	File         *packages_model.PackageFileDescriptor
	ExpectedSize int64
	IsLead       bool
}

func createFileFromBlobReference(ctx context.Context, pv, uploadVersion *packages_model.PackageVersion, ref *blobReference) error {
	if ref.File.Blob.Size != ref.ExpectedSize {
		return errSizeInvalid
	}

	if ref.Name == "" {
		ref.Name = strings.ToLower(fmt.Sprintf("sha256_%s", ref.File.Blob.HashSHA256))
	}

	pf := &packages_model.PackageFile{
		VersionID: pv.ID,
		BlobID:    ref.File.Blob.ID,
		Name:      ref.Name,
		LowerName: ref.Name,
		IsLead:    ref.IsLead,
	}
	var err error
	if pf, err = packages_model.TryInsertFile(ctx, pf); err != nil {
		if err == packages_model.ErrDuplicatePackageFile {
			// Skip this blob because the manifest contains the same filesystem layer multiple times.
			return nil
		}
		log.Error("Error inserting package file: %v", err)
		return err
	}

	props := map[string]string{
		container_module.PropertyMediaType: string(ref.MediaType),
		container_module.PropertyDigest:    string(ref.Digest),
	}
	for name, value := range props {
		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypeFile, pf.ID, name, value); err != nil {
			log.Error("Error setting package file property: %v", err)
			return err
		}
	}

	// Remove the file from the blob upload version
	if uploadVersion != nil && ref.File.File != nil && uploadVersion.ID == ref.File.File.VersionID {
		if err := packages_service.DeletePackageFile(ctx, ref.File.File); err != nil {
			return err
		}
	}

	return nil
}

func createManifestBlob(ctx context.Context, mci *manifestCreationInfo, pv *packages_model.PackageVersion, buf *packages_module.HashedBuffer) (*packages_model.PackageBlob, bool, string, error) {
	pb, exists, err := packages_model.GetOrInsertBlob(ctx, packages_service.NewPackageBlob(buf))
	if err != nil {
		log.Error("Error inserting package blob: %v", err)
		return nil, false, "", err
	}
	if !exists {
		contentStore := packages_module.NewContentStore()
		if err := contentStore.Save(packages_module.BlobHash256Key(pb.HashSHA256), buf, buf.Size()); err != nil {
			log.Error("Error saving package blob in content store: %v", err)
			return nil, false, "", err
		}
	}

	manifestDigest := digestFromHashSummer(buf)
	err = createFileFromBlobReference(ctx, pv, nil, &blobReference{
		Digest:       oci.Digest(manifestDigest),
		MediaType:    mci.MediaType,
		Name:         container_model.ManifestFilename,
		File:         &packages_model.PackageFileDescriptor{Blob: pb},
		ExpectedSize: pb.Size,
		IsLead:       true,
	})

	return pb, !exists, manifestDigest, err
}
