// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_HexToRBGColor(t *testing.T) {
	cases := []struct {
		colorString string
		expectedR   float64
		expectedG   float64
		expectedB   float64
	}{
		{"2b8685", 43, 134, 133},
		{"1e1", 17, 238, 17},
		{"#1e1", 17, 238, 17},
		{"1e16", 17, 238, 17},
		{"3bb6b3", 59, 182, 179},
		{"#3bb6b399", 59, 182, 179},
		{"#0", 0, 0, 0},
		{"#00000", 0, 0, 0},
		{"#1234567", 0, 0, 0},
	}
	for n, c := range cases {
		r, g, b := HexToRBGColor(c.colorString)
		assert.InDelta(t, c.expectedR, r, 0, "case %d: error R should match: expected %f, but get %f", n, c.expectedR, r)
		assert.InDelta(t, c.expectedG, g, 0, "case %d: error G should match: expected %f, but get %f", n, c.expectedG, g)
		assert.InDelta(t, c.expectedB, b, 0, "case %d: error B should match: expected %f, but get %f", n, c.expectedB, b)
	}
}

func Test_UseLightText(t *testing.T) {
	cases := []struct {
		color    string
		expected string
	}{
		{"#d73a4a", "#fff"},
		{"#0075ca", "#fff"},
		{"#cfd3d7", "#000"},
		{"#a2eeef", "#000"},
		{"#7057ff", "#fff"},
		{"#008672", "#fff"},
		{"#e4e669", "#000"},
		{"#d876e3", "#000"},
		{"#ffffff", "#000"},
		{"#2b8684", "#fff"},
		{"#2b8786", "#fff"},
		{"#2c8786", "#000"},
		{"#3bb6b3", "#000"},
		{"#7c7268", "#fff"},
		{"#7e716c", "#fff"},
		{"#81706d", "#fff"},
		{"#807070", "#fff"},
		{"#84b6eb", "#000"},
	}
	for n, c := range cases {
		assert.Equal(t, c.expected, ContrastColor(c.color), "case %d: error should match", n)
	}
}
