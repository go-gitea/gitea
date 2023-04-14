// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"errors"
	"html/template"
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/assetfs"

	"github.com/stretchr/testify/assert"
)

func TestExtractErrorLine(t *testing.T) {
	cases := []struct {
		code   string
		line   int
		pos    int
		target string
		expect string
	}{
		{"hello world\nfoo bar foo bar\ntest", 2, -1, "bar", `
foo bar foo bar
    ^^^     ^^^
`},

		{"hello world\nfoo bar foo bar\ntest", 2, 4, "bar", `
foo bar foo bar
    ^
`},

		{
			"hello world\nfoo bar foo bar\ntest", 2, 4, "",
			`
foo bar foo bar
    ^
`,
		},

		{
			"hello world\nfoo bar foo bar\ntest", 5, 0, "",
			`unable to find target line 5`,
		},
	}

	for _, c := range cases {
		actual := extractErrorLine([]byte(c.code), c.line, c.pos, c.target)
		assert.Equal(t, strings.TrimSpace(c.expect), strings.TrimSpace(actual))
	}
}

func TestHandleError(t *testing.T) {
	dir := t.TempDir()

	p := &templateErrorPrettier{assets: assetfs.Layered(assetfs.Local("tmp", dir))}

	test := func(s string, h func(error) string, expect string) {
		err := os.WriteFile(dir+"/test.tmpl", []byte(s), 0o644)
		assert.NoError(t, err)
		tmpl := template.New("test")
		_, err = tmpl.Parse(s)
		assert.Error(t, err)
		msg := h(err)
		assert.EqualValues(t, strings.TrimSpace(expect), strings.TrimSpace(msg))
	}

	test("{{", p.handleGenericTemplateError, `
template error: tmp:test:1 : unclosed action
----------------------------------------------------------------------
{{
----------------------------------------------------------------------
`)

	test("{{Func}}", p.handleFuncNotDefinedError, `
template error: tmp:test:1 : function "Func" not defined
----------------------------------------------------------------------
{{Func}}
  ^^^^
----------------------------------------------------------------------
`)

	test("{{'x'3}}", p.handleUnexpectedOperandError, `
template error: tmp:test:1 : unexpected "3" in operand
----------------------------------------------------------------------
{{'x'3}}
     ^
----------------------------------------------------------------------
`)

	// no idea about how to trigger such strange error, so mock an error to test it
	err := os.WriteFile(dir+"/test.tmpl", []byte("god knows XXX"), 0o644)
	assert.NoError(t, err)
	expectedMsg := `
template error: tmp:test:1 : expected end; found XXX
----------------------------------------------------------------------
god knows XXX
          ^^^
----------------------------------------------------------------------
`
	actualMsg := p.handleExpectedEndError(errors.New("template: test:1: expected end; found XXX"))
	assert.EqualValues(t, strings.TrimSpace(expectedMsg), strings.TrimSpace(actualMsg))
}
