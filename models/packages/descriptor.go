// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"codenest.dev/models/db"
	repo_model "codenest.dev/models/repo"
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/cache"
	"codenest.dev/modules/json"
	"codenest.dev/modules/packages/alpine"
	"codenest.dev/modules/packages/arch"
	"codenest.dev/modules/packages/cargo"
	"codenest.dev/modules/packages/chef"
	"codenest.dev/modules/packages/composer"
	"codenest.dev/modules/packages/conan"
	"codenest.dev/modules/packages/conda"
	"codenest.dev/modules/packages/container"
	"codenest.dev/modules/packages/cran"
	"codenest.dev/modules/packages/debian"
	"codenest.dev/modules/packages/helm"
	"codenest.dev/modules/packages/maven"
	"codenest.dev/modules/packages/npm"
	"codenest.dev/modules/packages/nuget"
	"codenest.dev/modules/packages/pub"
	"codenest.dev/modules/packages/pypi"
	"codenest.dev/modules/packages/rpm"
	"codenest.dev/modules/packages/rubygems"
	"codenest.dev/modules/packages/swift"
	"codenest.dev/modules/packages/vagrant"
	"codenest.dev/modules/util"

	"github.com/hashicorp/go-version"
)

// PackagePropertyList is a list of package properties
type PackagePropertyList []*PackageProperty

// GetByName gets the first property value with the specific name
func (l PackagePropertyList) GetByName(name string) string {
	for _, pp := range l {
		if pp.Name == name {
			return pp.Value
		}
	}
	return ""
}

// PackageDescriptor describes a package
type PackageDescriptor struct {
	// basic package info
	Package *Package
	Owner   *user_model.User

	// package version info
	Repository        *repo_model.Repository
	Version           *PackageVersion
	SemVer            *version.Version
	Creator           *user_model.User
	PackageProperties PackagePropertyList
	VersionProperties PackagePropertyList
	Metadata          any
	Files             []*PackageFileDescriptor
}

// PackageFileDescriptor describes a package file
type PackageFileDescriptor struct {
	File       *PackageFile
	Blob       *PackageBlob
	Properties PackagePropertyList
}

// PackageWebLink returns the relative package web link
func (pd *PackageDescriptor) PackageWebLink() string {
	return fmt.Sprintf("%s/-/packages/%s/%s", pd.Owner.HomeLink(), string(pd.Package.Type), url.PathEscape(pd.Package.LowerName))
}

// PackageSettingsLink returns the relative package settings link
func (pd *PackageDescriptor) PackageSettingsLink() string {
	return fmt.Sprintf("%s/-/packages/settings/%s/%s", pd.Owner.HomeLink(), string(pd.Package.Type), url.PathEscape(pd.Package.LowerName))
}

// VersionWebLink returns the relative package version web link
func (pd *PackageDescriptor) VersionWebLink() string {
	return fmt.Sprintf("%s/%s", pd.PackageWebLink(), url.PathEscape(pd.Version.LowerVersion))
}

// PackageHTMLURL returns the absolute package HTML URL
func (pd *PackageDescriptor) PackageHTMLURL(ctx context.Context) string {
	return fmt.Sprintf("%s/-/packages/%s/%s", pd.Owner.HTMLURL(ctx), string(pd.Package.Type), url.PathEscape(pd.Package.LowerName))
}

// VersionHTMLURL returns the absolute package version HTML URL
func (pd *PackageDescriptor) VersionHTMLURL(ctx context.Context) string {
	return fmt.Sprintf("%s/%s", pd.PackageHTMLURL(ctx), url.PathEscape(pd.Version.LowerVersion))
}

// CalculateBlobSize returns the total blobs size in bytes
func (pd *PackageDescriptor) CalculateBlobSize() int64 {
	size := int64(0)
	for _, f := range pd.Files {
		size += f.Blob.Size
	}
	return size
}

// GetPackageDescriptor gets the package description for a version
func GetPackageDescriptor(ctx context.Context, pv *PackageVersion) (*PackageDescriptor, error) {
	return GetPackageDescriptorWithCache(ctx, pv, cache.NewEphemeralCache())
}

