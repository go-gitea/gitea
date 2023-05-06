// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

// return luminance given RGB channels
func GetLuminance(r float64, g float64, b float64) float64 {
	// Reference from: https://firsching.ch/github_labels.html and https://www.w3.org/WAI/GL/wiki/Relative_luminance
	// In the future WCAG 3 APCA may be a better solution
	luminance := (0.2126*r + 0.7152*g + 0.0722*b) / 255
	return luminance
}

func IsUseLightColor(r float64, g float64, b float64) bool {
	luminance := GetLuminance(r, g, b)
	lightnessThreshold := 0.453
	return luminance < lightnessThreshold
}
