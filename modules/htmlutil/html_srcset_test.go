// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package htmlutil

import (
	"testing"

	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestSrcset(t *testing.T) {
	for _, test := range []struct {
		name       string
		input      string
		candidates []SrcSetCandidate
		encoded    string
	}{
		{
			name:       "bare URL",
			input:      "image.png",
			candidates: []SrcSetCandidate{{SrcLink: "image.png"}},
		},
		{
			name:       "bare URLs",
			input:      "a.png, b.png , c.png,",
			candidates: []SrcSetCandidate{{SrcLink: "a.png"}, {SrcLink: "b.png"}, {SrcLink: "c.png"}},
			encoded:    "a.png, b.png, c.png",
		},
		{
			name:       "empty candidates",
			input:      ", ,\ta.png 1x,, ,b.png 2x, ,",
			candidates: []SrcSetCandidate{{SrcLink: "a.png", SrcDesc: []string{"1x"}}, {SrcLink: "b.png", SrcDesc: []string{"2x"}}},
			encoded:    "a.png 1x, b.png 2x",
		},
		{
			name:       "trailing commas in URL",
			input:      "a.png,, b.png 2x",
			candidates: []SrcSetCandidate{{SrcLink: "a.png"}, {SrcLink: "b.png", SrcDesc: []string{"2x"}}},
			encoded:    "a.png, b.png 2x",
		},
		{
			name:       "comma in URL",
			input:      "comma,in,url.png",
			candidates: []SrcSetCandidate{{SrcLink: "comma,in,url.png"}},
		},
		{
			name:       "embedded data URL",
			input:      "data:image/png;base64,abcd 1x, image.png 2x",
			candidates: []SrcSetCandidate{{SrcLink: "data:image/png;base64,abcd", SrcDesc: []string{"1x"}}, {SrcLink: "image.png", SrcDesc: []string{"2x"}}},
		},
		{
			name:       "parenthesized descriptor",
			input:      "a.png (x,y) 1x, b.png 2x",
			candidates: []SrcSetCandidate{{SrcLink: "a.png", SrcDesc: []string{"(x,y)", "1x"}}, {SrcLink: "b.png", SrcDesc: []string{"2x"}}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidates := ParseSrcset(test.input)
			assert.Equal(t, test.candidates, candidates)
			encoded := EncodeSrcset(candidates)
			assert.Equal(t, util.IfZero(test.encoded, test.input), encoded)
		})
	}
}
