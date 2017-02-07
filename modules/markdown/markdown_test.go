package markdown_test

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"testing"

	. "code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"

	"github.com/russross/blackfriday"
	"github.com/stretchr/testify/assert"
)

const urlPrefix = "/prefix"

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
	u, _ := url.Parse(baseURL)
	u.Path = path.Join(u.Path, strconv.Itoa(index))
	return link(u.String(), fmt.Sprintf("#%d", index))
}

// alphanumLink an HTML link to an alphanumeric-style issue
func alphanumIssueLink(baseURL string, name string) string {
	u, _ := url.Parse(baseURL)
	u.Path = path.Join(u.Path, name)
	return link(u.String(), name)
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
		string(RenderIssueIndexPattern([]byte(input), urlPrefix, metas)))
}

func TestRenderIssueIndexPattern(t *testing.T) {
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

func TestRenderIssueIndexPattern2(t *testing.T) {
	// numeric: render inputs with valid mentions
	test := func(s, expectedFmt string, indices ...int) {
		links := make([]interface{}, len(indices))
		for i, index := range indices {
			links[i] = numericIssueLink(path.Join(urlPrefix, "issues"), index)
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

func TestRenderIssueIndexPattern3(t *testing.T) {
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

func TestRenderIssueIndexPattern4(t *testing.T) {
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

func TestRenderer_AutoLink(t *testing.T) {
	setting.AppURL = "http://localhost:3000/"
	htmlFlags := blackfriday.HTML_SKIP_STYLE | blackfriday.HTML_OMIT_CONTENTS
	renderer := &Renderer{
		Renderer: blackfriday.HtmlRenderer(htmlFlags, "", ""),
	}
	test := func(input, expected string) {
		buffer := new(bytes.Buffer)
		renderer.AutoLink(buffer, []byte(input), blackfriday.LINK_TYPE_NORMAL)
		assert.Equal(t, expected, buffer.String())
	}

	// render valid issue URLs
	test("http://localhost:3000/user/repo/issues/3333",
		numericIssueLink("http://localhost:3000/user/repo/issues/", 3333))

	// render, but not change, invalid issue URLs
	test("http://1111/2222/ssss-issues/3333?param=blah&blahh=333",
		urlContentsLink("http://1111/2222/ssss-issues/3333?param=blah&amp;blahh=333"))
	test("http://test.com/issues/33333", urlContentsLink("http://test.com/issues/33333"))
	test("https://issues/333", urlContentsLink("https://issues/333"))

	// render valid commit URLs
	test("http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae",
		" <code><a href=\"http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae\">d8a994ef24</a></code>")
	test("http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2",
		" <code><a href=\"http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2\">d8a994ef24</a></code>")

	// render other commit URLs
	test("https://external-link.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2",
		urlContentsLink("https://external-link.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2"))
	test("https://commit/d8a994ef243349f321568f9e36d5c3f444b99cae",
		urlContentsLink("https://commit/d8a994ef243349f321568f9e36d5c3f444b99cae"))
}
