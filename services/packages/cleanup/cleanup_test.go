// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"gitea.dev/models/db"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	container_module "gitea.dev/modules/packages/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestExecuteCleanupOneRulePackageContainerKeepCount(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	p, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{
		OwnerID:   2,
		Type:      packages_model.TypeContainer,
		Name:      "multi-arch",
		LowerName: "multi-arch",
	})
	require.NoError(t, err)

	created := int64(1000)
	insertVersion := func(t *testing.T, version string, references ...string) {
		pv, err := packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{
			PackageID:    p.ID,
			Version:      version,
			LowerVersion: version,
		})
		require.NoError(t, err)
		created++
		_, err = db.GetEngine(t.Context()).Exec("UPDATE package_version SET created_unix = ? WHERE id = ?", created, pv.ID)
		require.NoError(t, err)
		for _, ref := range references {
			_, err = packages_model.InsertProperty(t.Context(), packages_model.PropertyTypeVersion, pv.ID, container_module.PropertyManifestReference, ref)
			require.NoError(t, err)
		}
	}
	digestOf := func(s string) string {
		return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s)))
	}

	// every multi-arch push stores the untagged per-platform manifests and the tagged index
	for _, tag := range []string{"v1", "v2", "v3"} {
		amd64, arm64 := digestOf(tag+"-amd64"), digestOf(tag+"-arm64")
		insertVersion(t, amd64)
		insertVersion(t, arm64)
		insertVersion(t, tag, amd64, arm64)
	}

	pcr := &packages_model.PackageCleanupRule{
		Enabled:   true,
		OwnerID:   2,
		Type:      packages_model.TypeContainer,
		KeepCount: 2,
	}
	require.NoError(t, pcr.CompiledPattern())

	deleted, err := executeCleanupOneRulePackage(t.Context(), pcr, p)
	require.NoError(t, err)
	assert.True(t, deleted)

	pvs, err := packages_model.GetVersionsByPackageName(t.Context(), 2, packages_model.TypeContainer, "multi-arch")
	require.NoError(t, err)
	var remaining []string
	for _, pv := range pvs {
		remaining = append(remaining, pv.LowerVersion)
	}
	assert.ElementsMatch(t, []string{
		"v2", digestOf("v2-amd64"), digestOf("v2-arm64"),
		"v3", digestOf("v3-amd64"), digestOf("v3-arm64"),
	}, remaining)
}
