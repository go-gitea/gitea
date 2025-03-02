// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"mime/quotedprintable"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	texttmpl "text/template"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	sender_service "code.gitea.io/gitea/services/mailer/sender"

	"github.com/stretchr/testify/assert"
)

const subjectTpl = `
{{.SubjectPrefix}}[{{.Repo}}] @{{.Doer.Name}} #{{.Issue.Index}} - {{.Issue.Title}}
`

const bodyTpl = `
<!DOCTYPE html>
<html>
<head>
	<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
	<title>{{.Subject}}</title>
</head>

<body>
	<p>{{.Body}}</p>
	<p>
		---
		<br>
		<a href="{{.Link}}">View it on Gitea</a>.
	</p>
</body>
</html>
`

func prepareMailerTest(t *testing.T) (doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	mailService := setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"
	setting.AppURL = "https://try.gitea.io/"

	doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, Owner: doer})
	issue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1, Repo: repo, Poster: doer})
	assert.NoError(t, issue.LoadRepo(db.DefaultContext))
	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2, Issue: issue})
	return doer, repo, issue, comment
}

func TestComposeIssueCommentMessage(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)

	markup.Init(&markup.RenderHelperFuncs{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			return username == doer.Name
		},
	})

	setting.IncomingEmail.Enabled = true
	defer func() { setting.IncomingEmail.Enabled = false }()

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/comment").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/comment").Parse(bodyTpl))

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(&mailCommentContext{
		Context: t.Context(),
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCommentIssue,
		Content: fmt.Sprintf("test @%s %s#%d body", doer.Name, issue.Repo.FullName(), issue.Index),
		Comment: comment,
	}, "en-US", recipients, false, "issue comment")
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)
	gomailMsg := msgs[0].ToMessage()
	replyTo := gomailMsg.GetGenHeader("Reply-To")[0]
	subject := gomailMsg.GetGenHeader("Subject")[0]

	assert.Len(t, gomailMsg.GetAddrHeader("To"), 1, "exactly one recipient is expected in the To field")
	tokenRegex := regexp.MustCompile(`\Aincoming\+(.+)@localhost\z`)
	assert.Regexp(t, tokenRegex, replyTo)
	token := tokenRegex.FindAllStringSubmatch(replyTo, 1)[0][1]
	assert.Equal(t, "Re: ", subject[:4], "Comment reply subject should contain Re:")
	assert.Equal(t, "Re: [user2/repo1] @user2 #1 - issue1", subject)
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", gomailMsg.GetGenHeader("In-Reply-To")[0], "In-Reply-To header doesn't match")
	assert.ElementsMatch(t, []string{"<user2/repo1/issues/1@localhost>", "<reply-" + token + "@localhost>"}, gomailMsg.GetGenHeader("References"), "References header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1/comment/2@localhost>", gomailMsg.GetGenHeader("Message-ID")[0], "Message-ID header doesn't match")
	assert.Equal(t, "<mailto:"+replyTo+">", gomailMsg.GetGenHeader("List-Post")[0])
	assert.Len(t, gomailMsg.GetGenHeader("List-Unsubscribe"), 2) // url + mailto

	var buf bytes.Buffer
	gomailMsg.WriteTo(&buf)

	b, err := io.ReadAll(quotedprintable.NewReader(&buf))
	assert.NoError(t, err)

	// text/plain
	assert.Contains(t, string(b), fmt.Sprintf(`( %s )`, doer.HTMLURL()))
	assert.Contains(t, string(b), fmt.Sprintf(`( %s )`, issue.HTMLURL()))

	// text/html
	assert.Contains(t, string(b), fmt.Sprintf(`href="%s"`, doer.HTMLURL()))
	assert.Contains(t, string(b), fmt.Sprintf(`href="%s"`, issue.HTMLURL()))
}

