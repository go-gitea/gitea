// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	arch_model "code.gitea.io/gitea/models/packages/arch"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const (
	IndexArchiveFilename = "packages.db"
)

func AquireRegistryLock(ctx context.Context, ownerID int64) (globallock.ReleaseFunc, error) {
	return globallock.Lock(ctx, fmt.Sprintf("packages_arch_%d", ownerID))
}

// GetOrCreateRepositoryVersion gets or creates the internal repository package
// The Arch registry needs multiple index files which are stored in this package.
func GetOrCreateRepositoryVersion(ctx context.Context, ownerID int64) (*packages_model.PackageVersion, error) {
	return packages_service.GetOrCreateInternalPackageVersion(ctx, ownerID, packages_model.TypeArch, arch_module.RepositoryPackage, arch_module.RepositoryVersion)
}

// GetOrCreateKeyPair gets or creates the PGP keys used to sign repository files
func GetOrCreateKeyPair(ctx context.Context, ownerID int64) (string, string, error) {
	priv, err := user_model.GetSetting(ctx, ownerID, arch_module.SettingKeyPrivate)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	pub, err := user_model.GetSetting(ctx, ownerID, arch_module.SettingKeyPublic)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	if priv == "" || pub == "" {
		priv, pub, err = generateKeypair()
		if err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, arch_module.SettingKeyPrivate, priv); err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, arch_module.SettingKeyPublic, pub); err != nil {
			return "", "", err
		}
	}

	return priv, pub, nil
}

func generateKeypair() (string, string, error) {
	e, err := openpgp.NewEntity("", "Arch Registry", "", nil)
	if err != nil {
		return "", "", err
	}

	var priv strings.Builder
	var pub strings.Builder

	w, err := armor.Encode(&priv, openpgp.PrivateKeyType, nil)
	if err != nil {
		return "", "", err
	}
	if err := e.SerializePrivate(w, nil); err != nil {
		return "", "", err
	}
	w.Close()

	w, err = armor.Encode(&pub, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", "", err
	}
	if err := e.Serialize(w); err != nil {
		return "", "", err
	}
	w.Close()

	return priv.String(), pub.String(), nil
}

func SignData(ctx context.Context, ownerID int64, r io.Reader) ([]byte, error) {
	priv, _, err := GetOrCreateKeyPair(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	block, err := armor.Decode(strings.NewReader(priv))
	if err != nil {
		return nil, err
	}

	e, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err := openpgp.DetachSign(buf, e, r, nil); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// BuildAllRepositoryFiles (re)builds all repository files for every available repositories and architectures
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
	repositories, err := arch_model.GetRepositories(ctx, ownerID)
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		architectures, err := arch_model.GetArchitectures(ctx, ownerID, repository)
		if err != nil {
			return err
		}
		for _, architecture := range architectures {
			if err := buildPackagesIndex(ctx, ownerID, pv, repository, architecture); err != nil {
				return fmt.Errorf("failed to build repository files [%s/%s]: %w", repository, architecture, err)
			}
		}
	}

	return nil
}

// BuildSpecificRepositoryFiles builds index files for the repository
func BuildSpecificRepositoryFiles(ctx context.Context, ownerID int64, repository, architecture string) error {
	pv, err := GetOrCreateRepositoryVersion(ctx, ownerID)
	if err != nil {
		return err
	}

	architectures := container.SetOf(architecture)
	if architecture == arch_module.AnyArch {
		// Update all other architectures too when updating the any index
		additionalArchitectures, err := arch_model.GetArchitectures(ctx, ownerID, repository)
		if err != nil {
			return err
		}
		architectures.AddMultiple(additionalArchitectures...)
	}

	for architecture := range architectures {
		if err := buildPackagesIndex(ctx, ownerID, pv, repository, architecture); err != nil {
			return err
		}
	}
	return nil
}

