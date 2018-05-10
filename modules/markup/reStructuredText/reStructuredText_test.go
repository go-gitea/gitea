// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rst

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"
const Repo = "go-gitea/gitea"
const AppSubURL = AppURL + Repo + "/"

func test(t *testing.T, input, expected string) {
	buffer := RenderString(input, setting.AppSubURL, nil, false)
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
}

func TestRender_StandardLinks(t *testing.T) {
	var appURL = setting.AppURL
	var appSubURL = setting.AppSubURL
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL
	defer func() {
		setting.AppURL = appURL
		setting.AppSubURL = appSubURL
	}()

	googleRendered := `<p><a href="https://google.com/">reStructuredText</a></p>`
	test(t, "reStructuredText_\n\n.. _reStructuredText: https://google.com/\n", googleRendered)

	// TODO: gorst didn't support relative link.
	/*lnk := markup.URLJoin(AppSubURL, "WikiPage")
	test("WikiPage_\n\n.. _WikiPage: WikiPage\n",
		`<p><a href="`+lnk+`">WikiPage</a></p>`)*/
}

func TestRender_Images(t *testing.T) {
	var appURL = setting.AppURL
	var appSubURL = setting.AppSubURL
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL
	defer func() {
		setting.AppURL = appURL
		setting.AppSubURL = appSubURL
	}()

	// TODO: relative image link is not supported by gorst
	//url := "../../.images/src/02/train.jpg"
	//result := markup.URLJoin(AppSubURL, url)
	url := "https://help.github.com/assets/images/site/favicon.png"
	result := url

	test(t,
		".. image:: "+url,
		`<img src="`+result+`" alt="`+result+`" />`)
}
