// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package incoming

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/jhillyerd/enmime/v2"
	"github.com/stretchr/testify/assert"
)

func TestIsAutomaticReply(t *testing.T) {
	cases := []struct {
		Headers  map[string]string
		Expected bool
	}{
		{
			Headers:  map[string]string{},
			Expected: false,
		},
		{
			Headers: map[string]string{
				"Auto-Submitted": "no",
			},
			Expected: false,
		},
		{
			Headers: map[string]string{
				"Auto-Submitted": "yes",
			},
			Expected: true,
		},
		{
			Headers: map[string]string{
				"X-Autoreply": "no",
			},
			Expected: false,
		},
		{
			Headers: map[string]string{
				"X-Autoreply": "yes",
			},
			Expected: true,
		},
		{
			Headers: map[string]string{
				"X-Autorespond": "yes",
			},
			Expected: true,
		},
	}

	for _, c := range cases {
		b := enmime.Builder().
			From("Dummy", "dummy@gitea.io").
			To("Dummy", "dummy@gitea.io")
		for k, v := range c.Headers {
			b = b.Header(k, v)
		}
		root, err := b.Build()
		assert.NoError(t, err)
		env, err := enmime.EnvelopeFromPart(root)
		assert.NoError(t, err)

		assert.Equal(t, c.Expected, isAutomaticReply(env))
	}
}

func TestSearchTokenInHeadersCaseInsensitive(t *testing.T) {
	setting.IncomingEmail.ReplyToAddress = "InComing+%{token}@ExAmPle.com"
	setting.Domain = "DoMain.com"
	mkEnv := func(s string) *enmime.Envelope {
		env, _ := enmime.ReadEnvelope(strings.NewReader(s + "\r\n\r\n"))
		return env
	}
	assert.Equal(t, "abc", searchTokenInHeaders(mkEnv("To: incoming+abc@EXAMPLE.COM")))
	assert.Equal(t, "abc", searchTokenInHeaders(mkEnv("Delivered-To: INCOMING+abc@example.com")))
	assert.Equal(t, "abc", searchTokenInHeaders(mkEnv("References: <ReplY-abc@DomaiN.COM>")))
}

func TestGetContentFromMailReader(t *testing.T) {
	mailString := "Content-Type: multipart/mixed; boundary=message-boundary\r\n" +
		"\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: multipart/alternative; boundary=text-boundary\r\n" +
		"\r\n" +
		"--text-boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: inline\r\n" +
		"\r\n" +
		"mail content\r\n" +
		"--text-boundary--\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: attachment; filename=attachment.txt\r\n" +
		"\r\n" +
		"attachment content\r\n" +
		"--message-boundary--\r\n"

	env, err := enmime.ReadEnvelope(strings.NewReader(mailString))
	assert.NoError(t, err)
	content := getContentFromMailReader(env)
	assert.Equal(t, "mail content", content.Content)
	assert.Len(t, content.Attachments, 1)
	assert.Equal(t, "attachment.txt", content.Attachments[0].Name)
	assert.Equal(t, []byte("attachment content"), content.Attachments[0].Content)

	mailString = "Content-Type: multipart/mixed; boundary=message-boundary\r\n" +
		"\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: multipart/alternative; boundary=text-boundary\r\n" +
		"\r\n" +
		"--text-boundary\r\n" +
		"Content-Type: text/html\r\n" +
		"Content-Disposition: inline\r\n" +
		"\r\n" +
		"<p>mail content</p>\r\n" +
		"--text-boundary--\r\n" +
		"--message-boundary--\r\n"

	env, err = enmime.ReadEnvelope(strings.NewReader(mailString))
	assert.NoError(t, err)
	content = getContentFromMailReader(env)
	assert.Equal(t, "mail content", content.Content)
	assert.Empty(t, content.Attachments)

	mailString = "Content-Type: multipart/mixed; boundary=message-boundary\r\n" +
		"\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: multipart/alternative; boundary=text-boundary\r\n" +
		"\r\n" +
		"--text-boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: inline\r\n" +
		"\r\n" +
		"mail content without signature\r\n" +
		"--\r\n" +
		"signature\r\n" +
		"--text-boundary--\r\n" +
		"--message-boundary--\r\n"

	env, err = enmime.ReadEnvelope(strings.NewReader(mailString))
	assert.NoError(t, err)
	content = getContentFromMailReader(env)
	assert.NoError(t, err)
	assert.Equal(t, "mail content without signature", content.Content)
	assert.Empty(t, content.Attachments)
}

