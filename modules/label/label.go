// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package label

import (
	"fmt"
	"regexp"
	"strings"
)

// colorPattern is a regexp which can validate label color
var colorPattern = regexp.MustCompile("^#?(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})$")

// Label represents label information loaded from template
type Label struct {
	Name           string `yaml:"name"`
	Color          string `yaml:"color"`
	Description    string `yaml:"description,omitempty"`
	Exclusive      bool   `yaml:"exclusive,omitempty"`
	ExclusiveOrder int    `yaml:"exclusive_order,omitempty"`
}

// NormalizeColor normalizes a color string to a 6-character hex code
func NormalizeColor(color string) (string, error) {
	// normalize case
	color = strings.TrimSpace(strings.ToLower(color))

	// add leading hash
	if len(color) == 6 || len(color) == 3 {
		color = "#" + color
	}

	if !colorPattern.MatchString(color) {
		return "", fmt.Errorf("bad color code: %s", color)
	}

	// convert 3-character shorthand into 6-character version
	if len(color) == 4 {
		r := color[1]
		g := color[2]
		b := color[3]
		color = fmt.Sprintf("#%c%c%c%c%c%c", r, r, g, g, b, b)
	}

	return color, nil
}
