// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSnapshotBuildNumber(t *testing.T) {
	for filename, expected := range map[string]int{
		"my-app-1.0-20250311.083409-9.jar":                      9,
		"my-app-1.0-20250311.083409-10-sources.jar":             10,
		"my-app-1.0-20250311.083409-11-x-20240101.000000-1.jar": 11,
		"my-app-1.0-20250311.083409-12.jar.asc":                 12,
		"my-app-1.0-SNAPSHOT.jar":                               0,
		"my-app-2.0-20250311.083409-9.jar":                      0,
		"other-1.0-20250311.083409-9.jar":                       0,
		"maven-metadata.xml":                                    0,
	} {
		build, ok := ParseSnapshotBuildNumber("my-app", "1.0-SNAPSHOT", filename)
		assert.Equal(t, expected != 0, ok, filename)
		assert.Equal(t, expected, build, filename)
	}
}
