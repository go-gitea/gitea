// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

// Metadata represents the parsed metadata of a Terraform module package.
// Its shape mirrors the relevant fields of the HashiCorp public registry
// response so that consumers of the module-registry protocol see a
// familiar payload.
type Metadata struct {
	Description string                 `json:"description,omitempty"`
	Readme      string                 `json:"readme,omitempty"`
	Root        *Root                  `json:"root,omitempty"`
	Source      string                 `json:"source,omitempty"`
	Providers   []*ProviderRequirement `json:"providers,omitempty"`
	// ModuleDir is the directory inside the archive that holds the root
	// module. Empty means the .tf files sit at the archive root; a
	// non-empty value means they are wrapped in a single top-level
	// directory (e.g. a GitHub release tarball). The download handler
	// uses this to decide whether to append the go-getter `//*` subdir
	// glob to the X-Terraform-Get header.
	ModuleDir string `json:"module_dir,omitempty"`
}

// Root describes the root module contents extracted from the archive.
// Submodules and examples are intentionally omitted in v1: only the root
// module is parsed.
type Root struct {
	RequiredCore []string               `json:"required_core,omitempty"`
	Inputs       []*Input               `json:"inputs,omitempty"`
	Outputs      []*Output              `json:"outputs,omitempty"`
	Resources    []*Resource            `json:"resources,omitempty"`
	Dependencies []*ModuleReference     `json:"dependencies,omitempty"`
	Providers    []*ProviderRequirement `json:"providers,omitempty"`
}

// Input represents a Terraform `variable "name" { ... }` block.
type Input struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive,omitempty"`
}

// Output represents a Terraform `output "name" { ... }` block.
type Output struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
}

// Resource represents a `resource` or `data` block as `type.name`.
type Resource struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	IsData  bool   `json:"is_data,omitempty"`
	Address string `json:"address"`
}

// ModuleReference represents a `module "name" { source = ... }` block.
type ModuleReference struct {
	Name    string `json:"name"`
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
}

// ProviderRequirement represents an entry of
// `terraform { required_providers { ... } }`.
type ProviderRequirement struct {
	Name              string `json:"name"`
	Source            string `json:"source,omitempty"`
	VersionConstraint string `json:"version,omitempty"`
}
