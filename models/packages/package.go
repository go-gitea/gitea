// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models/db"
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
	TypeGeneric  Type = "generic"
	TypeNuGet    Type = "nuget"
	TypeNpm      Type = "npm"
	TypeMaven    Type = "maven"
	TypePyPI     Type = "pypi"
	TypeRubyGems Type = "rubygems"
)

// Name gets the name of the package type
func (pt Type) Name() string {
	switch pt {
	case TypeGeneric:
		return "Generic"
	case TypeNuGet:
		return "NuGet"
	case TypeNpm:
		return "npm"
	case TypeMaven:
		return "Maven"
	case TypePyPI:
		return "PyPI"
	case TypeRubyGems:
		return "RubyGems"
	}
	return ""
}

// SVGName gets the name of the package type svg image
func (pt Type) SVGName() string {
	switch pt {
	case TypeGeneric:
		return "octicon-package"
	case TypeNuGet:
		return "gitea-nuget"
	case TypeNpm:
		return "gitea-npm"
	case TypeMaven:
		return "gitea-maven"
	case TypePyPI:
		return "gitea-python"
	case TypeRubyGems:
		return "gitea-rubygems"
	}
	return ""
}

// Package represents a package
type Package struct {
	ID               int64 `xorm:"pk autoincr"`
	OwnerID          int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	RepoID           int64 `xorm:"INDEX"`
	Type             Type  `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name             string
	LowerName        string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	SemverCompatible bool
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
func SetRepositoryLink(packageID, repoID int64) error {
	_, err := db.GetEngine(db.DefaultContext).ID(packageID).Cols("repo_id").Update(&Package{RepoID: repoID})
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

// DeletePackageByIDIfUnreferenced deletes a package if there are no associated versions
func DeletePackageByIDIfUnreferenced(ctx context.Context, packageID int64) error {
	e := db.GetEngine(ctx)

	count, err := e.Count(&PackageVersion{
		PackageID: packageID,
	})
	if err != nil {
		return err
	}
	if count == 0 {
		_, err := e.ID(packageID).Delete(&Package{})
		return err
	}
	return nil
}

// HasOwnerPackages tests if a user/org has packages
func HasOwnerPackages(ownerID int64) (bool, error) {
	return db.GetEngine(db.DefaultContext).Where("owner_id = ?", ownerID).Exist(&Package{})
}

// HasRepositoryPackages tests if a repository has packages
func HasRepositoryPackages(repositoryID int64) (bool, error) {
	return db.GetEngine(db.DefaultContext).Where("repo_id = ?", repositoryID).Exist(&Package{})
}
