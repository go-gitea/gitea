// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// VisibleType defines the visibility of user and org
type VisibleType int

const (
	// VisibleTypePublic Visible for everyone
	VisibleTypePublic VisibleType = iota

	// VisibleTypeLimited Visible for every connected user
	VisibleTypeLimited

	// VisibleTypePrivate Visible only for self or admin user
	VisibleTypePrivate
)

// VisibilityModes is a map of Visibility types
var VisibilityModes = map[string]VisibleType{
	"public":  VisibleTypePublic,
	"limited": VisibleTypeLimited,
	"private": VisibleTypePrivate,
}

// IsPublic returns true if VisibleType is public
func (vt VisibleType) IsPublic() bool {
	return vt == VisibleTypePublic
}

// IsLimited returns true if VisibleType is limited
func (vt VisibleType) IsLimited() bool {
	return vt == VisibleTypeLimited
}

// IsPrivate returns true if VisibleType is private
func (vt VisibleType) IsPrivate() bool {
	return vt == VisibleTypePrivate
}

// VisibilityString provides the mode string of the visibility type (public, limited, private)
func (vt VisibleType) String() string {
	for k, v := range VisibilityModes {
		if vt == v {
			return k
		}
	}
	return ""
}

// ExtractKeysFromMapString provides a slice of keys from map
func ExtractKeysFromMapString(in map[string]VisibleType) (keys []string) {
	for k := range in {
		keys = append(keys, k)
	}
	return keys
}
