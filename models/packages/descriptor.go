// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/cargo"
	"code.gitea.io/gitea/modules/packages/chef"
	"code.gitea.io/gitea/modules/packages/composer"
	"code.gitea.io/gitea/modules/packages/conan"
	"code.gitea.io/gitea/modules/packages/conda"
	"code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/modules/packages/helm"
	"code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/packages/pub"
	"code.gitea.io/gitea/modules/packages/pypi"
	"code.gitea.io/gitea/modules/packages/rpm"
	"code.gitea.io/gitea/modules/packages/rubygems"
	"code.gitea.io/gitea/modules/packages/swift"
	"code.gitea.io/gitea/modules/packages/vagrant"
	"code.gitea.io/gitea/modules/util"

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
	Package           *Package
	Owner             *user_model.User
	Repository        *repo_model.Repository
	Version           *PackageVersion
	SemVer            *version.Version
	Creator           *user_model.User
	PackageProperties PackagePropertyList
	VersionProperties PackagePropertyList
	Metadata          interface{}
	Files             []*PackageFileDescriptor
}

// PackageFileDescriptor describes a package file
type PackageFileDescriptor struct {
	File       *PackageFile
	Blob       *PackageBlob
	Properties PackagePropertyList
}

// PackageWebLink returns the package web link
func (pd *PackageDescriptor) PackageWebLink() string {
	return fmt.Sprintf("%s/-/packages/%s/%s", pd.Owner.HomeLink(), string(pd.Package.Type), url.PathEscape(pd.Package.LowerName))
}

// FullWebLink returns the package version web link
func (pd *PackageDescriptor) FullWebLink() string {
	return fmt.Sprintf("%s/%s", pd.PackageWebLink(), url.PathEscape(pd.Version.LowerVersion))
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
	p, err := GetPackageByID(ctx, pv.PackageID)
	if err != nil {
		return nil, err
	}
	o, err := user_model.GetUserByID(ctx, p.OwnerID)
	if err != nil {
		return nil, err
	}
	repository, err := repo_model.GetRepositoryByID(ctx, p.RepoID)
	if err != nil && !repo_model.IsErrRepoNotExist(err) {
		return nil, err
	}
	creator, err := user_model.GetUserByID(ctx, pv.CreatorID)
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

	pfds, err := GetPackageFileDescriptors(ctx, pfs)
	if err != nil {
		return nil, err
	}

	var metadata interface{}
	switch p.Type {
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
	case TypeDebian:
		metadata = &debian.Metadata{}
	case TypeGeneric:
		// generic packages have no metadata
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
	case TypeVagrant:
		metadata = &vagrant.Metadata{}
	default:
		panic(fmt.Sprintf("unknown package type: %s", string(p.Type)))
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

// GetPackageFileDescriptor gets a package file descriptor for a package file
func GetPackageFileDescriptor(ctx context.Context, pf *PackageFile) (*PackageFileDescriptor, error) {
	pb, err := GetBlobByID(ctx, pf.BlobID)
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
	pds := make([]*PackageDescriptor, 0, len(pvs))
	for _, pv := range pvs {
		pd, err := GetPackageDescriptor(ctx, pv)
		if err != nil {
			return nil, err
		}
		pds = append(pds, pd)
	}
	return pds, nil
}
