// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package conan

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	conan_module "code.gitea.io/gitea/modules/packages/conan"

	"xorm.io/builder"
)

// buildCondition creates a Like condition if a wildcard is present. Otherwise Eq is used.
func buildCondition(name, value string) builder.Cond {
	if strings.Contains(value, "*") {
		return builder.Like{name, strings.ReplaceAll(strings.ReplaceAll(value, "_", "\\_"), "*", "%")}
	}
	return builder.Eq{name: value}
}

type RecipeSearchOptions struct {
	OwnerID int64
	Name    string
	Version string
	User    string
	Channel string
}

// SearchRecipes gets all recipes matching the search options
func SearchRecipes(ctx context.Context, opts *RecipeSearchOptions) ([]string, error) {
	var cond builder.Cond = builder.Eq{
		"package_file.is_lead":        true,
		"package.type":                packages.TypeConan,
		"package.owner_id":            opts.OwnerID,
		"package_version.is_internal": false,
	}

	if opts.Name != "" {
		cond = cond.And(buildCondition("package.lower_name", strings.ToLower(opts.Name)))
	}
	if opts.Version != "" {
		cond = cond.And(buildCondition("package_version.lower_version", strings.ToLower(opts.Version)))
	}
	if opts.User != "" || opts.Channel != "" {
		var propsCond builder.Cond = builder.Eq{
			"package_property.ref_type": packages.PropertyTypeFile,
		}
		propsCond = propsCond.And(builder.Expr("package_property.ref_id = package_file.id"))

		count := 0
		propsCondBlock := builder.NewCond()
		if opts.User != "" {
			count++
			propsCondBlock = propsCondBlock.Or(builder.Eq{"package_property.name": conan_module.PropertyRecipeUser}.And(buildCondition("package_property.value", opts.User)))
		}
		if opts.Channel != "" {
			count++
			propsCondBlock = propsCondBlock.Or(builder.Eq{"package_property.name": conan_module.PropertyRecipeChannel}.And(buildCondition("package_property.value", opts.Channel)))
		}
		propsCond = propsCond.And(propsCondBlock)

		cond = cond.And(builder.Eq{
			strconv.Itoa(count): builder.Select("COUNT(*)").Where(propsCond).From("package_property"),
		})
	}

	query := builder.
		Select("package.name, package_version.version, package_file.id").
		From("package_file").
		InnerJoin("package_version", "package_version.id = package_file.version_id").
		InnerJoin("package", "package.id = package_version.package_id").
		Where(cond)

	results := make([]struct {
		Name    string
		Version string
		ID      int64
	}, 0, 5)
	err := db.GetEngine(ctx).SQL(query).Find(&results)
	if err != nil {
		return nil, err
	}

	unique := make(map[string]bool)
	for _, info := range results {
		recipe := fmt.Sprintf("%s/%s", info.Name, info.Version)

		props, _ := packages.GetProperties(ctx, packages.PropertyTypeFile, info.ID)
		if len(props) > 0 {
			var (
				user    = ""
				channel = ""
			)
			for _, prop := range props {
				if prop.Name == conan_module.PropertyRecipeUser {
					user = prop.Value
				}
				if prop.Name == conan_module.PropertyRecipeChannel {
					channel = prop.Value
				}
			}
			if user != "" && channel != "" {
				recipe = fmt.Sprintf("%s@%s/%s", recipe, user, channel)
			}
		}

		unique[recipe] = true
	}

	recipes := make([]string, 0, len(unique))
	for recipe := range unique {
		recipes = append(recipes, recipe)
	}
	return recipes, nil
}

// GetPackageInfo gets the Conaninfo for a package
func GetPackageInfo(ctx context.Context, ownerID int64, ref *conan_module.PackageReference) (string, error) {
	values, err := findPropertyValues(
		ctx,
		conan_module.PropertyPackageInfo,
		ownerID,
		ref.Recipe.Name,
		ref.Recipe.Version,
		map[string]string{
			conan_module.PropertyRecipeUser:       ref.Recipe.User,
			conan_module.PropertyRecipeChannel:    ref.Recipe.Channel,
			conan_module.PropertyRecipeRevision:   ref.Recipe.Revision,
			conan_module.PropertyPackageReference: ref.Reference,
			conan_module.PropertyPackageRevision:  ref.Revision,
		},
	)
	if err != nil {
		return "", err
	}

	if len(values) == 0 {
		return "", ErrPackageReferenceNotExist
	}

	return values[0].Value, nil
}