func TestExtractReply(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "Email with only text.", "Email with only text."},
		{"crlf normalized", "line one\r\nline two\r\n", "line one\nline two"},
		{"trim blank lines", "\n\n\nactual reply\n\n\n", "actual reply"},
		{"signature delimiter", "the reply\n--\nJohn Doe\nAcme", "the reply"},
		{"rfc signature delimiter", "the reply\n-- \nJohn Doe", "the reply"},
		{"mobile signature", "My answer is yes.\n\nSent from my iPhone", "My answer is yes."},
		{"quote only kept", "> Email with only quote.", "> Email with only quote."},
		{"leading quote kept", "> This is a quote.\n\nAnd this is some text.", "> This is a quote.\n\nAnd this is some text."},
		{"trailing quote stripped", "My reply.\n\n> original line 1\n> original line 2", "My reply."},
		{"attribution and quote", "Looks good.\n\nOn Mon, Jan 1, 2024 John <j@x.com> wrote:\n> please review", "Looks good."},
		{"attribution without quote marks", "My reply.\n\nOn Wed, Sep 25, 2013, richard wrote:\noriginal text", "My reply."},
		{"original message separator", "Foo\n\n-------- Original Message --------\n\nTHE END.", "Foo"},
		{"outlook header block", "This is the actual reply.\n\nFrom: Some One <a@b.com>\nSent: Monday\nTo: Someone\nSubject: hi\n\nquoted body", "This is the actual reply."},
		{"french attribution", "C'est super !\n\nLe 4 janv. 2016 19:03, \"Neil\" <a@b.com> a écrit :\n> quoted", "C'est super !"},
		{"german attribution", "Hey :)\n\nAm 03.02.2016 3:35 schrieb Max <a@b.com>:\n> quoted", "Hey :)"},
		{"cyrillic wrote verb", "Yes.\n\n6 октября 2014 lidel написал:\n> quoted", "Yes."},
		{"localized signature", "My answer.\n\nEnvoyé depuis mon iPhone", "My answer."},
		{"swedish header block", "Hi everyone!\n\nFrån: Foo <a@b.com>\nSkickat: den 5 juni\nTill: x@y.com\nÄmne: hi\n\nbody", "Hi everyone!"},
		{"attribution only is empty", "On Mon, Jan 1, 2024 at 10:00 John <j@x.com> wrote:\n> please review", ""},
		{"prose ending in wrote kept", "Hi Bob,\nThanks for the report you wrote\nI'll fix it.", "Hi Bob,\nThanks for the report you wrote\nI'll fix it."},
		{"on with year and no time kept", "Hi,\nOn the 2024 roadmap we have three items.\nPlease review.", "Hi,\nOn the 2024 roadmap we have three items.\nPlease review."},
		{"date prose kept", "Notes:\n5 issues 2024 fixed at 9:15 today\nmore notes", "Notes:\n5 issues 2024 fixed at 9:15 today\nmore notes"},
		{"header needs from first", "Quick note:\nTo: which server?\nFrom: tests pass.\nThanks", "Quick note:\nTo: which server?\nFrom: tests pass.\nThanks"},
		{"indented header block", "Reply text.\n\n  From: A <a@b.com>\n  Sent: Monday\n  To: x\n  Subject: hi\n\nbody", "Reply text."},
		{"chinese signature", "回复内容\n\n發自我的iPhone", "回复内容"},
		{"japanese signature", "返信します\n\niPhoneから送信", "返信します"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, extractReply(c.input))
		})
	}
}
