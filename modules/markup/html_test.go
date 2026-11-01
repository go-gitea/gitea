// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"io"
	"maps"
	"strings"
	"testing"

	"gitea.dev/modules/emoji"
	"gitea.dev/modules/json"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/common"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	testModule "gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testRepoOwnerName = "user13"
	testRepoName      = "repo11"
	localMetas        = map[string]string{"user": testRepoOwnerName, "repo": testRepoName}
)

func testRenderString(ctx *markup.RenderContext, content string) (string, error) {
	var buf strings.Builder
	err := markup.Render(ctx, strings.NewReader(content), &buf)
	return buf.String(), err
}

func TestRender_Commits(t *testing.T) {
	test := func(input, expected string) {
		rctx := markup.NewTestRenderContext(markup.TestAppURL, localMetas).WithRelativePath("a.md")
		buffer, err := testRenderString(rctx, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	repo := markup.TestAppURL + testRepoOwnerName + "/" + testRepoName + "/"
	commit := repo + "commit/" + sha
	commitPath := "/user13/repo11/commit/" + sha
	tree := repo + "tree/" + sha + "/src"

	file := repo + "commit/" + sha + "/example.txt"
	fileWithExtra := file + ":"
	fileWithHash := file + "#L2"
	fileWithHasExtra := file + "#L2:"
	commitCompare := repo + "compare/" + sha + "..." + sha
	commitCompareWithHash := commitCompare + "#L2"

	test(sha, `<p><a href="`+commitPath+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test(sha[:7], `<p><a href="`+commitPath[:len(commitPath)-(40-7)]+`" class="commit" rel="nofollow"><code>65f1bf2</code></a></p>`)
	test(sha[:39], `<p><a href="`+commitPath[:len(commitPath)-(40-39)]+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test(commit, `<p><a href="`+commit+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test(tree, `<p><a href="`+tree+`" rel="nofollow">`+tree+`</a></p>`)

	test(file, `<p><a href="`+file+`" class="commit" rel="nofollow"><code>65f1bf27bc</code>/example.txt</a></p>`)
	test(fileWithExtra, `<p><a href="`+file+`" class="commit" rel="nofollow"><code>65f1bf27bc</code>/example.txt</a>:</p>`)
	test(fileWithHash, `<p><a href="`+fileWithHash+`" class="commit" rel="nofollow"><code>65f1bf27bc</code>/example.txt#L2</a></p>`)
	test(fileWithHasExtra, `<p><a href="`+fileWithHash+`" class="commit" rel="nofollow"><code>65f1bf27bc</code>/example.txt#L2</a>:</p>`)
	test(commitCompare, `<p><a href="`+commitCompare+`" class="compare" rel="nofollow"><code>65f1bf27bc...65f1bf27bc</code></a></p>`)
	test(commitCompareWithHash, `<p><a href="`+commitCompareWithHash+`" class="compare" rel="nofollow"><code>65f1bf27bc...65f1bf27bc</code>#L2</a></p>`)

	test("commit "+sha, `<p>commit <a href="`+commitPath+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test("/home/gitea/"+sha, "<p>/home/gitea/"+sha+"</p>")
	test("deadbeef", `<p>deadbeef</p>`)
	test("d27ace93", `<p>d27ace93</p>`)
	test(sha[:14]+".x", `<p>`+sha[:14]+`.x</p>`)

	expected14 := `<a href="` + commitPath[:len(commitPath)-(40-14)] + `" class="commit" rel="nofollow"><code>` + sha[:10] + `</code></a>`
	test(sha[:14]+".", `<p>`+expected14+`.</p>`)
	test(sha[:14]+",", `<p>`+expected14+`,</p>`)
	test("["+sha[:14]+"]", `<p>[`+expected14+`]</p>`)

	commitShort := repo + "commit/" + sha[:7]
	pullCommit := repo + "pulls/1/commits/" + sha
	otherCommit := markup.TestAppURL + "user2/repo1/commit/" + sha
	branchCompare := repo + "compare/main...feature/x"
	githubCommit := "https://github.com/a/b/commit/" + sha
	test(pullCommit+" fixes "+repo+"issues/2", `<p><a href="`+pullCommit+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a> fixes <a href="`+repo+`issues/2" class="ref-issue" rel="nofollow">#2</a></p>`)
	test(commitShort+"#diff-2.", `<p><a href="`+commitShort+`#diff-2" class="commit" rel="nofollow"><code>65f1bf2</code>#diff-2</a>.</p>`)
	test(commitShort+"?w=1", `<p><a href="`+commitShort+`?w=1" class="commit" rel="nofollow"><code>65f1bf2</code>?w=1</a></p>`)
	test(pullCommit, `<p><a href="`+pullCommit+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test(otherCommit, `<p><a href="`+otherCommit+`" class="commit" rel="nofollow">user2/repo1@<code>65f1bf27bc</code></a></p>`)
	test(branchCompare, `<p><a href="`+branchCompare+`" class="compare" rel="nofollow"><code>main...feature/x</code></a></p>`)
	test(githubCommit+" "+commitShort+" "+commit, `<p><a href="`+githubCommit+`" rel="nofollow">`+githubCommit+`</a> <a href="`+commitShort+`" class="commit" rel="nofollow"><code>65f1bf2</code></a> <a href="`+commit+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test("<"+commitShort+"> ["+commit+"]("+commit+")", `<p><a href="`+commitShort+`" class="commit" rel="nofollow"><code>65f1bf2</code></a> <a href="`+commit+`" class="commit" rel="nofollow"><code>65f1bf27bc</code></a></p>`)
	test("[[label|"+commitShort+"]]", `<p><a href="`+commitShort+`" rel="nofollow">label</a></p>`)
	for input, attrs := range map[string]string{
		commitShort + ".patch":                             ` class="commit" rel="nofollow"><code>65f1bf2</code>.patch`,
		repo + "compare/" + sha + ".." + sha:               ` class="compare" rel="nofollow"><code>65f1bf27bc..65f1bf27bc</code>`,
		commitCompare + ".diff?w=1":                        ` class="compare" rel="nofollow"><code>65f1bf27bc...65f1bf27bc</code>.diff?w=1`,
		repo + "compare/20240101...20240201":               ` class="compare" rel="nofollow"><code>20240101...20240201</code>`,
		repo + "src/commit/" + sha + "/README.md":          ` class="commit" rel="nofollow"><code>65f1bf27bc</code>/README.md`,
		repo + "commit/" + sha[:6]:                         ` rel="nofollow">` + repo + "commit/" + sha[:6],
		repo + "src/branch/main/sub-dir/commit/" + sha[:7]: ` rel="nofollow">` + repo + "src/branch/main/sub-dir/commit/" + sha[:7],
		repo + "src/branch/main/" + sha + ".txt":           ` rel="nofollow">` + repo + "src/branch/main/" + sha + ".txt",
	} {
		test(input, `<p><a href="`+input+`"`+attrs+`</a></p>`)
	}
}

func TestRender_CrossReferences(t *testing.T) {
	defer testModule.MockVariableValue(&setting.AppURL, markup.TestAppURL)()
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	test := func(input, expected string) {
		rctx := markup.NewTestRenderContext(markup.TestAppURL, localMetas).WithRelativePath("a.md")
		buffer, err := testRenderString(rctx, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	test(
		"test-owner/test-repo#12345",
		`<p><a href="/test-owner/test-repo/issues/12345" class="ref-issue" rel="nofollow">test-owner/test-repo#12345</a></p>`)
	test(
		"go-gitea/gitea#12345",
		`<p><a href="/go-gitea/gitea/issues/12345" class="ref-issue" rel="nofollow">go-gitea/gitea#12345</a></p>`)
	test(
		"/home/gitea/go-gitea/gitea#12345",
		`<p>/home/gitea/go-gitea/gitea#12345</p>`)
	test(
		markup.TestAppURL+"gogitea/gitea/issues/12345",
		`<p><a href="`+markup.TestAppURL+`gogitea/gitea/issues/12345" class="ref-issue" rel="nofollow">gogitea/gitea#12345</a></p>`)
	test(
		markup.TestAppURL+"go-gitea/gitea/issues/12345",
		`<p><a href="`+markup.TestAppURL+`go-gitea/gitea/issues/12345" class="ref-issue" rel="nofollow">go-gitea/gitea#12345</a></p>`)
	test(
		markup.TestAppURL+"gogitea/some-repo-name/issues/12345",
		`<p><a href="`+markup.TestAppURL+`gogitea/some-repo-name/issues/12345" class="ref-issue" rel="nofollow">gogitea/some-repo-name#12345</a></p>`)

	inputURL := setting.AppURL + "a/b/commit/0123456789012345678901234567890123456789/foo.txt?a=b#L2-L3"
	test(
		inputURL,
		`<p><a href="`+inputURL+`" class="commit" rel="nofollow">a/b@<code>0123456789</code>/foo.txt?a=b#L2-L3</a></p>`)

	inputURL = setting.AppURL + "owner/repo/archive/0123456789012345678901234567890123456789.tar.gz?key=val"
	test(inputURL, `<p><a href="`+inputURL+`" class="commit" rel="nofollow">owner/repo@<code>0123456789</code>.tar.gz?key=val</a></p>`)

	test("test-owner/test-repo@"+"0123456789012345678901234567890123456789", `<p><a href="/test-owner/test-repo/commit/0123456789012345678901234567890123456789" class="commit" rel="nofollow">test-owner/test-repo@<code>0123456789</code></a></p>`)
	test(testRepoOwnerName+"/"+testRepoName+"@"+"0123456789012345678901234567890123456789", `<p><a href="/user13/repo11/commit/0123456789012345678901234567890123456789" class="commit" rel="nofollow"><code>0123456789</code></a></p>`)
}

func TestRender_links(t *testing.T) {
	setting.AppURL = markup.TestAppURL
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()
	test := func(input, expected string) {
		buffer, err := testRenderString(markup.NewTestRenderContext().WithRelativePath("a.md"), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	oldCustomURLSchemes := setting.Markdown.CustomURLSchemes
	markup.ResetDefaultSanitizerForTesting()
	defer func() {
		setting.Markdown.CustomURLSchemes = oldCustomURLSchemes
		markup.ResetDefaultSanitizerForTesting()
		common.InitLinkURLSchemes(oldCustomURLSchemes)
	}()
	setting.Markdown.CustomURLSchemes = []string{"ftp", "magnet"}
	common.InitLinkURLSchemes(setting.Markdown.CustomURLSchemes)

	for _, testCase := range []struct {
		input, expected string
	}{
		// Text that should be turned into URL
		{"https://www.example.com", `<p><a href="https://www.example.com" rel="nofollow">https://www.example.com</a></p>`},
		{"http://www.example.com", `<p><a href="http://www.example.com" rel="nofollow">http://www.example.com</a></p>`},
		{"https://example.com", `<p><a href="https://example.com" rel="nofollow">https://example.com</a></p>`},
		{"http://example.com", `<p><a href="http://example.com" rel="nofollow">http://example.com</a></p>`},
		{"http://foo.com/blah_blah", `<p><a href="http://foo.com/blah_blah" rel="nofollow">http://foo.com/blah_blah</a></p>`},
		{"http://foo.com/blah_blah/", `<p><a href="http://foo.com/blah_blah/" rel="nofollow">http://foo.com/blah_blah/</a></p>`},
		{"http://www.example.com/wpstyle/?p=364", `<p><a href="http://www.example.com/wpstyle/?p=364" rel="nofollow">http://www.example.com/wpstyle/?p=364</a></p>`},
		{"https://www.example.com/foo/?bar=baz&inga=42&quux", `<p><a href="https://www.example.com/foo/?bar=baz&amp;inga=42&amp;quux" rel="nofollow">https://www.example.com/foo/?bar=baz&amp;inga=42&amp;quux</a></p>`},
		{"http://142.42.1.1/", `<p><a href="http://142.42.1.1/" rel="nofollow">http://142.42.1.1/</a></p>`},
		{"https://github.com/go-gitea/gitea/?p=aaa/bbb.html#ccc-ddd", `<p><a href="https://github.com/go-gitea/gitea/?p=aaa/bbb.html#ccc-ddd" rel="nofollow">https://github.com/go-gitea/gitea/?p=aaa/bbb.html#ccc-ddd</a></p>`},
		{"https://en.wikipedia.org/wiki/URL_(disambiguation)", `<p><a href="https://en.wikipedia.org/wiki/URL_(disambiguation)" rel="nofollow">https://en.wikipedia.org/wiki/URL_(disambiguation)</a></p>`},
		{"https://foo_bar.example.com/", `<p><a href="https://foo_bar.example.com/" rel="nofollow">https://foo_bar.example.com/</a></p>`},
		{"https://stackoverflow.com/questions/2896191/what-is-go-used-fore", `<p><a href="https://stackoverflow.com/questions/2896191/what-is-go-used-fore" rel="nofollow">https://stackoverflow.com/questions/2896191/what-is-go-used-fore</a></p>`},
		{"https://username:password@gitea.com", `<p><a href="https://username:password@gitea.com" rel="nofollow">https://username:password@gitea.com</a></p>`},
		{"ftp://gitea.com/file.txt", `<p><a href="ftp://gitea.com/file.txt" rel="nofollow">ftp://gitea.com/file.txt</a></p>`},
		{"magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcadabcde&dn=download", `<p><a href="magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcadabcde&amp;dn=download" rel="nofollow">magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcadabcde&amp;dn=download</a></p>`},
		{`[link](https://example.com)`, `<p><a href="https://example.com" rel="nofollow">link</a></p>`},
		{`[link](mailto:test@example.com)`, `<p><a href="mailto:test@example.com" rel="nofollow">link</a></p>`},
		{`[link](javascript:xss)`, `<p>link</p>`},

		// Test that should *not* be turned into URL
		{"www.example.com", `<p>www.example.com</p>`},
		{"example.com", `<p>example.com</p>`},
		{"test.example.com", `<p>test.example.com</p>`},
		{"http://", `<p>http://</p>`},
		{"https://", `<p>https://</p>`},
		{"://", `<p>://</p>`},
		{"www", `<p>www</p>`},
		{"ftps://gitea.com", `<p>ftps://gitea.com</p>`},
		{markup.TestAppURL + "user13/repo11/commit/65f1bf2", `<p><a href="` + markup.TestAppURL + `user13/repo11/commit/65f1bf2" rel="nofollow">` + markup.TestAppURL + `user13/repo11/commit/65f1bf2</a></p>`},
	} {
		test(testCase.input, testCase.expected)
	}

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
	for _, testCase := range []struct {
		input, expected string
	}{
		// Text that should be turned into email link
		{"info@gitea.com", `<p><a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a></p>`},
		{"(info@gitea.com)", `<p>(<a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a>)</p>`},
		{"[info@gitea.com]", `<p>[<a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a>]</p>`},
		{"info@gitea.com.", `<p><a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a>.</p>`},
		{"firstname+lastname@gitea.com", `<p><a href="mailto:firstname+lastname@gitea.com" rel="nofollow">firstname+lastname@gitea.com</a></p>`},
		{"send email to info@gitea.co.uk.", `<p>send email to <a href="mailto:info@gitea.co.uk" rel="nofollow">info@gitea.co.uk</a>.</p>`},

		{`j.doe@example.com,
	j.doe@example.com.
	j.doe@example.com;
	j.doe@example.com?
	j.doe@example.com!`, `<p><a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>,
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>.
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>;
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>?
<a href="mailto:j.doe@example.com" rel="nofollow">j.doe@example.com</a>!</p>`},

		// match GitHub behavior
		{"email@domain@domain.com", `<p>email@<a href="mailto:domain@domain.com" rel="nofollow">domain@domain.com</a></p>`},

		// match GitHub behavior
		{`"info@gitea.com"`, `<p>&#34;<a href="mailto:info@gitea.com" rel="nofollow">info@gitea.com</a>&#34;</p>`},

		// Test that should *not* be turned into email links
		{"/home/gitea/mailstore/info@gitea/com", `<p>/home/gitea/mailstore/info@gitea/com</p>`},
		{"git@try.gitea.io:go-gitea/gitea.git", `<p>git@try.gitea.io:go-gitea/gitea.git</p>`},
		{"https://foo:bar@gitea.io", `<p><a href="https://foo:bar@gitea.io" rel="nofollow">https://foo:bar@gitea.io</a></p>`},
		{"gitea@3", `<p>gitea@3</p>`},
		{"gitea@gmail.c", `<p>gitea@gmail.c</p>`},
		{"email@domain..com", `<p>email@domain..com</p>`},

		// match GitHub behavior
		{"?a@d.zz", `<p>?<a href="mailto:a@d.zz" rel="nofollow">a@d.zz</a></p>`},
		{"*a@d.zz", `<p>*<a href="mailto:a@d.zz" rel="nofollow">a@d.zz</a></p>`},
		{"~a@d.zz", `<p>~<a href="mailto:a@d.zz" rel="nofollow">a@d.zz</a></p>`},

		// the following cases don't match GitHub behavior, but they are valid email addresses ...
		// maybe we should reduce the candidate characters for the "name" part in the future
		{"a*a@d.zz", `<p><a href="mailto:a*a@d.zz" rel="nofollow">a*a@d.zz</a></p>`},
		{"a~a@d.zz", `<p><a href="mailto:a~a@d.zz" rel="nofollow">a~a@d.zz</a></p>`},
	} {
		res, err := testRenderString(markup.NewTestRenderContext().WithRelativePath("a.md"), testCase.input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(testCase.expected), strings.TrimSpace(res), "input: %s", testCase.input)
	}
}

func TestRender_emoji(t *testing.T) {
	defer testModule.MockVariableValue(&setting.AppURL, markup.TestAppURL)()
	defer testModule.MockVariableValue(&setting.StaticURLPrefix, strings.TrimSuffix(markup.TestAppURL, "/"))()
	defer testModule.MockVariableValue(&setting.UI.CustomEmojisMap, maps.Clone(setting.UI.CustomEmojisMap))()

	test := func(input, expected string) {
		expected = strings.ReplaceAll(expected, "&", "&amp;")
		buffer, err := testRenderString(markup.NewTestRenderContext().WithRelativePath("a.md"), input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}

	// Make sure we can successfully match every emoji in our dataset with regex
	data, err := public.AssetFS().ReadFile("assets", "emoji.json")
	require.NoError(t, err)
	var emojis []emoji.Emoji
	require.NoError(t, json.Unmarshal(data, &emojis))
	for _, e := range emojis {
		expected := `<p><span class="emoji" data-alias="` + e.Aliases[0] + `">` + e.Emoji + `</span></p>`
		test(e.Emoji, expected)
		test(":"+e.Aliases[0]+":", expected)
	}

	// Text that should be turned into or recognized as emoji
	test(
		":gitea:",
		`<p><span class="emoji"><img alt=":gitea:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/gitea.png"/></span></p>`)
	test(
		":custom-emoji:",
		`<p>:custom-emoji:</p>`)
	setting.UI.CustomEmojisMap["custom-emoji"] = ":custom-emoji:"
	test(
		":custom-emoji:",
		`<p><span class="emoji"><img alt=":custom-emoji:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/custom-emoji.png"/></span></p>`)
	test(
		"这是字符:1::+1: some🐊 \U0001f44d:custom-emoji: :gitea:",
		`<p>这是字符:1:<span class="emoji" data-alias="+1">👍</span> some<span class="emoji" data-alias="crocodile">🐊</span> `+
			`<span class="emoji" data-alias="+1">👍</span><span class="emoji"><img alt=":custom-emoji:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/custom-emoji.png"/></span> `+
			`<span class="emoji"><img alt=":gitea:" src="`+setting.StaticURLPrefix+`/assets/img/emoji/gitea.png"/></span></p>`)
	test(
		"Some text with 😄 in the middle",
		`<p>Some text with <span class="emoji" data-alias="smile">😄</span> in the middle</p>`)
	test(
		"Some text with :smile: in the middle",
		`<p>Some text with <span class="emoji" data-alias="smile">😄</span> in the middle</p>`)
	test(
		"Some text with 😄😄 2 emoji next to each other",
		`<p>Some text with <span class="emoji" data-alias="smile">😄</span><span class="emoji" data-alias="smile">😄</span> 2 emoji next to each other</p>`)
	test(
		"😎🤪🔐🤑❓",
		`<p><span class="emoji" data-alias="sunglasses">😎</span><span class="emoji" data-alias="zany_face">🤪</span><span class="emoji" data-alias="closed_lock_with_key">🔐</span><span class="emoji" data-alias="money_mouth_face">🤑</span><span class="emoji" data-alias="question">❓</span></p>`)
	test(
		"👍🏽🧑🏽‍💻👩🏿‍❤️‍👩🏿",
		`<p><span class="emoji" data-alias="+1">👍🏽</span><span class="emoji" data-alias="technologist">🧑🏽‍💻</span><span class="emoji" data-alias="couple_with_heart_woman_woman">👩🏿‍❤️‍👩🏿</span></p>`)

	// should match nothing
	test(":100:200", `<p>:100:200</p>`)
	test("std::thread::something", `<p>std::thread::something</p>`)
	test(":not exist:", `<p>:not exist:</p>`)
	test("foo `:smile:", "<p>foo `:smile:</p>")
	test("foo `:smile:`", `<p>foo <code>:smile:</code></p>`)
	test("foo ` :smile:", "<p>foo ` <span class=\"emoji\" data-alias=\"smile\">😄</span></p>")
}

func TestRender_ShortLinks(t *testing.T) {
	setting.AppURL = markup.TestAppURL
	tree := markup.TestRepoURL + "src/master"

	url := tree + "/Link"
	otherURL := tree + "/Other-Link"
	encodedURL := tree + "/Link%3F"
	imgurl := tree + "/Link.jpg"
	otherImgurl := tree + "/Link+Other.jpg"
	encodedImgurl := tree + "/Link+%23.jpg"
	notencodedImgurl := tree + "/some/path/Link%20#.jpg"
	renderableFileURL := tree + "/markdown_file.md"
	unrenderableFileURL := tree + "/file.zip"
	favicon := "http://google.com/favicon.ico"
	for _, testCase := range []struct {
		input, expected string
	}{
		{"[[Link]]", `<p><a href="` + url + `" rel="nofollow">Link</a></p>`},
		{"[[Link.-]]", `<p><a href="http://localhost:3000/test-owner/test-repo/src/master/Link.-" rel="nofollow">Link.-</a></p>`},
		{"[[Link.jpg]]", `<p><a href="` + imgurl + `" rel="nofollow"><img src="` + imgurl + `" title="Link.jpg" alt="Link.jpg"/></a></p>`},
		{"[[" + favicon + "]]", `<p><a href="` + favicon + `" rel="nofollow"><img src="` + favicon + `" title="favicon.ico" alt="` + favicon + `"/></a></p>`},
		{"[[Name|Link]]", `<p><a href="` + url + `" rel="nofollow">Name</a></p>`},
		{"[[Name|Link.jpg]]", `<p><a href="` + imgurl + `" rel="nofollow"><img src="` + imgurl + `" title="Name" alt="Name"/></a></p>`},
		{"[[Name|Link.jpg|alt=AltName]]", `<p><a href="` + imgurl + `" rel="nofollow"><img src="` + imgurl + `" title="AltName" alt="AltName"/></a></p>`},
		{"[[Name|Link.jpg|title=Title]]", `<p><a href="` + imgurl + `" rel="nofollow"><img src="` + imgurl + `" title="Title" alt="Title"/></a></p>`},
		{"[[Name|Link.jpg|alt=AltName|title=Title]]", `<p><a href="` + imgurl + `" rel="nofollow"><img src="` + imgurl + `" title="Title" alt="AltName"/></a></p>`},
		{"[[Name|Link.jpg|alt=\"AltName\"|title='Title']]", `<p><a href="` + imgurl + `" rel="nofollow"><img src="` + imgurl + `" title="Title" alt="AltName"/></a></p>`},
		{"[[Name|Link Other.jpg|alt=\"AltName\"|title='Title']]", `<p><a href="` + otherImgurl + `" rel="nofollow"><img src="` + otherImgurl + `" title="Title" alt="AltName"/></a></p>`},
		{"[[Link]] [[Other Link]]", `<p><a href="` + url + `" rel="nofollow">Link</a> <a href="` + otherURL + `" rel="nofollow">Other Link</a></p>`},
		{"[[Link?]]", `<p><a href="` + encodedURL + `" rel="nofollow">Link?</a></p>`},
		{"[[Link]] [[Other Link]] [[Link?]]", `<p><a href="` + url + `" rel="nofollow">Link</a> <a href="` + otherURL + `" rel="nofollow">Other Link</a> <a href="` + encodedURL + `" rel="nofollow">Link?</a></p>`},
		{"[[markdown_file.md]]", `<p><a href="` + renderableFileURL + `" rel="nofollow">markdown_file.md</a></p>`},
		{"[[file.zip]]", `<p><a href="` + unrenderableFileURL + `" rel="nofollow">file.zip</a></p>`},
		{"[[Link #.jpg]]", `<p><a href="` + encodedImgurl + `" rel="nofollow"><img src="` + encodedImgurl + `" title="Link #.jpg" alt="Link #.jpg"/></a></p>`},
		{"[[Name|Link #.jpg|alt=\"AltName\"|title='Title']]", `<p><a href="` + encodedImgurl + `" rel="nofollow"><img src="` + encodedImgurl + `" title="Title" alt="AltName"/></a></p>`},
		// FIXME: it's unable to resolve: [[link?k=v]]
		// FIXME: it is a wrong test case, it is not an image, but a link with anchor "#.jpg"
		{"[[some/path/Link #.jpg]]", `<p><a href="` + notencodedImgurl + `" rel="nofollow"><img src="` + notencodedImgurl + `" title="Link #.jpg" alt="some/path/Link #.jpg"/></a></p>`},
		{"<p><a href=\"https://example.org\">[[foobar]]</a></p>", `<p><a href="https://example.org" rel="nofollow">[[foobar]]</a></p>`},
	} {
		buffer, err := markdown.RenderString(markup.NewTestRenderContext(tree), testCase.input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(testCase.expected), strings.TrimSpace(string(buffer)))
	}
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

func TestPostProcess(t *testing.T) {
	setting.StaticURLPrefix = strings.TrimSuffix(markup.TestAppURL, "/") // can't run standalone
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()

	for _, testCase := range []struct {
		input, expected string
	}{
		// Issue index shouldn't be post-processing in a document.
		{"#1", "#1"},

		// But cross-referenced issue index should work.
		{"go-gitea/gitea#12345", `<a href="/go-gitea/gitea/issues/12345" class="ref-issue">go-gitea/gitea#12345</a>`},

		// Test that other post-processing still works.
		{":gitea:", `<span class="emoji"><img alt=":gitea:" src="` + setting.StaticURLPrefix + `/assets/img/emoji/gitea.png"/></span>`},
		{"Some text with 😄 in the middle", `Some text with <span class="emoji" data-alias="smile">😄</span> in the middle`},
		{"http://localhost:3000/person/repo/issues/4#issuecomment-1234", `<a href="http://localhost:3000/person/repo/issues/4#issuecomment-1234" class="ref-issue">person/repo#4 (comment)</a>`},
		{"<math><mtext>:gitea: go-gitea/gitea#12345</mtext></math>", "<math><mtext>:gitea: go-gitea/gitea#12345</mtext></math>"},

		// special tags, GitHub's behavior, and for unclosed tags, output as text content as much as possible
		{"<script>a", `&lt;script&gt;a`},
		{"<script>a</script>", `&lt;script&gt;a&lt;/script&gt;`},
		{"<STYLE>a", `&lt;STYLE&gt;a`},
		{"<style>a</STYLE>", `&lt;style&gt;a&lt;/STYLE&gt;`},

		// other special tags, our special behavior
		{"<?php\nfoo", "&lt;?php\nfoo"},
		{"<%asp\nfoo", "&lt;%asp\nfoo"},
	} {
		var res strings.Builder
		err := markup.PostProcessDefault(markup.NewTestRenderContext(markup.TestAppURL, map[string]string{"user": "go-gitea", "repo": "gitea"}), strings.NewReader(testCase.input), &res)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(testCase.expected), strings.TrimSpace(res.String()))
	}
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
	defer testModule.MockVariableValue(&setting.AppURL, markup.TestAppURL)()

	data := markup.TestAppURL + `org/repo/compare/783b039...da951ce`

	var res strings.Builder
	err := markup.PostProcessDefault(markup.NewTestRenderContext(localMetas), strings.NewReader(data), &res)

	assert.NoError(t, err)
	assert.Equal(t, `<a href="`+markup.TestAppURL+`org/repo/compare/783b039...da951ce" class="compare">org/repo@<code>783b039...da951ce</code></a>`, res.String())
}

func TestRender_TeamMention(t *testing.T) {
	// the "org" meta is lower-cased, see Repository.composeCommonMetas
	metas := map[string]string{"user": "Org1", "repo": "repo1", "org": "org1", "teams": ",developers,"}
	test := func(input, expected string) {
		rctx := markup.NewTestRenderContext(markup.TestAppURL, metas).WithRelativePath("a.md")
		buffer, err := testRenderString(rctx, input)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(buffer))
	}
	test("@org1/developers", `<p><a href="/org/org1/teams/developers" rel="nofollow">@org1/developers</a></p>`)
	test("@Org1/Developers", `<p><a href="/org/Org1/teams/Developers" rel="nofollow">@Org1/Developers</a></p>`)
	test("@org2/developers", `<p>@org2/developers</p>`)
	test("@org1/testers", `<p>@org1/testers</p>`)
}
