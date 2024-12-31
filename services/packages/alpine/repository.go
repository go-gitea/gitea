// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	alpine_model "code.gitea.io/gitea/models/packages/alpine"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	alpine_module "code.gitea.io/gitea/modules/packages/alpine"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
)

const (
	IndexFilename        = "APKINDEX"
	IndexArchiveFilename = IndexFilename + ".tar.gz"
)

// GetOrCreateRepositoryVersion gets or creates the internal repository package
// The Alpine registry needs multiple index files which are stored in this package.
func GetOrCreateRepositoryVersion(ctx context.Context, ownerID int64) (*packages_model.PackageVersion, error) {
	return packages_service.GetOrCreateInternalPackageVersion(ctx, ownerID, packages_model.TypeAlpine, alpine_module.RepositoryPackage, alpine_module.RepositoryVersion)
}

// GetOrCreateKeyPair gets or creates the RSA keys used to sign repository files
func GetOrCreateKeyPair(ctx context.Context, ownerID int64) (string, string, error) {
	priv, err := user_model.GetSetting(ctx, ownerID, alpine_module.SettingKeyPrivate)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	pub, err := user_model.GetSetting(ctx, ownerID, alpine_module.SettingKeyPublic)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	if priv == "" || pub == "" {
		priv, pub, err = util.GenerateKeyPair(4096)
		if err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, alpine_module.SettingKeyPrivate, priv); err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, alpine_module.SettingKeyPublic, pub); err != nil {
			return "", "", err
		}
	}

	return priv, pub, nil
}

// BuildAllRepositoryFiles (re)builds all repository files for every available branches, repositories and architectures
func BuildAllRepositoryFiles(ctx context.Context, ownerID int64) error {
	pv, err := GetOrCreateRepositoryVersion(ctx, ownerID)
	if err != nil {
		return err
	}

	// 1. Delete all existing repository files
	pfs, err := packages_model.GetFilesByVersionID(ctx, pv.ID)
	if err != nil {
		return err
	}

	for _, pf := range pfs {
		if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
			return err
		}
	}

	// 2. (Re)Build repository files for existing packages
	branches, err := alpine_model.GetBranches(ctx, ownerID)
	if err != nil {
		return err
	}
	for _, branch := range branches {
		repositories, err := alpine_model.GetRepositories(ctx, ownerID, branch)
		if err != nil {
			return err
		}
		for _, repository := range repositories {
			architectures, err := alpine_model.GetArchitectures(ctx, ownerID, repository)
			if err != nil {
				return err
			}
			for _, architecture := range architectures {
				if err := buildPackagesIndex(ctx, ownerID, pv, branch, repository, architecture); err != nil {
					return fmt.Errorf("failed to build repository files [%s/%s/%s]: %w", branch, repository, architecture, err)
				}
			}
		}
	}

	return nil
}

// BuildSpecificRepositoryFiles builds index files for the repository
func BuildSpecificRepositoryFiles(ctx context.Context, ownerID int64, branch, repository, architecture string) error {
	pv, err := GetOrCreateRepositoryVersion(ctx, ownerID)
	if err != nil {
		return err
	}

	architectures := container.SetOf(architecture)
	if architecture == alpine_module.NoArch {
		// Update all other architectures too when updating the noarch index
		additionalArchitectures, err := alpine_model.GetArchitectures(ctx, ownerID, repository)
		if err != nil {
			return err
		}
		architectures.AddMultiple(additionalArchitectures...)
	}

	for architecture := range architectures {
		if err := buildPackagesIndex(ctx, ownerID, pv, branch, repository, architecture); err != nil {
			return err
		}
	}
	return nil
}

type packageData struct {
	Package         *packages_model.Package
	Version         *packages_model.PackageVersion
	Blob            *packages_model.PackageBlob
	VersionMetadata *alpine_module.VersionMetadata
	FileMetadata    *alpine_module.FileMetadata
}

type packageCache = map[*packages_model.PackageFile]*packageData

func searchPackageFiles(ctx context.Context, ownerID int64, branch, repository, architecture string) ([]*packages_model.PackageFile, error) {
	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:     ownerID,
		PackageType: packages_model.TypeAlpine,
		Query:       "%.apk",
		Properties: map[string]string{
			alpine_module.PropertyBranch:       branch,
			alpine_module.PropertyRepository:   repository,
			alpine_module.PropertyArchitecture: architecture,
		},
	})
	if err != nil {
		return nil, err
	}
	return pfs, nil
}

