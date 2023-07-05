// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rubygems

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"gopkg.in/yaml.v3"
)

var (
	// ErrMissingMetadataFile indicates a missing metadata.gz file
	ErrMissingMetadataFile = util.NewInvalidArgumentErrorf("metadata.gz file is missing")
	// ErrInvalidName indicates an invalid id in the metadata.gz file
	ErrInvalidName = util.NewInvalidArgumentErrorf("package name is invalid")
	// ErrInvalidVersion indicates an invalid version in the metadata.gz file
	ErrInvalidVersion = util.NewInvalidArgumentErrorf("package version is invalid")
)

var versionMatcher = regexp.MustCompile(`\A[0-9]+(?:\.[0-9a-zA-Z]+)*(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z`)

// Package represents a RubyGems package
type Package struct {
	Name     string
	Version  string
	Metadata *Metadata
}

// Metadata represents the metadata of a RubyGems package
type Metadata struct {
	Platform                string               `json:"platform,omitempty"`
	Description             string               `json:"description,omitempty"`
	Summary                 string               `json:"summary,omitempty"`
	Authors                 []string             `json:"authors,omitempty"`
	Licenses                []string             `json:"licenses,omitempty"`
	RequiredRubyVersion     []VersionRequirement `json:"required_ruby_version,omitempty"`
	RequiredRubygemsVersion []VersionRequirement `json:"required_rubygems_version,omitempty"`
	ProjectURL              string               `json:"project_url,omitempty"`
	RuntimeDependencies     []Dependency         `json:"runtime_dependencies,omitempty"`
	DevelopmentDependencies []Dependency         `json:"development_dependencies,omitempty"`
}

// VersionRequirement represents a version restriction
type VersionRequirement struct {
	Restriction string `json:"restriction"`
	Version     string `json:"version"`
}

// Dependency represents a dependency of a RubyGems package
type Dependency struct {
	Name    string               `json:"name"`
	Version []VersionRequirement `json:"version"`
}

type gemspec struct {
	Name    string `yaml:"name"`
	Version struct {
		Version string `yaml:"version"`
	} `yaml:"version"`
	Platform     string   `yaml:"platform"`
	Authors      []string `yaml:"authors"`
	Autorequire  any      `yaml:"autorequire"`
	Bindir       string   `yaml:"bindir"`
	CertChain    []any    `yaml:"cert_chain"`
	Date         string   `yaml:"date"`
	Dependencies []struct {
		Name                string      `yaml:"name"`
		Requirement         requirement `yaml:"requirement"`
		Type                string      `yaml:"type"`
		Prerelease          bool        `yaml:"prerelease"`
		VersionRequirements requirement `yaml:"version_requirements"`
	} `yaml:"dependencies"`
	Description    string   `yaml:"description"`
	Executables    []string `yaml:"executables"`
	Extensions     []any    `yaml:"extensions"`
	ExtraRdocFiles []string `yaml:"extra_rdoc_files"`
	Files          []string `yaml:"files"`
	Homepage       string   `yaml:"homepage"`
	Licenses       []string `yaml:"licenses"`
	Metadata       struct {
		BugTrackerURI    string `yaml:"bug_tracker_uri"`
		ChangelogURI     string `yaml:"changelog_uri"`
		DocumentationURI string `yaml:"documentation_uri"`
		SourceCodeURI    string `yaml:"source_code_uri"`
	} `yaml:"metadata"`
	PostInstallMessage      any         `yaml:"post_install_message"`
	RdocOptions             []any       `yaml:"rdoc_options"`
	RequirePaths            []string    `yaml:"require_paths"`
	RequiredRubyVersion     requirement `yaml:"required_ruby_version"`
	RequiredRubygemsVersion requirement `yaml:"required_rubygems_version"`
	Requirements            []any       `yaml:"requirements"`
	RubygemsVersion         string      `yaml:"rubygems_version"`
	SigningKey              any         `yaml:"signing_key"`
	SpecificationVersion    int         `yaml:"specification_version"`
	Summary                 string      `yaml:"summary"`
	TestFiles               []any       `yaml:"test_files"`
}

type requirement struct {
	Requirements [][]any `yaml:"requirements"`
}

// AsVersionRequirement converts into []VersionRequirement
func (r requirement) AsVersionRequirement() []VersionRequirement {
	requirements := make([]VersionRequirement, 0, len(r.Requirements))
	for _, req := range r.Requirements {
		if len(req) != 2 {
			continue
		}
		restriction, ok := req[0].(string)
		if !ok {
			continue
		}
		vm, ok := req[1].(map[string]any)
		if !ok {
			continue
		}
		versionInt, ok := vm["version"]
		if !ok {
			continue
		}
		version, ok := versionInt.(string)
		if !ok || version == "0" {
			continue
		}

		requirements = append(requirements, VersionRequirement{
			Restriction: restriction,
			Version:     version,
		})
	}
	return requirements
}

// ParsePackageMetaData parses the metadata of a Gem package file
func ParsePackageMetaData(r io.Reader) (*Package, error) {
	archive := tar.NewReader(r)
	for {
		hdr, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == "metadata.gz" {
			return parseMetadataFile(archive)
		}
	}

	return nil, ErrMissingMetadataFile
}

func parseMetadataFile(r io.Reader) (*Package, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var spec gemspec
	if err := yaml.NewDecoder(zr).Decode(&spec); err != nil {
		return nil, err
	}

	if len(spec.Name) == 0 || strings.Contains(spec.Name, "/") {
		return nil, ErrInvalidName
	}

	if !versionMatcher.MatchString(spec.Version.Version) {
		return nil, ErrInvalidVersion
	}

	if !validation.IsValidURL(spec.Homepage) {
		spec.Homepage = ""
	}
	if !validation.IsValidURL(spec.Metadata.SourceCodeURI) {
		spec.Metadata.SourceCodeURI = ""
	}

	m := &Metadata{
		Platform:                spec.Platform,
		Description:             spec.Description,
		Summary:                 spec.Summary,
		Authors:                 spec.Authors,
		Licenses:                spec.Licenses,
		ProjectURL:              spec.Homepage,
		RequiredRubyVersion:     spec.RequiredRubyVersion.AsVersionRequirement(),
		RequiredRubygemsVersion: spec.RequiredRubygemsVersion.AsVersionRequirement(),
		DevelopmentDependencies: make([]Dependency, 0, 5),
		RuntimeDependencies:     make([]Dependency, 0, 5),
	}

	for _, gemdep := range spec.Dependencies {
		dep := Dependency{
			Name:    gemdep.Name,
			Version: gemdep.Requirement.AsVersionRequirement(),
		}
		if gemdep.Type == ":runtime" {
			m.RuntimeDependencies = append(m.RuntimeDependencies, dep)
		} else {
			m.DevelopmentDependencies = append(m.DevelopmentDependencies, dep)
		}
	}

	return &Package{
		Name:     spec.Name,
		Version:  spec.Version.Version,
		Metadata: m,
	}, nil
}
