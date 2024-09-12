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
	"code.gitea.io/gitea/modules/packages/alpine"
	"code.gitea.io/gitea/modules/packages/cargo"
	"code.gitea.io/gitea/modules/packages/chef"
	"code.gitea.io/gitea/modules/packages/composer"
	"code.gitea.io/gitea/modules/packages/conan"
	"code.gitea.io/gitea/modules/packages/conda"
	"code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/packages/cran"
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

// VersionWebLink returns the relative package version web link
func (pd *PackageDescriptor) VersionWebLink() string {
	return fmt.Sprintf("%s/%s", pd.PackageWebLink(), url.PathEscape(pd.Version.LowerVersion))
}

// PackageHTMLURL returns the absolute package HTML URL
func (pd *PackageDescriptor) PackageHTMLURL() string {
	return fmt.Sprintf("%s/-/packages/%s/%s", pd.Owner.HTMLURL(), string(pd.Package.Type), url.PathEscape(pd.Package.LowerName))
}

// VersionHTMLURL returns the absolute package version HTML URL
func (pd *PackageDescriptor) VersionHTMLURL() string {
	return fmt.Sprintf("%s/%s", pd.PackageHTMLURL(), url.PathEscape(pd.Version.LowerVersion))
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
	return getPackageDescriptor(ctx, pv, newQueryCache())
}

func getPackageDescriptor(ctx context.Context, pv *PackageVersion, c *cache) (*PackageDescriptor, error) {
	p, err := c.QueryPackage(ctx, pv.PackageID)
	if err != nil {
		return nil, err
	}
	o, err := c.QueryUser(ctx, p.OwnerID)
	if err != nil {
		return nil, err
	}
	repository, err := c.QueryRepository(ctx, p.RepoID)
	if err != nil {
		return nil, err
	}
	creator, err := c.QueryUser(ctx, pv.CreatorID)
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
	return getPackageFileDescriptor(ctx, pf, newQueryCache())
}

func getPackageFileDescriptor(ctx context.Context, pf *PackageFile, c *cache) (*PackageFileDescriptor, error) {
	pb, err := c.QueryBlob(ctx, pf.BlobID)
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
	return getPackageDescriptors(ctx, pvs, newQueryCache())
}

func getPackageDescriptors(ctx context.Context, pvs []*PackageVersion, c *cache) ([]*PackageDescriptor, error) {
	pds := make([]*PackageDescriptor, 0, len(pvs))
	for _, pv := range pvs {
		pd, err := getPackageDescriptor(ctx, pv, c)
		if err != nil {
			return nil, err
		}
		pds = append(pds, pd)
	}
	return pds, nil
}

type cache struct {
	Packages     map[int64]*Package
	Users        map[int64]*user_model.User
	Repositories map[int64]*repo_model.Repository
	Blobs        map[int64]*PackageBlob
}

func newQueryCache() *cache {
	return &cache{
		Packages:     make(map[int64]*Package),
		Users:        make(map[int64]*user_model.User),
		Repositories: map[int64]*repo_model.Repository{0: nil}, // 0 is an expected value
		Blobs:        make(map[int64]*PackageBlob),
	}
}

func (c *cache) QueryPackage(ctx context.Context, id int64) (*Package, error) {
	if p, found := c.Packages[id]; found {
		return p, nil
	}

	p, err := GetPackageByID(ctx, id)
	c.Packages[id] = p
	return p, err
}

func (c *cache) QueryUser(ctx context.Context, id int64) (*user_model.User, error) {
	if u, found := c.Users[id]; found {
		return u, nil
	}

	u, err := user_model.GetUserByID(ctx, id)
	c.Users[id] = u
	return u, err
}

func (c *cache) QueryRepository(ctx context.Context, id int64) (*repo_model.Repository, error) {
	if r, found := c.Repositories[id]; found {
		return r, nil
	}

	r, err := repo_model.GetRepositoryByID(ctx, id)
	if err != nil && !repo_model.IsErrRepoNotExist(err) {
		err = nil
	}
	c.Repositories[id] = r
	return r, err
}

func (c *cache) QueryBlob(ctx context.Context, id int64) (*PackageBlob, error) {
	if b, found := c.Blobs[id]; found {
		return b, nil
	}

	b, err := GetBlobByID(ctx, id)
	c.Blobs[id] = b
	return b, err
}
