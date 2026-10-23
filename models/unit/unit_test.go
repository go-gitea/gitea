// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unit

import (
	"slices"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func mockUnitConfig(t *testing.T) {
	disabledRepoUnits := DisabledRepoUnitsGet()
	t.Cleanup(func() { DisabledRepoUnitsSet(disabledRepoUnits) })
	t.Cleanup(test.MockVariableValue(&DefaultRepoUnits, slices.Clone(DefaultRepoUnits)))
	t.Cleanup(test.MockVariableValue(&DefaultForkRepoUnits, slices.Clone(DefaultForkRepoUnits)))
	t.Cleanup(test.MockVariableValue(&DefaultMirrorRepoUnits, slices.Clone(DefaultMirrorRepoUnits)))
	t.Cleanup(test.MockVariableValue(&DefaultTemplateRepoUnits, slices.Clone(DefaultTemplateRepoUnits)))
	t.Cleanup(test.MockVariableValue(&setting.Repository))
}

func TestLoadUnitConfig(t *testing.T) {
	t.Run("regular", func(t *testing.T) {
		mockUnitConfig(t)

		setting.Repository.DisabledRepoUnits = []string{"repo.issues"}
		setting.Repository.DefaultRepoUnits = []string{"repo.code", "repo.releases", "repo.issues", "repo.pulls"}
		setting.Repository.DefaultForkRepoUnits = []string{"repo.releases"}
		assert.NoError(t, LoadUnitConfig())
		assert.Equal(t, []Type{TypeIssues}, DisabledRepoUnitsGet())
		assert.Equal(t, []Type{TypeCode, TypeReleases, TypePullRequests}, DefaultRepoUnits)
		assert.Equal(t, []Type{TypeReleases}, DefaultForkRepoUnits)
	})
	t.Run("invalid", func(t *testing.T) {
		mockUnitConfig(t)

		setting.Repository.DisabledRepoUnits = []string{"repo.issues", "invalid.1"}
		setting.Repository.DefaultRepoUnits = []string{"repo.code", "invalid.2", "repo.releases", "repo.issues", "repo.pulls"}
		setting.Repository.DefaultForkRepoUnits = []string{"invalid.3", "repo.releases"}
		assert.NoError(t, LoadUnitConfig())
		assert.Equal(t, []Type{TypeIssues}, DisabledRepoUnitsGet())
		assert.Equal(t, []Type{TypeCode, TypeReleases, TypePullRequests}, DefaultRepoUnits)
		assert.Equal(t, []Type{TypeReleases}, DefaultForkRepoUnits)
	})
	t.Run("duplicate", func(t *testing.T) {
		mockUnitConfig(t)

		setting.Repository.DisabledRepoUnits = []string{"repo.issues", "repo.issues"}
		setting.Repository.DefaultRepoUnits = []string{"repo.code", "repo.releases", "repo.issues", "repo.pulls", "repo.code"}
		setting.Repository.DefaultForkRepoUnits = []string{"repo.releases", "repo.releases"}
		assert.NoError(t, LoadUnitConfig())
		assert.Equal(t, []Type{TypeIssues}, DisabledRepoUnitsGet())
		assert.Equal(t, []Type{TypeCode, TypeReleases, TypePullRequests}, DefaultRepoUnits)
		assert.Equal(t, []Type{TypeReleases}, DefaultForkRepoUnits)
	})
	t.Run("empty_default", func(t *testing.T) {
		mockUnitConfig(t)

		setting.Repository.DisabledRepoUnits = []string{"repo.issues", "repo.issues"}
		setting.Repository.DefaultRepoUnits = []string{}
		setting.Repository.DefaultForkRepoUnits = []string{"repo.releases", "repo.releases"}
		assert.NoError(t, LoadUnitConfig())
		assert.Equal(t, []Type{TypeIssues}, DisabledRepoUnitsGet())
		assert.ElementsMatch(t, []Type{TypeCode, TypePullRequests, TypeReleases, TypeWiki, TypePackages, TypeProjects, TypeActions}, DefaultRepoUnits)
		assert.Equal(t, []Type{TypeReleases}, DefaultForkRepoUnits)
	})
}
