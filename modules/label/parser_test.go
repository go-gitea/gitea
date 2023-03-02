// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package label

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYamlParser(t *testing.T) {
	data := []byte(`labels:
  - name: priority/low
    exclusive: true
    color: "#0000ee"
    description: "Low priority"
  - name: priority/medium
    exclusive: true
    color: "0e0"
    description: "Medium priority"
  - name: priority/high
    exclusive: true
    color: "#ee0000"
    description: "High priority"
  - name: type/bug
    color: "#f00"
    description: "Bug"`)

	labels, err := parseYamlFormat("test", data)
	require.NoError(t, err)
	require.Len(t, labels, 4)
	assert.Equal(t, "priority/low", labels[0].Name)
	assert.True(t, labels[0].Exclusive)
	assert.Equal(t, "#0000ee", labels[0].Color)
	assert.Equal(t, "Low priority", labels[0].Description)
	assert.Equal(t, "priority/medium", labels[1].Name)
	assert.True(t, labels[1].Exclusive)
	assert.Equal(t, "#00ee00", labels[1].Color)
	assert.Equal(t, "Medium priority", labels[1].Description)
	assert.Equal(t, "priority/high", labels[2].Name)
	assert.True(t, labels[2].Exclusive)
	assert.Equal(t, "#ee0000", labels[2].Color)
	assert.Equal(t, "High priority", labels[2].Description)
	assert.Equal(t, "type/bug", labels[3].Name)
	assert.False(t, labels[3].Exclusive)
	assert.Equal(t, "#ff0000", labels[3].Color)
	assert.Equal(t, "Bug", labels[3].Description)
}

func TestLegacyParser(t *testing.T) {
	data := []byte(`#ee0701 bug   ;   Something is not working
#cccccc   duplicate ; This issue or pull request already exists
#84b6eb enhancement`)

	labels, err := parseLegacyFormat("test", data)
	require.NoError(t, err)
	require.Len(t, labels, 3)
	assert.Equal(t, "bug", labels[0].Name)
	assert.False(t, labels[0].Exclusive)
	assert.Equal(t, "#ee0701", labels[0].Color)
	assert.Equal(t, "Something is not working", labels[0].Description)
	assert.Equal(t, "duplicate", labels[1].Name)
	assert.False(t, labels[1].Exclusive)
	assert.Equal(t, "#cccccc", labels[1].Color)
	assert.Equal(t, "This issue or pull request already exists", labels[1].Description)
	assert.Equal(t, "enhancement", labels[2].Name)
	assert.False(t, labels[2].Exclusive)
	assert.Equal(t, "#84b6eb", labels[2].Color)
	assert.Empty(t, labels[2].Description)
}
