// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	. "code.gitea.io/gitea/modules/markup"
	_ "code.gitea.io/gitea/modules/markup/markdown"
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

func testRenderIssueIndexPattern(t *testing.T, input, expected string, opts RenderIssueIndexPatternOptions) {
	if len(opts.URLPrefix) == 0 {
		opts.URLPrefix = AppSubURL
	}
	actual := string(RenderIssueIndexPattern([]byte(input), opts))
	assert.Equal(t, expected, actual)
}

func TestURLJoin(t *testing.T) {
	type test struct {
		Expected string
		Base     string
		Elements []string
	}
	newTest := func(expected, base string, elements ...string) test {
		return test{Expected: expected, Base: base, Elements: elements}
	}
	for _, test := range []test{
		newTest("https://try.gitea.io/a/b/c",
			"https://try.gitea.io", "a/b", "c"),
		newTest("https://try.gitea.io/a/b/c",
			"https://try.gitea.io/", "/a/b/", "/c/"),
		newTest("https://try.gitea.io/a/c",
			"https://try.gitea.io/", "/a/./b/", "../c/"),
		newTest("a/b/c",
			"a", "b/c/"),
		newTest("a/b/d",
			"a/", "b/c/", "/../d/"),
	} {
		assert.Equal(t, test.Expected, URLJoin(test.Base, test.Elements...))
	}
}

func TestRender_IssueIndexPattern(t *testing.T) {
	// numeric: render inputs without valid mentions
	test := func(s string) {
		testRenderIssueIndexPattern(t, s, s, RenderIssueIndexPatternOptions{})
		testRenderIssueIndexPattern(t, s, s, RenderIssueIndexPatternOptions{Metas: numericMetas})
	}

	// should not render anything when there are no mentions
	test("")
	test("this is a test")
	test("test 123 123 1234")
	test("#")
	test("# # #")
	test("# 123")
	test("#abcd")
	test("##1234")
	test("test#1234")
	test("#1234test")
	test(" test #1234test")

	// should not render issue mention without leading space
	test("test#54321 issue")

	// should not render issue mention without trailing space
	test("test #54321issue")
}

func TestRender_IssueIndexPattern2(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	// numeric: render inputs with valid mentions
	test := func(s, expectedFmt string, indices ...int) {
		links := make([]interface{}, len(indices))
		for i, index := range indices {
			links[i] = numericIssueLink(URLJoin(setting.AppSubURL, "issues"), index)
		}
		expectedNil := fmt.Sprintf(expectedFmt, links...)
		testRenderIssueIndexPattern(t, s, expectedNil, RenderIssueIndexPatternOptions{})

		for i, index := range indices {
			links[i] = numericIssueLink("https://someurl.com/someUser/someRepo/", index)
		}
		expectedNum := fmt.Sprintf(expectedFmt, links...)
		testRenderIssueIndexPattern(t, s, expectedNum, RenderIssueIndexPatternOptions{Metas: numericMetas})
	}

	// should render freestanding mentions
	test("#1234 test", "%s test", 1234)
	test("test #8 issue", "test %s issue", 8)
	test("test issue #1234", "test issue %s", 1234)

	// should render mentions in parentheses
	test("(#54321 issue)", "(%s issue)", 54321)
	test("test (#9801 extra) issue", "test (%s extra) issue", 9801)
	test("test (#1)", "test (%s)", 1)

	// should render multiple issue mentions in the same line
	test("#54321 #1243", "%s %s", 54321, 1243)
	test("wow (#54321 #1243)", "wow (%s %s)", 54321, 1243)
	test("(#4)(#5)", "(%s)(%s)", 4, 5)
	test("#1 (#4321) test", "%s (%s) test", 1, 4321)
}

func TestRender_IssueIndexPattern3(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	// alphanumeric: render inputs without valid mentions
	test := func(s string) {
		testRenderIssueIndexPattern(t, s, s, RenderIssueIndexPatternOptions{Metas: alphanumericMetas})
	}
	test("")
	test("this is a test")
	test("test 123 123 1234")
	test("#")
	test("##1234")
	test("# 123")
	test("#abcd")
	test("test #123")
	test("abc-1234")         // issue prefix must be capital
	test("ABc-1234")         // issue prefix must be _all_ capital
	test("ABCDEFGHIJK-1234") // the limit is 10 characters in the prefix
	test("ABC1234")          // dash is required
	test("test ABC- test")   // number is required
	test("test -1234 test")  // prefix is required
	test("testABC-123 test") // leading space is required
	test("test ABC-123test") // trailing space is required
	test("ABC-0123")         // no leading zero
}

