// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestShortLinkProcessor(t *testing.T) {
	test := func(input, expected string) {
		sb := new(strings.Builder)
		err := postProcess(NewTestRenderContext("/base"), []processor{shortLinkProcessor}, strings.NewReader(input), sb)
		assert.NoError(t, err)
		assert.Equal(t, test.NormalizeHTMLSpaces(expected), test.NormalizeHTMLSpaces(sb.String()))
	}
	test("[[name=foo|link=./link]]", `<a href="/base/link">foo</a>`)
	test("[[name=foo|link=javascript:bar]]", `[[name=foo|link=javascript:bar]]`)
	test("[[name=foo|link=javascript:bar]] and [[name=ok|link=./link]]", `[[name=foo|link=javascript:bar]] and <a href="/base/link">ok</a>`)
}

func TestLinkProcessorContinueAfterSkipped(t *testing.T) {
	defer test.MockVariableValue(&RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	run := func(input, expected string) {
		sb := new(strings.Builder)
		err := postProcess(NewTestRenderContext("/base"), []processor{linkProcessor}, strings.NewReader(input), sb)
		assert.NoError(t, err)
		assert.Equal(t, expected, sb.String())
	}
	run("See http://10… and https://example.com", `See http://10… and <a href="https://example.com">https://example.com</a>`)
	run("See https://example.com for more...", `See <a href="https://example.com">https://example.com</a> for more...`)
	run("http://10…", "http://10…")
	run("http://10.…", "http://10.…")
}

func TestEmailAddressProcessorContinueAfterSkipped(t *testing.T) {
	defer test.MockVariableValue(&RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	sb := new(strings.Builder)
	err := postProcess(NewTestRenderContext("/base"), []processor{emailAddressProcessor}, strings.NewReader("git@try.gitea.io:go-gitea/gitea.git and info@gitea.com"), sb)
	assert.NoError(t, err)
	assert.Equal(t, `git@try.gitea.io:go-gitea/gitea.git and <a href="mailto:info@gitea.com">info@gitea.com</a>`, sb.String())
}