func GetPackageDescriptorWithCache(ctx context.Context, pv *PackageVersion, c *cache.EphemeralCache) (*PackageDescriptor, error) {
	p, err := cache.GetWithEphemeralCache(ctx, c, "package", pv.PackageID, GetPackageByID)
	if err != nil {
		return nil, err
	}
	o, err := cache.GetWithEphemeralCache(ctx, c, "user", p.OwnerID, user_model.GetUserByID)
	if err != nil {
		return nil, err
	}
	var repository *repo_model.Repository
	if p.RepoID > 0 {
		repository, err = cache.GetWithEphemeralCache(ctx, c, "repo", p.RepoID, repo_model.GetRepositoryByID)
		if err != nil && !repo_model.IsErrRepoNotExist(err) {
			return nil, err
		}
	}
	creator, err := cache.GetWithEphemeralCache(ctx, c, "user", pv.CreatorID, user_model.GetUserByID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			creator = user_model.NewGhostUser()
		} else {
			return nil, err
		}
	}
	var semVer *version.Version
	if p.SemverCompatible {
		semVer, err = version.NewVersion(pv.Version)
		if err != nil {
			return nil, err
		}
	}
	pps, err := GetProperties(ctx, PropertyTypePackage, p.ID)
	if err != nil {
		return nil, err
	}
	pvps, err := GetProperties(ctx, PropertyTypeVersion, pv.ID)
	if err != nil {
		return nil, err
	}
	pfs, err := GetFilesByVersionID(ctx, pv.ID)
	if err != nil {
		return nil, err
	}

	pfds := make([]*PackageFileDescriptor, 0, len(pfs))
	for _, pf := range pfs {
		pfd, err := getPackageFileDescriptor(ctx, pf, c)
		if err != nil {
			return nil, err
		}
		pfds = append(pfds, pfd)
	}

	var metadata any
	switch p.Type {
	case TypeAlpine:
		metadata = &alpine.VersionMetadata{}
	case TypeArch:
		metadata = &arch.VersionMetadata{}
	case TypeCargo:
		metadata = &cargo.Metadata{}
	case TypeChef:
		metadata = &chef.Metadata{}
	case TypeComposer:
		metadata = &composer.Metadata{}
	case TypeConan:
		metadata = &conan.Metadata{}
	case TypeConda:
		metadata = &conda.VersionMetadata{}
	case TypeContainer:
		metadata = &container.Metadata{}
	case TypeCran:
		metadata = &cran.Metadata{}
	case TypeDebian:
		metadata = &debian.Metadata{}
	case TypeGeneric:
		// generic packages have no metadata
	case TypeGo:
		// go packages have no metadata
	case TypeHelm:
		metadata = &helm.Metadata{}
	case TypeNuGet:
		metadata = &nuget.Metadata{}
	case TypeNpm:
		metadata = &npm.Metadata{}
	case TypeMaven:
		metadata = &maven.Metadata{}
	case TypePub:
		metadata = &pub.Metadata{}
	case TypePyPI:
		metadata = &pypi.Metadata{}
	case TypeRpm:
		metadata = &rpm.VersionMetadata{}
	case TypeRubyGems:
		metadata = &rubygems.Metadata{}
	case TypeSwift:
		metadata = &swift.Metadata{}
	case TypeTerraformState:
		// terraform packages have no metadata
	case TypeVagrant:
		metadata = &vagrant.Metadata{}
	default:
		panic("unknown package type: " + string(p.Type))
	}
	if metadata != nil {
		if err := json.Unmarshal([]byte(pv.MetadataJSON), &metadata); err != nil {
			return nil, err
		}
	}

	return &PackageDescriptor{
		Package:           p,
		Owner:             o,
		Repository:        repository,
		Version:           pv,
		SemVer:            semVer,
		Creator:           creator,
		PackageProperties: PackagePropertyList(pps),
		VersionProperties: PackagePropertyList(pvps),
		Metadata:          metadata,
		Files:             pfds,
	}, nil
}

// DescriptorMetadata returns the descriptor's metadata, which getPackageDescriptor has created from the package type
func DescriptorMetadata[T interface{ *E }, E any](pd *PackageDescriptor) T {
	metadata, ok := pd.Metadata.(T)
	if !ok {
		panic(fmt.Errorf("package %s of type %s has metadata type %T instead of %T", pd.Package.Name, pd.Package.Type, pd.Metadata, metadata))
	}
	return metadata
}

// GetPackageFileDescriptor gets a package file descriptor for a package file
func GetPackageFileDescriptor(ctx context.Context, pf *PackageFile) (*PackageFileDescriptor, error) {
	return getPackageFileDescriptor(ctx, pf, cache.NewEphemeralCache())
}

func getPackageFileDescriptor(ctx context.Context, pf *PackageFile, c *cache.EphemeralCache) (*PackageFileDescriptor, error) {
	pb, err := cache.GetWithEphemeralCache(ctx, c, "package_file_blob", pf.BlobID, GetBlobByID)
	if err != nil {
		return nil, err
	}
	pfps, err := GetProperties(ctx, PropertyTypeFile, pf.ID)
	if err != nil {
		return nil, err
	}
	return &PackageFileDescriptor{
		pf,
		pb,
		PackagePropertyList(pfps),
	}, nil
}

// GetPackageFileDescriptors gets the package file descriptors for the package files
func GetPackageFileDescriptors(ctx context.Context, pfs []*PackageFile) ([]*PackageFileDescriptor, error) {
	pfds := make([]*PackageFileDescriptor, 0, len(pfs))
	for _, pf := range pfs {
		pfd, err := GetPackageFileDescriptor(ctx, pf)
		if err != nil {
			return nil, err
		}
		pfds = append(pfds, pfd)
	}
	return pfds, nil
}

// GetPackageDescriptors gets the package descriptions for the versions
func GetPackageDescriptors(ctx context.Context, pvs []*PackageVersion) ([]*PackageDescriptor, error) {
	return getPackageDescriptors(ctx, pvs, cache.NewEphemeralCache())
}

// GetAllPackageDescriptors gets all package descriptors for a package
func GetAllPackageDescriptors(ctx context.Context, p *Package) ([]*PackageDescriptor, error) {
	pvs := make([]*PackageVersion, 0, 10)
	if err := db.GetEngine(ctx).Where("package_id = ?", p.ID).Find(&pvs); err != nil {
		return nil, err
	}
	return getPackageDescriptors(ctx, pvs, cache.NewEphemeralCache())
}

func getPackageDescriptors(ctx context.Context, pvs []*PackageVersion, c *cache.EphemeralCache) ([]*PackageDescriptor, error) {
	pds := make([]*PackageDescriptor, 0, len(pvs))
	for _, pv := range pvs {
		pd, err := GetPackageDescriptorWithCache(ctx, pv, c)
		if err != nil {
			return nil, err
		}
		pds = append(pds, pd)
	}
	return pds, nil
}
