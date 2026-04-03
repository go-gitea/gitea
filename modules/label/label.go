// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package label

import (
	"code.gitea.io/gitea/modules/util"
)

// Label represents label information loaded from template
type Label struct {
	Name           string `yaml:"name"`
	Color          string `yaml:"color"`
	Description    string `yaml:"description,omitempty"`
	Exclusive      bool   `yaml:"exclusive,omitempty"`
	ExclusiveOrder int    `yaml:"exclusive_order,omitempty"`
}

// NormalizeColor normalizes a color string to a lowercase 6-character hex code, e.g. "#aabbcc"
func NormalizeColor(color string) (string, error) {
	return util.NormalizeColor(color)
}
