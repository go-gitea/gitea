// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"testing"

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
				TOC:  true,
				Meta: "table",
				Icon: "table",
				Lang: "",
			}, "include_toc: true",
		},
		{
			"tocfalse", &RenderConfig{
				TOC:  false,
				Meta: "table",
				Icon: "table",
				Lang: "",
			}, "include_toc: false",
		},
		{
			"toclang", &RenderConfig{
				Meta: "table",
				Icon: "table",
				TOC:  true,
				Lang: "testlang",
			}, `  include_toc: true
  lang: testlang`,
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
			"complex2", &RenderConfig{
				Lang: "two",
				Meta: "table",
				TOC:  true,
				Icon: "smiley",
			}, `
  lang: one
  include_toc: false
  gitea:
    details_icon: smiley
    meta: table
    include_toc: true
    lang: two
`,
		},
		{
			"complex3", &RenderConfig{
				Lang: "two",
				Meta: "table",
				TOC:  false,
				Icon: "smiley",
			}, `
  lang: one
  include_toc: true
  gitea:
    details_icon: smiley
    meta: table
    include_toc: false
    lang: two
`,
		},
		{
			"mathall", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
				Math: &MathConfig{
					InlineDollar:  true,
					InlineLatex:   true,
					DisplayDollar: true,
					DisplayLatex:  true,
				},
			}, `
  math: all
  gitea:
    lang: testlang
`,
		},
		{
			"mathtrue", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
				Math: &MathConfig{
					InlineDollar:  false,
					InlineLatex:   true,
					DisplayDollar: true,
					DisplayLatex:  true,
				},
			}, `
  math: true
  gitea:
    lang: testlang
`,
		},
		{
			"mathstrings", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
				Math: &MathConfig{
					InlineDollar:  true,
					InlineLatex:   false,
					DisplayDollar: true,
					DisplayLatex:  false,
				},
			}, `
  math: "display_dollar,inline_dollar"
  gitea:
    lang: testlang
`,
		},
		{
			"mathstringarray", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
				Math: &MathConfig{
					InlineDollar:  true,
					InlineLatex:   false,
					DisplayDollar: true,
					DisplayLatex:  false,
				},
			}, `
  math: [display_dollar,inline_dollar]
  gitea:
    lang: testlang
`,
		},
		{
			"mathstringarrayalone", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "",
				Math: &MathConfig{
					InlineDollar:  true,
					InlineLatex:   false,
					DisplayDollar: true,
					DisplayLatex:  false,
				},
			}, `math: [display_dollar,inline_dollar]`,
		},
		{
			"mathstruct", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
				Math: &MathConfig{
					InlineDollar:  false,
					InlineLatex:   true,
					DisplayDollar: true,
					DisplayLatex:  false,
				},
			}, `
  math:
    display_dollar: true
    inline_latex: true
  gitea:
    lang: testlang
`,
		},
		{
			"mathoverride", &RenderConfig{
				Meta: "table",
				Icon: "table",
				Lang: "testlang",
				Math: &MathConfig{
					InlineDollar:  false,
					InlineLatex:   true,
					DisplayDollar: true,
					DisplayLatex:  false,
				},
			}, `
  math:
    inline_dollar: true
    display_latex: true
  gitea:
    lang: testlang
    math:
      display_dollar: true
      inline_latex: true
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
			if err := yaml.Unmarshal([]byte(tt.args), got); err != nil {
				t.Errorf("RenderConfig.UnmarshalYAML() error = %v", err)
				return
			}

			if got.Meta != tt.expected.Meta {
				t.Errorf("Meta Expected %s Got %s", tt.expected.Meta, got.Meta)
			}
			if got.Icon != tt.expected.Icon {
				t.Errorf("Icon Expected %s Got %s", tt.expected.Icon, got.Icon)
			}
			if got.Lang != tt.expected.Lang {
				t.Errorf("Lang Expected %s Got %s", tt.expected.Lang, got.Lang)
			}
			if got.TOC != tt.expected.TOC {
				t.Errorf("TOC Expected %t Got %t", tt.expected.TOC, got.TOC)
			}
		})
	}
}
