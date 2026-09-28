// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"strings"
	"testing"

	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	packages_module "gitea.dev/modules/packages"
	maven_module "gitea.dev/modules/packages/maven"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	packages_service "gitea.dev/services/packages"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestCleanupSnapshotVersions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// inflated buildNumber and the missing build 5 check that retention counts stored builds
	const metadata = `<metadata><versioning><snapshot><buildNumber>100</buildNumber></snapshot><snapshotVersions>
<snapshotVersion><extension>jar</extension><value>1.0-20230101.000000-6</value></snapshotVersion>
<snapshotVersion><classifier>sources</classifier><extension>jar</extension><value>1.0-20230101.000000-3</value></snapshotVersion>
</snapshotVersions></versioning></metadata>`

	files := []struct {
		name string
		keep bool
	}{
		{"test-project-1.0-20230101.000000-1.jar", false},
		{"test-project-1.0-20230101.000000-2.pom", false},
		{"test-project-1.0-20230101.000000-3.jar", false},
		{"test-project-1.0-20230101.000000-3-javadoc.jar", false},
		{"test-project-1.0-20230101.000000-3-sources.jar", true},
		{"test-project-1.0-20230101.000000-3-sources.jar.asc", true},
		{"test-project-1.0-20230101.000000-4.jar", true},
		{"test-project-1.0-20230101.000000-6.jar", true},
		{"test-project-1.0-SNAPSHOT.jar", true},
		{"maven-metadata.xml", true},
	}

	var pv *packages_model.PackageVersion
	for _, file := range files {
		content := file.name
		if file.name == maven_module.MetadataFileName {
			content = metadata
		}
		buf, err := packages_module.CreateHashedBufferFromReader(strings.NewReader(content))
		require.NoError(t, err)
		pv, _, err = packages_service.CreatePackageOrAddFileToExisting(t.Context(), &packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       owner,
				PackageType: packages_model.TypeMaven,
				Name:        "com.gitea:test-project",
				Version:     "1.0-SNAPSHOT",
			},
			Creator:  owner,
			Metadata: &maven_module.Metadata{GroupID: "com.gitea", ArtifactID: "test-project"},
		}, &packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{Filename: file.name},
			Creator:         owner,
			Data:            buf,
		})
		buf.Close()
		require.NoError(t, err)
	}

	assertFiles := func(t *testing.T, deleteOld bool) {
		for _, file := range files {
			_, err := packages_model.GetFileForVersionByName(t.Context(), pv.ID, file.name, packages_model.EmptyFileKey)
			if file.keep || !deleteOld {
				assert.NoError(t, err, file.name)
			} else {
				assert.ErrorIs(t, err, packages_model.ErrPackageFileNotExist, file.name)
			}
		}
	}

	defer test.MockVariableValue(&setting.Packages.RetainMavenSnapshotBuilds, -1)()
	require.NoError(t, CleanupSnapshotVersions(t.Context()))
	assertFiles(t, false)

	setting.Packages.RetainMavenSnapshotBuilds = 2
	require.NoError(t, CleanupSnapshotVersions(t.Context()))
	assertFiles(t, true)
}
