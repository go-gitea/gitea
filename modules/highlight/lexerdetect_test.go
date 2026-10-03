// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectChromaLexer(t *testing.T) {
	globalVars().highlightMapping[".my-html"] = "HTML"
	t.Cleanup(func() { delete(globalVars().highlightMapping, ".my-html") })

	casesWithContent := []struct {
		fileName string
		language string
		content  string
		expected string
	}{
		{"test.v", "", "", "V"},
		{"test.v", "any-lang-name", "", "V"},

		{"any-file", "javascript", "", "JavaScript"},
		{"any-file", "", "/* vim: set filetype=python */", "Python"},
		{"any-file", "", "", "fallback"},

		{"test.fs", "", "", "FSharp"},
		{"test.fs", "F#", "", "FSharp"},
		{"test.fs", "", "let x = 1", "FSharp"},

		{"test.c", "", "", "C"},
		{"test.C", "", "", "C++"},
		{"OLD-CODE.PAS", "", "", "ObjectPascal"},
		{"test.my-html", "", "", "HTML"},

		{"a.php", "", "", "PHP"},
		{"a.sql", "", "", "SQL"},
		{"dhcpd.conf", "", "", "ISCdhcpd"},
		{".env.my-production", "", "", "Bash"},

		{"a.hcl", "", "", "HCL"}, // not the same as Chroma, enry detects "*.hcl" as "HCL"
		{"a.hcl", "HCL", "", "HCL"},
		{"a.hcl", "Terraform", "", "Terraform"},
	}
	for _, c := range casesWithContent {
		lexer := detectChromaLexerWithAnalyze(c.fileName, c.language, []byte(c.content))
		if assert.NotNil(t, lexer, "case: %+v", c) {
			assert.Equal(t, c.expected, lexer.Config().Name, "case: %+v", c)
		}
	}

	casesNameLang := []struct {
		fileName string
		language string
		expected string
		byLang   bool
	}{
		{"a.v", "", "V", false},
		{"a.v", "V", "V", true},
		{"a.v", "verilog", "verilog", true},
		{"a.v", "any-lang-name", "V", false},

		{"a.hcl", "", "Terraform", false}, // not the same as enry
		{"a.hcl", "HCL", "HCL", true},
		{"a.hcl", "Terraform", "Terraform", true},
	}
	for _, c := range casesNameLang {
		lexer, byLang := detectChromaLexerByFileName(c.fileName, c.language)
		assert.Equal(t, c.expected, lexer.Config().Name, "case: %+v", c)
		assert.Equal(t, c.byLang, byLang, "case: %+v", c)
	}
}
