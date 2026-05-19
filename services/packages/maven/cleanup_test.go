// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/setting"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// createTestMavenSnapshotPackage creates a maven snapshot package with 11 artifact files
// Files created:
//   - 5 base jars: build 1-5
//   - 6 classifier jars: sources + javadoc for builds 3, 4, 5
func createTestMavenSnapshotPackage(t *testing.T) *packages.PackageVersion {
	t.Helper()

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Create the first file to establish the package + version
	buf, err := packages_module.CreateHashedBufferFromReaderWithSize(
		strings.NewReader("test-content-1"), 1024,
	)
	assert.NoError(t, err)
	defer buf.Close()

	pv, _, err := packages_service.CreatePackageAndAddFile(t.Context(), &packages_service.PackageCreationInfo{
		PackageInfo: packages_service.PackageInfo{
			Owner:       owner,
			PackageType: packages.TypeMaven,
			Name:        "com.gitea:test-project",
			Version:     "1.0-SNAPSHOT",
		},
		Creator:          owner,
		SemverCompatible: false,
		Metadata: &maven.Metadata{
			GroupID:    "com.gitea",
			ArtifactID: "test-project",
		},
	}, &packages_service.PackageFileCreationInfo{
		PackageFileInfo: packages_service.PackageFileInfo{
			Filename: "gitea-test-1.0-20230101.000000-1.jar",
		},
		Creator: owner,
		Data:    buf,
		IsLead:  false,
	})
	assert.NoError(t, err)

	// Define remaining files to add (builds 2-5 base jars + classifier jars)
	additionalFiles := []string{
		"gitea-test-1.0-20230101.000000-2.jar",
		"gitea-test-1.0-20230101.000000-3.jar",
		"gitea-test-1.0-20230101.000000-4.jar",
		"gitea-test-1.0-20230101.000000-5.jar",
		"gitea-test-1.0-20230101.000000-3-sources.jar",
		"gitea-test-1.0-20230101.000000-3-javadoc.jar",
		"gitea-test-1.0-20230101.000000-4-sources.jar",
		"gitea-test-1.0-20230101.000000-4-javadoc.jar",
		"gitea-test-1.0-20230101.000000-5-sources.jar",
		"gitea-test-1.0-20230101.000000-5-javadoc.jar",
	}

	for i, filename := range additionalFiles {
		content := fmt.Sprintf("test-content-%d", i+2)
		fileBuf, err := packages_module.CreateHashedBufferFromReaderWithSize(
			strings.NewReader(content), 1024,
		)
		assert.NoError(t, err)

		_, err = packages_service.AddFileToPackageVersionInternal(t.Context(), pv, &packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: filename,
			},
			Data:   fileBuf,
			IsLead: false,
		})
		assert.NoError(t, err)
		fileBuf.Close()
	}

	return pv
}

func addMavenMetadataToPackageVersion(t *testing.T, pv *packages.PackageVersion) {
	t.Helper()

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

	metadataReader := bytes.NewReader([]byte(metadataXML))
	hsr, err := packages_module.CreateHashedBufferFromReader(metadataReader)
	assert.NoError(t, err)

	_, err = packages_service.AddFileToPackageVersionInternal(t.Context(), pv, &packages_service.PackageFileCreationInfo{
		PackageFileInfo: packages_service.PackageFileInfo{
			Filename: "maven-metadata.xml",
		},
		Data: hsr,
	})
	assert.NoError(t, err)
}

func TestCleanupSnapshotVersions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Should skip when retainBuilds is negative", func(t *testing.T) {
		setting.Packages.RetainMavenSnapshotBuilds = -1
		setting.Packages.DebugMavenCleanup = false
		err := CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should skip when retainBuilds is zero", func(t *testing.T) {
		setting.Packages.RetainMavenSnapshotBuilds = 0
		setting.Packages.DebugMavenCleanup = false
		err := CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)
	})

	t.Run("Should handle missing metadata file gracefully", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.RetainMavenSnapshotBuilds = 2
		setting.Packages.DebugMavenCleanup = false

		pv := createTestMavenSnapshotPackage(t)

		filesBefore, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 11)

		// No metadata file — should handle gracefully
		err = CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)

		filesAfter, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesAfter, 11, "All files should remain when metadata is missing")
	})

	t.Run("Should work with debug mode", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.RetainMavenSnapshotBuilds = 2
		setting.Packages.DebugMavenCleanup = true

		pv := createTestMavenSnapshotPackage(t)
		addMavenMetadataToPackageVersion(t, pv)

		filesBefore, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesBefore, 12) // 11 jar files + 1 metadata file

		err = CleanupSnapshotVersions(t.Context())
		assert.NoError(t, err)

		filesAfter, err := packages.GetFilesByVersionID(t.Context(), pv.ID)
		assert.NoError(t, err)
		assert.Len(t, filesAfter, 12, "All files should remain in debug mode")
	})

	t.Run("Should test actual cleanup with metadata", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		setting.Packages.DebugMavenCleanup = false
		setting.Packages.RetainMavenSnapshotBuilds = 2

		pv := createTestMavenSnapshotPackage(t)
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

		// Should have metadata file + 6 retained build artifacts (2 builds x 3 files each)
		assert.Len(t, filesAfter, 7)

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

		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-4.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-4-sources.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-4-javadoc.jar")

		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-5.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-5-sources.jar")
		assert.Contains(t, retainedBuilds, "gitea-test-1.0-20230101.000000-5-javadoc.jar")
	})
}