func TestComposeIssueMessage(t *testing.T) {
	doer, _, issue, _ := prepareMailerTest(t)

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/new").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/new").Parse(bodyTpl))

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}, {Name: "Test2", Email: "test2@gitea.com"}}
	msgs, err := composeIssueCommentMessages(&mailCommentContext{
		Context: t.Context(),
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCreateIssue,
		Content: "test body",
	}, "en-US", recipients, false, "issue create")
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)

	gomailMsg := msgs[0].ToMessage()
	mailto := gomailMsg.GetAddrHeader("To")
	subject := gomailMsg.GetGenHeader("Subject")
	messageID := gomailMsg.GetGenHeader("Message-ID")
	inReplyTo := gomailMsg.GetGenHeader("In-Reply-To")
	references := gomailMsg.GetGenHeader("References")

	assert.Len(t, mailto, 1, "exactly one recipient is expected in the To field")
	assert.Equal(t, "[user2/repo1] @user2 #1 - issue1", subject[0])
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", inReplyTo[0], "In-Reply-To header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", references[0], "References header doesn't match")
	assert.Equal(t, "<user2/repo1/issues/1@localhost>", messageID[0], "Message-ID header doesn't match")
	assert.Empty(t, gomailMsg.GetGenHeader("List-Post"))         // incoming mail feature disabled
	assert.Len(t, gomailMsg.GetGenHeader("List-Unsubscribe"), 1) // url without mailto
}

func TestTemplateSelection(t *testing.T) {
	doer, repo, issue, comment := prepareMailerTest(t)
	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/default").Parse("issue/default/subject"))
	texttmpl.Must(subjectTemplates.New("issue/new").Parse("issue/new/subject"))
	texttmpl.Must(subjectTemplates.New("pull/comment").Parse("pull/comment/subject"))
	texttmpl.Must(subjectTemplates.New("issue/close").Parse("")) // Must default to fallback subject

	bodyTemplates = template.Must(template.New("issue/default").Parse("issue/default/body"))
	template.Must(bodyTemplates.New("issue/new").Parse("issue/new/body"))
	template.Must(bodyTemplates.New("pull/comment").Parse("pull/comment/body"))
	template.Must(bodyTemplates.New("issue/close").Parse("issue/close/body"))

	expect := func(t *testing.T, msg *sender_service.Message, expSubject, expBody string) {
		subject := msg.ToMessage().GetGenHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()
		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, expBody)
	}

	msg := testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: t.Context(),
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCreateIssue,
		Content: "test body",
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/new/subject", "issue/new/body")

	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: t.Context(),
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCommentIssue,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "issue/default/subject", "issue/default/body")

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2, Repo: repo, Poster: doer})
	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 4, Issue: pull})
	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: t.Context(),
		Issue:   pull, Doer: doer, ActionType: activities_model.ActionCommentPull,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "pull/comment/subject", "pull/comment/body")

	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Context: t.Context(),
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCloseIssue,
		Content: "test body", Comment: comment,
	}, recipients, false, "TestTemplateSelection")
	expect(t, msg, "Re: [user2/repo1] issue1 (#1)", "issue/close/body")
}

