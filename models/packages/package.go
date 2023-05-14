// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Package))
}

var (
	// ErrDuplicatePackage indicates a duplicated package error
	ErrDuplicatePackage = util.NewAlreadyExistErrorf("package already exists")
	// ErrPackageNotExist indicates a package not exist error
	ErrPackageNotExist = util.NewNotExistErrorf("package does not exist")
)

// Type of a package
type Type string

// List of supported packages
const (
	TypeAlpine    Type = "alpine"
	TypeCargo     Type = "cargo"
	TypeChef      Type = "chef"
	TypeComposer  Type = "composer"
	TypeConan     Type = "conan"
	TypeConda     Type = "conda"
	TypeContainer Type = "container"
	TypeDebian    Type = "debian"
	TypeGeneric   Type = "generic"
	TypeHelm      Type = "helm"
	TypeMaven     Type = "maven"
	TypeNpm       Type = "npm"
	TypeNuGet     Type = "nuget"
	TypePub       Type = "pub"
	TypePyPI      Type = "pypi"
	TypeRpm       Type = "rpm"
	TypeRubyGems  Type = "rubygems"
	TypeSwift     Type = "swift"
	TypeVagrant   Type = "vagrant"
)

var TypeList = []Type{
	TypeAlpine,
	TypeCargo,
	TypeChef,
	TypeComposer,
	TypeConan,
	TypeConda,
	TypeContainer,
	TypeDebian,
	TypeGeneric,
	TypeHelm,
	TypeMaven,
	TypeNpm,
	TypeNuGet,
	TypePub,
	TypePyPI,
	TypeRpm,
	TypeRubyGems,
	TypeSwift,
	TypeVagrant,
}

// Name gets the name of the package type
func (pt Type) Name() string {
	switch pt {
	case TypeAlpine:
		return "Alpine"
	case TypeCargo:
		return "Cargo"
	case TypeChef:
		return "Chef"
	case TypeComposer:
		return "Composer"
	case TypeConan:
		return "Conan"
	case TypeConda:
		return "Conda"
	case TypeContainer:
		return "Container"
	case TypeDebian:
		return "Debian"
	case TypeGeneric:
		return "Generic"
	case TypeHelm:
		return "Helm"
	case TypeMaven:
		return "Maven"
	case TypeNpm:
		return "npm"
	case TypeNuGet:
		return "NuGet"
	case TypePub:
		return "Pub"
	case TypePyPI:
		return "PyPI"
	case TypeRpm:
		return "RPM"
	case TypeRubyGems:
		return "RubyGems"
	case TypeSwift:
		return "Swift"
	case TypeVagrant:
		return "Vagrant"
	}
	panic(fmt.Sprintf("unknown package type: %s", string(pt)))
}

// SVGName gets the name of the package type svg image
func (pt Type) SVGName() string {
	switch pt {
	case TypeAlpine:
		return "gitea-alpine"
	case TypeCargo:
		return "gitea-cargo"
	case TypeChef:
		return "gitea-chef"
	case TypeComposer:
		return "gitea-composer"
	case TypeConan:
		return "gitea-conan"
	case TypeConda:
		return "gitea-conda"
	case TypeContainer:
		return "octicon-container"
	case TypeDebian:
		return "gitea-debian"
	case TypeGeneric:
		return "octicon-package"
	case TypeHelm:
		return "gitea-helm"
	case TypeMaven:
		return "gitea-maven"
	case TypeNpm:
		return "gitea-npm"
	case TypeNuGet:
		return "gitea-nuget"
	case TypePub:
		return "gitea-pub"
	case TypePyPI:
		return "gitea-python"
	case TypeRpm:
		return "gitea-rpm"
	case TypeRubyGems:
		return "gitea-rubygems"
	case TypeSwift:
		return "gitea-swift"
	case TypeVagrant:
		return "gitea-vagrant"
	}
	panic(fmt.Sprintf("unknown package type: %s", string(pt)))
}

