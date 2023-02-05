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
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	debian_model "code.gitea.io/gitea/models/packages/debian"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
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
func GetOrCreateRepositoryVersion(owner *user_model.User) (*packages_model.PackageVersion, error) {
	var repositoryVersion *packages_model.PackageVersion

	return repositoryVersion, db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		p := &packages_model.Package{
			OwnerID:    owner.ID,
			Type:       packages_model.TypeDebian,
			Name:       debian_module.RepositoryPackage,
			LowerName:  debian_module.RepositoryPackage,
			IsInternal: true,
		}
		var err error
		if p, err = packages_model.TryInsertPackage(ctx, p); err != nil {
			if err != packages_model.ErrDuplicatePackage {
				log.Error("Error inserting package: %v", err)
				return err
			}
		}

		created := true
		pv := &packages_model.PackageVersion{
			PackageID:    p.ID,
			CreatorID:    owner.ID,
			Version:      debian_module.RepositoryVersion,
			LowerVersion: debian_module.RepositoryVersion,
			IsInternal:   true,
			MetadataJSON: "null",
		}
		if pv, err = packages_model.GetOrInsertVersion(ctx, pv); err != nil {
			if err == packages_model.ErrDuplicatePackageVersion {
				created = false
			} else {
				log.Error("Error inserting package version: %v", err)
				return err
			}
		}

		if created {
			priv, pub, err := generateKeypair()
			if err != nil {
				return err
			}

			_, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, debian_module.PropertyKeyPrivate, priv)
			if err != nil {
				return err
			}

			_, err = packages_model.InsertProperty(ctx, packages_model.PropertyTypeVersion, pv.ID, debian_module.PropertyKeyPublic, pub)
			if err != nil {
				return err
			}
		}

		repositoryVersion = pv

		return nil
	})
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

// GenerateRepositoryFiles generates index files for the repository
func GenerateRepositoryFiles(ctx context.Context, owner *user_model.User, distribution, component, architecture string) error {
	pv, err := GetOrCreateRepositoryVersion(owner)
	if err != nil {
		return err
	}

	if err := buildPackagesIndices(ctx, owner, pv, distribution, component, architecture); err != nil {
		return err
	}

	return buildReleaseFiles(ctx, owner, pv, distribution)
}

// https://wiki.debian.org/DebianRepository/Format#A.22Packages.22_Indices
func buildPackagesIndices(ctx context.Context, owner *user_model.User, repoVersion *packages_model.PackageVersion, distribution, component, architecture string) error {
	pfds, err := debian_model.SearchLatestPackages(ctx, &debian_model.PackageSearchOptions{
		OwnerID:      owner.ID,
		Distribution: distribution,
		Component:    component,
		Architecture: architecture,
	})
	if err != nil {
		return err
	}

	// Delete the package indices if there are no packages
	if len(pfds) == 0 {
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

	packagesGzipContent, _ := packages_module.NewHashedBuffer()
	gzw := gzip.NewWriter(packagesGzipContent)

	packagesXzContent, _ := packages_module.NewHashedBuffer()
	xzw, _ := xz.NewWriter(packagesXzContent)

	w := io.MultiWriter(packagesContent, gzw, xzw)

	addSeperator := false
	for _, pfd := range pfds {
		if addSeperator {
			fmt.Fprintln(w)
		}
		addSeperator = true

		fmt.Fprint(w, pfd.Properties.GetByName(debian_module.PropertyControl))

		fmt.Fprintf(w, "Filename: pool/%s/%s/%s\n", distribution, component, pfd.File.Name)
		fmt.Fprintf(w, "Size: %d\n", pfd.Blob.Size)
		fmt.Fprintf(w, "MD5sum: %s\n", pfd.Blob.HashMD5)
		fmt.Fprintf(w, "SHA1: %s\n", pfd.Blob.HashSHA1)
		fmt.Fprintf(w, "SHA256: %s\n", pfd.Blob.HashSHA256)
		fmt.Fprintf(w, "SHA512: %s\n", pfd.Blob.HashSHA512)
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
		_, err = packages_service.AddFileToPackageVersionInternal(
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
func buildReleaseFiles(ctx context.Context, owner *user_model.User, repoVersion *packages_model.PackageVersion, distribution string) error {
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

	components, err := debian_model.GetComponents(ctx, owner.ID, distribution)
	if err != nil {
		return err
	}

	architectures, err := debian_model.GetArchitectures(ctx, owner.ID, distribution)
	if err != nil {
		return err
	}

	pps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeVersion, repoVersion.ID, debian_module.PropertyKeyPrivate)
	if err != nil {
		return err
	}
	if len(pps) != 1 {
		panic("should have one private key in repository")
	}

	block, err := armor.Decode(strings.NewReader(pps[0].Value))
	if err != nil {
		return err
	}

	e, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		return err
	}

	inReleaseContent, _ := packages_module.NewHashedBuffer()
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
	if err := openpgp.ArmoredDetachSign(releaseGpgContent, e, bytes.NewReader(buf.Bytes()), nil); err != nil {
		return err
	}

	releaseContent, _ := packages_module.CreateHashedBufferFromReader(&buf)

	for _, file := range []struct {
		Name string
		Data packages_module.HashedSizeReader
	}{
		{"Release", releaseContent},
		{"Release.gpg", releaseGpgContent},
		{"InRelease", inReleaseContent},
	} {
		_, err = packages_service.AddFileToPackageVersionInternal(
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