func TestTemplateServices(t *testing.T) {
	doer, _, issue, comment := prepareMailerTest(t)
	assert.NoError(t, issue.LoadRepo(db.DefaultContext))

	expect := func(t *testing.T, issue *issues_model.Issue, comment *issues_model.Comment, doer *user_model.User,
		actionType activities_model.ActionType, fromMention bool, tplSubject, tplBody, expSubject, expBody string,
	) {
		subjectTemplates = texttmpl.Must(texttmpl.New("issue/default").Parse(tplSubject))
		bodyTemplates = template.Must(template.New("issue/default").Parse(tplBody))

		recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}
		msg := testComposeIssueCommentMessage(t, &mailCommentContext{
			Context: t.Context(),
			Issue:   issue, Doer: doer, ActionType: actionType,
			Content: "test body", Comment: comment,
		}, recipients, fromMention, "TestTemplateServices")

		subject := msg.ToMessage().GetGenHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()

		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, "\r\n"+expBody+"\r\n")
	}

	expect(t, issue, comment, doer, activities_model.ActionCommentIssue, false,
		"{{.SubjectPrefix}}[{{.Repo}}]: @{{.Doer.Name}} commented on #{{.Issue.Index}} - {{.Issue.Title}}",
		"//{{.ActionType}},{{.ActionName}},{{if .IsMention}}norender{{end}}//",
		"Re: [user2/repo1]: @user2 commented on #1 - issue1",
		"//issue,comment,//")

	expect(t, issue, comment, doer, activities_model.ActionCommentIssue, true,
		"{{if .IsMention}}must render{{end}}",
		"//subject is: {{.Subject}}//",
		"must render",
		"//subject is: must render//")

	expect(t, issue, comment, doer, activities_model.ActionCommentIssue, true,
		"{{.FallbackSubject}}",
		"//{{.SubjectPrefix}}//",
		"Re: [user2/repo1] issue1 (#1)",
		"//Re: //")
}

func testComposeIssueCommentMessage(t *testing.T, ctx *mailCommentContext, recipients []*user_model.User, fromMention bool, info string) *sender_service.Message {
	msgs, err := composeIssueCommentMessages(ctx, "en-US", recipients, fromMention, info)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	return msgs[0]
}

func TestGenerateAdditionalHeaders(t *testing.T) {
	doer, _, issue, _ := prepareMailerTest(t)

	ctx := &mailCommentContext{Context: t.Context(), Issue: issue, Doer: doer}
	recipient := &user_model.User{Name: "test", Email: "test@gitea.com"}

	headers := generateAdditionalHeaders(ctx, "dummy-reason", recipient)

	expected := map[string]string{
		"List-ID":                   "user2/repo1 <repo1.user2.localhost>",
		"List-Archive":              "<https://try.gitea.io/user2/repo1>",
		"X-Gitea-Reason":            "dummy-reason",
		"X-Gitea-Sender":            "user2",
		"X-Gitea-Recipient":         "test",
		"X-Gitea-Recipient-Address": "test@gitea.com",
		"X-Gitea-Repository":        "repo1",
		"X-Gitea-Repository-Path":   "user2/repo1",
		"X-Gitea-Repository-Link":   "https://try.gitea.io/user2/repo1",
		"X-Gitea-Issue-ID":          "1",
		"X-Gitea-Issue-Link":        "https://try.gitea.io/user2/repo1/issues/1",
	}

	for key, value := range expected {
		if assert.Contains(t, headers, key) {
			assert.Equal(t, value, headers[key])
		}
	}
}

