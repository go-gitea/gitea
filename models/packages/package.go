// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Package))
}

var (
	// ErrDuplicatePackage indicates a duplicated package error
	ErrDuplicatePackage = errors.New("Package does exist already")
	// ErrPackageNotExist indicates a package not exist error
	ErrPackageNotExist = errors.New("Package does not exist")
)

// Type of a package
type Type string

// List of supported packages
const (
	TypeComposer  Type = "composer"
	TypeConan     Type = "conan"
	TypeContainer Type = "container"
	TypeGeneric   Type = "generic"
	TypeHelm      Type = "helm"
	TypeMaven     Type = "maven"
	TypeNpm       Type = "npm"
	TypeNuGet     Type = "nuget"
	TypePyPI      Type = "pypi"
	TypeRubyGems  Type = "rubygems"
)

// Name gets the name of the package type
func (pt Type) Name() string {
	switch pt {
	case TypeComposer:
		return "Composer"
	case TypeConan:
		return "Conan"
	case TypeContainer:
		return "Container"
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
	case TypePyPI:
		return "PyPI"
	case TypeRubyGems:
		return "RubyGems"
	}
	panic(fmt.Sprintf("unknown package type: %s", string(pt)))
}

// SVGName gets the name of the package type svg image
func (pt Type) SVGName() string {
	switch pt {
	case TypeComposer:
		return "gitea-composer"
	case TypeConan:
		return "gitea-conan"
	case TypeContainer:
		return "octicon-container"
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
	case TypePyPI:
		return "gitea-python"
	case TypeRubyGems:
		return "gitea-rubygems"
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
		"package.owner_id":   ownerID,
		"package.type":       packageType,
		"package.lower_name": strings.ToLower(name),
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
		"package.owner_id": ownerID,
		"package.type":     packageType,
	}

	ps := make([]*Package, 0, 10)
	return ps, db.GetEngine(ctx).
		Where(cond).
		Find(&ps)
}

// DeletePackagesIfUnreferenced deletes a package if there are no associated versions
func DeletePackagesIfUnreferenced(ctx context.Context) error {
	in := builder.
		Select("package.id").
		From("package").
		LeftJoin("package_version", "package_version.package_id = package.id").
		Where(builder.Expr("package_version.id IS NULL"))

	_, err := db.GetEngine(ctx).
		// double select workaround for MySQL
		// https://stackoverflow.com/questions/4471277/mysql-delete-from-with-subquery-as-condition
		Where(builder.In("package.id", builder.Select("id").From(in, "temp"))).
		Delete(&Package{})

	return err
}

// HasOwnerPackages tests if a user/org has packages
func HasOwnerPackages(ctx context.Context, ownerID int64) (bool, error) {
	return db.GetEngine(ctx).Where("owner_id = ?", ownerID).Exist(&Package{})
}

// HasRepositoryPackages tests if a repository has packages
func HasRepositoryPackages(ctx context.Context, repositoryID int64) (bool, error) {
	return db.GetEngine(ctx).Where("repo_id = ?", repositoryID).Exist(&Package{})
}
