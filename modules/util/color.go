// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"math"
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
	color, err := strconv.ParseUint(hexString, 16, 32)
	color32 := uint32(color)
	if err != nil {
		return 0, 0, 0
	}
	r := float64(uint8(0xFF & (color32 >> 16)))
	g := float64(uint8(0xFF & (color32 >> 8)))
	b := float64(uint8(0xFF & color32))
	return r, g, b
}

// linearizeChannel undoes the sRGB transfer function, channel is in 0..255 range
func linearizeChannel(channel float64) float64 {
	srgb := channel / 255
	if srgb <= 0.04045 {
		return srgb / 12.92
	}
	return math.Pow((srgb+0.055)/1.055, 2.4)
}

// getRelativeLuminance returns relative luminance for a SRGB color - https://www.w3.org/TR/WCAG20/#relativeluminancedef
// Keep this in sync with web_src/js/utils/color.ts
func getRelativeLuminance(color string) float64 {
	r, g, b := HexToRBGColor(color)
	return 0.2126*linearizeChannel(r) + 0.7152*linearizeChannel(g) + 0.0722*linearizeChannel(b)
}

// GetPerceivedBrightness weights the gamma-encoded channels, so it stays in the same space as the
// raw channels and can scale them proportionally. Not a luminance, don't use it for contrast.
func GetPerceivedBrightness(color string) float64 {
	r, g, b := HexToRBGColor(color)
	return (0.2126729*r + 0.7151522*g + 0.0721750*b) / 255
}

func UseLightText(backgroundColor string) bool {
	return getRelativeLuminance(backgroundColor) < 0.36 // matches APCA better than WCAG's own 0.179
}

// ContrastColor returns a black or white foreground color that the highest contrast ratio.
func ContrastColor(backgroundColor string) string {
	if UseLightText(backgroundColor) {
		return "#fff"
	}
	return "#000"
}