func TestGenerateMessageIDForIssue(t *testing.T) {
	_, _, issue, comment := prepareMailerTest(t)
	_, _, pullIssue, _ := prepareMailerTest(t)
	pullIssue.IsPull = true

	type args struct {
		issue      *issues_model.Issue
		comment    *issues_model.Comment
		actionType activities_model.ActionType
	}
	tests := []struct {
		name   string
		args   args
		prefix string
	}{
		{
			name: "Open Issue",
			args: args{
				issue:      issue,
				actionType: activities_model.ActionCreateIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d@%s>", issue.Repo.FullName(), issue.Index, setting.Domain),
		},
		{
			name: "Open Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionCreatePullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d@%s>", issue.Repo.FullName(), issue.Index, setting.Domain),
		},
		{
			name: "Comment Issue",
			args: args{
				issue:      issue,
				comment:    comment,
				actionType: activities_model.ActionCommentIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d/comment/%d@%s>", issue.Repo.FullName(), issue.Index, comment.ID, setting.Domain),
		},
		{
			name: "Comment Pull",
			args: args{
				issue:      pullIssue,
				comment:    comment,
				actionType: activities_model.ActionCommentPull,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/comment/%d@%s>", issue.Repo.FullName(), issue.Index, comment.ID, setting.Domain),
		},
		{
			name: "Close Issue",
			args: args{
				issue:      issue,
				actionType: activities_model.ActionCloseIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d/close/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Close Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionClosePullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/close/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Reopen Issue",
			args: args{
				issue:      issue,
				actionType: activities_model.ActionReopenIssue,
			},
			prefix: fmt.Sprintf("<%s/issues/%d/reopen/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Reopen Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionReopenPullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/reopen/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Merge Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionMergePullRequest,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/merge/", issue.Repo.FullName(), issue.Index),
		},
		{
			name: "Ready Pull",
			args: args{
				issue:      pullIssue,
				actionType: activities_model.ActionPullRequestReadyForReview,
			},
			prefix: fmt.Sprintf("<%s/pulls/%d/ready/", issue.Repo.FullName(), issue.Index),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateMessageIDForIssue(tt.args.issue, tt.args.comment, tt.args.actionType)
			assert.True(t, strings.HasPrefix(got, tt.prefix), "%v, want %v", got, tt.prefix)
		})
	}
}

func TestGenerateMessageIDForRelease(t *testing.T) {
	msgID := generateMessageIDForRelease(&repo_model.Release{
		ID:   1,
		Repo: &repo_model.Repository{OwnerName: "owner", Name: "repo"},
	})
	assert.Equal(t, "<owner/repo/releases/1@localhost>", msgID)
}

func TestFromDisplayName(t *testing.T) {
	template, err := texttmpl.New("mailFrom").Parse("{{ .DisplayName }}")
	assert.NoError(t, err)
	setting.MailService = &setting.Mailer{FromDisplayNameFormatTemplate: template}
	defer func() { setting.MailService = nil }()

	tests := []struct {
		userDisplayName string
		fromDisplayName string
	}{{
		userDisplayName: "test",
		fromDisplayName: "test",
	}, {
		userDisplayName: "Hi Its <Mee>",
		fromDisplayName: "Hi Its <Mee>",
	}, {
		userDisplayName: "Æsir",
		fromDisplayName: "=?utf-8?q?=C3=86sir?=",
	}, {
		userDisplayName: "new😀user",
		fromDisplayName: "=?utf-8?q?new=F0=9F=98=80user?=",
	}}

	for _, tc := range tests {
		t.Run(tc.userDisplayName, func(t *testing.T) {
			user := &user_model.User{FullName: tc.userDisplayName, Name: "tmp"}
			got := fromDisplayName(user)
			assert.EqualValues(t, tc.fromDisplayName, got)
		})
	}

	t.Run("template with all available vars", func(t *testing.T) {
		template, err = texttmpl.New("mailFrom").Parse("{{ .DisplayName }} (by {{ .AppName }} on [{{ .Domain }}])")
		assert.NoError(t, err)
		setting.MailService = &setting.Mailer{FromDisplayNameFormatTemplate: template}
		oldAppName := setting.AppName
		setting.AppName = "Code IT"
		oldDomain := setting.Domain
		setting.Domain = "code.it"
		defer func() {
			setting.AppName = oldAppName
			setting.Domain = oldDomain
		}()

		assert.EqualValues(t, "Mister X (by Code IT on [code.it])", fromDisplayName(&user_model.User{FullName: "Mister X", Name: "tmp"}))
	})
}

func PrepareAttachmentsStorage(t testing.TB) { // same as in test_utils.go
	// prepare attachments directory and files
	assert.NoError(t, storage.Clean(storage.Attachments))

	s, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests", "testdata", "data", "attachments"),
	})
	assert.NoError(t, err)
	assert.NoError(t, s.IterateObjects("", func(p string, obj storage.Object) error {
		_, err = storage.Copy(storage.Attachments, p, s, p)
		return err
	}))
}

