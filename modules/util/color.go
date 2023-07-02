// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Check similar implementation in web_src/js/utils/color.js and keep synchronization

// Return R, G, B values defined in reletive luminance
func getLuminanceRGB(channel float64) float64 {
	sRGB := channel / 255
	if sRGB <= 0.03928 {
		return sRGB / 12.92
	}
	return math.Pow((sRGB+0.055)/1.055, 2.4)
}

// Get color as RGB values in 0..255 range from the hex color string (with or without #)
func HexToRBGColor(colorString string) (float64, float64, float64) {
	hexString := colorString
	if strings.HasPrefix(colorString, "#") {
		hexString = colorString[1:]
	}
	// only support transfer of rgb, rgba, rrggbb and rrggbbaa
	// if not in these formats, use default values 0, 0, 0
	if len(hexString) != 3 && len(hexString) != 4 && len(hexString) != 6 && len(hexString) != 8 {
		return 0, 0, 0
	}
	if len(hexString) == 3 || len(hexString) == 4 {
		hexString = fmt.Sprintf("%c%c%c%c%c%c", hexString[0], hexString[0], hexString[1], hexString[1], hexString[2], hexString[2])
	}
	if len(hexString) == 8 {
		hexString = hexString[0:6]
	}
	color, err := strconv.ParseUint(hexString, 16, 64)
	if err != nil {
		return 0, 0, 0
	}
	r := float64(uint8(0xFF & (uint32(color) >> 16)))
	g := float64(uint8(0xFF & (uint32(color) >> 8)))
	b := float64(uint8(0xFF & uint32(color)))
	return r, g, b
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
