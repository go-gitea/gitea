// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package util

import "math"

func getRGB(channel float64) float64 {
	result := channel / 255
	if result <= 0.03928 {
		return result / 12.92 * 255
	}
	return math.Pow((result+0.055)/1.055, 2.4) * 255
}

// return luminance given RGB channels
func GetLuminance(r, g, b float64) float64 {
	// Reference from: https://firsching.ch/github_labels.html and https://www.w3.org/WAI/GL/wiki/Relative_luminance
	// In the future WCAG 3 APCA may be a better solution
	R := getRGB(r)
	G := getRGB(g)
	B := getRGB(b)
	luminance := (0.2126*R + 0.7152*G + 0.0722*B) / 255
	// luminance := (0.2126*r + 0.7152*g + 0.0722*b) / 255
	return luminance
}

func IsUseLightColor(r, g, b float64) bool {
	return GetLuminance(r, g, b) < 0.453
}
