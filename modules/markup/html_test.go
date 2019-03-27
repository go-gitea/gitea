// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup_test

import (
	"strings"
	"testing"

	. "code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestRender_Commits(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer := RenderString(".md", input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	var sha = "b6dd6210eaebc915fd5be5579c58cce4da2e2579"
	var commit = util.URLJoin(AppSubURL, "commit", sha)
	var subtree = util.URLJoin(commit, "src")
	var tree = strings.Replace(subtree, "/commit/", "/tree/", -1)

	test(sha, `<p><a href="`+commit+`" rel="nofollow">b6dd6210ea</a></p>`)
	test(sha[:7], `<p><a href="`+commit[:len(commit)-(40-7)]+`" rel="nofollow">b6dd621</a></p>`)
	test(sha[:39], `<p><a href="`+commit[:len(commit)-(40-39)]+`" rel="nofollow">b6dd6210ea</a></p>`)
	test(commit, `<p><a href="`+commit+`" rel="nofollow">b6dd6210ea</a></p>`)
	test(tree, `<p><a href="`+tree+`" rel="nofollow">b6dd6210ea/src</a></p>`)
	test("commit "+sha, `<p>commit <a href="`+commit+`" rel="nofollow">b6dd6210ea</a></p>`)
}

func TestRender_CrossReferences(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer := RenderString("a.md", input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	test(
		"gogits/gogs#12345",
		`<p><a href="`+util.URLJoin(AppURL, "gogits", "gogs", "issues", "12345")+`" rel="nofollow">gogits/gogs#12345</a></p>`)
	test(
		"go-gitea/gitea#12345",
		`<p><a href="`+util.URLJoin(AppURL, "go-gitea", "gitea", "issues", "12345")+`" rel="nofollow">go-gitea/gitea#12345</a></p>`)
}

func TestMisc_IsSameDomain(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	var sha = "b6dd6210eaebc915fd5be5579c58cce4da2e2579"
	var commit = util.URLJoin(AppSubURL, "commit", sha)

	assert.True(t, IsSameDomain(commit))
	assert.False(t, IsSameDomain("http://google.com/ncr"))
	assert.False(t, IsSameDomain("favicon.ico"))
}

func TestRender_links(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer := RenderString("a.md", input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}
	// Text that should be turned into URL

	test(
		"https://www.example.com",
		`<p><a href="https://www.example.com" rel="nofollow">https://www.example.com</a></p>`)
	test(
		"http://www.example.com",
		`<p><a href="http://www.example.com" rel="nofollow">http://www.example.com</a></p>`)
	test(
		"https://example.com",
		`<p><a href="https://example.com" rel="nofollow">https://example.com</a></p>`)
	test(
		"http://example.com",
		`<p><a href="http://example.com" rel="nofollow">http://example.com</a></p>`)
	test(
		"http://foo.com/blah_blah",
		`<p><a href="http://foo.com/blah_blah" rel="nofollow">http://foo.com/blah_blah</a></p>`)
	test(
		"http://foo.com/blah_blah/",
		`<p><a href="http://foo.com/blah_blah/" rel="nofollow">http://foo.com/blah_blah/</a></p>`)
	test(
		"http://www.example.com/wpstyle/?p=364",
		`<p><a href="http://www.example.com/wpstyle/?p=364" rel="nofollow">http://www.example.com/wpstyle/?p=364</a></p>`)
	test(
		"https://www.example.com/foo/?bar=baz&inga=42&quux",
		`<p><a href="https://www.example.com/foo/?bar=baz&amp;inga=42&amp;quux" rel="nofollow">https://www.example.com/foo/?bar=baz&amp;inga=42&amp;quux</a></p>`)
	test(
		"http://142.42.1.1/",
		`<p><a href="http://142.42.1.1/" rel="nofollow">http://142.42.1.1/</a></p>`)
	test(
		"https://github.com/go-gitea/gitea/?p=aaa/bbb.html#ccc-ddd",
		`<p><a href="https://github.com/go-gitea/gitea/?p=aaa/bbb.html#ccc-ddd" rel="nofollow">https://github.com/go-gitea/gitea/?p=aaa/bbb.html#ccc-ddd</a></p>`)
	test(
		"https://en.wikipedia.org/wiki/URL_(disambiguation)",
		`<p><a href="https://en.wikipedia.org/wiki/URL_(disambiguation)" rel="nofollow">https://en.wikipedia.org/wiki/URL_(disambiguation)</a></p>`)
	test(
		"https://foo_bar.example.com/",
		`<p><a href="https://foo_bar.example.com/" rel="nofollow">https://foo_bar.example.com/</a></p>`)
	test(
		"https://stackoverflow.com/questions/2896191/what-is-go-used-fore",
		`<p><a href="https://stackoverflow.com/questions/2896191/what-is-go-used-fore" rel="nofollow">https://stackoverflow.com/questions/2896191/what-is-go-used-fore</a></p>`)
	test(
		"https://username:password@gitea.com",
		`<p><a href="https://username:password@gitea.com" rel="nofollow">https://username:password@gitea.com</a></p>`)

	// Test that should *not* be turned into URL
	test(
		"www.example.com",
		`<p>www.example.com</p>`)
	test(
		"example.com",
		`<p>example.com</p>`)
	test(
		"test.example.com",
		`<p>test.example.com</p>`)
	test(
		"http://",
		`<p>http://</p>`)
	test(
		"https://",
		`<p>https://</p>`)
	test(
		"://",
		`<p>://</p>`)
	test(
		"www",
		`<p>www</p>`)
}

func TestRender_ShortLinks(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL
	tree := util.URLJoin(AppSubURL, "src", "master")

	test := func(input, expected, expectedWiki string) {
		buffer := markdown.RenderString(input, tree, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
		buffer = markdown.RenderWiki([]byte(input), setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expectedWiki), strings.TrimSpace(string(buffer)))
	}

	rawtree := util.URLJoin(AppSubURL, "raw", "master")
	url := util.URLJoin(tree, "Link")
	otherURL := util.URLJoin(tree, "Other-Link")
	encodedURL := util.URLJoin(tree, "Link%3F")
	imgurl := util.URLJoin(rawtree, "Link.jpg")
	otherImgurl := util.URLJoin(rawtree, "Link+Other.jpg")
	encodedImgurl := util.URLJoin(rawtree, "Link+%23.jpg")
	notencodedImgurl := util.URLJoin(rawtree, "some", "path", "Link+#.jpg")
	urlWiki := util.URLJoin(AppSubURL, "wiki", "Link")
	otherURLWiki := util.URLJoin(AppSubURL, "wiki", "Other-Link")
	encodedURLWiki := util.URLJoin(AppSubURL, "wiki", "Link%3F")
	imgurlWiki := util.URLJoin(AppSubURL, "wiki", "raw", "Link.jpg")
	otherImgurlWiki := util.URLJoin(AppSubURL, "wiki", "raw", "Link+Other.jpg")
	encodedImgurlWiki := util.URLJoin(AppSubURL, "wiki", "raw", "Link+%23.jpg")
	notencodedImgurlWiki := util.URLJoin(AppSubURL, "wiki", "raw", "some", "path", "Link+#.jpg")
	favicon := "http://google.com/favicon.ico"

	test(
		"[[Link]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Link</a></p>`)
	test(
		"[[Link.jpg]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Link.jpg" alt="Link.jpg"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" title="Link.jpg" alt="Link.jpg"/></a></p>`)
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
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Name" alt="Name"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" title="Name" alt="Name"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=AltName]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="AltName" alt="AltName"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" title="AltName" alt="AltName"/></a></p>`)
	test(
		"[[Name|Link.jpg|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Title" alt="Title"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" title="Title" alt="Title"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=AltName|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Title" alt="AltName"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" title="Title" alt="AltName"/></a></p>`)
	test(
		"[[Name|Link.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Title" alt="AltName"/></a></p>`,
		`<p><a href="`+imgurlWiki+`" rel="nofollow"><img src="`+imgurlWiki+`" title="Title" alt="AltName"/></a></p>`)
	test(
		"[[Name|Link Other.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+otherImgurl+`" rel="nofollow"><img src="`+otherImgurl+`" title="Title" alt="AltName"/></a></p>`,
		`<p><a href="`+otherImgurlWiki+`" rel="nofollow"><img src="`+otherImgurlWiki+`" title="Title" alt="AltName"/></a></p>`)
	test(
		"[[Link]] [[Other Link]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a> <a href="`+otherURL+`" rel="nofollow">Other Link</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Link</a> <a href="`+otherURLWiki+`" rel="nofollow">Other Link</a></p>`)
	test(
		"[[Link?]]",
		`<p><a href="`+encodedURL+`" rel="nofollow">Link?</a></p>`,
		`<p><a href="`+encodedURLWiki+`" rel="nofollow">Link?</a></p>`)
	test(
		"[[Link]] [[Other Link]] [[Link?]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a> <a href="`+otherURL+`" rel="nofollow">Other Link</a> <a href="`+encodedURL+`" rel="nofollow">Link?</a></p>`,
		`<p><a href="`+urlWiki+`" rel="nofollow">Link</a> <a href="`+otherURLWiki+`" rel="nofollow">Other Link</a> <a href="`+encodedURLWiki+`" rel="nofollow">Link?</a></p>`)
	test(
		"[[Link #.jpg]]",
		`<p><a href="`+encodedImgurl+`" rel="nofollow"><img src="`+encodedImgurl+`"/></a></p>`,
		`<p><a href="`+encodedImgurlWiki+`" rel="nofollow"><img src="`+encodedImgurlWiki+`"/></a></p>`)
	test(
		"[[Name|Link #.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+encodedImgurl+`" rel="nofollow"><img src="`+encodedImgurl+`" title="Title" alt="AltName"/></a></p>`,
		`<p><a href="`+encodedImgurlWiki+`" rel="nofollow"><img src="`+encodedImgurlWiki+`" title="Title" alt="AltName"/></a></p>`)
	test(
		"[[some/path/Link #.jpg]]",
		`<p><a href="`+notencodedImgurl+`" rel="nofollow"><img src="`+notencodedImgurl+`"/></a></p>`,
		`<p><a href="`+notencodedImgurlWiki+`" rel="nofollow"><img src="`+notencodedImgurlWiki+`"/></a></p>`)
	test(
		"<p><a href=\"https://example.org\">[[foobar]]</a></p>",
		`<p><a href="https://example.org" rel="nofollow">[[foobar]]</a></p>`,
		`<p><a href="https://example.org" rel="nofollow">[[foobar]]</a></p>`)
}
