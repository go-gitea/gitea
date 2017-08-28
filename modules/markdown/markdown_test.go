// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	. "code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"
const Repo = "gogits/gogs"
const AppSubURL = AppURL + Repo + "/"

var numericMetas = map[string]string{
	"format": "https://someurl.com/{user}/{repo}/{index}",
	"user":   "someUser",
	"repo":   "someRepo",
	"style":  IssueNameStyleNumeric,
}

var alphanumericMetas = map[string]string{
	"format": "https://someurl.com/{user}/{repo}/{index}",
	"user":   "someUser",
	"repo":   "someRepo",
	"style":  IssueNameStyleAlphanumeric,
}

// numericLink an HTML to a numeric-style issue
func numericIssueLink(baseURL string, index int) string {
	return link(URLJoin(baseURL, strconv.Itoa(index)), fmt.Sprintf("#%d", index))
}

// alphanumLink an HTML link to an alphanumeric-style issue
func alphanumIssueLink(baseURL string, name string) string {
	return link(URLJoin(baseURL, name), name)
}

// urlContentsLink an HTML link whose contents is the target URL
func urlContentsLink(href string) string {
	return link(href, href)
}

// link an HTML link
func link(href, contents string) string {
	return fmt.Sprintf("<a href=\"%s\">%s</a>", href, contents)
}

func testRenderIssueIndexPattern(t *testing.T, input, expected string, metas map[string]string) {
	assert.Equal(t, expected,
		string(RenderIssueIndexPattern([]byte(input), AppSubURL, metas)))
}

func TestRender_StandardLinks(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected, expectedWiki string) {
		buffer := RenderString(input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
		bufferWiki := RenderWiki([]byte(input), setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expectedWiki), strings.TrimSpace(bufferWiki))
	}

	googleRendered := `<p><a href="https://google.com/" rel="nofollow">https://google.com/</a></p>`
	test("<https://google.com/>", googleRendered, googleRendered)

	lnk := URLJoin(AppSubURL, "WikiPage")
	lnkWiki := URLJoin(AppSubURL, "wiki", "WikiPage")
	test("[WikiPage](WikiPage)",
		`<p><a href="`+lnk+`" rel="nofollow">WikiPage</a></p>`,
		`<p><a href="`+lnkWiki+`" rel="nofollow">WikiPage</a></p>`)
}

