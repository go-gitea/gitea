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
var VisibilityModes = map[VisibilityString]VisibleType{
	VisibilityStringPublic:  VisibleTypePublic,
	VisibilityStringLimited: VisibleTypeLimited,
	VisibilityStringPrivate: VisibleTypePrivate,
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
			return string(k)
		}
	}
	return ""
}

// VisibilityModeKeys returns the visibility mode names (public, limited, private)
func VisibilityModeKeys() (keys []string) {
	for k := range VisibilityModes {
		keys = append(keys, string(k))
	}
	return keys
}

// VisibilityString defines the visibility level of a user/organization/team as
// rendered in API and config payloads. The DB representation is VisibleType (int).
// swagger:enum VisibilityString
type VisibilityString string

const (
	VisibilityStringPublic  VisibilityString = "public"
	VisibilityStringLimited VisibilityString = "limited"
	VisibilityStringPrivate VisibilityString = "private"
)
