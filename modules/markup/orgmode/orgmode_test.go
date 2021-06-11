// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"
const Repo = "gogits/gogs"
const AppSubURL = AppURL + Repo + "/"

func TestRender_StandardLinks(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer, err := RenderString(&markup.RenderContext{
			URLPrefix: setting.AppSubURL,
		}, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	googleRendered := "<p><a href=\"https://google.com/\" title=\"https://google.com/\">https://google.com/</a></p>"
	test("[[https://google.com/]]", googleRendered)

	lnk := util.URLJoin(AppSubURL, "WikiPage")
	test("[[WikiPage][WikiPage]]",
		"<p><a href=\""+lnk+"\" title=\"WikiPage\">WikiPage</a></p>")
}

func TestRender_Images(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer, err := RenderString(&markup.RenderContext{
			URLPrefix: setting.AppSubURL,
		}, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	url := "../../.images/src/02/train.jpg"
	result := util.URLJoin(AppSubURL, url)

	test("[[file:"+url+"]]",
		"<p><img src=\""+result+"\" alt=\""+result+"\" title=\""+result+"\" /></p>")
}
