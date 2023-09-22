// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeCustomLabels(t *testing.T) {
	files := mergeCustomLabelFiles(optionFileList{
		all:    []string{"a", "a.yaml", "a.yml"},
		custom: nil,
	})
	assert.EqualValues(t, []string{"a.yaml"}, files, "yaml file should win")

	files = mergeCustomLabelFiles(optionFileList{
		all:    []string{"a", "a.yaml"},
		custom: []string{"a"},
	})
	assert.EqualValues(t, []string{"a"}, files, "custom file should win")

	files = mergeCustomLabelFiles(optionFileList{
		all:    []string{"a", "a.yml", "a.yaml"},
		custom: []string{"a", "a.yml"},
	})
	assert.EqualValues(t, []string{"a.yml"}, files, "custom yml file should win if no yaml")
}
