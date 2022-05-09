// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package conan

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	conan_module "code.gitea.io/gitea/modules/packages/conan"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

var (
	ErrRecipeReferenceNotExist  = errors.New("Recipe reference does not exist")
	ErrPackageReferenceNotExist = errors.New("Package reference does not exist")
)

// RecipeExists checks if a recipe exists
func RecipeExists(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) (bool, error) {
	revisions, err := GetRecipeRevisions(ctx, ownerID, ref)
	if err != nil {
		return false, err
	}

	return len(revisions) != 0, nil
}

type PropertyValue struct {
	Value       string
	CreatedUnix timeutil.TimeStamp
}

func findPropertyValues(ctx context.Context, propertyName string, ownerID int64, name, version string, propertyFilter map[string]string) ([]*PropertyValue, error) {
	var propsCond builder.Cond = builder.Eq{
		"package_property.ref_type": packages.PropertyTypeFile,
	}
	propsCond = propsCond.And(builder.Expr("package_property.ref_id = package_file.id"))

	propsCondBlock := builder.NewCond()
	for name, value := range propertyFilter {
		propsCondBlock = propsCondBlock.Or(builder.Eq{
			"package_property.name":  name,
			"package_property.value": value,
		})
	}
	propsCond = propsCond.And(propsCondBlock)

	var cond builder.Cond = builder.Eq{
		"package.type":                    packages.TypeConan,
		"package.owner_id":                ownerID,
		"package.lower_name":              strings.ToLower(name),
		"package_version.lower_version":   strings.ToLower(version),
		"package_version.is_internal":     false,
		strconv.Itoa(len(propertyFilter)): builder.Select("COUNT(*)").Where(propsCond).From("package_property"),
	}

	in2 := builder.
		Select("package_file.id").
		From("package_file").
		InnerJoin("package_version", "package_version.id = package_file.version_id").
		InnerJoin("package", "package.id = package_version.package_id").
		Where(cond)

	query := builder.
		Select("package_property.value, MAX(package_file.created_unix) AS created_unix").
		From("package_property").
		InnerJoin("package_file", "package_file.id = package_property.ref_id").
		Where(builder.Eq{"package_property.name": propertyName}.And(builder.In("package_property.ref_id", in2))).
		GroupBy("package_property.value").
		OrderBy("created_unix DESC")

	var values []*PropertyValue
	return values, db.GetEngine(ctx).SQL(query).Find(&values)
}

// GetRecipeRevisions gets all revisions of a recipe
func GetRecipeRevisions(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) ([]*PropertyValue, error) {
	values, err := findPropertyValues(
		ctx,
		conan_module.PropertyRecipeRevision,
		ownerID,
		ref.Name,
		ref.Version,
		map[string]string{
			conan_module.PropertyRecipeUser:    ref.User,
			conan_module.PropertyRecipeChannel: ref.Channel,
		},
	)
	if err != nil {
		return nil, err
	}

	return values, nil
}

// GetLastRecipeRevision gets the latest recipe revision
func GetLastRecipeRevision(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) (*PropertyValue, error) {
	revisions, err := GetRecipeRevisions(ctx, ownerID, ref)
	if err != nil {
		return nil, err
	}

	if len(revisions) == 0 {
		return nil, ErrRecipeReferenceNotExist
	}
	return revisions[0], nil
}

// GetPackageReferences gets all package references of a recipe
func GetPackageReferences(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) ([]*PropertyValue, error) {
	values, err := findPropertyValues(
		ctx,
		conan_module.PropertyPackageReference,
		ownerID,
		ref.Name,
		ref.Version,
		map[string]string{
			conan_module.PropertyRecipeUser:     ref.User,
			conan_module.PropertyRecipeChannel:  ref.Channel,
			conan_module.PropertyRecipeRevision: ref.Revision,
		},
	)
	if err != nil {
		return nil, err
	}

	return values, nil
}

// GetPackageRevisions gets all revision of a package
func GetPackageRevisions(ctx context.Context, ownerID int64, ref *conan_module.PackageReference) ([]*PropertyValue, error) {
	values, err := findPropertyValues(
		ctx,
		conan_module.PropertyPackageRevision,
		ownerID,
		ref.Recipe.Name,
		ref.Recipe.Version,
		map[string]string{
			conan_module.PropertyRecipeUser:       ref.Recipe.User,
			conan_module.PropertyRecipeChannel:    ref.Recipe.Channel,
			conan_module.PropertyRecipeRevision:   ref.Recipe.Revision,
			conan_module.PropertyPackageReference: ref.Reference,
		},
	)
	if err != nil {
		return nil, err
	}

	return values, nil
}

// GetLastPackageRevision gets the latest package revision
func GetLastPackageRevision(ctx context.Context, ownerID int64, ref *conan_module.PackageReference) (*PropertyValue, error) {
	revisions, err := GetPackageRevisions(ctx, ownerID, ref)
	if err != nil {
		return nil, err
	}

	if len(revisions) == 0 {
		return nil, ErrPackageReferenceNotExist
	}
	return revisions[0], nil
}
