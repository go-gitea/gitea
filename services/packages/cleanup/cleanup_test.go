// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"testing"

	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	container_module "gitea.dev/modules/packages/container"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestExecuteCleanupOneRulePackageContainerKeepCount(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	p, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{Type: packages_model.TypeContainer, LowerName: "multi-arch"})
	require.NoError(t, err)

	insertVersion := func(version string, references ...string) {
		pv, err := packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{PackageID: p.ID, LowerVersion: version})
		require.NoError(t, err)
		for _, ref := range references {
			_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestReference, ref)
			require.NoError(t, err)
		}
	}
	digestOf := func(s string) string { return digest.FromString(s).String() }
	for _, tag := range []string{"v1", "v2"} {
		insertVersion(digestOf(tag))
		insertVersion(tag, digestOf(tag))
	}
	insertVersion("latest")
	insertVersion(digestOf("pushed-by-digest"))

	versionNames := func(pvs []*packages_model.PackageVersion) (names []string) {
		for _, pv := range pvs {
			names = append(names, pv.LowerVersion)
		}
		return names
	}
	pcr := &packages_model.PackageCleanupRule{Type: packages_model.TypeContainer, KeepCount: 1}

	removed, err := executeCleanupOneRulePackage(t.Context(), pcr, p, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"v1"}, versionNames(removed))

	removed, err = executeCleanupOneRulePackage(t.Context(), pcr, p, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"v1", digestOf("v1")}, versionNames(removed))
}
