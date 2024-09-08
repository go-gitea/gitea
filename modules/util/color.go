// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"strconv"
	"strings"
)

// HexToRBGColor parses color as RGB values in 0..255 range from the hex color string (with or without #)
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

// GetRelativeLuminance returns relative luminance for a SRGB color - https://en.wikipedia.org/wiki/Relative_luminance
// Keep this in sync with web_src/js/utils/color.js
func GetRelativeLuminance(color string) float64 {
	r, g, b := HexToRBGColor(color)
	return (0.2126729*r + 0.7151522*g + 0.0721750*b) / 255
}

func UseLightText(backgroundColor string) bool {
	return GetRelativeLuminance(backgroundColor) < 0.453
}

// ContrastColor returns a black or white foreground color that the highest contrast ratio.
// In the future, the APCA contrast function, or CSS `contrast-color` will be better.
// https://github.com/color-js/color.js/blob/eb7b53f7a13bb716ec8b28c7a56f052cd599acd9/src/contrast/APCA.js#L42
func ContrastColor(backgroundColor string) string {
	if UseLightText(backgroundColor) {
		return "#fff"
	}
	return "#000"
}
