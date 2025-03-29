// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"io"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	testModule "code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

var (
	testRepoOwnerName = "user13"
	testRepoName      = "repo11"
	localMetas        = map[string]string{"user": testRepoOwnerName, "repo": testRepoName}
)

func TestRender_Commits(t *testing.T) {
	test := func(input, expected string) {
		rctx := markup.NewTestRenderContext(markup.TestAppURL, localMetas).WithRelativePath("a.md")
		buffer, err := markup.RenderString(rctx, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	repo := markup.TestAppURL + testRepoOwnerName + "/" + testRepoName + "/"
	commit := util.URLJoin(repo, "commit", sha)
	tree := util.URLJoin(repo, "tree", sha, "src")

	file := util.URLJoin(repo, "commit", sha, "example.txt")
	fileWithExtra := file + ":"
	fileWithHash := file + "#L2"
	fileWithHasExtra := file + "#L2:"
	commitCompare := util.URLJoin(repo, "compare", sha+"..."+sha)
	commitCompareWithHash := commitCompare + "#L2"

	test(sha, `<p><a href="`+commit+`" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test(sha[:7], `<p><a href="`+commit[:len(commit)-(40-7)]+`" rel="nofollow"><code>65f1bf2</code></a></p>`)
	test(sha[:39], `<p><a href="`+commit[:len(commit)-(40-39)]+`" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test(commit, `<p><a href="`+commit+`" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test(tree, `<p><a href="`+tree+`" rel="nofollow"><code>65f1bf27bc/src</code></a></p>`)

	test(file, `<p><a href="`+file+`" rel="nofollow"><code>65f1bf27bc/example.txt</code></a></p>`)
	test(fileWithExtra, `<p><a href="`+file+`" rel="nofollow"><code>65f1bf27bc/example.txt</code></a>:</p>`)
	test(fileWithHash, `<p><a href="`+fileWithHash+`" rel="nofollow"><code>65f1bf27bc/example.txt (L2)</code></a></p>`)
	test(fileWithHasExtra, `<p><a href="`+fileWithHash+`" rel="nofollow"><code>65f1bf27bc/example.txt (L2)</code></a>:</p>`)
	test(commitCompare, `<p><a href="`+commitCompare+`" rel="nofollow"><code>65f1bf27bc...65f1bf27bc</code></a></p>`)
	test(commitCompareWithHash, `<p><a href="`+commitCompareWithHash+`" rel="nofollow"><code>65f1bf27bc...65f1bf27bc (L2)</code></a></p>`)

	test("commit "+sha, `<p>commit <a href="`+commit+`" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test("/home/gitea/"+sha, "<p>/home/gitea/"+sha+"</p>")
	test("deadbeef", `<p>deadbeef</p>`)
	test("d27ace93", `<p>d27ace93</p>`)
	test(sha[:14]+".x", `<p>`+sha[:14]+`.x</p>`)

	expected14 := `<a href="` + commit[:len(commit)-(40-14)] + `" rel="nofollow"><code>` + sha[:10] + `</code></a>`
	test(sha[:14]+".", `<p>`+expected14+`.</p>`)
	test(sha[:14]+",", `<p>`+expected14+`,</p>`)
	test("["+sha[:14]+"]", `<p>[`+expected14+`]</p>`)
}

func TestRender_CrossReferences(t *testing.T) {
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	test := func(input, expected string) {
		rctx := markup.NewTestRenderContext(markup.TestAppURL, localMetas).WithRelativePath("a.md")
		buffer, err := markup.RenderString(rctx, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test(
		"test-owner/test-repo#12345",
		`<p><a href="`+util.URLJoin(markup.TestAppURL, "test-owner", "test-repo", "issues", "12345")+`" class="ref-issue" rel="nofollow">test-owner/test-repo#12345</a></p>`)
	test(
		"go-gitea/gitea#12345",
		`<p><a href="`+util.URLJoin(markup.TestAppURL, "go-gitea", "gitea", "issues", "12345")+`" class="ref-issue" rel="nofollow">go-gitea/gitea#12345</a></p>`)
	test(
		"/home/gitea/go-gitea/gitea#12345",
		`<p>/home/gitea/go-gitea/gitea#12345</p>`)
	test(
		util.URLJoin(markup.TestAppURL, "gogitea", "gitea", "issues", "12345"),
		`<p><a href="`+util.URLJoin(markup.TestAppURL, "gogitea", "gitea", "issues", "12345")+`" class="ref-issue" rel="nofollow">gogitea/gitea#12345</a></p>`)
	test(
		util.URLJoin(markup.TestAppURL, "go-gitea", "gitea", "issues", "12345"),
		`<p><a href="`+util.URLJoin(markup.TestAppURL, "go-gitea", "gitea", "issues", "12345")+`" class="ref-issue" rel="nofollow">go-gitea/gitea#12345</a></p>`)
	test(
		util.URLJoin(markup.TestAppURL, "gogitea", "some-repo-name", "issues", "12345"),
		`<p><a href="`+util.URLJoin(markup.TestAppURL, "gogitea", "some-repo-name", "issues", "12345")+`" class="ref-issue" rel="nofollow">gogitea/some-repo-name#12345</a></p>`)

	inputURL := "https://host/a/b/commit/0123456789012345678901234567890123456789/foo.txt?a=b#L2-L3"
	test(
		inputURL,
		`<p><a href="`+inputURL+`" rel="nofollow"><code>0123456789/foo.txt (L2-L3)</code></a></p>`)
}

func TestRender_links(t *testing.T) {
	setting.AppURL = markup.TestAppURL
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	test := func(input, expected string) {
		buffer, err := markup.RenderString(markup.NewTestRenderContext().WithRelativePath("a.md"), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	oldCustomURLSchemes := setting.Markdown.CustomURLSchemes
	markup.ResetDefaultSanitizerForTesting()
	defer func() {
		setting.Markdown.CustomURLSchemes = oldCustomURLSchemes
		markup.ResetDefaultSanitizerForTesting()
		markup.CustomLinkURLSchemes(oldCustomURLSchemes)
	}()
	setting.Markdown.CustomURLSchemes = []string{"ftp", "magnet"}
	markup.CustomLinkURLSchemes(setting.Markdown.CustomURLSchemes)

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
	test(
		"ftp://gitea.com/file.txt",
		`<p><a href="ftp://gitea.com/file.txt" rel="nofollow">ftp://gitea.com/file.txt</a></p>`)
	test(
		"magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcadabcde&dn=download",
		`<p><a href="magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcadabcde&amp;dn=download" rel="nofollow">magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcadabcde&amp;dn=download</a></p>`)
	test(
		`[link](https://example.com)`,
		`<p><a href="https://example.com" rel="nofollow">link</a></p>`)
	test(
		`[link](mailto:test@example.com)`,
		`<p><a href="mailto:test@example.com" rel="nofollow">link</a></p>`)
	test(
		`[link](javascript:xss)`,
		`<p>link</p>`)

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
	test(
		"ftps://gitea.com",
		`<p>ftps://gitea.com</p>`)

	t.Run("LinkEllipsis", func(t *testing.T) {
		input := util.EllipsisDisplayString("http://10.1.2.3", 12)
		assert.Equal(t, "http://10…", input)
		test(input, "<p>http://10…</p>")

		input = util.EllipsisDisplayString("http://10.1.2.3", 13)
		assert.Equal(t, "http://10.…", input)
		test(input, "<p>http://10.…</p>")
	})
}

func TestRender_email(t *testing.T) {
	setting.AppURL = markup.TestAppURL
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	test := func(input, expected string) {
		res, err := markup.RenderString(markup.NewTestRenderContext().WithRelativePath("a.md"), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(res))
	}
	// Text that should be turned into email link

	test(
		"info@gitea.com",
		`<p><a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a></p>`)
	test(
		"(info@gitea.com)",
		`<p>(<a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a>)</p>`)
	test(
		"[info@gitea.com]",
		`<p>[<a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a>]</p>`)
	test(
		"info@gitea.com.",
		`<p><a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a>.</p>`)
	test(
		"firstname+lastname@gitea.com",
		`<p><a href="mailto:firstname+lastname@gitea.com" rel="nofollow">firstname+lastname@gitea.com</a></p>`)
	test(
		"send email to info@gitea.co.uk.",
		`<p>send email to <a href="mailto:info@gitea.co.uk" rel="nofollow">info@gitea.co.uk</a>.</p>`)

	test(
		`j.doe@example.com,
	j.doe@example.com.
	j.doe@example.com;
	j.doe@example.com?
	j.doe@example.com!`,
		`<p><a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>,
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>.
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>;
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>?
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>!</p>`)

	// Test that should *not* be turned into email links
	test(
		"\"info@gitea.com\"",
		`<p>&#34;info@gitea.com&#34;</p>`)
	test(
		"/home/gitea/mailstore/info@gitea/com",
		`<p>/home/gitea/mailstore/info@gitea/com</p>`)
	test(
		"git@try.gitea.io:go-gitea/gitea.git",
		`<p>git@try.gitea.io:go-gitea/gitea.git</p>`)
	test(
		"gitea@3",
		`<p>gitea@3</p>`)
	test(
		"gitea@gmail.c",
		`<p>gitea@gmail.c</p>`)
	test(
		"email@domain@domain.com",
		`<p>email@domain@domain.com</p>`)
	test(
		"email@domain..com",
		`<p>email@domain..com</p>`)
}

func TestRender_emoji(t *testing.T) {
	setting.AppURL = markup.TestAppURL
	setting.StaticURLPrefix = markup.TestAppURL

	test := func(input, expected string) {
		expected = strings.ReplaceAll(expected, "&", "&amp;")
		buffer, err := markup.RenderString(markup.NewTestRenderContext().WithRelativePath("a.md"), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	// Make sure we can successfully match every emoji in our dataset with regex
	for i := range emoji.GemojiData {
		test(
			emoji.GemojiData[i].Emoji,
			`<p><span class="emoji" aria-label="`+emoji.GemojiData[i].Description+`">`+emoji.GemojiData[i].Emoji+`</span></p>`)
	}
	for i := range emoji.GemojiData {
		test(
			":"+emoji.GemojiData[i].Aliases[0]+":",
			`<p><span class="emoji" aria-label="`+emoji.GemojiData[i].Description+`">`+emoji.GemojiData[i].Emoji+`</span></p>`)
	}

	// Text that should be turned into or recognized as emoji
	test(
		":gitea:",
		`<p><span class="emoji" aria-label="gitea"><img alt=":gitea:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/gitea.png"/></span></p>`)
	test(
		":custom-emoji:",
		`<p>:custom-emoji:</p>`)
	setting.UI.CustomEmojisMap["custom-emoji"] = ":custom-emoji:"
	test(
		":custom-emoji:",
		`<p><span class="emoji" aria-label="custom-emoji"><img alt=":custom-emoji:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/custom-emoji.png"/></span></p>`)
	test(
		"这是字符:1::+1: some🐊 \U0001f44d:custom-emoji: :gitea:",
		`<p>这是字符:1:<span class="emoji" aria-label="thumbs up">👍</span> some<span class="emoji" aria-label="crocodile">🐊</span> `+
			`<span class="emoji" aria-label="thumbs up">👍</span><span class="emoji" aria-label="custom-emoji"><img alt=":custom-emoji:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/custom-emoji.png"/></span> `+
			`<span class="emoji" aria-label="gitea"><img alt=":gitea:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/gitea.png"/></span></p>`)
	test(
		"Some text with 😄 in the middle",
		`<p>Some text with <span class="emoji" aria-label="grinning face with smiling eyes">😄</span> in the middle</p>`)
	test(
		"Some text with :smile: in the middle",
		`<p>Some text with <span class="emoji" aria-label="grinning face with smiling eyes">😄</span> in the middle</p>`)
	test(
		"Some text with 😄😄 2 emoji next to each other",
		`<p>Some text with <span class="emoji" aria-label="grinning face with smiling eyes">😄</span><span class="emoji" aria-label="grinning face with smiling eyes">😄</span> 2 emoji next to each other</p>`)
	test(
		"😎🤪🔐🤑❓",
		`<p><span class="emoji" aria-label="smiling face with sunglasses">😎</span><span class="emoji" aria-label="zany face">🤪</span><span class="emoji" aria-label="locked with key">🔐</span><span class="emoji" aria-label="money-mouth face">🤑</span><span class="emoji" aria-label="red question mark">❓</span></p>`)

	// should match nothing
	test(
		"2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		`<p>2001:0db8:85a3:0000:0000:8a2e:0370:7334</p>`)
	test(
		":not exist:",
		`<p>:not exist:</p>`)
}

func TestRender_ShortLinks(t *testing.T) {
	setting.AppURL = markup.TestAppURL
	tree := util.URLJoin(markup.TestRepoURL, "src", "master")

	test := func(input, expected string) {
		buffer, err := markdown.RenderString(markup.NewTestRenderContext(tree), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	url := util.URLJoin(tree, "Link")
	otherURL := util.URLJoin(tree, "Other-Link")
	encodedURL := util.URLJoin(tree, "Link%3F")
	imgurl := util.URLJoin(tree, "Link.jpg")
	otherImgurl := util.URLJoin(tree, "Link+Other.jpg")
	encodedImgurl := util.URLJoin(tree, "Link+%23.jpg")
	notencodedImgurl := util.URLJoin(tree, "some", "path", "Link+#.jpg")
	renderableFileURL := util.URLJoin(tree, "markdown_file.md")
	unrenderableFileURL := util.URLJoin(tree, "file.zip")
	favicon := "http://google.com/favicon.ico"

	test(
		"[[Link]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a></p>`,
	)
	test(
		"[[Link.-]]",
		`<p><a href="http://localhost:3000/test-owner/test-repo/src/master/Link.-" rel="nofollow">Link.-</a></p>`,
	)
	test(
		"[[Link.jpg]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Link.jpg" alt="Link.jpg"/></a></p>`,
	)
	test(
		"[["+favicon+"]]",
		`<p><a href="`+favicon+`" rel="nofollow"><img src="`+favicon+`" title="favicon.ico" alt="`+favicon+`"/></a></p>`,
	)
	test(
		"[[Name|Link]]",
		`<p><a href="`+url+`" rel="nofollow">Name</a></p>`,
	)
	test(
		"[[Name|Link.jpg]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Name" alt="Name"/></a></p>`,
	)
	test(
		"[[Name|Link.jpg|alt=AltName]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="AltName" alt="AltName"/></a></p>`,
	)
	test(
		"[[Name|Link.jpg|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Title" alt="Title"/></a></p>`,
	)
	test(
		"[[Name|Link.jpg|alt=AltName|title=Title]]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Title" alt="AltName"/></a></p>`,
	)
	test(
		"[[Name|Link.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+imgurl+`" rel="nofollow"><img src="`+imgurl+`" title="Title" alt="AltName"/></a></p>`,
	)
	test(
		"[[Name|Link Other.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+otherImgurl+`" rel="nofollow"><img src="`+otherImgurl+`" title="Title" alt="AltName"/></a></p>`,
	)
	test(
		"[[Link]] [[Other Link]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a> <a href="`+otherURL+`" rel="nofollow">Other Link</a></p>`,
	)
	test(
		"[[Link?]]",
		`<p><a href="`+encodedURL+`" rel="nofollow">Link?</a></p>`,
	)
	test(
		"[[Link]] [[Other Link]] [[Link?]]",
		`<p><a href="`+url+`" rel="nofollow">Link</a> <a href="`+otherURL+`" rel="nofollow">Other Link</a> <a href="`+encodedURL+`" rel="nofollow">Link?</a></p>`,
	)
	test(
		"[[markdown_file.md]]",
		`<p><a href="`+renderableFileURL+`" rel="nofollow">markdown_file.md</a></p>`,
	)
	test(
		"[[file.zip]]",
		`<p><a href="`+unrenderableFileURL+`" rel="nofollow">file.zip</a></p>`,
	)
	test(
		"[[Link #.jpg]]",
		`<p><a href="`+encodedImgurl+`" rel="nofollow"><img src="`+encodedImgurl+`" title="Link #.jpg" alt="Link #.jpg"/></a></p>`,
	)
	test(
		"[[Name|Link #.jpg|alt=\"AltName\"|title='Title']]",
		`<p><a href="`+encodedImgurl+`" rel="nofollow"><img src="`+encodedImgurl+`" title="Title" alt="AltName"/></a></p>`,
	)
	test(
		"[[some/path/Link #.jpg]]",
		`<p><a href="`+notencodedImgurl+`" rel="nofollow"><img src="`+notencodedImgurl+`" title="Link #.jpg" alt="some/path/Link #.jpg"/></a></p>`,
	)
	test(
		"<p><a href=\"https://example.org\">[[foobar]]</a></p>",
		`<p><a href="https://example.org" rel="nofollow">[[foobar]]</a></p>`,
	)
}

func Test_ParseClusterFuzz(t *testing.T) {
	setting.AppURL = markup.TestAppURL

	localMetas := map[string]string{"user": "go-gitea", "repo": "gitea"}

	data := "<A><maTH><tr><MN><bodY ÿ><temPlate></template><tH><tr></A><tH><d<bodY "

	var res strings.Builder
	err := markup.PostProcessDefault(markup.NewTestRenderContext(localMetas), strings.NewReader(data), &res)
	assert.NoError(t, err)
	assert.NotContains(t, res.String(), "<html")

	data = "<!DOCTYPE html>\n<A><maTH><tr><MN><bodY ÿ><temPlate></template><tH><tr></A><tH><d<bodY "

	res.Reset()
	err = markup.PostProcessDefault(markup.NewTestRenderContext(localMetas), strings.NewReader(data), &res)

	assert.NoError(t, err)
	assert.NotContains(t, res.String(), "<html")
}

func TestPostProcess_RenderDocument(t *testing.T) {
	setting.StaticURLPrefix = markup.TestAppURL // can't run standalone
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()

	test := func(input, expected string) {
		var res strings.Builder
		err := markup.PostProcessDefault(markup.NewTestRenderContext(markup.TestAppURL, map[string]string{"user": "go-gitea", "repo": "gitea"}), strings.NewReader(input), &res)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(res.String()))
	}

	// Issue index shouldn't be post processing in a document.
	test(
		"#1",
		"#1")

	// But cross-referenced issue index should work.
	test(
		"go-gitea/gitea#12345",
		`<a href="`+util.URLJoin(markup.TestAppURL, "go-gitea", "gitea", "issues", "12345")+`" class="ref-issue">go-gitea/gitea#12345</a>`)

	// Test that other post processing still works.
	test(
		":gitea:",
		`<span class="emoji" aria-label="gitea"><img alt=":gitea:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/gitea.png"/></span>`)
	test(
		"Some text with 😄 in the middle",
		`Some text with <span class="emoji" aria-label="grinning face with smiling eyes">😄</span> in the middle`)
	test("http://localhost:3000/person/repo/issues/4#issuecomment-1234",
		`<a href="http://localhost:3000/person/repo/issues/4#issuecomment-1234" class="ref-issue">person/repo#4 (comment)</a>`)
}

func TestIssue16020(t *testing.T) {
	setting.AppURL = markup.TestAppURL

	localMetas := map[string]string{
		"user": "go-gitea",
		"repo": "gitea",
	}

	data := `<img src="data:image/png;base64,i//V"/>`

	var res strings.Builder
	err := markup.PostProcessDefault(markup.NewTestRenderContext(localMetas), strings.NewReader(data), &res)
	assert.NoError(t, err)
	assert.Equal(t, data, res.String())
}

func BenchmarkEmojiPostprocess(b *testing.B) {
	data := "🥰 "
	for len(data) < 1<<16 {
		data += data
	}
	b.ResetTimer()
	for b.Loop() {
		var res strings.Builder
		err := markup.PostProcessDefault(markup.NewTestRenderContext(localMetas), strings.NewReader(data), &res)
		assert.NoError(b, err)
	}
}

func TestFuzz(t *testing.T) {
	s := "t/l/issues/8#/../../a"
	renderContext := markup.NewTestRenderContext()
	err := markup.PostProcessDefault(renderContext, strings.NewReader(s), io.Discard)
	assert.NoError(t, err)
}

func TestIssue18471(t *testing.T) {
	data := `http://domain/org/repo/compare/783b039...da951ce`

	var res strings.Builder
	err := markup.PostProcessDefault(markup.NewTestRenderContext(localMetas), strings.NewReader(data), &res)

	assert.NoError(t, err)
	assert.Equal(t, `<a href="http://domain/org/repo/compare/783b039...da951ce" class="compare"><code class="nohighlight">783b039...da951ce</code></a>`, res.String())
}

func TestIsFullURL(t *testing.T) {
	assert.True(t, markup.IsFullURLString("https://example.com"))
	assert.True(t, markup.IsFullURLString("mailto:test@example.com"))
	assert.True(t, markup.IsFullURLString("data:image/11111"))
	assert.False(t, markup.IsFullURLString("/foo:bar"))
}