// https://wiki.alpinelinux.org/wiki/Apk_spec#APKINDEX_Format
func buildPackagesIndex(ctx context.Context, ownerID int64, repoVersion *packages_model.PackageVersion, branch, repository, architecture string) error {
	pfs, err := searchPackageFiles(ctx, ownerID, branch, repository, architecture)
	if err != nil {
		return err
	}
	if architecture != alpine_module.NoArch {
		// Add all noarch packages too
		noarchFiles, err := searchPackageFiles(ctx, ownerID, branch, repository, alpine_module.NoArch)
		if err != nil {
			return err
		}
		pfs = append(pfs, noarchFiles...)
	}

	// Delete the package indices if there are no packages
	if len(pfs) == 0 {
		pf, err := packages_model.GetFileForVersionByName(ctx, repoVersion.ID, IndexArchiveFilename, fmt.Sprintf("%s|%s|%s", branch, repository, architecture))
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		} else if pf == nil {
			return nil
		}

		return packages_service.DeletePackageFile(ctx, pf)
	}

	// Cache data needed for all repository files
	cache := make(packageCache)
	for _, pf := range pfs {
		pv, err := packages_model.GetVersionByID(ctx, pf.VersionID)
		if err != nil {
			return err
		}
		p, err := packages_model.GetPackageByID(ctx, pv.PackageID)
		if err != nil {
			return err
		}
		pb, err := packages_model.GetBlobByID(ctx, pf.BlobID)
		if err != nil {
			return err
		}
		pps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeFile, pf.ID, alpine_module.PropertyMetadata)
		if err != nil {
			return err
		}

		pd := &packageData{
			Package: p,
			Version: pv,
			Blob:    pb,
		}

		if err := json.Unmarshal([]byte(pv.MetadataJSON), &pd.VersionMetadata); err != nil {
			return err
		}
		if len(pps) > 0 {
			if err := json.Unmarshal([]byte(pps[0].Value), &pd.FileMetadata); err != nil {
				return err
			}
		}

		cache[pf] = pd
	}

	var buf bytes.Buffer
	for _, pf := range pfs {
		pd := cache[pf]

		fmt.Fprintf(&buf, "C:%s\n", pd.FileMetadata.Checksum)
		fmt.Fprintf(&buf, "P:%s\n", pd.Package.Name)
		fmt.Fprintf(&buf, "V:%s\n", pd.Version.Version)
		fmt.Fprintf(&buf, "A:%s\n", architecture)
		if pd.VersionMetadata.Description != "" {
			fmt.Fprintf(&buf, "T:%s\n", pd.VersionMetadata.Description)
		}
		if pd.VersionMetadata.ProjectURL != "" {
			fmt.Fprintf(&buf, "U:%s\n", pd.VersionMetadata.ProjectURL)
		}
		if pd.VersionMetadata.License != "" {
			fmt.Fprintf(&buf, "L:%s\n", pd.VersionMetadata.License)
		}
		fmt.Fprintf(&buf, "S:%d\n", pd.Blob.Size)
		fmt.Fprintf(&buf, "I:%d\n", pd.FileMetadata.Size)
		fmt.Fprintf(&buf, "o:%s\n", pd.FileMetadata.Origin)
		fmt.Fprintf(&buf, "m:%s\n", pd.VersionMetadata.Maintainer)
		fmt.Fprintf(&buf, "t:%d\n", pd.FileMetadata.BuildDate)
		if pd.FileMetadata.CommitHash != "" {
			fmt.Fprintf(&buf, "c:%s\n", pd.FileMetadata.CommitHash)
		}
		if len(pd.FileMetadata.Dependencies) > 0 {
			fmt.Fprintf(&buf, "D:%s\n", strings.Join(pd.FileMetadata.Dependencies, " "))
		}
		if len(pd.FileMetadata.Provides) > 0 {
			fmt.Fprintf(&buf, "p:%s\n", strings.Join(pd.FileMetadata.Provides, " "))
		}
		if pd.FileMetadata.InstallIf != "" {
			fmt.Fprintf(&buf, "i:%s\n", pd.FileMetadata.InstallIf)
		}
		if pd.FileMetadata.ProviderPriority > 0 {
			fmt.Fprintf(&buf, "k:%d\n", pd.FileMetadata.ProviderPriority)
		}
		fmt.Fprint(&buf, "\n")
	}

	unsignedIndexContent, _ := packages_module.NewHashedBuffer()
	defer unsignedIndexContent.Close()

	h := sha1.New()

	if err := writeGzipStream(io.MultiWriter(unsignedIndexContent, h), IndexFilename, buf.Bytes(), true); err != nil {
		return err
	}

	priv, _, err := GetOrCreateKeyPair(ctx, ownerID)
	if err != nil {
		return err
	}

	privPem, _ := pem.Decode([]byte(priv))
	if privPem == nil {
		return fmt.Errorf("failed to decode private key pem")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(privPem.Bytes)
	if err != nil {
		return err
	}

	sign, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA1, h.Sum(nil))
	if err != nil {
		return err
	}

	owner, err := user_model.GetUserByID(ctx, ownerID)
	if err != nil {
		return err
	}

	fingerprint, err := util.CreatePublicKeyFingerprint(&privKey.PublicKey)
	if err != nil {
		return err
	}

	signedIndexContent, _ := packages_module.NewHashedBuffer()
	defer signedIndexContent.Close()

	if err := writeGzipStream(
		signedIndexContent,
		fmt.Sprintf(".SIGN.RSA.%s@%s.rsa.pub", owner.LowerName, hex.EncodeToString(fingerprint)),
		sign,
		false,
	); err != nil {
		return err
	}

	if _, err := io.Copy(signedIndexContent, unsignedIndexContent); err != nil {
		return err
	}

	_, err = packages_service.AddFileToPackageVersionInternal(
		ctx,
		repoVersion,
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     IndexArchiveFilename,
				CompositeKey: fmt.Sprintf("%s|%s|%s", branch, repository, architecture),
			},
			Creator:           user_model.NewGhostUser(),
			Data:              signedIndexContent,
			IsLead:            false,
			OverwriteExisting: true,
			Properties: map[string]string{
				alpine_module.PropertyBranch:       branch,
				alpine_module.PropertyRepository:   repository,
				alpine_module.PropertyArchitecture: architecture,
			},
		},
	)
	return err
}

func writeGzipStream(w io.Writer, filename string, content []byte, addTarEnd bool) error {
	zw := gzip.NewWriter(w)
	defer zw.Close()

	tw := tar.NewWriter(zw)
	if addTarEnd {
		defer tw.Close()
	}
	hdr := &tar.Header{
		Name: filename,
		Mode: 0o600,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	return nil
}