func TestRender_ShortLinks(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL
	tree := URLJoin(AppSubURL, "src", "master")

	test := func(input, expected, expectedWiki string) {
		buffer := RenderString(input, tree, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
		buffer = RenderWiki([]byte(input), setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expectedWiki), strings.TrimSpace(string(buffer)))
	}

	rawtree := URLJoin(AppSubURL, "raw", "master")
	url := URLJoin(tree, "Link")
	otherUrl := URLJoin(tree, "OtherLink")
	imgurl := URLJoin(rawtree, "Link.jpg")
	urlWiki := URLJoin(AppSubURL, "wiki", "Link")
	otherUrlWiki := URLJoin(AppSubURL, "wiki", "OtherLink")
	imgurlWiki := URLJoin(AppSubURL, "wiki", "raw", "Link.jpg")
	favicon := "http://google.com/favicon.ico"

	test(
		"[[Link]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Link</a></p>`)
	test(
		"[[Link.jpg]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="Link.jpg" title="Link.jpg"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="Link.jpg" title="Link.jpg"/></a></p>`)
	test(
		"[["+favicon+"]]",
		`<p><a href="`+favicon+`" rel="nofollow"><img src="`+favicon+`" title="favicon.ico"/></a></p>`,
		`<p><a href="`+favicon+`" rel="nofollow"><img src="`+favicon+`" title="favicon.ico"/></a></p>`)
	test(
		"[[Name|Link]]",
		`<p><a href="`+url+`" rel="nofollow">Name</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Name</a></p>`)
	test(
		"[[Name|Link.jpg]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="Name" title="Name"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="Name" title="Name"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=AltName]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="AltName" title="AltName"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="AltName" title="AltName"/></a></p>`)
	test(
		"[[Name|Link.jpg|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="Title" title="Title"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="Title" title="Title"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=AltName|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="AltName" title="Title"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="AltName" title="Title"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="AltName" title="Title"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="AltName" title="Title"/></a></p>`)
	test(
		"[[Link]] [[OtherLink]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a> <a href="`+otherUrl+`" rel="nofollow">OtherLink</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Link</a> <a href="`+otherUrlWiki+`" rel="nofollow">OtherLink</a></p>`)
}

func TestMisc_IsMarkdownFile(t *testing.T) {
	setting.Markdown.FileExtensions = []string{".md", ".markdown", ".mdown", ".mkd"}
	trueTestCases := []string{
		"test.md",
		"wow.MARKDOWN",
		"LOL.mDoWn",
	}
	falseTestCases := []string{
		"test",
		"abcdefg",
		"abcdefghijklmnopqrstuvwxyz",
		"test.md.test",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, IsMarkdownFile(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, IsMarkdownFile(testCase))
	}
}

func TestRender_ShortLinks(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL
	tree := URLJoin(AppSubURL, "src", "master")

	test := func(input, expected, expectedWiki string) {
		buffer := RenderString(input, tree, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
		buffer = RenderWiki([]byte(input), setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expectedWiki), strings.TrimSpace(string(buffer)))
	}

	rawtree := URLJoin(AppSubURL, "raw", "master")
	url := URLJoin(tree, "Link")
	otherUrl := URLJoin(tree, "OtherLink")
	imgurl := URLJoin(rawtree, "Link.jpg")
	urlWiki := URLJoin(AppSubURL, "wiki", "Link")
	otherUrlWiki := URLJoin(AppSubURL, "wiki", "OtherLink")
	imgurlWiki := URLJoin(AppSubURL, "wiki", "raw", "Link.jpg")
	favicon := "http://google.com/favicon.ico"

	test(
		"[[Link]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Link</a></p>`)
	test(
		"[[Link.jpg]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="Link.jpg" title="Link.jpg"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="Link.jpg" title="Link.jpg"/></a></p>`)
	test(
		"[["+favicon+"]]",
		`<p><a href="`+favicon+`" rel="nofollow"><img src="`+favicon+`" title="favicon.ico"/></a></p>`,
		`<p><a href="`+favicon+`" rel="nofollow"><img src="`+favicon+`" title="favicon.ico"/></a></p>`)
	test(
		"[[Name|Link]]",
		`<p><a href="`+url+`" rel="nofollow">Name</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Name</a></p>`)
	test(
		"[[Name|Link.jpg]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="Name" title="Name"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="Name" title="Name"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=AltName]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="AltName" title="AltName"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="AltName" title="AltName"/></a></p>`)
	test(
		"[[Name|Link.jpg|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="Title" title="Title"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="Title" title="Title"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=AltName|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="AltName" title="Title"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="AltName" title="Title"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" alt="AltName" title="Title"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" alt="AltName" title="Title"/></a></p>`)
	test(
		"[[Link]] [[OtherLink]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a> <a href="`+otherUrl+`" rel="nofollow">OtherLink</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Link</a> <a href="`+otherUrlWiki+`" rel="nofollow">OtherLink</a></p>`)
}

func TestRender_Images(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer := RenderString(input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	url := "../../.images/src/02/train.jpg"
	title := "Train"
	result := URLJoin(AppSubURL, url)

	test(
		"!["+title+"]("+url+")",
		`<p><a href="`+result+`" rel="nofollow"><img src="`+result+`" alt="`+title+`"></a></p>`)

	test(
		"[["+title+"|"+url+"]]",
		`<p><a href="`+result+`" rel="nofollow"><img src="`+result+`" alt="`+title+`" title="`+title+`"/></a></p>`)
}

func TestRegExp_ShortLinkPattern(t *testing.T) {
	trueTestCases := []string{
		"[[stuff]]",
		"[[]]",
		"[[stuff|title=Difficult name with spaces*!]]",
	}
	falseTestCases := []string{
		"test",
		"abcdefg",
		"[[]",
		"[[",
		"[]",
		"]]",
		"abcdefghijklmnopqrstuvwxyz",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, ShortLinkPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, ShortLinkPattern.MatchString(testCase))
	}
}

func TestTotal_RenderWiki(t *testing.T) {
	answers := testAnswers(URLJoin(AppSubURL, "wiki/"), URLJoin(AppSubURL, "wiki", "raw/"))

	for i := 0; i < len(sameCases); i++ {
		line := RenderWiki([]byte(sameCases[i]), AppSubURL, nil)
		assert.Equal(t, answers[i], line)
	}

	testCases := []string{
		// Guard wiki sidebar: special syntax
		`[[Guardfile-DSL / Configuring-Guard|Guardfile-DSL---Configuring-Guard]]`,
		// rendered
		`<p><a href="` + AppSubURL + `wiki/Guardfile-DSL---Configuring-Guard" rel="nofollow">Guardfile-DSL / Configuring-Guard</a></p>
`,
		// special syntax
		`[[Name|Link]]`,
		// rendered
		`<p><a href="` + AppSubURL + `wiki/Link" rel="nofollow">Name</a></p>
`,
	}

	for i := 0; i < len(testCases); i += 2 {
		line := RenderWiki([]byte(testCases[i]), AppSubURL, nil)
		assert.Equal(t, testCases[i+1], line)
	}
}
