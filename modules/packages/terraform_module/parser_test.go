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
	assert.Empty(t, mod.RootDir, "flat archive: module sits at the root")

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

func TestParseModuleArchive_WrappedSingleDir(t *testing.T) {
	// A GitHub-style release tarball wraps the whole module in one
	// top-level directory. The parser must descend into it and report the
	// directory via RootDir so the upload handler can normalize it to flat.
	archive := buildArchive(t, map[string]string{
		"mod-1.0.0/main.tf":                `variable "region" { type = string }`,
		"mod-1.0.0/README.md":              "# wrapped\n",
		"mod-1.0.0/examples/basic/main.tf": `variable "ignored" { type = string }`,
	}, "mod-1.0.0/", "mod-1.0.0/examples/", "mod-1.0.0/examples/basic/")

	mod, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.NoError(t, err)
	require.NotNil(t, mod.Metadata.Root)
	assert.Equal(t, "mod-1.0.0", mod.RootDir)
	assert.Equal(t, "# wrapped\n", mod.Metadata.Readme)
	require.Len(t, mod.Metadata.Root.Inputs, 1)
	assert.Equal(t, "region", mod.Metadata.Root.Inputs[0].Name)
}

func TestNormalizeArchive(t *testing.T) {
	// A wrapped archive must be rewritten so the wrapper directory's
	// contents become the archive root: examples/ is re-rooted, a stray
	// file outside the wrapper is dropped, and the result parses flat.
	wrapped := buildArchive(t, map[string]string{
		"mod-1.0.0/main.tf":                `variable "region" { type = string }`,
		"mod-1.0.0/README.md":              "# wrapped\n",
		"mod-1.0.0/examples/basic/main.tf": `variable "ignored" { type = string }`,
		"stray.txt":                        "outside the module",
	}, "mod-1.0.0/", "mod-1.0.0/examples/", "mod-1.0.0/examples/basic/")

	var flat bytes.Buffer
	require.NoError(t, NormalizeArchive(&flat, bytes.NewReader(wrapped), "mod-1.0.0"))

	// Inspect the rewritten entries.
	names := map[string]bool{}
	gz, err := gzip.NewReader(bytes.NewReader(flat.Bytes()))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names[hdr.Name] = true
	}
	assert.True(t, names["main.tf"], "module file should be at the root")
	assert.True(t, names["README.md"], "readme should be re-rooted")
	assert.True(t, names["examples/basic/main.tf"], "examples should be preserved, re-rooted")
	assert.False(t, names["mod-1.0.0/main.tf"], "wrapper prefix must be gone")
	assert.False(t, names["stray.txt"], "entries outside the wrapper must be dropped")

	// The normalized archive must now parse as a flat module.
	mod, err := ParseModuleArchive(bytes.NewReader(flat.Bytes()), 1<<20)
	require.NoError(t, err)
	assert.Empty(t, mod.RootDir)
	require.Len(t, mod.Metadata.Root.Inputs, 1)
	assert.Equal(t, "region", mod.Metadata.Root.Inputs[0].Name)
	assert.Equal(t, "# wrapped\n", mod.Metadata.Readme)
}

func TestParseModuleArchive_IgnoresAppleDoubleJunk(t *testing.T) {
	// macOS tar emits `._foo` AppleDouble sidecars and a `__MACOSX/` tree.
	// `._main.tf` would crash the HCL parser if not filtered out.
	archive := buildArchive(t, map[string]string{
		"main.tf":            `variable "x" { type = string }`,
		"._main.tf":          "\x00\x05\x16\x07garbage",
		"._README.md":        "\x00garbage",
		"__MACOSX/._main.tf": "\x00garbage",
	}, "__MACOSX/")

	mod, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.NoError(t, err)
	assert.Empty(t, mod.RootDir)
	require.Len(t, mod.Metadata.Root.Inputs, 1)
	assert.Equal(t, "x", mod.Metadata.Root.Inputs[0].Name)
}

func TestParseModuleArchive_FlatWinsOverSubdir(t *testing.T) {
	// Root-level .tf files take precedence: a sibling examples/ directory
	// must not be mistaken for the module root.
	archive := buildArchive(t, map[string]string{
		"main.tf":              `variable "x" { type = string }`,
		"examples/basic/ex.tf": `variable "y" { type = string }`,
	}, "examples/", "examples/basic/")

	mod, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.NoError(t, err)
	assert.Empty(t, mod.RootDir)
	require.Len(t, mod.Metadata.Root.Inputs, 1)
	assert.Equal(t, "x", mod.Metadata.Root.Inputs[0].Name)
}

func TestParseModuleArchive_AmbiguousTopLevelDirs(t *testing.T) {
	// No root .tf and more than one top-level directory: the module root
	// is undeterminable and must be rejected, not guessed.
	archive := buildArchive(t, map[string]string{
		"a/main.tf": `variable "x" { type = string }`,
		"b/main.tf": `variable "y" { type = string }`,
	}, "a/", "b/")

	_, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.ErrorIs(t, err, ErrAmbiguousModuleRoot)
}

func TestParseModuleArchive_WrappedTFJSONOnly(t *testing.T) {
	// A wrapped module whose only sources are .tf.json must surface the
	// unsupported-format error, just like the flat case.
	archive := buildArchive(t, map[string]string{
		"mod/main.tf.json": `{"variable": {"x": [{"type": "string"}]}}`,
	}, "mod/")

	_, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.ErrorIs(t, err, ErrUnsupportedTFFormat)
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

// TestParseModuleArchive_DecompressionBomb proves the hard parse ceiling
// clamps an "unlimited" (-1) caller: a small archive that decompresses to
// more than maxParseSize must be rejected rather than buffered whole.
func TestParseModuleArchive_DecompressionBomb(t *testing.T) {
	// A single .tf entry whose decompressed body exceeds maxParseSize.
	// Highly compressible content keeps the archive itself tiny.
	bomb := strings.Repeat("a", maxParseSize+(1<<20))
	archive := buildArchive(t, map[string]string{"main.tf": bomb})

	// Sanity: the compressed archive is orders of magnitude smaller than
	// the ceiling, so only the decompressed-size guard can catch it.
	require.Less(t, len(archive), 1<<20)

	// maxSize = -1 means "unlimited storage quota"; the parser must still
	// clamp to maxParseSize and refuse the bomb.
	_, err := ParseModuleArchive(bytes.NewReader(archive), -1)
	require.ErrorIs(t, err, ErrArchiveTooLarge)
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

func TestValidateNameProvider(t *testing.T) {
	cases := []struct {
		fn   func(string) error
		in   string
		want error
	}{
		{ValidateName, "vpc", nil},
		{ValidateName, "vpc-prod", nil},
		{ValidateName, "vpc/sub", ErrInvalidName},
		{ValidateName, "", ErrInvalidName},
		{ValidateName, "VPC", ErrInvalidName},
		{ValidateProvider, "aws", nil},
		{ValidateProvider, "AWS", ErrInvalidProvider},
		{ValidateProvider, "aws_legacy", ErrInvalidProvider}, // providers reject underscores
	}
	for _, c := range cases {
		got := c.fn(c.in)
		require.Equal(t, c.want, got, "input=%q", c.in)
	}
}
