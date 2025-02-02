// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	packages_model "code.gitea.io/gitea/models/packages"
	rpm_model "code.gitea.io/gitea/models/packages/rpm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	rpm_module "code.gitea.io/gitea/modules/packages/rpm"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// GetOrCreateRepositoryVersion gets or creates the internal repository package
// The RPM registry needs multiple metadata files which are stored in this package.
func GetOrCreateRepositoryVersion(ctx context.Context, ownerID int64) (*packages_model.PackageVersion, error) {
	return packages_service.GetOrCreateInternalPackageVersion(ctx, ownerID, packages_model.TypeRpm, rpm_module.RepositoryPackage, rpm_module.RepositoryVersion)
}

// GetOrCreateKeyPair gets or creates the PGP keys used to sign repository metadata files
func GetOrCreateKeyPair(ctx context.Context, ownerID int64) (string, string, error) {
	priv, err := user_model.GetSetting(ctx, ownerID, rpm_module.SettingKeyPrivate)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	pub, err := user_model.GetSetting(ctx, ownerID, rpm_module.SettingKeyPublic)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	if priv == "" || pub == "" {
		priv, pub, err = generateKeypair()
		if err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, rpm_module.SettingKeyPrivate, priv); err != nil {
			return "", "", err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, rpm_module.SettingKeyPublic, pub); err != nil {
			return "", "", err
		}
	}

	return priv, pub, nil
}

func generateKeypair() (string, string, error) {
	e, err := openpgp.NewEntity("", "RPM Registry", "", nil)
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

// BuildAllRepositoryFiles (re)builds all repository files for every available group
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
	groups, err := rpm_model.GetGroups(ctx, ownerID)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if err := BuildSpecificRepositoryFiles(ctx, ownerID, group); err != nil {
			return fmt.Errorf("failed to build repository files [%s]: %w", group, err)
		}
	}

	return nil
}

type repoChecksum struct {
	Value string `xml:",chardata"`
	Type  string `xml:"type,attr"`
}

type repoLocation struct {
	Href string `xml:"href,attr"`
}

type repoData struct {
	Type         string       `xml:"type,attr"`
	Checksum     repoChecksum `xml:"checksum"`
	OpenChecksum repoChecksum `xml:"open-checksum"`
	Location     repoLocation `xml:"location"`
	Timestamp    int64        `xml:"timestamp"`
	Size         int64        `xml:"size"`
	OpenSize     int64        `xml:"open-size"`
}

type packageData struct {
	Package         *packages_model.Package
	Version         *packages_model.PackageVersion
	Blob            *packages_model.PackageBlob
	VersionMetadata *rpm_module.VersionMetadata
	FileMetadata    *rpm_module.FileMetadata
}

type packageCache = map[*packages_model.PackageFile]*packageData

// BuildSpecificRepositoryFiles builds metadata files for the repository
func BuildSpecificRepositoryFiles(ctx context.Context, ownerID int64, group string) error {
	pv, err := GetOrCreateRepositoryVersion(ctx, ownerID)
	if err != nil {
		return err
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		OwnerID:      ownerID,
		PackageType:  packages_model.TypeRpm,
		Query:        "%.rpm",
		CompositeKey: group,
	})
	if err != nil {
		return err
	}

	// Delete the repository files if there are no packages
	if len(pfs) == 0 {
		pfs, err := packages_model.GetFilesByVersionID(ctx, pv.ID)
		if err != nil {
			return err
		}
		for _, pf := range pfs {
			if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
				return err
			}
		}

		return nil
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
		pps, err := packages_model.GetPropertiesByName(ctx, packages_model.PropertyTypeFile, pf.ID, rpm_module.PropertyMetadata)
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

	primary, err := buildPrimary(ctx, pv, pfs, cache, group)
	if err != nil {
		return err
	}
	filelists, err := buildFilelists(ctx, pv, pfs, cache, group)
	if err != nil {
		return err
	}
	other, err := buildOther(ctx, pv, pfs, cache, group)
	if err != nil {
		return err
	}

	return buildRepomd(
		ctx,
		pv,
		ownerID,
		[]*repoData{
			primary,
			filelists,
			other,
		},
		group,
	)
}

