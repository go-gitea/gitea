// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUses(t *testing.T) {
	t.Run("LocalSameRepo", func(t *testing.T) {
		cases := []struct {
			name string
			in   string
			want UsesRef
		}{
			{
				name: "gitea dir, .yml",
				in:   "./.gitea/workflows/build.yml",
				want: UsesRef{Kind: UsesKindLocalSameRepo, Path: ".gitea/workflows/build.yml"},
			},
			{
				name: "github dir, .yml",
				in:   "./.github/workflows/build.yml",
				want: UsesRef{Kind: UsesKindLocalSameRepo, Path: ".github/workflows/build.yml"},
			},
			{
				name: "gitea dir, .yaml",
				in:   "./.gitea/workflows/build.yaml",
				want: UsesRef{Kind: UsesKindLocalSameRepo, Path: ".gitea/workflows/build.yaml"},
			},
			{
				name: "filename containing dots is allowed",
				in:   "./.gitea/workflows/foo..bar.yml",
				want: UsesRef{Kind: UsesKindLocalSameRepo, Path: ".gitea/workflows/foo..bar.yml"},
			},
			{
				name: "nested subdirectory",
				in:   "./.gitea/workflows/sub/build.yml",
				want: UsesRef{Kind: UsesKindLocalSameRepo, Path: ".gitea/workflows/sub/build.yml"},
			},
			{
				name: "leading/trailing whitespace is trimmed",
				in:   "  ./.gitea/workflows/build.yml  ",
				want: UsesRef{Kind: UsesKindLocalSameRepo, Path: ".gitea/workflows/build.yml"},
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				got, err := ParseUses(c.in)
				require.NoError(t, err)
				assert.Equal(t, c.want, *got)
			})
		}
	})

	t.Run("LocalCrossRepo", func(t *testing.T) {
		cases := []struct {
			name string
			in   string
			want UsesRef
		}{
			{
				name: "gitea dir, simple ref",
				in:   "owner/repo/.gitea/workflows/build.yml@v1",
				want: UsesRef{
					Kind:  UsesKindLocalCrossRepo,
					Owner: "owner",
					Repo:  "repo",
					Path:  ".gitea/workflows/build.yml",
					Ref:   "v1",
				},
			},
			{
				name: "github dir, branch ref",
				in:   "owner/repo/.github/workflows/build.yml@main",
				want: UsesRef{
					Kind:  UsesKindLocalCrossRepo,
					Owner: "owner",
					Repo:  "repo",
					Path:  ".github/workflows/build.yml",
					Ref:   "main",
				},
			},
			{
				name: ".yaml extension",
				in:   "owner/repo/.gitea/workflows/build.yaml@abc123",
				want: UsesRef{
					Kind:  UsesKindLocalCrossRepo,
					Owner: "owner",
					Repo:  "repo",
					Path:  ".gitea/workflows/build.yaml",
					Ref:   "abc123",
				},
			},
			{
				name: "ref with slashes (refs/heads/feature)",
				in:   "owner/repo/.gitea/workflows/build.yml@refs/heads/feature",
				want: UsesRef{
					Kind:  UsesKindLocalCrossRepo,
					Owner: "owner",
					Repo:  "repo",
					Path:  ".gitea/workflows/build.yml",
					Ref:   "refs/heads/feature",
				},
			},
			{
				name: "nested subdirectory under workflows",
				in:   "owner/repo/.gitea/workflows/sub/build.yml@v1",
				want: UsesRef{
					Kind:  UsesKindLocalCrossRepo,
					Owner: "owner",
					Repo:  "repo",
					Path:  ".gitea/workflows/sub/build.yml",
					Ref:   "v1",
				},
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				got, err := ParseUses(c.in)
				require.NoError(t, err)
				assert.Equal(t, c.want, *got)
			})
		}
	})

	t.Run("Errors", func(t *testing.T) {
		cases := []struct {
			name string
			in   string
		}{
			{name: "empty string", in: ""},
			{name: "whitespace only", in: "   "},

			// Same-repo malformed
			{name: "same-repo with @ref", in: "./.gitea/workflows/build.yml@v1"},
			{name: "same-repo wrong directory", in: "./not-workflows/build.yml"},
			{name: "same-repo wrong extension", in: "./.gitea/workflows/build.txt"},
			{name: "same-repo missing extension", in: "./.gitea/workflows/build"},
			{name: "same-repo absolute path", in: "/.gitea/workflows/build.yml"},
			{name: "same-repo path traversal", in: "./.gitea/workflows/../escape.yml"},
			{name: "same-repo double slash", in: "./.gitea/workflows//build.yml"},
			{name: "same-repo redundant ./", in: "./.gitea/workflows/./build.yml"},
			{name: "same-repo no filename", in: "./.gitea/workflows/.yml"},

			// Cross-repo malformed
			{name: "cross-repo missing @ref", in: "owner/repo/.gitea/workflows/build.yml"},
			{name: "cross-repo empty ref", in: "owner/repo/.gitea/workflows/build.yml@"},
			{name: "cross-repo missing owner", in: "/repo/.gitea/workflows/build.yml@v1"},
			{name: "cross-repo missing repo", in: "owner//.gitea/workflows/build.yml@v1"},
			{name: "cross-repo wrong workflows dir", in: "owner/repo/workflows/build.yml@v1"},
			{name: "cross-repo wrong extension", in: "owner/repo/.gitea/workflows/build.txt@v1"},
			{name: "cross-repo path traversal", in: "owner/repo/.gitea/workflows/../escape.yml@v1"},
			{name: "cross-repo double slash in path", in: "owner/repo/.gitea/workflows//build.yml@v1"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				_, err := ParseUses(c.in)
				assert.Error(t, err)
			})
		}
	})
}
