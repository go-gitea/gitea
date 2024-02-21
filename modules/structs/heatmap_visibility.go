// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// HeatmapVisibility defines the activities shown in heatmap
type HeatmapVisibility int

const (
	// HeatmapVisibilityPublic show public activities in heatmap
	HeatmapVisibilityPublic HeatmapVisibility = iota

	// HeatmapVisibilityAll shows all activities in heatmap
	HeatmapVisibilityAll

	// HeatmapVisibilityNone show no activities in heatmap
	HeatmapVisibilityNone
)

// HeatmapVisibilities is a map of HeatmapVisibility types
var HeatmapVisibilities = map[string]HeatmapVisibility{
	"public": HeatmapVisibilityPublic,
	"all":    HeatmapVisibilityAll,
	"none":   HeatmapVisibilityNone,
}

// ShowPublic returns true if HeatmapVisibility is public
func (vt HeatmapVisibility) ShowPublic() bool {
	return vt == HeatmapVisibilityPublic
}

// ShowAll returns true if HeatmapVisibility is all
func (vt HeatmapVisibility) ShowAll() bool {
	return vt == HeatmapVisibilityAll
}

// ShowNone returns true if HeatmapVisibility is none
func (vt HeatmapVisibility) ShowNone() bool {
	return vt == HeatmapVisibilityNone
}

// String provides the mode string of the visibility type (public, all, none)
func (vt HeatmapVisibility) String() string {
	for k, v := range HeatmapVisibilities {
		if vt == v {
			return k
		}
	}
	return ""
}
