// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package label

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// colorPattern is a regexp which can validate label color
var colorPattern = regexp.MustCompile("^#?(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})$")

// Priority represents label priority
type Priority string

var priorityValues = map[Priority]int{
	"critical": 1000,
	"high":     100,
	"medium":   0,
	"low":      -100,
}

// Label represents label information loaded from template
type Label struct {
	Name        string   `yaml:"name"`
	Exclusive   bool     `yaml:"exclusive,omitempty"`
	Color       string   `yaml:"color"`
	Priority    Priority `yaml:"priority,omitempty"`
	Description string   `yaml:"description,omitempty"`
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

var priorities []Priority

// Value returns numeric value for priority
func (p Priority) Value() int {
	v, ok := priorityValues[p]
	if !ok {
		return 0
	}
	return v
}

// Valid checks if priority is valid
func (p Priority) IsValid() bool {
	if p.IsEmpty() {
		return true
	}
	_, ok := priorityValues[p]
	return ok
}

// IsEmpty check if priority is not set
func (p Priority) IsEmpty() bool {
	return len(p) == 0
}

// GetPriorities returns list of priorities
func GetPriorities() []Priority {
	return priorities
}

func init() {
	type kv struct {
		Key   Priority
		Value int
	}
	var ss []kv
	for k, v := range priorityValues {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})
	priorities = make([]Priority, len(priorityValues))
	for i, kv := range ss {
		priorities[i] = kv.Key
	}
}
