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
	test("[[name=foo|link=javascript:bar]] and [[name=ok|link=./link]]", `[[name=foo|link=javascript:bar]] and <a href="/base/link">ok</a>`)
}
