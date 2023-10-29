// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	debian_model "code.gitea.io/gitea/models/packages/debian"
	user_model "code.gitea.io/gitea/models/user"
	packages_module "code.gitea.io/gitea/modules/packages"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
	"github.com/keybase/go-crypto/openpgp/clearsign"
	"github.com/keybase/go-crypto/openpgp/packet"
	"github.com/ulikunitz/xz"
)

// GetOrCreateRepositoryVersion gets or creates the internal repository package
// The Debian registry needs multiple index files which are stored in this package.
func GetOrCreateRepositoryVersion(ctx context.Context, ownerID int64) (*packages_model.PackageVersion, error) {
	return packages_service.GetOrCreateInternalPackageVersion(ctx, ownerID, packages_model.TypeDebian, debian_module.RepositoryPackage, debian_module.RepositoryVersion)
}

// GetOrCreateKeyPair gets or creates the PGP keys used to sign repository files
func GetOrCreateKeyPair(ctx context.Context, ownerID int64) (string, string, error) {
	priv, err := user_model.GetSetting(ctx, ownerID, debian_module.SettingKeyPrivate)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	pub, err := user_model.GetSetting(ctx, ownerID, debian_module.SettingKeyPublic)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	if priv == "" || pub == "" {
		priv, pub, err = generateKeypair()
		if err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, debian_module.SettingKeyPrivate, priv); err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, debian_module.SettingKeyPublic, pub); err != nil {
			return "", "", err
		}
	}

	return priv, pub, nil
}

func generateKeypair() (string, string, error) {
	e, err := openpgp.NewEntity(setting.AppName, "Debian Registry", "", nil)
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

// BuildAllRepositoryFiles (re)builds all repository files for every available distributions, components and architectures
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
		if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeFile, pf.ID); err != nil {
			return err
		}
		if err := packages_model.DeleteFileByID(ctx, pf.ID); err != nil {
			return err
		}
	}

	// 2. (Re)Build repository files for existing packages
	distributions, err := debian_model.GetDistributions(ctx, ownerID)
	if err != nil {
		return err
	}
	for _, distribution := range distributions {
		components, err := debian_model.GetComponents(ctx, ownerID, distribution)
		if err != nil {
			return err
		}
		architectures, err := debian_model.GetArchitectures(ctx, ownerID, distribution)
		if err != nil {
			return err
		}

		for _, component := range components {
			for _, architecture := range architectures {
				if err := buildRepositoryFiles(ctx, ownerID, pv, distribution, component, architecture); err != nil {
					return fmt.Errorf("failed to build repository files [%s/%s/%s]: %w", distribution, component, architecture, err)
				}
			}
		}
	}

	return nil
}

// BuildSpecificRepositoryFiles builds index files for the repository
func BuildSpecificRepositoryFiles(ctx context.Context, ownerID int64, distribution, component, architecture string) error {
	pv, err := GetOrCreateRepositoryVersion(ctx, ownerID)
	if err != nil {
		return err
	}

	return buildRepositoryFiles(ctx, ownerID, pv, distribution, component, architecture)
}

func buildRepositoryFiles(ctx context.Context, ownerID int64, repoVersion *packages_model.PackageVersion, distribution, component, architecture string) error {
	if err := buildPackagesIndices(ctx, ownerID, repoVersion, distribution, component, architecture); err != nil {
		return err
	}

	return buildReleaseFiles(ctx, ownerID, repoVersion, distribution)
}

