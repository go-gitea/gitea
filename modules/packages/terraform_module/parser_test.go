// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"bytes"
	"compress/gzip"
	"slices"
	"strings"
	"testing"

	packages_module "gitea.dev/modules/packages"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildArchive(files map[string]string) *bytes.Buffer {
	return test.WriteTarCompression(gzip.NewWriter, files)
}

func TestParseModuleArchive(t *testing.T) {
	metadata, err := ParseModuleArchive(buildArchive(map[string]string{
		"./main.tf": "\xef\xbb\xbf" + `terraform {
  required_providers {
    aws = {
      source                = "hashicorp/aws"
      version               = "~> 5.0"
      configuration_aliases = [aws.alternate]
    }
    random = "~> 3.0"
  }
}
# an unbalanced { in a comment
locals {
  joined = "%{ for k, v in { a = "b" } }${k}%{ endfor }"
}
variable "region" {
  type        = string
  description = "AWS region, not $${var.template} }"
  default     = "eu-west-1" // trailing comment
}
variable "tags" {
  type = map(string)
  description = <<EOT
Tags }
EOT
}
output "vpc_id" {
  description = "VPC id"
  value       = aws_vpc.this.id
}
`,
		"README.md":                 "# vpc\n",
		"._main.tf":                 `variable "ignored" {}`,
		"backup.TF":                 `variable "ignored" {}`,
		"examples/basic/main.tf":    `variable "ignored" {}`,
		"modules/net/main.tf":       "variable \"cidr\" {}\n",
		"modules/net/README.md":     "# Net submodule\n",
		"modules/net/LICENSE":       "must not become the readme",
		"modules/net/deep/main.tf":  "",
		"modules/json/main.tf.json": "",
	}))
	require.NoError(t, err)
	assert.Equal(t, &Metadata{
		Readme: "# vpc\n",
		Root: &Module{
			Inputs: []*Input{
				{Name: "region", Type: "string", Description: "AWS region, not ${var.template} }", Default: `"eu-west-1"`},
				{Name: "tags", Type: "map(string)", Description: "Tags }\n", Required: true},
			},
			Outputs: []*Output{{Name: "vpc_id", Description: "VPC id"}},
			Providers: []*Provider{
				{Name: "aws", Source: "hashicorp/aws", Version: "~> 5.0"},
				{Name: "random", Version: "~> 3.0"},
			},
		},
		Submodules: map[string]*Submodule{
			"json": {Module: &Module{}},
			"net":  {Readme: "# Net submodule\n", Module: &Module{Inputs: []*Input{{Name: "cidr", Required: true}}}},
		},
	}, metadata)

	metadata, err = ParseModuleArchive(buildArchive(map[string]string{"modules/net/main.tf": ""}))
	require.NoError(t, err)
	assert.Equal(t, &Metadata{Submodules: map[string]*Submodule{"net": {Module: &Module{}}}}, metadata)

	metadata, err = ParseModuleArchive(buildArchive(map[string]string{
		"main.tf":               "",
		"README.md":             strings.Repeat("#", maxReadmeSize+1),
		"modules/net/main.tf":   "",
		"modules/net/README.md": strings.Repeat("#", maxReadmeSize+1),
	}))
	require.NoError(t, err)
	assert.Equal(t, &Metadata{Root: &Module{}, Submodules: map[string]*Submodule{"net": {Module: &Module{}}}}, metadata)

	_, err = ParseModuleArchive(buildArchive(map[string]string{"mod-1.0.0/main.tf": ""}))
	assert.ErrorIs(t, err, ErrNoRootModule)
	_, err = ParseModuleArchive(buildArchive(map[string]string{"main.tf": strings.Repeat("#", packages_module.MaxMetadataSize)}))
	assert.ErrorIs(t, err, packages_module.ErrContentTooLarge)

	for _, src := range []string{`variable "x" { description = "`, `variable "x" { description = "\`, `x = <<EOT`, `/*`, `variable {`, `"${"${`, `}}}`} {
		assert.NotNil(t, parseModule(map[string][]byte{"main.tf": slices.Clip([]byte(src))}))
	}
}

func TestValidateAddress(t *testing.T) {
	assert.NoError(t, ValidateAddress("org", "My_vpc-2", "azurerm"))
	assert.ErrorIs(t, ValidateAddress("john.doe", "vpc", "aws"), ErrInvalidNamespace)
	assert.ErrorIs(t, ValidateAddress("org", "vpc-", "aws"), ErrInvalidName)
	assert.ErrorIs(t, ValidateAddress("org", "vpc", "my-cloud"), ErrInvalidProvider)
}
