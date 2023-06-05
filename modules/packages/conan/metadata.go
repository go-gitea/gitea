// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

const (
	PropertyRecipeUser       = "conan.recipe.user"
	PropertyRecipeChannel    = "conan.recipe.channel"
	PropertyRecipeRevision   = "conan.recipe.revision"
	PropertyPackageReference = "conan.package.reference"
	PropertyPackageRevision  = "conan.package.revision"
	PropertyPackageInfo      = "conan.package.info"
)

// Metadata represents the metadata of a Conan package
type Metadata struct {
	Author        string   `json:"author,omitempty"`
	License       string   `json:"license,omitempty"`
	ProjectURL    string   `json:"project_url,omitempty"`
	RepositoryURL string   `json:"repository_url,omitempty"`
	Description   string   `json:"description,omitempty"`
	Keywords      []string `json:"keywords,omitempty"`
}
