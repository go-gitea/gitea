// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"bytes"
	"testing"

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
	// Maven metadata structure explanation:
	// - <snapshot>: Contains the latest snapshot timestamp and build number
	// - <snapshotVersions>: Lists all available files for each build number
	//   - <extension>: File extension (jar, pom, etc.)
	//   - <classifier>: Optional classifier (sources, javadoc, tests, etc.)
	//   - <value>: The actual version string with timestamp and build number
	//   - <updated>: Timestamp when the artifact was deployed
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
    <snapshotVersions>
      <snapshotVersion>
        <extension>jar</extension>
        <value>1.0-20230101.000000-1</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <extension>jar</extension>
        <value>1.0-20230101.000000-2</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <extension>jar</extension>
        <value>1.0-20230101.000000-3</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <classifier>sources</classifier>
        <extension>jar</extension>
        <value>1.0-20230101.000000-3</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <classifier>javadoc</classifier>
        <extension>jar</extension>
        <value>1.0-20230101.000000-3</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <extension>jar</extension>
        <value>1.0-20230101.000000-4</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <classifier>sources</classifier>
        <extension>jar</extension>
        <value>1.0-20230101.000000-4</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <classifier>javadoc</classifier>
        <extension>jar</extension>
        <value>1.0-20230101.000000-4</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <extension>jar</extension>
        <value>1.0-20230101.000000-5</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <classifier>sources</classifier>
        <extension>jar</extension>
        <value>1.0-20230101.000000-5</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
      <snapshotVersion>
        <classifier>javadoc</classifier>
        <extension>jar</extension>
        <value>1.0-20230101.000000-5</value>
        <updated>20230101000000</updated>
      </snapshotVersion>
    </snapshotVersions>
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

	_, err = packages_service.AddFileToPackageVersionInternal(t.Context(), pv, pfci)
	assert.NoError(t, err)
}

func TestCleanupSnapshotVersions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Should skip when retainBuilds is negative", func(t *testing.T) {
		setting.Packages.RetainMavenSnapshotBuilds = -1
		setting.Packages.DebugMavenCleanup = false
		t.Logf("Test settings: retainBuilds=%d, debug=%t", setting.Packages.RetainMavenSnapshotBuilds, setting.Packages.DebugMavenCleanup)
		err := CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should skip when retainBuilds is zero", func(t *testing.T) {
		setting.Packages.RetainMavenSnapshotBuilds = 0
		setting.Packages.DebugMavenCleanup = false
		t.Logf("Test settings: retainBuilds=%d, debug=%t", setting.Packages.RetainMavenSnapshotBuilds, setting.Packages.DebugMavenCleanup)
		err := CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should handle missing metadata file gracefully", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.RetainMavenSnapshotBuilds = 2
		setting.Packages.DebugMavenCleanup = false

		// Get the existing package version from fixtures (ID 1)
		pv, err := packages.GetVersionByID(t.Context(), 1)
		assert.NoError(t, err)

		// Verify all 11 files exist before cleanup (5 base jars + 6 classifier jars)
		filesBefore, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 11) // 5 base jars + 6 classifier jars (sources + javadoc for builds 3,4,5)

		// No metadata file exists in fixtures - should handle gracefully
		err = CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)

		// Verify all 11 files still exist after cleanup (no cleanup should occur without metadata)
		filesAfter, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesAfter, 11, "All files should remain when metadata is missing")
	})

	t.Run("Should work with debug mode", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.RetainMavenSnapshotBuilds = 2
		setting.Packages.DebugMavenCleanup = true

		pv, err := packages.GetVersionByID(t.Context(), 1)
		assert.NoError(t, err)

		addMavenMetadataToPackageVersion(t, pv)

		filesBefore, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 12) // 11 jar files + 1 metadata file

		err = CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)

		// Verify all files still exist after cleanup (debug mode should not delete anything)
		filesAfter, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesAfter, 12, "All files should remain in debug mode")
	})

	t.Run("Should test actual cleanup with metadata", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.DebugMavenCleanup = false
		setting.Packages.RetainMavenSnapshotBuilds = 2
		t.Logf("Test settings: retainBuilds=%d, debug=%t", setting.Packages.RetainMavenSnapshotBuilds, setting.Packages.DebugMavenCleanup)

		// Get the existing package version from fixtures (ID 1)
		pv, err := packages.GetVersionByID(t.Context(), 1)
		assert.NoError(t, err)
		assert.Equal(t, "1.0-SNAPSHOT", pv.Version)

		addMavenMetadataToPackageVersion(t, pv)

		filesBefore, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 12) // 11 jar files + 1 metadata file

		// Test cleanup with retainBuilds = 2 (should keep builds 4 and 5, remove 1, 2, 3)
		// Build 4: base jar + sources + javadoc = 3 files
		// Build 5: base jar + sources + javadoc = 3 files
		// Total retained: 6 files + 1 metadata = 7 files
		err = CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)

		filesAfter, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)

		// Should have metadata file + 6 retained build artifacts (2 builds × 3 files each)
		assert.Len(t, filesAfter, 7)

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
		assert.Len(t, retainedBuilds, 6, "Should retain exactly 6 files (2 builds with 3 artifacts each)")

		t.Logf("Retained builds: %v", retainedBuilds)

		// Verify build 4 artifacts are retained
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-4.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-4-sources.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-4-javadoc.jar")

		// Verify build 5 artifacts are retained
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-5.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-5-sources.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-5-javadoc.jar")
	})
}
