// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	repo_module "gitea.dev/modules/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_detectLicense(t *testing.T) {
	type DetectLicenseTest struct {
		name string
		arg  string
		want []string
	}

	tests := []DetectLicenseTest{
		{
			name: "empty",
			arg:  "",
			want: nil,
		},
		{
			name: "no detected license",
			arg:  "Copyright (c) 2023 Gitea",
			want: nil,
		},
	}

	require.NoError(t, repo_module.LoadRepoConfig())
	for _, licenseName := range repo_module.Licenses {
		license, err := repo_module.GetLicense(licenseName, &repo_module.LicenseValues{
			Owner: "Gitea",
			Email: "teabot@gitea.io",
			Repo:  "gitea",
			Year:  "2024",
		})
		assert.NoError(t, err)

		tests = append(tests, DetectLicenseTest{
			name: "single license test: " + licenseName,
			arg:  string(license),
			want: []string{licenseName},
		})
	}

	require.NoError(t, InitLicenseClassifier())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			license, err := detectLicense(strings.NewReader(tt.arg))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, license)
		})
	}

	result, err := detectLicense(strings.NewReader(tests[2].arg + tests[3].arg + tests[4].arg))
	assert.NoError(t, err)
	t.Run("multiple licenses test", func(t *testing.T) {
		assert.Len(t, result, 3)
		assert.Contains(t, result, tests[2].want[0])
		assert.Contains(t, result, tests[3].want[0])
		assert.Contains(t, result, tests[4].want[0])
	})
}

func detectedLicenseIDs(licenses []repo_model.DetectedLicense) []string {
	ids := make([]string, 0, len(licenses))
	for _, l := range licenses {
		ids = append(ids, l.SPDXID)
	}
	return ids
}

func Test_resolveLicenses(t *testing.T) {
	require.NoError(t, repo_module.LoadRepoConfig())
	require.NoError(t, InitLicenseClassifier())

	mitLicense, err := repo_module.GetLicense("MIT", &repo_module.LicenseValues{
		Owner: "Test",
		Email: "test@test.com",
		Repo:  "test",
		Year:  "2024",
	})
	require.NoError(t, err)

	repoDir := filepath.Join(t.TempDir(), "repo.git")
	require.NoError(t, git.InitRepositoryLocal(t.Context(), repoDir, true, "sha1"))
	gitRepo, err := git.OpenRepositoryLocal(repoDir)
	require.NoError(t, err)
	defer gitRepo.Close()

	commit, err := gitRepo.GetBranchCommit(t.Context(), "master")
	// no commits yet — should error
	assert.Error(t, err)

	// 1. repo with no license at all
	require.NoError(t, git.ForceFastImport(t.Context(), gitRepo, []git.FastImportCommit{{
		Ref:     "refs/heads/master",
		Message: "empty",
	}}))
	commit, err = gitRepo.GetBranchCommit(t.Context(), "master")
	require.NoError(t, err)

	licenses, err := resolveLicenses(t.Context(), gitRepo, commit)
	assert.Empty(t, licenses)
	assert.NoError(t, err)

	// 2. repo with a plain LICENSE file — classifier should detect MIT
	require.NoError(t, git.ForceFastImport(t.Context(), gitRepo, []git.FastImportCommit{{
		Ref:     "refs/heads/master",
		Message: "add LICENSE",
		Files: []git.FastImportFile{
			{Mode: git.EntryModeBlob, Path: "LICENSE", Content: string(mitLicense)},
		},
	}}))
	commit, err = gitRepo.GetBranchCommit(t.Context(), "master")
	require.NoError(t, err)

	licenses, err = resolveLicenses(t.Context(), gitRepo, commit)
	assert.NoError(t, err)
	assert.Len(t, licenses, 1)
	assert.Equal(t, "MIT", licenses[0].SPDXID)
	assert.Equal(t, "LICENSE", licenses[0].LicensePath)

	// 3. repo with REUSE LICENSES/ dir — should take priority over root LICENSE
	require.NoError(t, git.ForceFastImport(t.Context(), gitRepo, []git.FastImportCommit{{
		Ref:     "refs/heads/master",
		Message: "add LICENSES dir",
		Files: []git.FastImportFile{
			{Mode: git.EntryModeBlob, Path: "LICENSE", Content: string(mitLicense)},
			{Mode: git.EntryModeBlob, Path: "LICENSES/MIT.txt", Content: "MIT license text"},
			{Mode: git.EntryModeBlob, Path: "LICENSES/Apache-2.0.txt", Content: "Apache license text"},
		},
	}}))
	commit, err = gitRepo.GetBranchCommit(t.Context(), "master")
	require.NoError(t, err)

	licenses, err = resolveLicenses(t.Context(), gitRepo, commit)
	assert.NoError(t, err)
	assert.Len(t, licenses, 2)

	ids := detectedLicenseIDs(licenses)
	slices.Sort(ids)
	assert.Equal(t, []string{"Apache-2.0", "MIT"}, ids)
	// REUSE entries carry full path including LICENSES/ prefix
	for _, l := range licenses {
		assert.True(t, strings.HasPrefix(l.LicensePath, "LICENSES/"))
	}

	// 4. remove all licenses — should return not exist
	require.NoError(t, git.ForceFastImport(t.Context(), gitRepo, []git.FastImportCommit{{
		Ref:     "refs/heads/master",
		Message: "remove licenses",
	}}))
	commit, err = gitRepo.GetBranchCommit(t.Context(), "master")
	require.NoError(t, err)

	licenses, err = resolveLicenses(t.Context(), gitRepo, commit)
	assert.Empty(t, licenses)
	assert.NoError(t, err)
}

func Test_isLicenseFile(t *testing.T) {
	shouldMatch := []string{
		"LICENSE",
		"LICENCE",
		"COPYING",
		"License",
		"Licence",
		"copying",
		"LICENSE.txt",
		"LICENSE.md",
		"LICENCE.txt",
		"LICENCE.md",
		"COPYING.txt",
		"COPYING.md",
		"LICENSE.TXT",
		"LICENSE.MD",
		"license.txt",
		"licence.md",
		"copying.TXT",
		"LICENSE.a.b.c",
		"LICENSE.a.",
	}

	shouldNotMatch := []string{
		"LICENSE.",
		"README",
		"README.md",
		"NOTICE",
		"AUTHORS",
		"LICENSING",
		"COPYLEFT",
	}

	for _, name := range shouldMatch {
		t.Run("match/"+name, func(t *testing.T) {
			assert.True(t, isLicenseFile(name), "expected %q to be recognized as a license file", name)
		})
	}
	for _, name := range shouldNotMatch {
		noMatch := name
		t.Run("nomatch/"+noMatch, func(t *testing.T) {
			assert.False(t, isLicenseFile(noMatch), "expected %q to NOT be recognized as a license file", noMatch)
		})
	}
}
