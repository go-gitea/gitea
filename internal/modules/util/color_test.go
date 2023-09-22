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
		assert.Equal(t, c.expectedR, r, "case %d: error R should match: expected %f, but get %f", n, c.expectedR, r)
		assert.Equal(t, c.expectedG, g, "case %d: error G should match: expected %f, but get %f", n, c.expectedG, g)
		assert.Equal(t, c.expectedB, b, "case %d: error B should match: expected %f, but get %f", n, c.expectedB, b)
	}
}

func Test_UseLightTextOnBackground(t *testing.T) {
	cases := []struct {
		r        float64
		g        float64
		b        float64
		expected bool
	}{
		{215, 58, 74, true},
		{0, 117, 202, true},
		{207, 211, 215, false},
		{162, 238, 239, false},
		{112, 87, 255, true},
		{0, 134, 114, true},
		{228, 230, 105, false},
		{216, 118, 227, true},
		{255, 255, 255, false},
		{43, 134, 133, true},
		{43, 135, 134, true},
		{44, 135, 134, true},
		{59, 182, 179, true},
		{124, 114, 104, true},
		{126, 113, 108, true},
		{129, 112, 109, true},
		{128, 112, 112, true},
	}
	for n, c := range cases {
		result := UseLightTextOnBackground(c.r, c.g, c.b)
		assert.Equal(t, c.expected, result, "case %d: error should match", n)
	}
}