func TestEmbedBase64ImagesInEmail(t *testing.T) {
	// Fake context setup
	doer, repo, _, _ := prepareMailerTest(t)
	PrepareAttachmentsStorage(t)
	setting.MailService.Base64EmbedImages = true
	setting.MailService.Base64EmbedImagesMaxSizePerEmail = 10 * 1024 * 1024
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 23, Repo: repo, Poster: doer})
	assert.NoError(t, issue.LoadRepo(db.DefaultContext))

	subjectTemplates = texttmpl.Must(texttmpl.New("issue/new").Parse(subjectTpl))
	bodyTemplates = template.Must(template.New("issue/new").Parse(bodyTpl))

	recipients := []*user_model.User{{Name: "Test", Email: "test@gitea.com"}}
	msgs, err := composeIssueCommentMessages(&mailCommentContext{
		Context: context.TODO(), // TODO: use a correct context
		Issue:   issue, Doer: doer, ActionType: activities_model.ActionCreateIssue,
		Content: strings.ReplaceAll(issue.Content, `src="`, `src="`+setting.AppURL),
	}, "en-US", recipients, false, "issue create")

	mailBody := msgs[0].Body
	re := regexp.MustCompile(`(?s)<body>(.*?)</body>`)
	matches := re.FindStringSubmatch(mailBody)
	if len(matches) > 1 {
		mailBody = matches[1]
	}
	// check if the mail body was correctly generated
	assert.NoError(t, err)
	assert.Contains(t, mailBody, "content including this image")

	// check if an image was embedded
	assert.Contains(t, mailBody, "data:image/png;base64,")

	// check if the image was embedded only once
	assert.Equal(t, 1, strings.Count(mailBody, "data:image/png;base64,"))

	img2InternalBase64 := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAFAAAAAxAQMAAAB3d7wRAAAABlBMVEVgmyF6qkqITHmkAAAAAXRSTlMBN+Ho8AAAAJhJREFUKM+V0DsOwyAQBNCxXLjkCFwk0t7McDQfhS4tpQuEzWc/iaUU2eo1zC4DUMWYF3DxVKzGTXjBGb2RsjJEo6ZhN1Zj+cEgi/9hBQl3YflkkIsbo5IO5glKTuhPpavM3Hp4C7WdjEWYrL5GMkp/R+s4GPlh/CZn4MEwv9aHHiyD3ujm5X22eaMyDa5yAm+O0B1TPa1l3W2qZWMg+KgtAAAAAElFTkSuQmCC"

	// check if the image was embedded correctly
	assert.Contains(t, mailBody, img2InternalBase64)
}

