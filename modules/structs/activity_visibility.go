// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// ActivityVisibility defines the activities shown
type ActivityVisibility int

const (
	// ActivityVisibilityPublic show public activities
	ActivityVisibilityPublic ActivityVisibility = iota

	// ActivityVisibilityAll shows all activities
	ActivityVisibilityAll

	// ActivityVisibilityNone show no activities
	ActivityVisibilityNone
)

// ActivityVisibilities is a map of ActivityVisibility types
var ActivityVisibilities = map[string]ActivityVisibility{
	"public": ActivityVisibilityPublic,
	"all":    ActivityVisibilityAll,
	"none":   ActivityVisibilityNone,
}

// ShowPublic returns true if ActivityVisibility is public
func (vt ActivityVisibility) ShowPublic() bool {
	return vt == ActivityVisibilityPublic
}

// ShowAll returns true if ActivityVisibility is all
func (vt ActivityVisibility) ShowAll() bool {
	return vt == ActivityVisibilityAll
}

// ShowNone returns true if ActivityVisibility is none
func (vt ActivityVisibility) ShowNone() bool {
	return vt == ActivityVisibilityNone
}

// String provides the mode string of the visibility type (public, all, none)
func (vt ActivityVisibility) String() string {
	for k, v := range ActivityVisibilities {
		if vt == v {
			return k
		}
	}
	return ""
}
