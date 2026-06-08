// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildArchive returns a gzipped tarball containing the named entries.
// dirs lets the caller emit directory entries to exercise the
// "files-not-at-root" skip path.
func buildArchive(t *testing.T, files map[string]string, dirs ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, d := range dirs {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: d, Typeflag: tar.TypeDir, Mode: 0o755}))
	}
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestParseModuleArchive_HappyPath(t *testing.T) {
	main := `
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "region" {
  type        = string
  description = "AWS region"
  default     = "eu-west-1"
}

variable "instance_count" {
  type = number
}

output "vpc_id" {
  description = "Resulting VPC id"
  value       = aws_vpc.this.id
  sensitive   = false
}

resource "aws_vpc" "this" {
  cidr_block = "10.0.0.0/16"
}

data "aws_caller_identity" "current" {}

module "subnets" {
  source  = "terraform-aws-modules/vpc/aws//modules/subnets"
  version = "5.1.2"
}
`
	archive := buildArchive(t, map[string]string{
		"main.tf":   main,
		"README.md": "# example\n",
		// File deeper than root should be ignored, not error.
		"modules/foo/foo.tf": `variable "ignored" { type = string }`,
	}, "modules/", "modules/foo/")

	mod, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.NoError(t, err)
	require.NotNil(t, mod)
	require.NotNil(t, mod.Metadata)
	assert.Equal(t, "# example\n", mod.Metadata.Readme)

	root := mod.Metadata.Root
	require.NotNil(t, root)

	require.Len(t, root.Inputs, 2)
	// sorted by file then file order; only main.tf present, so file order applies.
	assert.Equal(t, "region", root.Inputs[0].Name)
	assert.Equal(t, "AWS region", root.Inputs[0].Description)
	assert.Equal(t, "string", root.Inputs[0].Type)
	assert.Equal(t, `"eu-west-1"`, root.Inputs[0].Default)
	assert.False(t, root.Inputs[0].Required)
	assert.Equal(t, "instance_count", root.Inputs[1].Name)
	assert.True(t, root.Inputs[1].Required)

	require.Len(t, root.Outputs, 1)
	assert.Equal(t, "vpc_id", root.Outputs[0].Name)
	assert.Equal(t, "Resulting VPC id", root.Outputs[0].Description)
	assert.False(t, root.Outputs[0].Sensitive)

	require.Len(t, root.Resources, 2)
	assert.Equal(t, "resource.aws_vpc.this", root.Resources[0].Address)
	assert.False(t, root.Resources[0].IsData)
	assert.Equal(t, "data.aws_caller_identity.current", root.Resources[1].Address)
	assert.True(t, root.Resources[1].IsData)

	require.Len(t, root.Dependencies, 1)
	assert.Equal(t, "subnets", root.Dependencies[0].Name)
	assert.Equal(t, "5.1.2", root.Dependencies[0].Version)

	require.Len(t, root.Providers, 1)
	assert.Equal(t, "aws", root.Providers[0].Name)
	assert.Equal(t, "hashicorp/aws", root.Providers[0].Source)
	assert.Equal(t, "~> 5.0", root.Providers[0].VersionConstraint)

	require.Len(t, root.RequiredCore, 1)
	assert.Equal(t, ">= 1.5.0", root.RequiredCore[0])
}

func TestParseModuleArchive_RejectsTraversal(t *testing.T) {
	// tar.Header.Name with `..` must be refused, regardless of payload.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "../escape.tf",
		Typeflag: tar.TypeReg,
		Size:     0,
		Mode:     0o644,
	}))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	_, err := ParseModuleArchive(&buf, 1<<20)
	require.ErrorIs(t, err, ErrUnsafeArchivePath)
}

func TestParseModuleArchive_EnforcesSizeLimit(t *testing.T) {
	big := strings.Repeat("a", 4096)
	archive := buildArchive(t, map[string]string{
		"main.tf": "variable \"x\" { type = string }\n" + big,
	})
	_, err := ParseModuleArchive(bytes.NewReader(archive), 64)
	require.ErrorIs(t, err, ErrArchiveTooLarge)
}

func TestParseModuleArchive_NoTFFiles(t *testing.T) {
	archive := buildArchive(t, map[string]string{
		"README.md": "# nothing here\n",
	})
	_, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.ErrorIs(t, err, ErrEmptyModule)
}

func TestParseModuleArchive_TFJSONOnly(t *testing.T) {
	archive := buildArchive(t, map[string]string{
		"main.tf.json": `{"variable": {"x": [{"type": "string"}]}}`,
	})
	_, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.ErrorIs(t, err, ErrUnsupportedTFFormat)
}

func TestParseModuleArchive_BadGzip(t *testing.T) {
	_, err := ParseModuleArchive(bytes.NewReader([]byte("not gzip")), 1024)
	require.Error(t, err)
}

func TestParseModuleArchive_MalformedHCL(t *testing.T) {
	archive := buildArchive(t, map[string]string{
		"main.tf": `variable "x" { type = `, // truncated expression
	})
	_, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.Error(t, err)
	// We surface the underlying parse error; ensure it's not one of the
	// known sentinel errors that callers might branch on.
	for _, sentinel := range []error{ErrEmptyModule, ErrUnsafeArchivePath, ErrArchiveTooLarge, ErrUnsupportedTFFormat} {
		require.NotErrorIs(t, err, sentinel)
	}
}

func TestValidateNamespaceNameProvider(t *testing.T) {
	cases := []struct {
		fn   func(string) error
		in   string
		want error
	}{
		{ValidateNamespace, "acme", nil},
		{ValidateNamespace, "acme_corp", nil},
		{ValidateNamespace, "ACME", ErrInvalidNamespace},
		{ValidateNamespace, "", ErrInvalidNamespace},
		{ValidateName, "vpc", nil},
		{ValidateName, "vpc-prod", nil},
		{ValidateName, "vpc/sub", ErrInvalidName},
		{ValidateProvider, "aws", nil},
		{ValidateProvider, "AWS", ErrInvalidProvider},
		{ValidateProvider, "aws_legacy", ErrInvalidProvider}, // providers reject underscores
	}
	for _, c := range cases {
		got := c.fn(c.in)
		require.Equal(t, c.want, got, "input=%q", c.in)
	}
}
