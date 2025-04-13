// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cargo

import (
	"encoding/binary"
	"errors"
	"io"
	"regexp"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
)

const PropertyYanked = "cargo.yanked"

var (
	ErrInvalidName    = errors.New("package name is invalid")
	ErrInvalidVersion = errors.New("package version is invalid")
)

// Package represents a Cargo package
type Package struct {
	Name        string
	Version     string
	Metadata    *Metadata
	Content     io.Reader
	ContentSize int64
}

// Metadata represents the metadata of a Cargo package
type Metadata struct {
	Dependencies     []*Dependency       `json:"dependencies,omitempty"`
	Features         map[string][]string `json:"features,omitempty"`
	Authors          []string            `json:"authors,omitempty"`
	Description      string              `json:"description,omitempty"`
	DocumentationURL string              `json:"documentation_url,omitempty"`
	ProjectURL       string              `json:"project_url,omitempty"`
	Readme           string              `json:"readme,omitempty"`
	Keywords         []string            `json:"keywords,omitempty"`
	Categories       []string            `json:"categories,omitempty"`
	License          string              `json:"license,omitempty"`
	RepositoryURL    string              `json:"repository_url,omitempty"`
	Links            string              `json:"links,omitempty"`
}

type Dependency struct {
	Name            string   `json:"name"`
	Req             string   `json:"req"`
	Features        []string `json:"features"`
	Optional        bool     `json:"optional"`
	DefaultFeatures bool     `json:"default_features"`
	Target          *string  `json:"target"`
	Kind            string   `json:"kind"`
	Registry        *string  `json:"registry"`
	Package         *string  `json:"package"`
}

var nameMatch = regexp.MustCompile(`\A[a-zA-Z][a-zA-Z0-9-_]{0,63}\z`)

// ParsePackage reads the metadata and content of a package
func ParsePackage(r io.Reader) (*Package, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return nil, err
	}

	p, err := parsePackage(io.LimitReader(r, int64(size)))
	if err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return nil, err
	}

	p.Content = io.LimitReader(r, int64(size))
	p.ContentSize = int64(size)

	return p, nil
}

func parsePackage(r io.Reader) (*Package, error) {
	var meta struct {
		Name string `json:"name"`
		Vers string `json:"vers"`
		Deps []struct {
			Name               string   `json:"name"`
			VersionReq         string   `json:"version_req"`
			Features           []string `json:"features"`
			Optional           bool     `json:"optional"`
			DefaultFeatures    bool     `json:"default_features"`
			Target             *string  `json:"target"`
			Kind               string   `json:"kind"`
			Registry           *string  `json:"registry"`
			ExplicitNameInToml string   `json:"explicit_name_in_toml"`
		} `json:"deps"`
		Features      map[string][]string `json:"features"`
		Authors       []string            `json:"authors"`
		Description   string              `json:"description"`
		Documentation string              `json:"documentation"`
		Homepage      string              `json:"homepage"`
		Readme        string              `json:"readme"`
		ReadmeFile    string              `json:"readme_file"`
		Keywords      []string            `json:"keywords"`
		Categories    []string            `json:"categories"`
		License       string              `json:"license"`
		LicenseFile   string              `json:"license_file"`
		Repository    string              `json:"repository"`
		Links         string              `json:"links"`
	}
	if err := json.NewDecoder(r).Decode(&meta); err != nil {
		return nil, err
	}

	if !nameMatch.MatchString(meta.Name) {
		return nil, ErrInvalidName
	}

	if _, err := version.NewSemver(meta.Vers); err != nil {
		return nil, ErrInvalidVersion
	}

	if !validation.IsValidURL(meta.Homepage) {
		meta.Homepage = ""
	}
	if !validation.IsValidURL(meta.Documentation) {
		meta.Documentation = ""
	}
	if !validation.IsValidURL(meta.Repository) {
		meta.Repository = ""
	}

	dependencies := make([]*Dependency, 0, len(meta.Deps))
	for _, dep := range meta.Deps {
		// https://doc.rust-lang.org/cargo/reference/registry-web-api.html#publish
		// It is a string of the new package name if the dependency is renamed, otherwise empty
		name := dep.ExplicitNameInToml
		pkg := &dep.Name
		if name == "" {
			name = dep.Name
			pkg = nil
		}
		dependencies = append(dependencies, &Dependency{
			Name:            name,
			Req:             dep.VersionReq,
			Features:        dep.Features,
			Optional:        dep.Optional,
			DefaultFeatures: dep.DefaultFeatures,
			Target:          dep.Target,
			Kind:            dep.Kind,
			Registry:        dep.Registry,
			Package:         pkg,
		})
	}

	return &Package{
		Name:    meta.Name,
		Version: meta.Vers,
		Metadata: &Metadata{
			Dependencies:     dependencies,
			Features:         meta.Features,
			Authors:          meta.Authors,
			Description:      meta.Description,
			DocumentationURL: meta.Documentation,
			ProjectURL:       meta.Homepage,
			Readme:           meta.Readme,
			Keywords:         meta.Keywords,
			Categories:       meta.Categories,
			License:          meta.License,
			RepositoryURL:    meta.Repository,
			Links:            meta.Links,
		},
	}, nil
}
