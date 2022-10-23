// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package highlight

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func lines(s string) []string {
	return strings.Split(strings.ReplaceAll(strings.TrimSpace(s), `\n`, "\n"), "\n")
}

func TestFile(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "empty.py",
			code: "",
			want: lines(""),
		},
		{
			name: "tags.txt",
			code: "<>",
			want: lines("&lt;&gt;"),
		},
		{
			name: "tags.py",
			code: "<>",
			want: lines(`<span class="o">&lt;</span><span class="o">&gt;</span>`),
		},
		{
			name: "eol-no.py",
			code: "a=1",
			want: lines(`<span class="n">a</span><span class="o">=</span><span class="mi">1</span>`),
		},
		{
			name: "eol-newline1.py",
			code: "a=1\n",
			want: lines(`<span class="n">a</span><span class="o">=</span><span class="mi">1</span>\n`),
		},
		{
			name: "eol-newline2.py",
			code: "a=1\n\n",
			want: lines(`
<span class="n">a</span><span class="o">=</span><span class="mi">1</span>\n
\n
			`,
			),
		},
		{
			name: "empty-line-with-space.py",
			code: strings.ReplaceAll(strings.TrimSpace(`
def:
    a=1

b=''
{space}
c=2
			`), "{space}", "    "),
			want: lines(`
<span class="n">def</span><span class="p">:</span>\n
    <span class="n">a</span><span class="o">=</span><span class="mi">1</span>\n
\n
<span class="n">b</span><span class="o">=</span><span class="sa"></span><span class="s1">&#39;</span><span class="s1">&#39;</span>\n
    \n
<span class="n">c</span><span class="o">=</span><span class="mi">2</span>`,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := File(tt.name, "", []byte(tt.code))
			assert.NoError(t, err)
			expected := strings.Join(tt.want, "\n")
			actual := strings.Join(out, "\n")
			assert.Equal(t, strings.Count(actual, "<span"), strings.Count(actual, "</span>"))
			assert.EqualValues(t, expected, actual)
		})
	}
}

func TestPlainText(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "empty.py",
			code: "",
			want: lines(""),
		},
		{
			name: "tags.py",
			code: "<>",
			want: lines("&lt;&gt;"),
		},
		{
			name: "eol-no.py",
			code: "a=1",
			want: lines(`a=1`),
		},
		{
			name: "eol-newline1.py",
			code: "a=1\n",
			want: lines(`a=1\n`),
		},
		{
			name: "eol-newline2.py",
			code: "a=1\n\n",
			want: lines(`
a=1\n
\n
			`),
		},
		{
			name: "empty-line-with-space.py",
			code: strings.ReplaceAll(strings.TrimSpace(`
def:
    a=1

b=''
{space}
c=2
			`), "{space}", "    "),
			want: lines(`
def:\n
    a=1\n
\n
b=&#39;&#39;\n
    \n
c=2`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := PlainText([]byte(tt.code))
			expected := strings.Join(tt.want, "\n")
			actual := strings.Join(out, "\n")
			assert.EqualValues(t, expected, actual)
		})
	}
}
