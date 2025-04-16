// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Attribute(t *testing.T) {
	assert.Empty(t, Attribute("").ToString().Value())
	assert.Empty(t, Attribute("unspecified").ToString().Value())
	assert.Equal(t, "python", Attribute("python").ToString().Value())
	assert.Equal(t, "Java", Attribute("Java").ToString().Value())

	attributes := Attributes{
		m: map[string]Attribute{
			LinguistGenerated:     "true",
			LinguistDocumentation: "false",
			LinguistDetectable:    "set",
			LinguistLanguage:      "Python",
			GitlabLanguage:        "Java",
			"filter":              "unspecified",
			"test":                "",
		},
	}

	assert.Empty(t, attributes.Get("test").ToString().Value())
	assert.Empty(t, attributes.Get("filter").ToString().Value())
	assert.Equal(t, "Python", attributes.Get(LinguistLanguage).ToString().Value())
	assert.Equal(t, "Java", attributes.Get(GitlabLanguage).ToString().Value())
	assert.True(t, attributes.Get(LinguistGenerated).ToBool().Value())
	assert.False(t, attributes.Get(LinguistDocumentation).ToBool().Value())
	assert.True(t, attributes.Get(LinguistDetectable).ToBool().Value())
}