func TestEmbedBase64Images(t *testing.T) {
	user, repo, _, _ := prepareMailerTest(t)
	PrepareAttachmentsStorage(t)
	setting.MailService.Base64EmbedImages = true
	setting.MailService.Base64EmbedImagesMaxSizePerEmail = 10 * 1024 * 1024

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 23, Repo: repo, Poster: user})

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 13, IssueID: issue.ID, RepoID: repo.ID})
	ctx0 := context.Background()

	ctx := &mailCommentContext{Context: ctx0 /* TODO: use a correct context */, Issue: issue, Doer: user}

	img1ExternalURL := "https://via.placeholder.com/10"
	img1ExternalImg := "<img src=\"" + img1ExternalURL + "\"/>"

	img2InternalURL := setting.AppURL + repo.Owner.Name + "/" + repo.Name + "/attachments/" + attachment.UUID
	img2InternalImg := "<img src=\"" + img2InternalURL + "\"/>"
	img2InternalBase64 := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAFAAAAAxAQMAAAB3d7wRAAAABlBMVEVgmyF6qkqITHmkAAAAAXRSTlMBN+Ho8AAAAJhJREFUKM+V0DsOwyAQBNCxXLjkCFwk0t7McDQfhS4tpQuEzWc/iaUU2eo1zC4DUMWYF3DxVKzGTXjBGb2RsjJEo6ZhN1Zj+cEgi/9hBQl3YflkkIsbo5IO5glKTuhPpavM3Hp4C7WdjEWYrL5GMkp/R+s4GPlh/CZn4MEwv9aHHiyD3ujm5X22eaMyDa5yAm+O0B1TPa1l3W2qZWMg+KgtAAAAAElFTkSuQmCC"
	img2InternalBase64Img := "<img src=\"" + img2InternalBase64 + "\"/>"

	// 1st Test: convert internal image to base64
	t.Run("replaceSpecifiedBase64ImagesInternal", func(t *testing.T) {
		totalEmbeddedImagesSize := int64(0)

		resultImg1Internal, err := AttachmentSrcToBase64DataURI(img2InternalURL, ctx, &totalEmbeddedImagesSize)
		assert.NoError(t, err)
		assert.Equal(t, img2InternalBase64, resultImg1Internal) // replace cause internal image
	})

	// 2nd Test: convert external image to base64 -> abort cause external image
	t.Run("replaceSpecifiedBase64ImagesExternal", func(t *testing.T) {
		totalEmbeddedImagesSize := int64(0)

		resultImg1External, err := AttachmentSrcToBase64DataURI(img1ExternalURL, ctx, &totalEmbeddedImagesSize)
		assert.Error(t, err)
		assert.Equal(t, "", resultImg1External) // don't replace cause external image
	})

	// 3rd Test: generate email body with 1 internal and 1 external image, expect the result to have the internal image replaced with base64 data and the external not replaced
	t.Run("generateEmailBody", func(t *testing.T) {
		mailBody := "<html><head></head><body><p>Test1</p>" + img1ExternalImg + "<p>Test2</p>" + img2InternalImg + "<p>Test3</p></body></html>"
		expectedMailBody := "<html><head></head><body><p>Test1</p>" + img1ExternalImg + "<p>Test2</p>" + img2InternalBase64Img + "<p>Test3</p></body></html>"
		resultMailBody, err := Base64InlineImages(mailBody, ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedMailBody, resultMailBody)
	})

	// 4th Test, generate email body with 2 internal images, but set Mailer.Base64EmbedImagesMaxSizePerEmail to the size of the first image (+1), expect the first image to be replaced and the second not
	t.Run("generateEmailBodyWithMaxSize", func(t *testing.T) {
		setting.MailService.Base64EmbedImagesMaxSizePerEmail = int64(len(img2InternalBase64) + 1)

		mailBody := "<html><head></head><body><p>Test1</p>" + img2InternalImg + "<p>Test2</p>" + img2InternalImg + "<p>Test3</p></body></html>"
		expectedMailBody := "<html><head></head><body><p>Test1</p>" + img2InternalBase64Img + "<p>Test2</p>" + img2InternalImg + "<p>Test3</p></body></html>"
		resultMailBody, err := Base64InlineImages(mailBody, ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedMailBody, resultMailBody)
	})

	// 5th Test, generate email body with 3 internal images, but set Mailer.Base64EmbedImagesMaxSizePerEmail to the size of all 3 images (+1), expect all images to be replaced
	t.Run("generateEmailBodyWith3Images", func(t *testing.T) {
		setting.MailService.Base64EmbedImagesMaxSizePerEmail = int64(len(img2InternalBase64)*3 + 1)

		mailBody := "<html><head></head><body><p>Test1</p>" + img2InternalImg + "<p>Test2</p>" + img2InternalImg + "<p>Test3</p>" + img2InternalImg + "</body></html>"
		expectedMailBody := "<html><head></head><body><p>Test1</p>" + img2InternalBase64Img + "<p>Test2</p>" + img2InternalBase64Img + "<p>Test3</p>" + img2InternalBase64Img + "</body></html>"
		resultMailBody, err := Base64InlineImages(mailBody, ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedMailBody, resultMailBody)
	})
}
