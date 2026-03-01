// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func lines(s string) (out []template.HTML) {
	// "" => [], "a" => ["a"], "a\n" => ["a\n"], "a\nb" => ["a\n", "b"] (each line always includes EOL "\n" if it exists)
	out = make([]template.HTML, 0)
	s = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "\n", ""), `\n`, "\n")
	for {
		if p := strings.IndexByte(s, '\n'); p != -1 {
			out = append(out, template.HTML(s[:p+1]))
			s = s[p+1:]
		} else {
			break
		}
	}
	if s != "" {
		out = append(out, template.HTML(s))
	}
	return out
}

func TestFile(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		want      []template.HTML
		lexerName string
	}{
		{
			name:      "empty.py",
			code:      "",
			want:      lines(""),
			lexerName: "Python",
		},
		{
			name:      "empty.js",
			code:      "",
			want:      lines(""),
			lexerName: "JavaScript",
		},
		{
			name:      "empty.yaml",
			code:      "",
			want:      lines(""),
			lexerName: "YAML",
		},
		{
			name:      "tags.txt",
			code:      "<>",
			want:      lines("&lt;&gt;"),
			lexerName: "Plaintext",
		},
		{
			name:      "tags.py",
			code:      "<>",
			want:      lines("&lt;&gt;"),
			lexerName: "Python",
		},
		{
			name:      "eol-no.py",
			code:      "a=1",
			want:      lines(`<span class="nv">a</span><span class="o">=</span><span class="m">1</span>`),
			lexerName: "Python",
		},
		{
			name:      "eol-newline1.py",
			code:      "a=1\n",
			want:      lines(`<span class="nv">a</span><span class="o">=</span><span class="m">1</span>\n`),
			lexerName: "Python",
		},
		{
			name: "eol-newline2.py",
			code: "a=1\n\n",
			want: lines(`
<span class="nv">a</span><span class="o">=</span><span class="m">1</span>\n
\n
			`,
			),
			lexerName: "Python",
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
<span class="k">def</span>:\n
    <span class="nv">a</span>=1\n
\n
b=&#39;&#39;\n
    \n
c=2`,
			),
			lexerName: "Python",
		},
		{
			name:      "test.sql",
			code:      "--\nSELECT",
			want:      []template.HTML{"<span class=\"c\">--</span>\n", `<span class="k">SELECT</span>`},
			lexerName: "SQL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, lexerName, err := RenderFullFile(tt.name, "", []byte(tt.code))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, out)
			assert.Equal(t, tt.lexerName, lexerName)
		})
	}
}

func TestPlainText(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []template.HTML
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
			out := RenderPlainText([]byte(tt.code))
			assert.Equal(t, tt.want, out)
		})
	}
}

func TestUnsafeSplitHighlightedLines(t *testing.T) {
	ret := UnsafeSplitHighlightedLines("")
	assert.Empty(t, ret)

	ret = UnsafeSplitHighlightedLines("a")
	assert.Len(t, ret, 1)
	assert.Equal(t, "a", string(ret[0]))

	ret = UnsafeSplitHighlightedLines("\n")
	assert.Len(t, ret, 1)
	assert.Equal(t, "\n", string(ret[0]))

	ret = UnsafeSplitHighlightedLines("<span>a</span>\n<span>b\n</span>")
	assert.Len(t, ret, 2)
	assert.Equal(t, "<span>a</span>\n", string(ret[0]))
	assert.Equal(t, "<span>b\n</span>", string(ret[1]))
}