func searchPackageFiles(ctx context.Context, ownerID int64, repository, architecture string) ([]*packages_model.PackageFile, error) {
	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:     ownerID,
		PackageType: packages_model.TypeArch,
		Query:       "%.pkg.tar.%",
		Properties: map[string]string{
			arch_module.PropertyRepository:   repository,
			arch_module.PropertyArchitecture: architecture,
		},
	})
	if err != nil {
		return nil, err
	}
	return pfs, nil
}

func buildPackagesIndex(ctx context.Context, ownerID int64, repoVersion *packages_model.PackageVersion, repository, architecture string) error {
	pfs, err := searchPackageFiles(ctx, ownerID, repository, architecture)
	if err != nil {
		return err
	}
	if architecture != arch_module.AnyArch {
		// Add all any packages too
		anyarchFiles, err := searchPackageFiles(ctx, ownerID, repository, arch_module.AnyArch)
		if err != nil {
			return err
		}
		pfs = append(pfs, anyarchFiles...)
	}

	// Delete the package indices if there are no packages
	if len(pfs) == 0 {
		pf, err := packages_model.GetFileForVersionByName(ctx, repoVersion.ID, IndexArchiveFilename, fmt.Sprintf("%s|%s", repository, architecture))
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		} else if pf == nil {
			return nil
		}

		return packages_service.DeletePackageFile(ctx, pf)
	}

	vpfs := make(map[int64]*entryOptions)
	for _, pf := range pfs {
		current := &entryOptions{
			File: pf,
		}
		current.Version, err = packages_model.GetVersionByID(ctx, pf.VersionID)
		if err != nil {
			return err
		}

		// here we compare the versions but not using SearchLatestVersions because we shouldn't allow "downgrading" to a older version by "latest" one.
		// https://wiki.archlinux.org/title/Downgrading_packages : randomly downgrading can mess up dependencies:
		// If a downgrade involves a soname change, all dependencies may need downgrading or rebuilding too.
		if old, ok := vpfs[current.Version.PackageID]; ok {
			if compareVersions(old.Version.Version, current.Version.Version) == -1 {
				vpfs[current.Version.PackageID] = current
			}
		} else {
			vpfs[current.Version.PackageID] = current
		}
	}

	indexContent, _ := packages_module.NewHashedBuffer()
	defer indexContent.Close()

	gw := gzip.NewWriter(indexContent)
	tw := tar.NewWriter(gw)

	cache := make(map[int64]*packages_model.Package)

	for _, opts := range vpfs {
		if err := json.Unmarshal([]byte(opts.Version.MetadataJSON), &opts.VersionMetadata); err != nil {
			return err
		}
		opts.Package = cache[opts.Version.PackageID]
		if opts.Package == nil {
			opts.Package, err = packages_model.GetPackageByID(ctx, opts.Version.PackageID)
			if err != nil {
				return err
			}
			cache[opts.Package.ID] = opts.Package
		}
		opts.Blob, err = packages_model.GetBlobByID(ctx, opts.File.BlobID)
		if err != nil {
			return err
		}

		sig, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeFile, opts.File.ID, arch_module.PropertySignature)
		if err != nil {
			return err
		}
		if len(sig) == 0 {
			return util.ErrNotExist
		}
		opts.Signature = sig[0].Value

		meta, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeFile, opts.File.ID, arch_module.PropertyMetadata)
		if err != nil {
			return err
		}
		if len(meta) == 0 {
			return util.ErrNotExist
		}
		if err := json.Unmarshal([]byte(meta[0].Value), &opts.FileMetadata); err != nil {
			return err
		}

		if err := writeFiles(tw, opts); err != nil {
			return err
		}
		if err := writeDescription(tw, opts); err != nil {
			return err
		}
	}

	tw.Close()
	gw.Close()

	signature, err := SignData(ctx, ownerID, indexContent)
	if err != nil {
		return err
	}

	if _, err := indexContent.Seek(0, io.SeekStart); err != nil {
		return err
	}

	_, err = packages_service.AddFileToPackageVersionInternal(
		ctx,
		repoVersion,
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     IndexArchiveFilename,
				CompositeKey: fmt.Sprintf("%s|%s", repository, architecture),
			},
			Creator:           user_model.NewGhostUser(),
			Data:              indexContent,
			IsLead:            false,
			OverwriteExisting: true,
			Properties: map[string]string{
				arch_module.PropertyRepository:   repository,
				arch_module.PropertyArchitecture: architecture,
				arch_module.PropertySignature:    base64.StdEncoding.EncodeToString(signature),
			},
		},
	)
	return err
}

