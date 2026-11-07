// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Package represents a package
type Package struct {
	// The unique identifier of the package
	ID int64 `json:"id"`
	// The owner of the package
	Owner *User `json:"owner"`
	// The repository that contains this package
	Repository *Repository `json:"repository"`
	// The user who created this package
	Creator *User `json:"creator"`
	// The type of the package (e.g., npm, maven, docker)
	Type string `json:"type"`
	// The name of the package
	Name string `json:"name"`
	// The version of the package
	Version string `json:"version"`
	// The HTML URL to view the package
	HTMLURL string `json:"html_url"`
	// swagger:strfmt date-time
	// The date and time when the package was created
	CreatedAt time.Time `json:"created_at"`
	// The type specific metadata of the version. It is only returned when a single version is requested, and only
	// for the package types that publish it.
	Metadata *PackageMetadata `json:"metadata,omitempty"`
}

// PackageMetadata holds the metadata of a package version, in the field of its package type
type PackageMetadata struct {
	// The metadata of a PyPI package
	PyPI *PackagePyPIMetadata `json:"pypi,omitempty"`
	// The metadata of a Cargo package
	Cargo *PackageCargoMetadata `json:"cargo,omitempty"`
}

// PackagePyPIMetadata represents the metadata of a PyPI package version
type PackagePyPIMetadata struct {
	Author          string `json:"author,omitempty"`
	Description     string `json:"description,omitempty"`
	LongDescription string `json:"long_description,omitempty"`
	Summary         string `json:"summary,omitempty"`
	ProjectURL      string `json:"project_url,omitempty"`
	License         string `json:"license,omitempty"`
	RequiresPython  string `json:"requires_python,omitempty"`
}

// PackageCargoMetadata represents the metadata of a Cargo package version
type PackageCargoMetadata struct {
	Dependencies     []*PackageCargoDependency `json:"dependencies,omitempty"`
	Features         map[string][]string       `json:"features,omitempty"`
	Authors          []string                  `json:"authors,omitempty"`
	Description      string                    `json:"description,omitempty"`
	DocumentationURL string                    `json:"documentation_url,omitempty"`
	ProjectURL       string                    `json:"project_url,omitempty"`
	Readme           string                    `json:"readme,omitempty"`
	Keywords         []string                  `json:"keywords,omitempty"`
	Categories       []string                  `json:"categories,omitempty"`
	License          string                    `json:"license,omitempty"`
	RepositoryURL    string                    `json:"repository_url,omitempty"`
	Links            string                    `json:"links,omitempty"`
}

// PackageCargoDependency represents a dependency of a Cargo package version
type PackageCargoDependency struct {
	// The name of the dependency as it is used in Cargo.toml
	Name string `json:"name"`
	// The version requirement
	Req             string   `json:"req"`
	Features        []string `json:"features"`
	Optional        bool     `json:"optional"`
	DefaultFeatures bool     `json:"default_features"`
	// The target the dependency is limited to, e.g. cfg(unix)
	Target *string `json:"target"`
	// The kind of the dependency: normal, dev or build
	Kind     string  `json:"kind"`
	Registry *string `json:"registry"`
	// The real name of the crate if the dependency is renamed in Cargo.toml
	Package *string `json:"package"`
}

// PackageFile represents a package file
type PackageFile struct {
	// The unique identifier of the package file
	ID int64 `json:"id"`
	// The size of the package file in bytes
	Size int64 `json:"size"`
	// The name of the package file
	Name string `json:"name"`
	// The MD5 hash of the package file
	HashMD5 string `json:"md5"`
	// The SHA1 hash of the package file
	HashSHA1 string `json:"sha1"`
	// The SHA256 hash of the package file
	HashSHA256 string `json:"sha256"`
	// The SHA512 hash of the package file
	HashSHA512 string `json:"sha512"`
	// The type specific properties locating the package file, e.g. the distribution and component of a Debian package
	Properties map[string]string `json:"properties,omitempty"`
}
