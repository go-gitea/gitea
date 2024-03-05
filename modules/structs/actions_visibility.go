// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// ActionsVisibility defines the activities shown
type ActionsVisibility int

const (
	// ActionsVisibilityPublic show public activities
	ActionsVisibilityPublic ActionsVisibility = iota

	// ActionsVisibilityAll shows all activities
	ActionsVisibilityAll

	// ActionsVisibilityNone show no activities
	ActionsVisibilityNone
)

// ActionsVisibilities is a map of ActionsVisibility types
var ActionsVisibilities = map[string]ActionsVisibility{
	"public": ActionsVisibilityPublic,
	"all":    ActionsVisibilityAll,
	"none":   ActionsVisibilityNone,
}

// ShowPublic returns true if ActionsVisibility is public
func (vt ActionsVisibility) ShowPublic() bool {
	return vt == ActionsVisibilityPublic
}

// ShowAll returns true if ActionsVisibility is all
func (vt ActionsVisibility) ShowAll() bool {
	return vt == ActionsVisibilityAll
}

// ShowNone returns true if ActionsVisibility is none
func (vt ActionsVisibility) ShowNone() bool {
	return vt == ActionsVisibilityNone
}

// String provides the mode string of the visibility type (public, all, none)
func (vt ActionsVisibility) String() string {
	for k, v := range ActionsVisibilities {
		if vt == v {
			return k
		}
	}
	return ""
}
