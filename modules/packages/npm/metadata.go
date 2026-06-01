// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

// TagProperty is the name of the property for tag management
const TagProperty = "npm.tag"

// Metadata represents the metadata of a npm package
type Metadata struct {
	Scope                   string            `json:"scope,omitempty"`
	Name                    string            `json:"name,omitempty"`
	Description             string            `json:"description,omitempty"`
	Author                  string            `json:"author,omitempty"`
	License                 License           `json:"license,omitempty"`
	ProjectURL              string            `json:"project_url,omitempty"`
	Keywords                []string          `json:"keywords,omitempty"`
	Dependencies            map[string]string `json:"dependencies,omitempty"`
	BundleDependencies      []string          `json:"bundleDependencies,omitempty"`
	DevelopmentDependencies map[string]string `json:"development_dependencies,omitempty"`
	PeerDependencies        map[string]string `json:"peer_dependencies,omitempty"`
	PeerDependenciesMeta    map[string]any    `json:"peer_dependencies_meta,omitempty"`
	OptionalDependencies    map[string]string `json:"optional_dependencies,omitempty"`
	Bin                     map[string]string `json:"bin,omitempty"`
	Readme                  string            `json:"readme,omitempty"`
	Repository              Repository        `json:"repository"`
	Scripts                 map[string]string `json:"scripts,omitempty"`
	HasInstallScript        bool              `json:"hasInstallScript,omitempty"`
	HasShrinkwrap           bool              `json:"hasShrinkwrap,omitempty"`
	Engines                 map[string]string `json:"engines,omitempty"`
	CPU                     []string          `json:"cpu,omitempty"`
	OS                      []string          `json:"os,omitempty"`
	Directories             map[string]string `json:"directories,omitempty"`
	Funding                 any               `json:"funding,omitempty"`
	AcceptDependencies      map[string]string `json:"accept_dependencies,omitempty"`
	Deprecated              string            `json:"deprecated,omitempty"`
}
