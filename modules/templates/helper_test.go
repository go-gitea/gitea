// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubjectBodySeparator(t *testing.T) {
	test := func(input, subject, body string) {
		loc := mailSubjectSplit.FindIndex([]byte(input))
		if loc == nil {
			assert.Empty(t, subject, "no subject found, but one expected")
			assert.Equal(t, body, input)
		} else {
			assert.Equal(t, subject, input[0:loc[0]])
			assert.Equal(t, body, input[loc[1]:])
		}
	}

	test("Simple\n---------------\nCase",
		"Simple\n",
		"\nCase")
	test("Only\nBody",
		"",
		"Only\nBody")
	test("Minimal\n---\nseparator",
		"Minimal\n",
		"\nseparator")
	test("False --- separator",
		"",
		"False --- separator")
	test("False\n--- separator",
		"",
		"False\n--- separator")
	test("False ---\nseparator",
		"",
		"False ---\nseparator")
	test("With extra spaces\n-----   \t   \nBody",
		"With extra spaces\n",
		"\nBody")
	test("With leading spaces\n   -------\nOnly body",
		"",
		"With leading spaces\n   -------\nOnly body")
	test("Multiple\n---\n-------\n---\nSeparators",
		"Multiple\n",
		"\n-------\n---\nSeparators")
	test("Insufficient\n--\nSeparators",
		"",
		"Insufficient\n--\nSeparators")
}

func TestJSEscapeSafe(t *testing.T) {
	assert.EqualValues(t, `\u0026\u003C\u003E\'\"`, JSEscapeSafe(`&<>'"`))
}

func TestHTMLFormat(t *testing.T) {
	assert.Equal(t, template.HTML("<a>&lt; < 1</a>"), HTMLFormat("<a>%s %s %d</a>", "<", template.HTML("<"), 1))
}

func TestSanitizeHTML(t *testing.T) {
	assert.Equal(t, template.HTML(`<a href="/" rel="nofollow">link</a> xss <div>inline</div>`), SanitizeHTML(`<a href="/">link</a> <a href="javascript:">xss</a> <div style="dangerous">inline</div>`))
}

func TestIsTruthy(t *testing.T) {
	var test any
	assert.Equal(t, false, IsTruthy(test))
	assert.Equal(t, false, IsTruthy(nil))
	assert.Equal(t, false, IsTruthy(""))
	assert.Equal(t, true, IsTruthy("non-empty"))
	assert.Equal(t, true, IsTruthy(-1))
	assert.Equal(t, false, IsTruthy(0))
	assert.Equal(t, true, IsTruthy(42))
	assert.Equal(t, false, IsTruthy(0.0))
	assert.Equal(t, true, IsTruthy(3.14))
	assert.Equal(t, false, IsTruthy([]int{}))
	assert.Equal(t, true, IsTruthy([]int{1}))
}