// Package represents a package
type Package struct {
	ID               int64  `xorm:"pk autoincr"`
	OwnerID          int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
	RepoID           int64  `xorm:"INDEX"`
	Type             Type   `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name             string `xorm:"NOT NULL"`
	LowerName        string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	SemverCompatible bool   `xorm:"NOT NULL DEFAULT false"`
	IsInternal       bool   `xorm:"NOT NULL DEFAULT false"`
}

// TryInsertPackage inserts a package. If a package exists already, ErrDuplicatePackage is returned
func TryInsertPackage(ctx context.Context, p *Package) (*Package, error) {
	e := db.GetEngine(ctx)

	key := &Package{
		OwnerID:   p.OwnerID,
		Type:      p.Type,
		LowerName: p.LowerName,
	}

	has, err := e.Get(key)
	if err != nil {
		return nil, err
	}
	if has {
		return key, ErrDuplicatePackage
	}
	if _, err = e.Insert(p); err != nil {
		return nil, err
	}
	return p, nil
}

// DeletePackageByID deletes a package by id
func DeletePackageByID(ctx context.Context, packageID int64) error {
	_, err := db.GetEngine(ctx).ID(packageID).Delete(&Package{})
	return err
}

// SetRepositoryLink sets the linked repository
func SetRepositoryLink(ctx context.Context, packageID, repoID int64) error {
	_, err := db.GetEngine(ctx).ID(packageID).Cols("repo_id").Update(&Package{RepoID: repoID})
	return err
}

// UnlinkRepositoryFromAllPackages unlinks every package from the repository
func UnlinkRepositoryFromAllPackages(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Cols("repo_id").Update(&Package{})
	return err
}

// GetPackageByID gets a package by id
func GetPackageByID(ctx context.Context, packageID int64) (*Package, error) {
	p := &Package{}

	has, err := db.GetEngine(ctx).ID(packageID).Get(p)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageNotExist
	}
	return p, nil
}

// GetPackageByName gets a package by name
func GetPackageByName(ctx context.Context, ownerID int64, packageType Type, name string) (*Package, error) {
	var cond builder.Cond = builder.Eq{
		"package.owner_id":    ownerID,
		"package.type":        packageType,
		"package.lower_name":  strings.ToLower(name),
		"package.is_internal": false,
	}

	p := &Package{}

	has, err := db.GetEngine(ctx).
		Where(cond).
		Get(p)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageNotExist
	}
	return p, nil
}

// GetPackagesByType gets all packages of a specific type
func GetPackagesByType(ctx context.Context, ownerID int64, packageType Type) ([]*Package, error) {
	var cond builder.Cond = builder.Eq{
		"package.owner_id":    ownerID,
		"package.type":        packageType,
		"package.is_internal": false,
	}

	ps := make([]*Package, 0, 10)
	return ps, db.GetEngine(ctx).
		Where(cond).
		Find(&ps)
}

// FindUnreferencedPackages gets all packages without associated versions
func FindUnreferencedPackages(ctx context.Context) ([]*Package, error) {
	in := builder.
		Select("package.id").
		From("package").
		LeftJoin("package_version", "package_version.package_id = package.id").
		Where(builder.Expr("package_version.id IS NULL"))

	ps := make([]*Package, 0, 10)
	return ps, db.GetEngine(ctx).
		// double select workaround for MySQL
		// https://stackoverflow.com/questions/4471277/mysql-delete-from-with-subquery-as-condition
		Where(builder.In("package.id", builder.Select("id").From(in, "temp"))).
		Find(&ps)
}

// HasOwnerPackages tests if a user/org has accessible packages
func HasOwnerPackages(ctx context.Context, ownerID int64) (bool, error) {
	return db.GetEngine(ctx).
		Table("package_version").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(builder.Eq{
			"package_version.is_internal": false,
			"package.owner_id":            ownerID,
		}).
		Exist(&PackageVersion{})
}

// HasRepositoryPackages tests if a repository has packages
func HasRepositoryPackages(ctx context.Context, repositoryID int64) (bool, error) {
	return db.GetEngine(ctx).Where("repo_id = ?", repositoryID).Exist(&Package{})
}
