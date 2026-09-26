// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildArchive(files map[string]string) []byte {
	return test.WriteTarCompression(gzip.NewWriter, files).Bytes()
}

func TestParseModuleArchive(t *testing.T) {
	archive := buildArchive(map[string]string{
		"./main.tf": `
terraform {
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
		"examples/basic/main.tf":    `variable "ignored" {}`,
		"modules/net/main.tf":       "",
		"modules/net/deep/main.tf":  "",
		"modules/json/main.tf.json": "",
	})
	metadata, err := ParseModuleArchive(bytes.NewReader(archive))
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
		Submodules: []string{"json", "net"},
	}, metadata)

	metadata, err = ParseModuleArchive(bytes.NewReader(buildArchive(map[string]string{"modules/net/main.tf": ""})))
	require.NoError(t, err)
	assert.Equal(t, &Metadata{Submodules: []string{"net"}}, metadata)

	_, err = ParseModuleArchive(bytes.NewReader(buildArchive(map[string]string{"mod-1.0.0/main.tf": ""})))
	assert.ErrorIs(t, err, ErrNoRootModule)
	_, err = ParseModuleArchive(bytes.NewReader(buildArchive(map[string]string{"main.tf": strings.Repeat("#", maxParseSize)})))
	assert.ErrorIs(t, err, ErrArchiveTooLarge)

	for _, src := range []string{`variable "x" { description = "`, `x = <<EOT`, `/*`, `variable {`, `"${"${`, `}}}`} {
		assert.NotNil(t, parseModule(map[string][]byte{"main.tf": []byte(src)}))
	}
}

func TestValidateNameAndProvider(t *testing.T) {
	assert.NoError(t, ValidateNameAndProvider("My_vpc-2", "azurerm"))
	assert.ErrorIs(t, ValidateNameAndProvider("vpc-", "aws"), ErrInvalidName)
	assert.ErrorIs(t, ValidateNameAndProvider("vpc", "my-cloud"), ErrInvalidProvider)
}
