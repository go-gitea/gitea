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
variable "region" {
  type        = string
  description = "AWS region"
  default     = "eu-west-1"
}
variable "tags" {
  type = map(string)
}
output "vpc_id" {
  description = "VPC id"
  value       = aws_vpc.this.id
}
`,
		"README.md":                "# vpc\n",
		"._main.tf":                "\x00\x05\x16\x07",
		"examples/basic/main.tf":   `variable "ignored" {}`,
		"modules/net/main.tf":      `output "id" { value = "x" }`,
		"modules/net/deep/main.tf": `variable "ignored" {}`,
	})
	metadata, err := ParseModuleArchive(bytes.NewReader(archive))
	require.NoError(t, err)
	assert.Equal(t, &Metadata{
		Readme: "# vpc\n",
		Root: &Module{
			Inputs: []*Input{
				{Name: "region", Type: "string", Description: "AWS region", Default: `"eu-west-1"`},
				{Name: "tags", Type: "map(string)", Required: true},
			},
			Outputs: []*Output{{Name: "vpc_id", Description: "VPC id"}},
			Providers: []*Provider{
				{Name: "aws", Source: "hashicorp/aws", Version: "~> 5.0"},
				{Name: "random", Version: "~> 3.0"},
			},
		},
		Submodules: []*Module{{Name: "net", Outputs: []*Output{{Name: "id"}}}},
	}, metadata)

	metadata, err = ParseModuleArchive(bytes.NewReader(buildArchive(map[string]string{"modules/net/main.tf": `variable "cidr" {}`})))
	require.NoError(t, err)
	assert.Nil(t, metadata.Root)
	assert.Len(t, metadata.Submodules, 1)
}

func TestParseModuleArchiveErrors(t *testing.T) {
	cases := map[string]struct {
		archive []byte
		err     error
	}{
		"wrapped module":     {buildArchive(map[string]string{"mod-1.0.0/main.tf": `variable "x" {}`}), ErrNoRootModule},
		"wrapped collection": {buildArchive(map[string]string{"mod-1.0.0/modules/net/main.tf": `variable "x" {}`}), ErrNoRootModule},
		"examples only":      {buildArchive(map[string]string{"README.md": "", "examples/basic/main.tf": `variable "x" {}`}), ErrNoRootModule},
		"tf.json only":       {buildArchive(map[string]string{"main.tf.json": `{}`}), ErrUnsupportedTFFormat},
		"too large":          {buildArchive(map[string]string{"main.tf": strings.Repeat("#", maxParseSize)}), ErrArchiveTooLarge},
		"malformed hcl":      {buildArchive(map[string]string{"main.tf": `variable "x" {`}), nil},
		"not gzip":           {[]byte("not gzip"), nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseModuleArchive(bytes.NewReader(c.archive))
			require.Error(t, err)
			if c.err != nil {
				assert.ErrorIs(t, err, c.err)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	for in, want := range map[string]string{"1.0.0": "1.0.0", "v1.0.0": "1.0.0", "1.0": "1.0.0", "v1.2.3-rc.1": "1.2.3-rc.1"} {
		got, err := NormalizeVersion(in)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	for _, in := range []string{"", "not-semver", "V1.0.0"} {
		_, err := NormalizeVersion(in)
		assert.ErrorIs(t, err, ErrInvalidVersion)
	}
}

func TestValidateNameAndProvider(t *testing.T) {
	assert.NoError(t, ValidateNameAndProvider("vpc", "aws"))
	assert.NoError(t, ValidateNameAndProvider("My_vpc-2", "azurerm"))
	assert.ErrorIs(t, ValidateNameAndProvider("vpc-", "aws"), ErrInvalidName)
	assert.ErrorIs(t, ValidateNameAndProvider("vpc/sub", "aws"), ErrInvalidName)
	assert.ErrorIs(t, ValidateNameAndProvider("vpc", "AWS"), ErrInvalidProvider)
	assert.ErrorIs(t, ValidateNameAndProvider("vpc", "my-cloud"), ErrInvalidProvider)
}
