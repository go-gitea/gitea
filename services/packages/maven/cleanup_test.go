// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"bytes"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/setting"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func addMavenMetadataToPackageVersion(t *testing.T, pv *packages.PackageVersion) {
	// Create maven-metadata.xml content with build number 5 (matching the fixtures)
	metadataXML := `<?xml version="1.0" encoding="UTF-8"?>
<metadata>
  <groupId>com.gitea</groupId>
  <artifactId>test-project</artifactId>
  <version>1.0-SNAPSHOT</version>
  <versioning>
    <snapshot>
      <timestamp>20230101.000000</timestamp>
      <buildNumber>5</buildNumber>
    </snapshot>
    <lastUpdated>20230101000000</lastUpdated>
  </versioning>
</metadata>`

	// Add metadata file to the existing package version using service method
	metadataReader := bytes.NewReader([]byte(metadataXML))
	hsr, err := packages_module.CreateHashedBufferFromReader(metadataReader)
	assert.NoError(t, err)

	pfci := &packages_service.PackageFileCreationInfo{
		PackageFileInfo: packages_service.PackageFileInfo{
			Filename: "maven-metadata.xml",
		},
		Data: hsr,
	}

	_, err = packages_service.AddFileToPackageVersionInternal(db.DefaultContext, pv, pfci)
	assert.NoError(t, err)
}

func TestCleanupSnapshotVersions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Should skip when retainBuilds is negative", func(t *testing.T) {
		setting.Packages.RetainMavenSnapshotBuilds = -1
		setting.Packages.DebugMavenCleanup = false
		t.Logf("Test settings: retainBuilds=%d, debug=%t", setting.Packages.RetainMavenSnapshotBuilds, setting.Packages.DebugMavenCleanup)
		err := CleanupSnapshotVersions(db.DefaultContext)
		assert.NoError(t, err)
	})

	t.Run("Should skip when retainBuilds is zero", func(t *testing.T) {
		setting.Packages.RetainMavenSnapshotBuilds = 0
		setting.Packages.DebugMavenCleanup = false
		t.Logf("Test settings: retainBuilds=%d, debug=%t", setting.Packages.RetainMavenSnapshotBuilds, setting.Packages.DebugMavenCleanup)
		err := CleanupSnapshotVersions(db.DefaultContext)
		assert.NoError(t, err)
	})

	t.Run("Should handle missing metadata file gracefully", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.RetainMavenSnapshotBuilds = 2
		setting.Packages.DebugMavenCleanup = false

		// Get the existing package version from fixtures (ID 1)
		pv, err := packages.GetVersionByID(db.DefaultContext, 1)
		assert.NoError(t, err)

		// Verify all 5 files exist before cleanup
		filesBefore, err := packages.GetFilesByVersionID(db.DefaultContext, pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 5) // 5 jar files, no metadata

		// No metadata file exists in fixtures - should handle gracefully
		err = CleanupSnapshotVersions(db.DefaultContext)
		assert.NoError(t, err)

		// Verify all 5 files still exist after cleanup (no cleanup should occur without metadata)
		filesAfter, err := packages.GetFilesByVersionID(db.DefaultContext, pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesAfter, 5, "All files should remain when metadata is missing")
	})

	t.Run("Should work with debug mode", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.RetainMavenSnapshotBuilds = 2
		setting.Packages.DebugMavenCleanup = true

		pv, err := packages.GetVersionByID(db.DefaultContext, 1)
		assert.NoError(t, err)

		addMavenMetadataToPackageVersion(t, pv)

		filesBefore, err := packages.GetFilesByVersionID(db.DefaultContext, pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 6) // 5 jar files + 1 metadata file

		err = CleanupSnapshotVersions(db.DefaultContext)
		assert.NoError(t, err)

		// Verify all files still exist after cleanup (debug mode should not delete anything)
		filesAfter, err := packages.GetFilesByVersionID(db.DefaultContext, pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesAfter, 6, "All files should remain in debug mode")
	})

	t.Run("Should test actual cleanup with metadata", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.DebugMavenCleanup = false
		setting.Packages.RetainMavenSnapshotBuilds = 2
		t.Logf("Test settings: retainBuilds=%d, debug=%t", setting.Packages.RetainMavenSnapshotBuilds, setting.Packages.DebugMavenCleanup)

		// Get the existing package version from fixtures (ID 1)
		pv, err := packages.GetVersionByID(db.DefaultContext, 1)
		assert.NoError(t, err)
		assert.Equal(t, "1.0-SNAPSHOT", pv.Version)

		addMavenMetadataToPackageVersion(t, pv)

		filesBefore, err := packages.GetFilesByVersionID(db.DefaultContext, pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 6) // 5 jar files + 1 metadata file

		// Test cleanup with retainBuilds = 2 (should keep builds 4 and 5, remove 1, 2, 3)
		err = CleanupSnapshotVersions(db.DefaultContext)
		assert.NoError(t, err)

		filesAfter, err := packages.GetFilesByVersionID(db.DefaultContext, pv.ID)
		assert.NoError(t, err)

		// Should have metadata file + 2 retained builds
		assert.Len(t, filesAfter, 3)

		// Check that metadata file is still there
		var hasMetadata bool
		var retainedBuilds []string
		for _, file := range filesAfter {
			if file.Name == "maven-metadata.xml" {
				hasMetadata = true
			} else {
				retainedBuilds = append(retainedBuilds, file.Name)
			}
		}

		assert.True(t, hasMetadata, "maven-metadata.xml should be retained")
		assert.Len(t, retainedBuilds, 2, "Should retain exactly 2 builds")

		t.Logf("Retained builds: %v", retainedBuilds)

		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-4.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-5.jar")
	})
}