type entryOptions struct {
	Package         *packages_model.Package
	Version         *packages_model.PackageVersion
	VersionMetadata *arch_module.VersionMetadata
	File            *packages_model.PackageFile
	FileMetadata    *arch_module.FileMetadata
	Blob            *packages_model.PackageBlob
	Signature       string
}

type keyValue struct {
	Key   string
	Value string
}

func writeFiles(tw *tar.Writer, opts *entryOptions) error {
	return writeFields(tw, fmt.Sprintf("%s-%s/files", opts.Package.Name, opts.Version.Version), []keyValue{
		{"FILES", strings.Join(opts.FileMetadata.Files, "\n")},
	})
}

// https://gitlab.archlinux.org/pacman/pacman/-/blob/master/lib/libalpm/be_sync.c#L562
func writeDescription(tw *tar.Writer, opts *entryOptions) error {
	return writeFields(tw, fmt.Sprintf("%s-%s/desc", opts.Package.Name, opts.Version.Version), []keyValue{
		{"FILENAME", opts.File.Name},
		{"MD5SUM", opts.Blob.HashMD5},
		{"SHA256SUM", opts.Blob.HashSHA256},
		{"PGPSIG", opts.Signature},
		{"CSIZE", fmt.Sprintf("%d", opts.Blob.Size)},
		{"ISIZE", fmt.Sprintf("%d", opts.FileMetadata.InstalledSize)},
		{"NAME", opts.Package.Name},
		{"BASE", opts.FileMetadata.Base},
		{"ARCH", opts.FileMetadata.Architecture},
		{"VERSION", opts.Version.Version},
		{"DESC", opts.VersionMetadata.Description},
		{"URL", opts.VersionMetadata.ProjectURL},
		{"LICENSE", strings.Join(opts.VersionMetadata.Licenses, "\n")},
		{"GROUPS", strings.Join(opts.FileMetadata.Groups, "\n")},
		{"BUILDDATE", fmt.Sprintf("%d", opts.FileMetadata.BuildDate)},
		{"PACKAGER", opts.FileMetadata.Packager},
		{"PROVIDES", strings.Join(opts.FileMetadata.Provides, "\n")},
		{"REPLACES", strings.Join(opts.FileMetadata.Replaces, "\n")},
		{"CONFLICTS", strings.Join(opts.FileMetadata.Conflicts, "\n")},
		{"DEPENDS", strings.Join(opts.FileMetadata.Depends, "\n")},
		{"OPTDEPENDS", strings.Join(opts.FileMetadata.OptDepends, "\n")},
		{"MAKEDEPENDS", strings.Join(opts.FileMetadata.MakeDepends, "\n")},
		{"CHECKDEPENDS", strings.Join(opts.FileMetadata.CheckDepends, "\n")},
	})
}

func writeFields(tw *tar.Writer, filename string, fields []keyValue) error {
	buf := &bytes.Buffer{}
	for _, kv := range fields {
		if kv.Value == "" {
			continue
		}
		fmt.Fprintf(buf, "%%%s%%\n%s\n\n", kv.Key, kv.Value)
	}

	if err := tw.WriteHeader(&tar.Header{
		Name: filename,
		Size: int64(buf.Len()),
		Mode: int64(os.ModePerm),
	}); err != nil {
		return err
	}

	_, err := io.Copy(tw, buf)
	return err
}
