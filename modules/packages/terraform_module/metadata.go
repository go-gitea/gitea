// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

type Metadata struct {
	Readme     string   `json:"readme,omitempty"`
	Root       *Module  `json:"root,omitempty"`
	Submodules []string `json:"submodules,omitempty"`
}

type Module struct {
	Inputs    []*Input    `json:"inputs,omitempty"`
	Outputs   []*Output   `json:"outputs,omitempty"`
	Providers []*Provider `json:"providers,omitempty"`
}

type Input struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
}

type Output struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Provider struct {
	Name    string `json:"name"`
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
}
