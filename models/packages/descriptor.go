// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/alpine"
	"code.gitea.io/gitea/modules/packages/arch"
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

func unmarshalPackageMetadata(packageType Type, metadataJSON string) (any, error) {
	var metadata any
	switch packageType {
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
		panic("unknown package type: " + string(packageType))
	}
	if metadata != nil {
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return nil, err
		}
	}
	return metadata, nil
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

	metadata, err := unmarshalPackageMetadata(p.Type, pv.MetadataJSON)
	if err != nil {
		return nil, err
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
	blobIDs := make(map[int64]struct{}, len(pfs))
	fileIDs := make(map[int64]struct{}, len(pfs))
	for _, pf := range pfs {
		addID(blobIDs, pf.BlobID)
		addID(fileIDs, pf.ID)
	}

	blobs, err := GetBlobsByIDs(ctx, idsFromSet(blobIDs))
	if err != nil {
		return nil, err
	}
	properties, err := GetPropertiesByRefIDs(ctx, PropertyTypeFile, idsFromSet(fileIDs))
	if err != nil {
		return nil, err
	}
	return packageFileDescriptorsFromMaps(pfs, blobs, properties)
}

func packageFileDescriptorsFromMaps(pfs []*PackageFile, blobs map[int64]*PackageBlob, properties map[int64][]*PackageProperty) ([]*PackageFileDescriptor, error) {
	pfds := make([]*PackageFileDescriptor, 0, len(pfs))
	for _, pf := range pfs {
		blob, ok := blobs[pf.BlobID]
		if !ok {
			return nil, ErrPackageBlobNotExist
		}
		pfds = append(pfds, &PackageFileDescriptor{
			File:       pf,
			Blob:       blob,
			Properties: PackagePropertyList(properties[pf.ID]),
		})
	}
	return pfds, nil
}

// GetPackageDescriptors gets the package descriptions for the versions
func GetPackageDescriptors(ctx context.Context, pvs []*PackageVersion) ([]*PackageDescriptor, error) {
	return getPackageDescriptors(ctx, pvs)
}

// GetAllPackageDescriptors gets all package descriptors for a package
func GetAllPackageDescriptors(ctx context.Context, p *Package) ([]*PackageDescriptor, error) {
	pvs := make([]*PackageVersion, 0, 10)
	if err := db.GetEngine(ctx).Where("package_id = ?", p.ID).Find(&pvs); err != nil {
		return nil, err
	}
	return getPackageDescriptors(ctx, pvs)
}

func getPackageDescriptors(ctx context.Context, pvs []*PackageVersion) ([]*PackageDescriptor, error) {
	batch, err := loadPackageDescriptorBatch(ctx, pvs)
	if err != nil {
		return nil, err
	}

	pds := make([]*PackageDescriptor, 0, len(pvs))
	for _, pv := range pvs {
		pd, err := batch.getPackageDescriptor(pv)
		if err != nil {
			return nil, err
		}
		pds = append(pds, pd)
	}
	return pds, nil
}

type packageDescriptorBatch struct {
	packages          map[int64]*Package
	users             map[int64]*user_model.User
	repositories      map[int64]*repo_model.Repository
	packageProperties map[int64][]*PackageProperty
	versionProperties map[int64][]*PackageProperty
	files             map[int64][]*PackageFile
	blobs             map[int64]*PackageBlob
	fileProperties    map[int64][]*PackageProperty
}

func loadPackageDescriptorBatch(ctx context.Context, pvs []*PackageVersion) (*packageDescriptorBatch, error) {
	packageIDs := make(map[int64]struct{}, len(pvs))
	versionIDs := make(map[int64]struct{}, len(pvs))
	userIDs := make(map[int64]struct{}, len(pvs))
	for _, pv := range pvs {
		addID(packageIDs, pv.PackageID)
		addID(versionIDs, pv.ID)
		addID(userIDs, pv.CreatorID)
	}

	packages, err := GetPackagesByIDs(ctx, idsFromSet(packageIDs))
	if err != nil {
		return nil, err
	}

	repoIDs := make(map[int64]struct{}, len(packages))
	for _, p := range packages {
		addID(userIDs, p.OwnerID)
		addID(repoIDs, p.RepoID)
	}

	users, err := user_model.GetUsersMapByIDs(ctx, idsFromSet(userIDs))
	if err != nil {
		return nil, err
	}

	repositories, err := repo_model.GetRepositoriesMapByIDs(ctx, idsFromSet(repoIDs))
	if err != nil {
		return nil, err
	}

	packageProperties, err := GetPropertiesByRefIDs(ctx, PropertyTypePackage, idsFromSet(packageIDs))
	if err != nil {
		return nil, err
	}

	versionProperties, err := GetPropertiesByRefIDs(ctx, PropertyTypeVersion, idsFromSet(versionIDs))
	if err != nil {
		return nil, err
	}

	files, err := GetFilesByVersionIDs(ctx, idsFromSet(versionIDs))
	if err != nil {
		return nil, err
	}

	blobIDs := make(map[int64]struct{})
	fileIDs := make(map[int64]struct{})
	for _, pfs := range files {
		for _, pf := range pfs {
			addID(blobIDs, pf.BlobID)
			addID(fileIDs, pf.ID)
		}
	}

	blobs, err := GetBlobsByIDs(ctx, idsFromSet(blobIDs))
	if err != nil {
		return nil, err
	}

	fileProperties, err := GetPropertiesByRefIDs(ctx, PropertyTypeFile, idsFromSet(fileIDs))
	if err != nil {
		return nil, err
	}

	return &packageDescriptorBatch{
		packages:          packages,
		users:             users,
		repositories:      repositories,
		packageProperties: packageProperties,
		versionProperties: versionProperties,
		files:             files,
		blobs:             blobs,
		fileProperties:    fileProperties,
	}, nil
}

func (b *packageDescriptorBatch) getPackageDescriptor(pv *PackageVersion) (*PackageDescriptor, error) {
	p, ok := b.packages[pv.PackageID]
	if !ok {
		return nil, ErrPackageNotExist
	}

	owner, ok := b.users[p.OwnerID]
	if !ok {
		return nil, user_model.ErrUserNotExist{UID: p.OwnerID}
	}

	var repository *repo_model.Repository
	if p.RepoID > 0 {
		repository = b.repositories[p.RepoID]
	}

	creator, ok := b.users[pv.CreatorID]
	if !ok {
		creator = user_model.NewGhostUser()
	}

	var semVer *version.Version
	if p.SemverCompatible {
		var err error
		semVer, err = version.NewVersion(pv.Version)
		if err != nil {
			return nil, err
		}
	}

	pfds, err := packageFileDescriptorsFromMaps(b.files[pv.ID], b.blobs, b.fileProperties)
	if err != nil {
		return nil, err
	}

	metadata, err := unmarshalPackageMetadata(p.Type, pv.MetadataJSON)
	if err != nil {
		return nil, err
	}

	return &PackageDescriptor{
		Package:           p,
		Owner:             owner,
		Repository:        repository,
		Version:           pv,
		SemVer:            semVer,
		Creator:           creator,
		PackageProperties: PackagePropertyList(b.packageProperties[p.ID]),
		VersionProperties: PackagePropertyList(b.versionProperties[pv.ID]),
		Metadata:          metadata,
		Files:             pfds,
	}, nil
}

func addID(ids map[int64]struct{}, id int64) {
	if id > 0 {
		ids[id] = struct{}{}
	}
}

func idsFromSet(ids map[int64]struct{}) []int64 {
	values := make([]int64, 0, len(ids))
	for id := range ids {
		values = append(values, id)
	}
	return values
}
