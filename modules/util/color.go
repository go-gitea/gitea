// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package util

import (
	"math"
	"strconv"
	"strings"
)

// Check similar implementation in modules/util/color.go and keep synchronization

// Return R, G, B values defined in reletive luminance
func getLuminanceRGB(channel float64) float64 {
	sRGB := channel / 255
	if sRGB <= 0.03928 {
		return sRGB / 12.92
	}
	return math.Pow((sRGB+0.055)/1.055, 2.4)
}

// Get color as RGB values in 0..255 range from the hex color string (with or without #)
func HexToRBGColor(colorString string) (float64, float64, float64, error) {
	var color uint64
	var err error
	if strings.HasPrefix(colorString, "#") {
		color, err = strconv.ParseUint(colorString[1:], 16, 64)
	} else {
		color, err = strconv.ParseUint(colorString, 16, 64)
	}
	if err != nil {
		return 0, 0, 0, err
	}
	r := float64(uint8(0xFF & (uint32(color) >> 16)))
	g := float64(uint8(0xFF & (uint32(color) >> 8)))
	b := float64(uint8(0xFF & uint32(color)))
	return r, g, b, nil
}

// return luminance given RGB channels
// Reference from: https://www.w3.org/WAI/GL/wiki/Relative_luminance
func GetLuminance(r, g, b float64) float64 {
	R := getLuminanceRGB(r)
	G := getLuminanceRGB(g)
	B := getLuminanceRGB(b)
	luminance := 0.2126*R + 0.7152*G + 0.0722*B
	return luminance
}

// Reference from: https://firsching.ch/github_labels.html
// In the future WCAG 3 APCA may be a better solution.
// Check if text should use light color based on RGB of background
func UseLightTextOnBackground(r, g, b float64) bool {
	return GetLuminance(r, g, b) < 0.453
}