func TestRender_IssueIndexPattern4(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	// alphanumeric: render inputs with valid mentions
	test := func(s, expectedFmt string, names ...string) {
		links := make([]interface{}, len(names))
		for i, name := range names {
			links[i] = alphanumIssueLink("https://someurl.com/someUser/someRepo/", name)
		}
		expected := fmt.Sprintf(expectedFmt, links...)
		testRenderIssueIndexPattern(t, s, expected, RenderIssueIndexPatternOptions{Metas: alphanumericMetas})
	}
	test("OTT-1234 test", "%s test", "OTT-1234")
	test("test T-12 issue", "test %s issue", "T-12")
	test("test issue ABCDEFGHIJ-1234567890", "test issue %s", "ABCDEFGHIJ-1234567890")
}

func TestRenderIssueIndexPatternWithDefaultURL(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input string, expected string) {
		testRenderIssueIndexPattern(t, input, expected, RenderIssueIndexPatternOptions{
			DefaultURL: AppURL,
		})
	}
	test("hello #123 world",
		fmt.Sprintf(`<a rel="nofollow" href="%s">hello</a> `, AppURL)+
			fmt.Sprintf(`<a href="%sissues/123">#123</a> `, AppSubURL)+
			fmt.Sprintf(`<a rel="nofollow" href="%s">world</a>`, AppURL))
	test("hello (#123) world",
		fmt.Sprintf(`<a rel="nofollow" href="%s">hello </a>`, AppURL)+
			fmt.Sprintf(`(<a href="%sissues/123">#123</a>)`, AppSubURL)+
			fmt.Sprintf(`<a rel="nofollow" href="%s"> world</a>`, AppURL))
}

func TestRender_AutoLink(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer := RenderSpecialLink([]byte(input), setting.AppSubURL, nil, false)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
		buffer = RenderSpecialLink([]byte(input), setting.AppSubURL, nil, true)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	// render valid issue URLs
	test(URLJoin(setting.AppSubURL, "issues", "3333"),
		numericIssueLink(URLJoin(setting.AppSubURL, "issues"), 3333))

	// render external issue URLs
	for _, externalURL := range []string{
		"http://1111/2222/ssss-issues/3333?param=blah&blahh=333",
		"http://test.com/issues/33333",
		"https://issues/333"} {
		test(externalURL, externalURL)
	}

	// render valid commit URLs
	tmp := URLJoin(AppSubURL, "commit", "d8a994ef243349f321568f9e36d5c3f444b99cae")
	test(tmp, "<a href=\""+tmp+"\">d8a994ef24</a>")
	tmp += "#diff-2"
	test(tmp, "<a href=\""+tmp+"\">d8a994ef24 (diff-2)</a>")

	// render other commit URLs
	tmp = "https://external-link.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2"
	test(tmp, "<a href=\""+tmp+"\">d8a994ef24 (diff-2)</a>")
}

func TestRender_Commits(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer := RenderString(".md", input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	var sha = "b6dd6210eaebc915fd5be5579c58cce4da2e2579"
	var commit = URLJoin(AppSubURL, "commit", sha)
	var subtree = URLJoin(commit, "src")
	var tree = strings.Replace(subtree, "/commit/", "/tree/", -1)
	var src = strings.Replace(subtree, "/commit/", "/src/", -1)

	test(sha, `<p><a href="`+commit+`" rel="nofollow">b6dd6210ea</a></p>`)
	test(sha[:7], `<p><a href="`+commit[:len(commit)-(40-7)]+`" rel="nofollow">b6dd621</a></p>`)
	test(sha[:39], `<p><a href="`+commit[:len(commit)-(40-39)]+`" rel="nofollow">b6dd6210ea</a></p>`)
	test(commit, `<p><a href="`+commit+`" rel="nofollow">b6dd6210ea</a></p>`)
	test(tree, `<p><a href="`+src+`" rel="nofollow">b6dd6210ea/src</a></p>`)
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
		`<p><a href="`+URLJoin(AppURL, "gogits", "gogs", "issues", "12345")+`" rel="nofollow">gogits/gogs#12345</a></p>`)
	test(
		"go-gitea/gitea#12345",
		`<p><a href="`+URLJoin(AppURL, "go-gitea", "gitea", "issues", "12345")+`" rel="nofollow">go-gitea/gitea#12345</a></p>`)
}

func TestRender_FullIssueURLs(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		result := RenderFullIssuePattern([]byte(input))
		assert.Equal(t, expected, string(result))
	}
	test("Here is a link https://git.osgeo.org/gogs/postgis/postgis/pulls/6",
		"Here is a link https://git.osgeo.org/gogs/postgis/postgis/pulls/6")
	test("Look here http://localhost:3000/person/repo/issues/4",
		`Look here <a href="http://localhost:3000/person/repo/issues/4">#4</a>`)
	test("http://localhost:3000/person/repo/issues/4#issuecomment-1234",
		`<a href="http://localhost:3000/person/repo/issues/4#issuecomment-1234">#4</a>`)
}

