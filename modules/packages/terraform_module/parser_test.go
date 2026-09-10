// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
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

func TestParseModuleArchive_RejectsWrappedArchive(t *testing.T) {
	// Archives are stored verbatim, so a module wrapped in a single
	// top-level directory (a GitHub release tarball) would be stored
	// unusable: nothing at the root for terraform to find. Reject it.
	for name, files := range map[string]map[string]string{
		"single root module": {
			"mod-1.0.0/main.tf":   `variable "region" { type = string }`,
			"mod-1.0.0/README.md": "# wrapped\n",
		},
		"collection": {
			"mod-1.0.0/modules/network/main.tf": `variable "cidr" { type = string }`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseModuleArchive(bytes.NewReader(buildArchive(t, files, "mod-1.0.0/")), 1<<20)
			require.ErrorIs(t, err, ErrNoRootModule)
		})
	}
}

func TestParseModuleArchive_RejectsExamplesOnly(t *testing.T) {
	// .tf files that only live under examples/ (or deeper than a submodule)
	// are not a consumable module.
	archive := buildArchive(t, map[string]string{
		"README.md":                  "# nope\n",
		"examples/basic/main.tf":     `variable "x" { type = string }`,
		"modules/a/deep/nested/x.tf": `variable "y" { type = string }`,
	}, "examples/", "examples/basic/", "modules/", "modules/a/", "modules/a/deep/", "modules/a/deep/nested/")
	_, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.ErrorIs(t, err, ErrNoRootModule)
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
	require.Len(t, mod.Metadata.Root.Inputs, 1)
	assert.Equal(t, "x", mod.Metadata.Root.Inputs[0].Name)
}

func TestParseModuleArchive_SubmoduleCollectionFlat(t *testing.T) {
	// A collection with no root module — only submodules under modules/ —
	// is valid. It must be accepted (not rejected) with empty root
	// metadata, and modules/ must not be mistaken for a wrapper directory.
	archive := buildArchive(t, map[string]string{
		"README.md":                  "# collection\n",
		"modules/network/main.tf":    `variable "cidr" { type = string }`,
		"modules/network/outputs.tf": `output "id" { value = "x" }`,
	}, "modules/", "modules/network/")

	mod, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.NoError(t, err)
	require.NotNil(t, mod.Metadata.Root)
	assert.Empty(t, mod.Metadata.Root.Inputs, "no root module: empty root metadata")
	assert.Equal(t, "# collection\n", mod.Metadata.Readme)
}

func TestNormalizeVersion(t *testing.T) {
	// Terraform accepts a leading `v`; normalizing keeps one naming scheme
	// in the registry instead of mixing v1.0.0 and 1.0.0.
	cases := []struct{ in, want string }{
		{"1.0.0", "1.0.0"},
		{"v1.0.0", "1.0.0"},
		{"1.0", "1.0.0"},
		{"v1.2.3-rc.1", "1.2.3-rc.1"},
	}
	for _, c := range cases {
		got, err := NormalizeVersion(c.in)
		require.NoError(t, err, "input=%q", c.in)
		assert.Equal(t, c.want, got, "input=%q", c.in)
	}
	for _, bad := range []string{"", "not-semver", "V1.0.0"} {
		_, err := NormalizeVersion(bad)
		require.ErrorIs(t, err, ErrInvalidVersion, "input=%q", bad)
	}
}

func TestParseModuleArchive_Submodules(t *testing.T) {
	// A module made only of submodules must still expose their metadata.
	archive := buildArchive(t, map[string]string{
		"README.md":                  "# collection\n",
		"modules/network/main.tf":    `variable "cidr" { type = string }`,
		"modules/network/outputs.tf": `output "vpc_id" { value = "x" }`,
		"modules/db/main.tf":         `variable "size" { type = number }`,
		// Examples are not indexed.
		"examples/basic/main.tf": `variable "ignored" { type = string }`,
	}, "modules/", "modules/network/", "modules/db/", "examples/", "examples/basic/")

	mod, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.NoError(t, err)
	require.Len(t, mod.Metadata.Submodules, 2, "both submodules should be indexed")
	// Sorted by name: db, network.
	assert.Equal(t, "db", mod.Metadata.Submodules[0].Name)
	assert.Equal(t, "network", mod.Metadata.Submodules[1].Name)

	network := mod.Metadata.Submodules[1]
	require.Len(t, network.Root.Inputs, 1)
	assert.Equal(t, "cidr", network.Root.Inputs[0].Name)
	require.Len(t, network.Root.Outputs, 1)
	assert.Equal(t, "vpc_id", network.Root.Outputs[0].Name)
}

func TestParseModuleArchive_RejectsEntryFlood(t *testing.T) {
	// Empty headers carry no payload, so without an entry cap an archive of
	// nothing but empty entries would compress to almost nothing yet cost
	// one allocation each — never tripping the byte ceiling.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i := range maxArchiveEntries + 10 {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     fmt.Sprintf("f%d.tf", i),
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     0,
		}))
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	// The archive is tiny but must still be refused.
	require.Less(t, buf.Len(), 1<<20)
	_, err := ParseModuleArchive(bytes.NewReader(buf.Bytes()), -1)
	require.ErrorIs(t, err, ErrTooManyArchiveEntries)
}

func TestParseModuleArchive_EnforcesSizeLimit(t *testing.T) {
	big := strings.Repeat("a", 4096)
	archive := buildArchive(t, map[string]string{
		"main.tf": "variable \"x\" { type = string }\n" + big,
	})
	_, err := ParseModuleArchive(bytes.NewReader(archive), 64)
	require.ErrorIs(t, err, ErrArchiveTooLarge)
}

func TestParseModuleArchive_ChargesPaddedPayload(t *testing.T) {
	// A payload occupies whole 512-byte blocks on the wire and tar.Reader
	// consumes the padding implicitly, so the ceiling must charge the padded
	// size or it can be exceeded by up to 511 bytes per entry.
	body := strings.Repeat("#", 520) // comment-only .tf: one block + 8 bytes -> padded to 1024
	archive := buildArchive(t, map[string]string{"main.tf": body})

	// Header (512) + raw payload (520) = 1032 would fit under 1100, but the
	// padded wire size (512 + 1024 = 1536) must not.
	_, err := ParseModuleArchive(bytes.NewReader(archive), 1100)
	require.ErrorIs(t, err, ErrArchiveTooLarge)

	// Exactly the padded size is allowed.
	_, err = ParseModuleArchive(bytes.NewReader(archive), 1536)
	require.NoError(t, err)
}

func TestParseModuleArchive_NoTFFiles(t *testing.T) {
	archive := buildArchive(t, map[string]string{
		"README.md": "# nothing here\n",
	})
	_, err := ParseModuleArchive(bytes.NewReader(archive), 1<<20)
	require.ErrorIs(t, err, ErrNoRootModule)
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
	for _, sentinel := range []error{ErrNoRootModule, ErrArchiveTooLarge, ErrUnsupportedTFFormat} {
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
