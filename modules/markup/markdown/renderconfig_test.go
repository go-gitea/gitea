// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRenderConfig_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		expected *RenderConfig
		args     string
	}{
		{
			"empty", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "",
			}, "",
		},
		{
			"lang", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "test",
			}, "lang: test",
		},
		{
			"metatable", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "",
			}, "gitea: table",
		},
		{
			"metanone", &RenderConfig{
				Meta: "none",
				Icon: "table",
				Lang: "",
			}, "gitea: none",
		},
		{
			"metadetails", &RenderConfig{
				Meta: "details",
				Icon: "table",
				Lang: "",
			}, "gitea: details",
		},
		{
			"metawrong", &RenderConfig{
				Meta: "details",
				Icon: "table",
				Lang: "",
			}, "gitea: wrong",
		},
		{
			"toc", &RenderConfig{
				TOC:  "true",
				Meta: "table",
				Icon: "table",
				Lang: "",
			}, "include_toc: true",
		},
		{
			"tocfalse", &RenderConfig{
				TOC:  "false",
				Meta: "table",
				Icon: "table",
				Lang: "",
			}, "include_toc: false",
		},
		{
			"toclang", &RenderConfig{
				Meta: "table",
				Icon: "table",
				TOC:  "true",
				Lang: "testlang",
			}, `
				include_toc: true
				lang: testlang
				`,
		},
		{
			"complexlang", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
			}, `
				gitea:
					lang: testlang
				`,
		},
		{
			"complexlang2", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
			}, `
	lang: notright
	gitea:
		lang: testlang
`,
		},
		{
			"complexlang", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
			}, `
	gitea:
		lang: testlang
`,
		},
		{
			"complex2", &RenderConfig{
				Lang: "two",
				Meta: "table",
				TOC:  "true",
				Icon: "smiley",
			}, `
	lang: one
	include_toc: true
	gitea:
		details_icon: smiley
		meta: table
		include_toc: true
		lang: two
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "",
			}
			err := yaml.Unmarshal([]byte(strings.ReplaceAll(tt.args, "\t", "    ")), got)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Meta, got.Meta)
			assert.Equal(t, tt.expected.Icon, got.Icon)
			assert.Equal(t, tt.expected.Lang, got.Lang)
			assert.Equal(t, tt.expected.TOC, got.TOC)
		})
	}
}
