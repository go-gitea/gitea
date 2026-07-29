// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lettersdigits

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func terms(input string) []string {
	toks := NewTokenizer().Tokenize([]byte(input))
	out := make([]string, len(toks))
	for i, tk := range toks {
		out[i] = string(tk.Term)
	}
	return out
}

func TestTokenizeLettersDigits(t *testing.T) {
	assert.Equal(t, []string{"console", "log"}, terms("console.log"))
	assert.Equal(t, []string{"vlan", "699", "configuration"}, terms("vlan 699 configuration"))
	assert.Equal(t, []string{"file3"}, terms("file3"))
	assert.Equal(t, []string{"你", "好", "世", "界"}, terms("你好世界"))
	assert.Equal(t, []string{"变", "量", "123", "名", "称"}, terms("变量123名称"))
	assert.Equal(t, []string{"hello", "你", "好", "world", "世", "界", "123"}, terms("hello 你好 world 世界 123"))
	assert.Empty(t, terms(""))
	assert.Equal(t, []string{"a"}, terms("a"))
}

func TestTokenizeOffsetsAreValidByteRanges(t *testing.T) {
	input := "变量123名称 vlan.699"
	toks := NewTokenizer().Tokenize([]byte(input))
	for _, tk := range toks {
		assert.Equal(t, string(tk.Term), input[tk.Start:tk.End], "token %q offsets must slice back to itself", tk.Term)
	}
	for i := 1; i < len(toks); i++ {
		assert.Positive(t, toks[i].Position-toks[i-1].Position, "position must strictly increase")
	}
}
