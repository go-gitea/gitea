// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderplugin

import (
	"path"

	"code.gitea.io/gitea/modules/storage"
)

// Storage returns the object storage used for render plugins.
func Storage() storage.ObjectStorage {
	return storage.RenderPlugins
}

// ObjectPath builds a storage-relative path for a plugin asset.
func ObjectPath(identifier string, elems ...string) string {
	joined := path.Join(elems...)
	if joined == "." || joined == "" {
		return path.Join(identifier)
	}
	return path.Join(identifier, joined)
}

// ObjectPrefix returns the storage prefix for a plugin identifier.
func ObjectPrefix(identifier string) string {
	if identifier == "" {
		return ""
	}
	return identifier + "/"
}
