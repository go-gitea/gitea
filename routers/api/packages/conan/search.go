// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

import (
	"errors"
	"net/http"
	"strings"

	conan_model "code.gitea.io/gitea/models/packages/conan"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	conan_module "code.gitea.io/gitea/modules/packages/conan"
	"code.gitea.io/gitea/services/context"
)

// SearchResult contains the found recipe names
type SearchResult struct {
	Results []string `json:"results"`
}

// SearchRecipes searches all recipes matching the query
func SearchRecipes(ctx *context.Context) {
	q := ctx.FormTrim("q")

	opts := parseQuery(ctx.Package.Owner, q)

	results, err := conan_model.SearchRecipes(ctx, opts)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	jsonResponse(ctx, http.StatusOK, &SearchResult{
		Results: results,
	})
}

// parseQuery creates search options for the given query
func parseQuery(owner *user_model.User, query string) *conan_model.RecipeSearchOptions {
	opts := &conan_model.RecipeSearchOptions{
		OwnerID: owner.ID,
	}

	if query != "" {
		parts := strings.Split(strings.ReplaceAll(query, "@", "/"), "/")

		opts.Name = parts[0]
		if len(parts) > 1 && parts[1] != "*" {
			opts.Version = parts[1]
		}
		if len(parts) > 2 && parts[2] != "*" {
			opts.User = parts[2]
		}
		if len(parts) > 3 && parts[3] != "*" {
			opts.Channel = parts[3]
		}
	}

	return opts
}

// SearchPackagesV1 searches all packages of a recipe (Conan v1 endpoint)
func SearchPackagesV1(ctx *context.Context) {
	searchPackages(ctx, true)
}

// SearchPackagesV2 searches all packages of a recipe (Conan v2 endpoint)
func SearchPackagesV2(ctx *context.Context) {
	searchPackages(ctx, false)
}

func searchPackages(ctx *context.Context, searchAllRevisions bool) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	if !searchAllRevisions && rref.Revision == "" {
		lastRevision, err := conan_model.GetLastRecipeRevision(ctx, ctx.Package.Owner.ID, rref)
		if err != nil {
			if errors.Is(err, conan_model.ErrRecipeReferenceNotExist) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}
		rref = rref.WithRevision(lastRevision.Value)
	} else {
		has, err := conan_model.RecipeExists(ctx, ctx.Package.Owner.ID, rref)
		if err != nil {
			if errors.Is(err, conan_model.ErrRecipeReferenceNotExist) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}
		if !has {
			apiError(ctx, http.StatusNotFound, nil)
			return
		}
	}

	recipeRevisions := []*conan_model.PropertyValue{{Value: rref.Revision}}
	if searchAllRevisions {
		var err error
		recipeRevisions, err = conan_model.GetRecipeRevisions(ctx, ctx.Package.Owner.ID, rref)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	result := make(map[string]*conan_module.Conaninfo)

	for _, recipeRevision := range recipeRevisions {
		currentRef := rref
		if recipeRevision.Value != "" {
			currentRef = rref.WithRevision(recipeRevision.Value)
		}
		packageReferences, err := conan_model.GetPackageReferences(ctx, ctx.Package.Owner.ID, currentRef)
		if err != nil {
			if errors.Is(err, conan_model.ErrRecipeReferenceNotExist) {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}
		for _, packageReference := range packageReferences {
			if _, ok := result[packageReference.Value]; ok {
				continue
			}
			pref, _ := conan_module.NewPackageReference(currentRef, packageReference.Value, "")
			lastPackageRevision, err := conan_model.GetLastPackageRevision(ctx, ctx.Package.Owner.ID, pref)
			if err != nil {
				if errors.Is(err, conan_model.ErrPackageReferenceNotExist) {
					apiError(ctx, http.StatusNotFound, err)
				} else {
					apiError(ctx, http.StatusInternalServerError, err)
				}
				return
			}
			pref = pref.WithRevision(lastPackageRevision.Value)
			infoRaw, err := conan_model.GetPackageInfo(ctx, ctx.Package.Owner.ID, pref)
			if err != nil {
				if errors.Is(err, conan_model.ErrPackageReferenceNotExist) {
					apiError(ctx, http.StatusNotFound, err)
				} else {
					apiError(ctx, http.StatusInternalServerError, err)
				}
				return
			}
			var info *conan_module.Conaninfo
			if err := json.Unmarshal([]byte(infoRaw), &info); err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			result[pref.Reference] = info
		}
	}

	jsonResponse(ctx, http.StatusOK, result)
}
