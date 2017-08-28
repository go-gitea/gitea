// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"
const Repo = "go-gitea/gitea"
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

func TestRender_IssueIndexPattern(t *testing.T) {
	// numeric: render inputs without valid mentions
	test := func(s string) {
		testRenderIssueIndexPattern(t, s, s, nil)
		testRenderIssueIndexPattern(t, s, s, numericMetas)
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
		testRenderIssueIndexPattern(t, s, expectedNil, nil)

		for i, index := range indices {
			links[i] = numericIssueLink("https://someurl.com/someUser/someRepo/", index)
		}
		expectedNum := fmt.Sprintf(expectedFmt, links...)
		testRenderIssueIndexPattern(t, s, expectedNum, numericMetas)
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
		testRenderIssueIndexPattern(t, s, s, alphanumericMetas)
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
		testRenderIssueIndexPattern(t, s, expected, alphanumericMetas)
	}
	test("OTT-1234 test", "%s test", "OTT-1234")
	test("test T-12 issue", "test %s issue", "T-12")
	test("test issue ABCDEFGHIJ-1234567890", "test issue %s", "ABCDEFGHIJ-1234567890")
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
	tmp := "http://1111/2222/ssss-issues/3333?param=blah&blahh=333"
	test(tmp, "<a href=\""+tmp+"\">#3333 <i class='comment icon'></i></a>")
	test("http://test.com/issues/33333", numericIssueLink("http://test.com/issues", 33333))
	test("https://issues/333", numericIssueLink("https://issues", 333))

	// render valid commit URLs
	tmp = URLJoin(AppSubURL, "commit", "d8a994ef243349f321568f9e36d5c3f444b99cae")
	test(tmp, "<a href=\""+tmp+"\">d8a994ef24</a>")
	tmp += "#diff-2"
	test(tmp, "<a href=\""+tmp+"\">d8a994ef24 (diff-2)</a>")

	// render other commit URLs
	tmp = "https://external-link.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2"
	test(tmp, "<a href=\""+tmp+"\">d8a994ef24 (diff-2)</a>")
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

func TestRegExp_IssueFullPattern(t *testing.T) {
	testCases := map[string][]string{
		"https://github.com/gogits/gogs/pull/3244": {
			"https",
			"github.com/gogits/gogs/pull/",
			"3244",
			"",
			"",
		},
		"https://github.com/gogits/gogs/issues/3247#issuecomment-231517079": {
			"https",
			"github.com/gogits/gogs/issues/",
			"3247",
			"#issuecomment-231517079",
			"",
		},
		"https://try.gogs.io/gogs/gogs/issues/4#issue-685": {
			"https",
			"try.gogs.io/gogs/gogs/issues/",
			"4",
			"#issue-685",
			"",
		},
		"https://youtrack.jetbrains.com/issue/JT-36485": {
			"https",
			"youtrack.jetbrains.com/issue/",
			"JT-36485",
			"",
			"",
		},
		"https://youtrack.jetbrains.com/issue/JT-36485#comment=27-1508676": {
			"https",
			"youtrack.jetbrains.com/issue/",
			"JT-36485",
			"#comment=27-1508676",
			"",
		},
	}

	for k, v := range testCases {
		assert.Equal(t, IssueFullPattern.FindStringSubmatch(k)[1:], v)
	}
}