func TestRegExp_MentionPattern(t *testing.T) {
	trueTestCases := []string{
		"@Unknwon",
		"@ANT_123",
		"@xxx-DiN0-z-A..uru..s-xxx",
		"   @lol   ",
		" @Te/st",
	}
	falseTestCases := []string{
		"@ 0",
		"@ ",
		"@",
		"",
		"ABC",
	}

	for _, testCase := range trueTestCases {
		res := MentionPattern.MatchString(testCase)
		if !res {
			println()
			println(testCase)
		}
		assert.True(t, res)
	}
	for _, testCase := range falseTestCases {
		res := MentionPattern.MatchString(testCase)
		if res {
			println()
			println(testCase)
		}
		assert.False(t, res)
	}
}

func TestRegExp_IssueNumericPattern(t *testing.T) {
	trueTestCases := []string{
		"#1234",
		"#0",
		"#1234567890987654321",
		"[#1234]",
	}
	falseTestCases := []string{
		"# 1234",
		"# 0",
		"# ",
		"#",
		"#ABC",
		"#1A2B",
		"",
		"ABC",
		"[]",
		"[x]",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, IssueNumericPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, IssueNumericPattern.MatchString(testCase))
	}
}

func TestRegExp_IssueAlphanumericPattern(t *testing.T) {
	trueTestCases := []string{
		"ABC-1234",
		"A-1",
		"RC-80",
		"ABCDEFGHIJ-1234567890987654321234567890",
		"[JIRA-134]",
	}
	falseTestCases := []string{
		"RC-08",
		"PR-0",
		"ABCDEFGHIJK-1",
		"PR_1",
		"",
		"#ABC",
		"",
		"ABC",
		"GG-",
		"rm-1",
		"[]",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, IssueAlphanumericPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, IssueAlphanumericPattern.MatchString(testCase))
	}
}

func TestRegExp_Sha1CurrentPattern(t *testing.T) {
	trueTestCases := []string{
		"d8a994ef243349f321568f9e36d5c3f444b99cae",
		"abcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}
	falseTestCases := []string{
		"test",
		"abcdefg",
		"abcdefghijklmnopqrstuvwxyzabcdefghijklmn",
		"abcdefghijklmnopqrstuvwxyzabcdefghijklmO",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, Sha1CurrentPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, Sha1CurrentPattern.MatchString(testCase))
	}
}

func TestRegExp_AnySHA1Pattern(t *testing.T) {
	testCases := map[string][]string{
		"https://github.com/jquery/jquery/blob/a644101ed04d0beacea864ce805e0c4f86ba1cd1/test/unit/event.js#L2703": {
			"https",
			"github.com",
			"jquery",
			"jquery",
			"blob",
			"a644101ed04d0beacea864ce805e0c4f86ba1cd1",
			"test/unit/event.js",
			"L2703",
		},
		"https://github.com/jquery/jquery/blob/a644101ed04d0beacea864ce805e0c4f86ba1cd1/test/unit/event.js": {
			"https",
			"github.com",
			"jquery",
			"jquery",
			"blob",
			"a644101ed04d0beacea864ce805e0c4f86ba1cd1",
			"test/unit/event.js",
			"",
		},
		"https://github.com/jquery/jquery/commit/0705be475092aede1eddae01319ec931fb9c65fc": {
			"https",
			"github.com",
			"jquery",
			"jquery",
			"commit",
			"0705be475092aede1eddae01319ec931fb9c65fc",
			"",
			"",
		},
		"https://github.com/jquery/jquery/tree/0705be475092aede1eddae01319ec931fb9c65fc/src": {
			"https",
			"github.com",
			"jquery",
			"jquery",
			"tree",
			"0705be475092aede1eddae01319ec931fb9c65fc",
			"src",
			"",
		},
		"https://try.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2": {
			"https",
			"try.gogs.io",
			"gogs",
			"gogs",
			"commit",
			"d8a994ef243349f321568f9e36d5c3f444b99cae",
			"",
			"diff-2",
		},
	}

	for k, v := range testCases {
		assert.Equal(t, AnySHA1Pattern.FindStringSubmatch(k)[1:], v)
	}
}

func TestMisc_IsSameDomain(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	var sha = "b6dd6210eaebc915fd5be5579c58cce4da2e2579"
	var commit = URLJoin(AppSubURL, "commit", sha)

	assert.True(t, IsSameDomain(commit))
	assert.False(t, IsSameDomain("http://google.com/ncr"))
	assert.False(t, IsSameDomain("favicon.ico"))
}
