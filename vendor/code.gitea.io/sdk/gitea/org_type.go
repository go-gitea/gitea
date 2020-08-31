// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

// VisibleType defines the visibility (Organization only)
type VisibleType int

const (
	// VisibleTypePublic Visible for everyone
	VisibleTypePublic VisibleType = iota

	// VisibleTypeLimited Visible for every connected user
	VisibleTypeLimited

	// VisibleTypePrivate Visible only for organization's members
	VisibleTypePrivate
)

// ExtractKeysFromMapString provides a slice of keys from map
func ExtractKeysFromMapString(in map[string]VisibleType) (keys []string) {
	for k := range in {
		keys = append(keys, k)
	}
	return
}
