// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

import (
	"context"
	"strconv"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/packages"
	conan_module "gitea.dev/modules/packages/conan"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

var (
	ErrRecipeReferenceNotExist  = util.NewNotExistErrorf("recipe reference does not exist")
	ErrPackageReferenceNotExist = util.NewNotExistErrorf("package reference does not exist")
)

// RecipeExists checks if a recipe exists (team_id = 0)
func RecipeExists(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) (bool, error) {
	return TeamRecipeExists(ctx, ownerID, 0, ref)
}

// TeamRecipeExists checks if a recipe exists for a team
func TeamRecipeExists(ctx context.Context, ownerID, teamID int64, ref *conan_module.RecipeReference) (bool, error) {
	revisions, err := GetTeamRecipeRevisions(ctx, ownerID, teamID, ref)
	if err != nil {
		return false, err
	}

	return len(revisions) != 0, nil
}

type PropertyValue struct {
	Value       string
	CreatedUnix timeutil.TimeStamp
}

func findPropertyValues(ctx context.Context, propertyName string, ownerID, teamID int64, name, version string, propertyFilter map[string]string) ([]*PropertyValue, error) {
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
		"package.team_id":                 teamID,
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

// GetRecipeRevisions gets all revisions of a recipe (team_id = 0)
func GetRecipeRevisions(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) ([]*PropertyValue, error) {
	return GetTeamRecipeRevisions(ctx, ownerID, 0, ref)
}

// GetTeamRecipeRevisions gets all revisions of a recipe for a team
func GetTeamRecipeRevisions(ctx context.Context, ownerID, teamID int64, ref *conan_module.RecipeReference) ([]*PropertyValue, error) {
	values, err := findPropertyValues(
		ctx,
		conan_module.PropertyRecipeRevision,
		ownerID,
		teamID,
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

// GetLastRecipeRevision gets the latest recipe revision (team_id = 0)
func GetLastRecipeRevision(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) (*PropertyValue, error) {
	return GetTeamLastRecipeRevision(ctx, ownerID, 0, ref)
}

// GetTeamLastRecipeRevision gets the latest recipe revision for a team
func GetTeamLastRecipeRevision(ctx context.Context, ownerID, teamID int64, ref *conan_module.RecipeReference) (*PropertyValue, error) {
	revisions, err := GetTeamRecipeRevisions(ctx, ownerID, teamID, ref)
	if err != nil {
		return nil, err
	}

	if len(revisions) == 0 {
		return nil, ErrRecipeReferenceNotExist
	}
	return revisions[0], nil
}

// GetPackageReferences gets all package references of a recipe (team_id = 0)
func GetPackageReferences(ctx context.Context, ownerID int64, ref *conan_module.RecipeReference) ([]*PropertyValue, error) {
	return GetTeamPackageReferences(ctx, ownerID, 0, ref)
}

// GetTeamPackageReferences gets all package references of a recipe for a team
func GetTeamPackageReferences(ctx context.Context, ownerID, teamID int64, ref *conan_module.RecipeReference) ([]*PropertyValue, error) {
	values, err := findPropertyValues(
		ctx,
		conan_module.PropertyPackageReference,
		ownerID,
		teamID,
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

// GetPackageRevisions gets all revision of a package (team_id = 0)
func GetPackageRevisions(ctx context.Context, ownerID int64, ref *conan_module.PackageReference) ([]*PropertyValue, error) {
	return GetTeamPackageRevisions(ctx, ownerID, 0, ref)
}

// GetTeamPackageRevisions gets all revision of a package for a team
func GetTeamPackageRevisions(ctx context.Context, ownerID, teamID int64, ref *conan_module.PackageReference) ([]*PropertyValue, error) {
	values, err := findPropertyValues(
		ctx,
		conan_module.PropertyPackageRevision,
		ownerID,
		teamID,
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

// GetLastPackageRevision gets the latest package revision (team_id = 0)
func GetLastPackageRevision(ctx context.Context, ownerID int64, ref *conan_module.PackageReference) (*PropertyValue, error) {
	return GetTeamLastPackageRevision(ctx, ownerID, 0, ref)
}

// GetTeamLastPackageRevision gets the latest package revision for a team
func GetTeamLastPackageRevision(ctx context.Context, ownerID, teamID int64, ref *conan_module.PackageReference) (*PropertyValue, error) {
	revisions, err := GetTeamPackageRevisions(ctx, ownerID, teamID, ref)
	if err != nil {
		return nil, err
	}

	if len(revisions) == 0 {
		return nil, ErrPackageReferenceNotExist
	}
	return revisions[0], nil
}
