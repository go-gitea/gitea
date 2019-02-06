// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

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

// VisibilityModes is a map of org Visibility types
var VisibilityModes = map[string]VisibleType{
	"public":  VisibleTypePublic,
	"limited": VisibleTypeLimited,
	"private": VisibleTypePrivate,
}

func (vt VisibleType) IsPublic() bool {
	return vt == VisibleTypePublic
}

func (vt VisibleType) IsLimited() bool {
	return vt == VisibleTypeLimited
}

func (vt VisibleType) IsPrivate() bool {
	return vt == VisibleTypePrivate
}
