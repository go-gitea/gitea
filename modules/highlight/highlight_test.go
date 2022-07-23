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
	return strings.Split(strings.TrimSpace(s), "\n")
}

func TestFile(t *testing.T) {
	defaultNewLineInHTML := newLineInHTML
	defer func() {
		newLineInHTML = defaultNewLineInHTML
	}()

	newLineInHTML = "&#10;"
	tests := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "empty.py",
			code: "",
			want: []string{""},
		},
		{
			name: "tags.txt",
			code: "<>",
			want: []string{"&lt;&gt;"},
		},
		{
			name: "tags.py",
			code: "<>",
			want: []string{`<span class="o">&lt;</span><span class="o">&gt;</span>`},
		},
		{
			name: "eol-no.py",
			code: "a=1",
			want: []string{`<span class="n">a</span><span class="o">=</span><span class="mi">1</span>`},
		},
		{
			name: "eol-newline1.py",
			code: "a=1\n",
			want: []string{
				`<span class="n">a</span><span class="o">=</span><span class="mi">1</span>&#10;`,
			},
		},
		{
			name: "eol-newline2.py",
			code: "a=1\n\n",
			want: []string{
				`<span class="n">a</span><span class="o">=</span><span class="mi">1</span>&#10;`,
				`&#10;`,
			},
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
<span class="n">def</span><span class="p">:</span>&#10;
    <span class="n">a</span><span class="o">=</span><span class="mi">1</span>&#10;
&#10;
<span class="n">b</span><span class="o">=</span><span class="sa"></span><span class="s1">&#39;</span><span class="s1">&#39;</span>&#10;
    &#10;
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

	newLineInHTML = ""
	out, err := File("test-original-newline.py", "", []byte("a=1\n"))
	assert.NoError(t, err)
	assert.EqualValues(t, `<span class="n">a</span><span class="o">=</span><span class="mi">1</span>`+"\n", strings.Join(out, ""))
}

func TestPlainText(t *testing.T) {
	defaultNewLineInHTML := newLineInHTML
	defer func() {
		newLineInHTML = defaultNewLineInHTML
	}()

	newLineInHTML = "&#10;"
	tests := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "empty.py",
			code: "",
			want: []string{""},
		},
		{
			name: "tags.py",
			code: "<>",
			want: []string{"&lt;&gt;"},
		},
		{
			name: "eol-no.py",
			code: "a=1",
			want: []string{`a=1`},
		},
		{
			name: "eol-newline1.py",
			code: "a=1\n",
			want: []string{
				`a=1&#10;`,
			},
		},
		{
			name: "eol-newline2.py",
			code: "a=1\n\n",
			want: []string{
				`a=1&#10;`,
				`&#10;`,
			},
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
def:&#10;
    a=1&#10;
&#10;
b=&#39;&#39;&#10;
    &#10;
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

	newLineInHTML = ""
	out := PlainText([]byte("a=1\n"))
	assert.EqualValues(t, "a=1\n", strings.Join(out, ""))
}