// https://docs.pulpproject.org/en/2.19/plugins/pulp_rpm/tech-reference/rpm.html#repomd-xml
func buildRepomd(ctx context.Context, pv *packages_model.PackageVersion, ownerID int64, data []*repoData, group string) error {
	type Repomd struct {
		XMLName  xml.Name    `xml:"repomd"`
		Xmlns    string      `xml:"xmlns,attr"`
		XmlnsRpm string      `xml:"xmlns:rpm,attr"`
		Data     []*repoData `xml:"data"`
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(&Repomd{
		Xmlns:    "http://linux.duke.edu/metadata/repo",
		XmlnsRpm: "http://linux.duke.edu/metadata/rpm",
		Data:     data,
	}); err != nil {
		return err
	}

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

	repomdAscContent, _ := packages_module.NewHashedBuffer()
	defer repomdAscContent.Close()

	if err := openpgp.ArmoredDetachSign(repomdAscContent, e, bytes.NewReader(buf.Bytes()), nil); err != nil {
		return err
	}

	repomdContent, _ := packages_module.CreateHashedBufferFromReader(&buf)
	defer repomdContent.Close()

	for _, file := range []struct {
		Name string
		Data packages_module.HashedSizeReader
	}{
		{"repomd.xml", repomdContent},
		{"repomd.xml.asc", repomdAscContent},
	} {
		_, err = packages_service.AddFileToPackageVersionInternal(
			ctx,
			pv,
			&packages_service.PackageFileCreationInfo{
				PackageFileInfo: packages_service.PackageFileInfo{
					Filename:     file.Name,
					CompositeKey: group,
				},
				Creator:           user_model.NewGhostUser(),
				Data:              file.Data,
				IsLead:            false,
				OverwriteExisting: true,
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// https://docs.pulpproject.org/en/2.19/plugins/pulp_rpm/tech-reference/rpm.html#primary-xml
func buildPrimary(ctx context.Context, pv *packages_model.PackageVersion, pfs []*packages_model.PackageFile, c packageCache, group string) (*repoData, error) {
	type Version struct {
		Epoch   string `xml:"epoch,attr"`
		Version string `xml:"ver,attr"`
		Release string `xml:"rel,attr"`
	}

	type Checksum struct {
		Checksum string `xml:",chardata"`
		Type     string `xml:"type,attr"`
		Pkgid    string `xml:"pkgid,attr"`
	}

	type Times struct {
		File  uint64 `xml:"file,attr"`
		Build uint64 `xml:"build,attr"`
	}

	type Sizes struct {
		Package   int64  `xml:"package,attr"`
		Installed uint64 `xml:"installed,attr"`
		Archive   uint64 `xml:"archive,attr"`
	}

	type Location struct {
		Href string `xml:"href,attr"`
	}

	type EntryList struct {
		Entries []*rpm_module.Entry `xml:"rpm:entry"`
	}

	type Format struct {
		License   string             `xml:"rpm:license"`
		Vendor    string             `xml:"rpm:vendor"`
		Group     string             `xml:"rpm:group"`
		Buildhost string             `xml:"rpm:buildhost"`
		Sourcerpm string             `xml:"rpm:sourcerpm"`
		Provides  EntryList          `xml:"rpm:provides"`
		Requires  EntryList          `xml:"rpm:requires"`
		Conflicts EntryList          `xml:"rpm:conflicts"`
		Obsoletes EntryList          `xml:"rpm:obsoletes"`
		Files     []*rpm_module.File `xml:"file"`
	}

	type Package struct {
		XMLName      xml.Name `xml:"package"`
		Type         string   `xml:"type,attr"`
		Name         string   `xml:"name"`
		Architecture string   `xml:"arch"`
		Version      Version  `xml:"version"`
		Checksum     Checksum `xml:"checksum"`
		Summary      string   `xml:"summary"`
		Description  string   `xml:"description"`
		Packager     string   `xml:"packager"`
		URL          string   `xml:"url"`
		Time         Times    `xml:"time"`
		Size         Sizes    `xml:"size"`
		Location     Location `xml:"location"`
		Format       Format   `xml:"format"`
	}

	type Metadata struct {
		XMLName      xml.Name   `xml:"metadata"`
		Xmlns        string     `xml:"xmlns,attr"`
		XmlnsRpm     string     `xml:"xmlns:rpm,attr"`
		PackageCount int        `xml:"packages,attr"`
		Packages     []*Package `xml:"package"`
	}

	packages := make([]*Package, 0, len(pfs))
	for _, pf := range pfs {
		pd := c[pf]

		files := make([]*rpm_module.File, 0, 3)
		for _, f := range pd.FileMetadata.Files {
			if f.IsExecutable {
				files = append(files, f)
			}
		}
		packageVersion := fmt.Sprintf("%s-%s", pd.FileMetadata.Version, pd.FileMetadata.Release)
		packages = append(packages, &Package{
			Type:         "rpm",
			Name:         pd.Package.Name,
			Architecture: pd.FileMetadata.Architecture,
			Version: Version{
				Epoch:   pd.FileMetadata.Epoch,
				Version: pd.FileMetadata.Version,
				Release: pd.FileMetadata.Release,
			},
			Checksum: Checksum{
				Type:     "sha256",
				Checksum: pd.Blob.HashSHA256,
				Pkgid:    "YES",
			},
			Summary:     pd.VersionMetadata.Summary,
			Description: pd.VersionMetadata.Description,
			Packager:    pd.FileMetadata.Packager,
			URL:         pd.VersionMetadata.ProjectURL,
			Time: Times{
				File:  pd.FileMetadata.FileTime,
				Build: pd.FileMetadata.BuildTime,
			},
			Size: Sizes{
				Package:   pd.Blob.Size,
				Installed: pd.FileMetadata.InstalledSize,
				Archive:   pd.FileMetadata.ArchiveSize,
			},
			Location: Location{
				Href: fmt.Sprintf("package/%s/%s/%s/%s-%s.%s.rpm", pd.Package.Name, packageVersion, pd.FileMetadata.Architecture, pd.Package.Name, packageVersion, pd.FileMetadata.Architecture),
			},
			Format: Format{
				License:   pd.VersionMetadata.License,
				Vendor:    pd.FileMetadata.Vendor,
				Group:     pd.FileMetadata.Group,
				Buildhost: pd.FileMetadata.BuildHost,
				Sourcerpm: pd.FileMetadata.SourceRpm,
				Provides: EntryList{
					Entries: pd.FileMetadata.Provides,
				},
				Requires: EntryList{
					Entries: pd.FileMetadata.Requires,
				},
				Conflicts: EntryList{
					Entries: pd.FileMetadata.Conflicts,
				},
				Obsoletes: EntryList{
					Entries: pd.FileMetadata.Obsoletes,
				},
				Files: files,
			},
		})
	}

	return addDataAsFileToRepo(ctx, pv, "primary", &Metadata{
		Xmlns:        "http://linux.duke.edu/metadata/common",
		XmlnsRpm:     "http://linux.duke.edu/metadata/rpm",
		PackageCount: len(pfs),
		Packages:     packages,
	}, group)
}

// https://docs.pulpproject.org/en/2.19/plugins/pulp_rpm/tech-reference/rpm.html#filelists-xml
func buildFilelists(ctx context.Context, pv *packages_model.PackageVersion, pfs []*packages_model.PackageFile, c packageCache, group string) (*repoData, error) { //nolint:dupl
	type Version struct {
		Epoch   string `xml:"epoch,attr"`
		Version string `xml:"ver,attr"`
		Release string `xml:"rel,attr"`
	}

	type Package struct {
		Pkgid        string             `xml:"pkgid,attr"`
		Name         string             `xml:"name,attr"`
		Architecture string             `xml:"arch,attr"`
		Version      Version            `xml:"version"`
		Files        []*rpm_module.File `xml:"file"`
	}

	type Filelists struct {
		XMLName      xml.Name   `xml:"filelists"`
		Xmlns        string     `xml:"xmlns,attr"`
		PackageCount int        `xml:"packages,attr"`
		Packages     []*Package `xml:"package"`
	}

	packages := make([]*Package, 0, len(pfs))
	for _, pf := range pfs {
		pd := c[pf]

		packages = append(packages, &Package{
			Pkgid:        pd.Blob.HashSHA256,
			Name:         pd.Package.Name,
			Architecture: pd.FileMetadata.Architecture,
			Version: Version{
				Epoch:   pd.FileMetadata.Epoch,
				Version: pd.FileMetadata.Version,
				Release: pd.FileMetadata.Release,
			},
			Files: pd.FileMetadata.Files,
		})
	}

	return addDataAsFileToRepo(ctx, pv, "filelists", &Filelists{
		Xmlns:        "http://linux.duke.edu/metadata/other",
		PackageCount: len(pfs),
		Packages:     packages,
	}, group)
}

// https://docs.pulpproject.org/en/2.19/plugins/pulp_rpm/tech-reference/rpm.html#other-xml
func buildOther(ctx context.Context, pv *packages_model.PackageVersion, pfs []*packages_model.PackageFile, c packageCache, group string) (*repoData, error) { //nolint:dupl
	type Version struct {
		Epoch   string `xml:"epoch,attr"`
		Version string `xml:"ver,attr"`
		Release string `xml:"rel,attr"`
	}

	type Package struct {
		Pkgid        string                  `xml:"pkgid,attr"`
		Name         string                  `xml:"name,attr"`
		Architecture string                  `xml:"arch,attr"`
		Version      Version                 `xml:"version"`
		Changelogs   []*rpm_module.Changelog `xml:"changelog"`
	}

	type Otherdata struct {
		XMLName      xml.Name   `xml:"otherdata"`
		Xmlns        string     `xml:"xmlns,attr"`
		PackageCount int        `xml:"packages,attr"`
		Packages     []*Package `xml:"package"`
	}

	packages := make([]*Package, 0, len(pfs))
	for _, pf := range pfs {
		pd := c[pf]

		packages = append(packages, &Package{
			Pkgid:        pd.Blob.HashSHA256,
			Name:         pd.Package.Name,
			Architecture: pd.FileMetadata.Architecture,
			Version: Version{
				Epoch:   pd.FileMetadata.Epoch,
				Version: pd.FileMetadata.Version,
				Release: pd.FileMetadata.Release,
			},
			Changelogs: pd.FileMetadata.Changelogs,
		})
	}

	return addDataAsFileToRepo(ctx, pv, "other", &Otherdata{
		Xmlns:        "http://linux.duke.edu/metadata/other",
		PackageCount: len(pfs),
		Packages:     packages,
	}, group)
}

// writtenCounter counts all written bytes
type writtenCounter struct {
	written int64
}

func (wc *writtenCounter) Write(buf []byte) (int, error) {
	n := len(buf)

	wc.written += int64(n)

	return n, nil
}

func (wc *writtenCounter) Written() int64 {
	return wc.written
}

func addDataAsFileToRepo(ctx context.Context, pv *packages_model.PackageVersion, filetype string, obj any, group string) (*repoData, error) {
	content, _ := packages_module.NewHashedBuffer()
	defer content.Close()

	gzw := gzip.NewWriter(content)
	wc := &writtenCounter{}
	h := sha256.New()

	w := io.MultiWriter(gzw, wc, h)
	_, _ = w.Write([]byte(xml.Header))

	if err := xml.NewEncoder(w).Encode(obj); err != nil {
		return nil, err
	}

	if err := gzw.Close(); err != nil {
		return nil, err
	}

	filename := filetype + ".xml.gz"

	_, err := packages_service.AddFileToPackageVersionInternal(
		ctx,
		pv,
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename:     filename,
				CompositeKey: group,
			},
			Creator:           user_model.NewGhostUser(),
			Data:              content,
			IsLead:            false,
			OverwriteExisting: true,
		},
	)
	if err != nil {
		return nil, err
	}

	_, _, hashSHA256, _ := content.Sums()

	return &repoData{
		Type: filetype,
		Checksum: repoChecksum{
			Type:  "sha256",
			Value: hex.EncodeToString(hashSHA256),
		},
		OpenChecksum: repoChecksum{
			Type:  "sha256",
			Value: hex.EncodeToString(h.Sum(nil)),
		},
		Location: repoLocation{
			Href: "repodata/" + filename,
		},
		Timestamp: time.Now().Unix(),
		Size:      content.Size(),
		OpenSize:  wc.Written(),
	}, nil
}