// https://wiki.debian.org/DebianRepository/Format#A.22Packages.22_Indices
func buildPackagesIndices(ctx context.Context, ownerID int64, repoVersion *packages_model.PackageVersion, distribution, component, architecture string) error {
	opts := &debian_model.PackageSearchOptions{
		OwnerID:      ownerID,
		Distribution: distribution,
		Component:    component,
		Architecture: architecture,
	}

	// Delete the package indices if there are no packages
	if has, err := debian_model.ExistPackages(ctx, opts); err != nil {
		return err
	} else if !has {
		key := fmt.Sprintf("%s|%s|%s", distribution, component, architecture)
		for _, filename := range []string{"Packages", "Packages.gz", "Packages.xz"} {
			pf, err := packages_model.GetFileForVersionByName(ctx, repoVersion.ID, filename, key)
			if err != nil && !errors.Is(err, util.ErrNotExist) {
				return err
			}

			if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeFile, pf.ID); err != nil {
				return err
			}
			if err := packages_model.DeleteFileByID(ctx, pf.ID); err != nil {
				return err
			}
		}

		return nil
	}

	packagesContent, _ := packages_module.NewHashedBuffer()
	defer packagesContent.Close()

	packagesGzipContent, _ := packages_module.NewHashedBuffer()
	defer packagesGzipContent.Close()

	gzw := gzip.NewWriter(packagesGzipContent)

	packagesXzContent, _ := packages_module.NewHashedBuffer()
	defer packagesXzContent.Close()

	xzw, _ := xz.NewWriter(packagesXzContent)

	w := io.MultiWriter(packagesContent, gzw, xzw)

	addSeparator := false
	if err := debian_model.SearchPackages(ctx, opts, func(pfd *packages_model.PackageFileDescriptor) {
		if addSeparator {
			fmt.Fprintln(w)
		}
		addSeparator = true

		fmt.Fprintf(w, "%s\n", strings.TrimSpace(pfd.Properties.GetByName(debian_module.PropertyControl)))

		fmt.Fprintf(w, "Filename: pool/%s/%s/%s\n", distribution, component, pfd.File.Name)
		fmt.Fprintf(w, "Size: %d\n", pfd.Blob.Size)
		fmt.Fprintf(w, "MD5sum: %s\n", pfd.Blob.HashMD5)
		fmt.Fprintf(w, "SHA1: %s\n", pfd.Blob.HashSHA1)
		fmt.Fprintf(w, "SHA256: %s\n", pfd.Blob.HashSHA256)
		fmt.Fprintf(w, "SHA512: %s\n", pfd.Blob.HashSHA512)
	}); err != nil {
		return err
	}

	gzw.Close()
	xzw.Close()

	for _, file := range []struct {
		Name string
		Data packages_module.HashedSizeReader
	}{
		{"Packages", packagesContent},
		{"Packages.gz", packagesGzipContent},
		{"Packages.xz", packagesXzContent},
	} {
		_, err := packages_service.AddFileToPackageVersionInternal(
			ctx,
			repoVersion,
			&packages_service.PackageFileCreationInfo{
				PackageFileInfo: packages_service.PackageFileInfo{
					Filename:     file.Name,
					CompositeKey: fmt.Sprintf("%s|%s|%s", distribution, component, architecture),
				},
				Creator:           user_model.NewGhostUser(),
				Data:              file.Data,
				IsLead:            false,
				OverwriteExisting: true,
				Properties: map[string]string{
					debian_module.PropertyRepositoryIncludeInRelease: "",
					debian_module.PropertyDistribution:               distribution,
					debian_module.PropertyComponent:                  component,
					debian_module.PropertyArchitecture:               architecture,
				},
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// https://wiki.debian.org/DebianRepository/Format#A.22Release.22_files
func buildReleaseFiles(ctx context.Context, ownerID int64, repoVersion *packages_model.PackageVersion, distribution string) error {
	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		VersionID: repoVersion.ID,
		Properties: map[string]string{
			debian_module.PropertyRepositoryIncludeInRelease: "",
			debian_module.PropertyDistribution:               distribution,
		},
	})
	if err != nil {
		return err
	}

	// Delete the release files if there are no packages
	if len(pfs) == 0 {
		for _, filename := range []string{"Release", "Release.gpg", "InRelease"} {
			pf, err := packages_model.GetFileForVersionByName(ctx, repoVersion.ID, filename, distribution)
			if err != nil && !errors.Is(err, util.ErrNotExist) {
				return err
			}

			if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypeFile, pf.ID); err != nil {
				return err
			}
			if err := packages_model.DeleteFileByID(ctx, pf.ID); err != nil {
				return err
			}
		}

		return nil
	}

	components, err := debian_model.GetComponents(ctx, ownerID, distribution)
	if err != nil {
		return err
	}

	sort.Strings(components)

	architectures, err := debian_model.GetArchitectures(ctx, ownerID, distribution)
	if err != nil {
		return err
	}

	sort.Strings(architectures)

	priv, _, err := GetOrCreateKeyPair(ctx, ownerID)
	if err != nil {
		return err
	}

	block, err := armor.Decode(strings.NewReader(priv))
	if err != nil {
		return err
	}

	e, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		return err
	}

	inReleaseContent, _ := packages_module.NewHashedBuffer()
	defer inReleaseContent.Close()

	sw, err := clearsign.Encode(inReleaseContent, e.PrivateKey, nil)
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	w := io.MultiWriter(sw, &buf)

	fmt.Fprintf(w, "Origin: %s\n", setting.AppName)
	fmt.Fprintf(w, "Label: %s\n", setting.AppName)
	fmt.Fprintf(w, "Suite: %s\n", distribution)
	fmt.Fprintf(w, "Codename: %s\n", distribution)
	fmt.Fprintf(w, "Components: %s\n", strings.Join(components, " "))
	fmt.Fprintf(w, "Architectures: %s\n", strings.Join(architectures, " "))
	fmt.Fprintf(w, "Date: %s\n", time.Now().UTC().Format(time.RFC1123))
	fmt.Fprint(w, "Acquire-By-Hash: yes")

	pfds, err := packages_model.GetPackageFileDescriptors(ctx, pfs)
	if err != nil {
		return err
	}

	var md5, sha1, sha256, sha512 strings.Builder
	for _, pfd := range pfds {
		path := fmt.Sprintf("%s/binary-%s/%s", pfd.Properties.GetByName(debian_module.PropertyComponent), pfd.Properties.GetByName(debian_module.PropertyArchitecture), pfd.File.Name)
		fmt.Fprintf(&md5, " %s %d %s\n", pfd.Blob.HashMD5, pfd.Blob.Size, path)
		fmt.Fprintf(&sha1, " %s %d %s\n", pfd.Blob.HashSHA1, pfd.Blob.Size, path)
		fmt.Fprintf(&sha256, " %s %d %s\n", pfd.Blob.HashSHA256, pfd.Blob.Size, path)
		fmt.Fprintf(&sha512, " %s %d %s\n", pfd.Blob.HashSHA512, pfd.Blob.Size, path)
	}

	fmt.Fprintln(w, "MD5Sum:")
	fmt.Fprint(w, md5.String())
	fmt.Fprintln(w, "SHA1:")
	fmt.Fprint(w, sha1.String())
	fmt.Fprintln(w, "SHA256:")
	fmt.Fprint(w, sha256.String())
	fmt.Fprintln(w, "SHA512:")
	fmt.Fprint(w, sha512.String())

	sw.Close()

	releaseGpgContent, _ := packages_module.NewHashedBuffer()
	defer releaseGpgContent.Close()

	if err := openpgp.ArmoredDetachSign(releaseGpgContent, e, bytes.NewReader(buf.Bytes()), nil); err != nil {
		return err
	}

	releaseContent, _ := packages_module.CreateHashedBufferFromReader(&buf)
	defer releaseContent.Close()

	for _, file := range []struct {
		Name string
		Data packages_module.HashedSizeReader
	}{
		{"Release", releaseContent},
		{"Release.gpg", releaseGpgContent},
		{"InRelease", inReleaseContent},
	} {
		_, err = packages_service.AddFileToPackageVersionInternal(
			ctx,
			repoVersion,
			&packages_service.PackageFileCreationInfo{
				PackageFileInfo: packages_service.PackageFileInfo{
					Filename:     file.Name,
					CompositeKey: distribution,
				},
				Creator:           user_model.NewGhostUser(),
				Data:              file.Data,
				IsLead:            false,
				OverwriteExisting: true,
				Properties: map[string]string{
					debian_module.PropertyDistribution: distribution,
				},
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}
