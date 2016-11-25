package markdown_test

import (
	"bytes"
	"testing"

	. "code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
	"github.com/russross/blackfriday"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMarkdown(t *testing.T) {
	Convey("Rendering an issue mention", t, func() {
		var (
			urlPrefix                   = "/prefix"
			metas     map[string]string = nil
		)
		setting.AppSubURLDepth = 0

		Convey("To the internal issue tracker", func() {
			Convey("It should not render anything when there are no mentions", func() {
				testCases := []string{
					"",
					"this is a test",
					"test 123 123 1234",
					"#",
					"# # #",
					"# 123",
					"#abcd",
					"##1234",
					"test#1234",
					"#1234test",
					" test #1234test",
				}

				for i := 0; i < len(testCases); i++ {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i])
				}
			})
			Convey("It should render freestanding mentions", func() {
				testCases := []string{
					"#1234 test", "<a href=\"/prefix/issues/1234\">#1234</a> test",
					"test #1234 issue", "test <a href=\"/prefix/issues/1234\">#1234</a> issue",
					"test issue #1234", "test issue <a href=\"/prefix/issues/1234\">#1234</a>",
					"#5 test", "<a href=\"/prefix/issues/5\">#5</a> test",
					"test #5 issue", "test <a href=\"/prefix/issues/5\">#5</a> issue",
					"test issue #5", "test issue <a href=\"/prefix/issues/5\">#5</a>",
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
			Convey("It should not render issue mention without leading space", func() {
				input := []byte("test#54321 issue")
				expected := "test#54321 issue"
				So(string(RenderIssueIndexPattern(input, urlPrefix, metas)), ShouldEqual, expected)
			})
			Convey("It should not render issue mention without trailing space", func() {
				input := []byte("test #54321issue")
				expected := "test #54321issue"
				So(string(RenderIssueIndexPattern(input, urlPrefix, metas)), ShouldEqual, expected)
			})
			Convey("It should render issue mention in parentheses", func() {
				testCases := []string{
					"(#54321 issue)", "(<a href=\"/prefix/issues/54321\">#54321</a> issue)",
					"test (#54321) issue", "test (<a href=\"/prefix/issues/54321\">#54321</a>) issue",
					"test (#54321 extra) issue", "test (<a href=\"/prefix/issues/54321\">#54321</a> extra) issue",
					"test (#54321 issue)", "test (<a href=\"/prefix/issues/54321\">#54321</a> issue)",
					"test (#54321)", "test (<a href=\"/prefix/issues/54321\">#54321</a>)",
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
			Convey("It should render multiple issue mentions in the same line", func() {
				testCases := []string{
					"#54321 #1243", "<a href=\"/prefix/issues/54321\">#54321</a> <a href=\"/prefix/issues/1243\">#1243</a>",
					"test #54321 #1243", "test <a href=\"/prefix/issues/54321\">#54321</a> <a href=\"/prefix/issues/1243\">#1243</a>",
					"(#54321 #1243)", "(<a href=\"/prefix/issues/54321\">#54321</a> <a href=\"/prefix/issues/1243\">#1243</a>)",
					"(#54321)(#1243)", "(<a href=\"/prefix/issues/54321\">#54321</a>)(<a href=\"/prefix/issues/1243\">#1243</a>)",
					"text #54321 test #1243 issue", "text <a href=\"/prefix/issues/54321\">#54321</a> test <a href=\"/prefix/issues/1243\">#1243</a> issue",
					"#1 (#4321) test", "<a href=\"/prefix/issues/1\">#1</a> (<a href=\"/prefix/issues/4321\">#4321</a>) test",
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
		})
		Convey("To an external issue tracker with numeric style", func() {
			metas = make(map[string]string)
			metas["format"] = "https://someurl.com/{user}/{repo}/{index}"
			metas["user"] = "someuser"
			metas["repo"] = "somerepo"
			metas["style"] = IssueNameStyleNumeric

			Convey("should not render anything when there are no mentions", func() {
				testCases := []string{
					"this is a test",
					"test 123 123 1234",
					"#",
					"# # #",
					"# 123",
					"#abcd",
				}

				for i := 0; i < len(testCases); i++ {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i])
				}
			})
			Convey("It should render freestanding issue mentions", func() {
				testCases := []string{
					"#1234 test", "<a href=\"https://someurl.com/someuser/somerepo/1234\">#1234</a> test",
					"test #1234 issue", "test <a href=\"https://someurl.com/someuser/somerepo/1234\">#1234</a> issue",
					"test issue #1234", "test issue <a href=\"https://someurl.com/someuser/somerepo/1234\">#1234</a>",
					"#5 test", "<a href=\"https://someurl.com/someuser/somerepo/5\">#5</a> test",
					"test #5 issue", "test <a href=\"https://someurl.com/someuser/somerepo/5\">#5</a> issue",
					"test issue #5", "test issue <a href=\"https://someurl.com/someuser/somerepo/5\">#5</a>",
				}
				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
			Convey("It should not render issue mention without leading space", func() {
				input := []byte("test#54321 issue")
				expected := "test#54321 issue"
				So(string(RenderIssueIndexPattern(input, urlPrefix, metas)), ShouldEqual, expected)
			})
			Convey("It should not render issue mention without trailing space", func() {
				input := []byte("test #54321issue")
				expected := "test #54321issue"
				So(string(RenderIssueIndexPattern(input, urlPrefix, metas)), ShouldEqual, expected)
			})
			Convey("It should render issue mention in parentheses", func() {
				testCases := []string{
					"(#54321 issue)", "(<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a> issue)",
					"test (#54321) issue", "test (<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a>) issue",
					"test (#54321 extra) issue", "test (<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a> extra) issue",
					"test (#54321 issue)", "test (<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a> issue)",
					"test (#54321)", "test (<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a>)",
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
			Convey("It should render multiple issue mentions in the same line", func() {
				testCases := []string{
					"#54321 #1243", "<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a> <a href=\"https://someurl.com/someuser/somerepo/1243\">#1243</a>",
					"test #54321 #1243", "test <a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a> <a href=\"https://someurl.com/someuser/somerepo/1243\">#1243</a>",
					"(#54321 #1243)", "(<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a> <a href=\"https://someurl.com/someuser/somerepo/1243\">#1243</a>)",
					"(#54321)(#1243)", "(<a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a>)(<a href=\"https://someurl.com/someuser/somerepo/1243\">#1243</a>)",
					"text #54321 test #1243 issue", "text <a href=\"https://someurl.com/someuser/somerepo/54321\">#54321</a> test <a href=\"https://someurl.com/someuser/somerepo/1243\">#1243</a> issue",
					"#1 (#4321) test", "<a href=\"https://someurl.com/someuser/somerepo/1\">#1</a> (<a href=\"https://someurl.com/someuser/somerepo/4321\">#4321</a>) test",
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
		})
		Convey("To an external issue tracker with alphanumeric style", func() {
			metas = make(map[string]string)
			metas["format"] = "https://someurl.com/{user}/{repo}/?b={index}"
			metas["user"] = "someuser"
			metas["repo"] = "somerepo"
			metas["style"] = IssueNameStyleAlphanumeric
			Convey("It should not render anything when there are no mentions", func() {
				testCases := []string{
					"",
					"this is a test",
					"test 123 123 1234",
					"#",
					"##1234",
					"# 123",
					"#abcd",
					"test #123",
					"abc-1234",         // issue prefix must be capital
					"ABc-1234",         // issue prefix must be _all_ capital
					"ABCDEFGHIJK-1234", // the limit is 10 characters in the prefix
					"ABC1234",          // dash is required
					"test ABC- test",   // number is required
					"test -1234 test",  // prefix is required
					"testABC-123 test", // leading space is required
					"test ABC-123test", // trailing space is required
					"ABC-0123",         // no leading zero
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i])
				}
			})
			Convey("It should render freestanding issue mention", func() {
				testCases := []string{
					"OTT-1234 test", "<a href=\"https://someurl.com/someuser/somerepo/?b=OTT-1234\">OTT-1234</a> test",
					"test T-12 issue", "test <a href=\"https://someurl.com/someuser/somerepo/?b=T-12\">T-12</a> issue",
					"test issue ABCDEFGHIJ-1234567890", "test issue <a href=\"https://someurl.com/someuser/somerepo/?b=ABCDEFGHIJ-1234567890\">ABCDEFGHIJ-1234567890</a>",
					"A-1 test", "<a href=\"https://someurl.com/someuser/somerepo/?b=A-1\">A-1</a> test",
					"test ZED-1 issue", "test <a href=\"https://someurl.com/someuser/somerepo/?b=ZED-1\">ZED-1</a> issue",
					"test issue DEED-7154", "test issue <a href=\"https://someurl.com/someuser/somerepo/?b=DEED-7154\">DEED-7154</a>",
				}
				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
			Convey("It should render issue mention in parentheses", func() {
				testCases := []string{
					"(ABG-124 issue)", "(<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a> issue)",
					"test (ABG-124) issue", "test (<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a>) issue",
					"test (ABG-124 extra) issue", "test (<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a> extra) issue",
					"test (ABG-124 issue)", "test (<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a> issue)",
					"test (ABG-124)", "test (<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a>)",
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
			Convey("It should render multiple issue mentions in the same line", func() {
				testCases := []string{
					"ABG-124 OTT-4321", "<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a> <a href=\"https://someurl.com/someuser/somerepo/?b=OTT-4321\">OTT-4321</a>",
					"test ABG-124 OTT-4321", "test <a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a> <a href=\"https://someurl.com/someuser/somerepo/?b=OTT-4321\">OTT-4321</a>",
					"(ABG-124 OTT-4321)", "(<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a> <a href=\"https://someurl.com/someuser/somerepo/?b=OTT-4321\">OTT-4321</a>)",
					"(ABG-124)(OTT-4321)", "(<a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a>)(<a href=\"https://someurl.com/someuser/somerepo/?b=OTT-4321\">OTT-4321</a>)",
					"text ABG-124 test OTT-4321 issue", "text <a href=\"https://someurl.com/someuser/somerepo/?b=ABG-124\">ABG-124</a> test <a href=\"https://someurl.com/someuser/somerepo/?b=OTT-4321\">OTT-4321</a> issue",
					"A-1 (RRE-345) test", "<a href=\"https://someurl.com/someuser/somerepo/?b=A-1\">A-1</a> (<a href=\"https://someurl.com/someuser/somerepo/?b=RRE-345\">RRE-345</a>) test",
				}

				for i := 0; i < len(testCases); i += 2 {
					So(string(RenderIssueIndexPattern([]byte(testCases[i]), urlPrefix, metas)), ShouldEqual, testCases[i+1])
				}
			})
		})
	})

	Convey("Rendering an issue URL", t, func() {
		setting.AppURL = "http://localhost:3000/"
		htmlFlags := 0
		htmlFlags |= blackfriday.HTML_SKIP_STYLE
		htmlFlags |= blackfriday.HTML_OMIT_CONTENTS
		renderer := &Renderer{
			Renderer: blackfriday.HtmlRenderer(htmlFlags, "", ""),
		}
		buffer := new(bytes.Buffer)
		Convey("To the internal issue tracker", func() {
			Convey("It should render valid issue URLs", func() {
				testCases := []string{
					"http://localhost:3000/user/repo/issues/3333", "<a href=\"http://localhost:3000/user/repo/issues/3333\">#3333</a>",
				}

				for i := 0; i < len(testCases); i += 2 {
					renderer.AutoLink(buffer, []byte(testCases[i]), blackfriday.LINK_TYPE_NORMAL)

					line, _ := buffer.ReadString(0)
					So(line, ShouldEqual, testCases[i+1])
				}
			})
			Convey("It should render but not change non-issue URLs", func() {
				testCases := []string{
					"http://1111/2222/ssss-issues/3333?param=blah&blahh=333", "<a href=\"http://1111/2222/ssss-issues/3333?param=blah&amp;blahh=333\">http://1111/2222/ssss-issues/3333?param=blah&amp;blahh=333</a>",
					"http://test.com/issues/33333", "<a href=\"http://test.com/issues/33333\">http://test.com/issues/33333</a>",
					"http://test.com/issues/3", "<a href=\"http://test.com/issues/3\">http://test.com/issues/3</a>",
					"http://issues/333", "<a href=\"http://issues/333\">http://issues/333</a>",
					"https://issues/333", "<a href=\"https://issues/333\">https://issues/333</a>",
					"http://tissues/0", "<a href=\"http://tissues/0\">http://tissues/0</a>",
				}

				for i := 0; i < len(testCases); i += 2 {
					renderer.AutoLink(buffer, []byte(testCases[i]), blackfriday.LINK_TYPE_NORMAL)

					line, _ := buffer.ReadString(0)
					So(line, ShouldEqual, testCases[i+1])
				}
			})
		})
	})

	Convey("Rendering a commit URL", t, func() {
		setting.AppURL = "http://localhost:3000/"
		htmlFlags := 0
		htmlFlags |= blackfriday.HTML_SKIP_STYLE
		htmlFlags |= blackfriday.HTML_OMIT_CONTENTS
		renderer := &Renderer{
			Renderer: blackfriday.HtmlRenderer(htmlFlags, "", ""),
		}
		buffer := new(bytes.Buffer)
		Convey("To the internal issue tracker", func() {
			Convey("It should correctly convert URLs", func() {
				testCases := []string{
					"http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae", " <code><a href=\"http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae\">d8a994ef24</a></code>",
					"http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2", " <code><a href=\"http://localhost:3000/user/project/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2\">d8a994ef24</a></code>",
					"https://external-link.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2", "<a href=\"https://external-link.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2\">https://external-link.gogs.io/gogs/gogs/commit/d8a994ef243349f321568f9e36d5c3f444b99cae#diff-2</a>",
					"https://commit/d8a994ef243349f321568f9e36d5c3f444b99cae", "<a href=\"https://commit/d8a994ef243349f321568f9e36d5c3f444b99cae\">https://commit/d8a994ef243349f321568f9e36d5c3f444b99cae</a>",
				}

				for i := 0; i < len(testCases); i += 2 {
					renderer.AutoLink(buffer, []byte(testCases[i]), blackfriday.LINK_TYPE_NORMAL)

					line, _ := buffer.ReadString(0)
					So(line, ShouldEqual, testCases[i+1])
				}
			})
		})
	})
}
