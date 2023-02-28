// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package history

import (
	version "github.com/hashicorp/go-version"
)

var versionCache map[string]*version.Version = make(map[string]*version.Version) // Multiple settings share the same version, so cache it instead of always creating a new version

func getVersion(stringVersion string) *version.Version {
	if _, ok := versionCache[stringVersion]; !ok {
		versionCache[stringVersion] = version.Must(version.NewVersion(stringVersion))
	}
	return versionCache[stringVersion]
}
