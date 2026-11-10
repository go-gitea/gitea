// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	sender_service "gitea.dev/services/mailer/sender"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailSecurityAdvisory(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	defer test.MockVariableValue(&setting.MailService, &setting.Mailer{From: "Gitea", FromEmail: "noreply@example.com"})()
	defer test.MockVariableValue(&setting.AppURL, "https://example.com/")()
	defer mockMailTemplates(string(tplSecurityAdvisory), "{{.Subject}}", "<p>{{.Title}}</p>")()

	var sent []*sender_service.Message
	defer test.MockVariableValue(&SendAsync, func(msgs ...*sender_service.Message) { sent = append(sent, msgs...) })()
	popSentTos := func() (tos []string) {
		for _, m := range sent {
			tos = append(tos, m.To)
		}
		sent = nil
		return tos
	}
	// mails are not end-to-end encrypted, so they must not reveal the vulnerability before it is published
	assertNoticeOnly := func() {
		t.Helper()
		require.NotEmpty(t, sent)
		for _, m := range sent {
			for _, secret := range []string{"XSS", "details", "confirmed"} {
				assert.NotContains(t, m.Subject+m.Body, secret)
			}
		}
	}

	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{RepoID: 1, Type: unit.TypeSecurityAdvisories}))
	user := func(id int64) *user_model.User { return unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: id}) }
	admin, reporter, collaborator, credited := user(2), user(5), user(8), user(10)

	a := &advisory_model.Advisory{
		RepoID: 1, Summary: "XSS", Description: "details", State: advisory_model.StateTriage, IsReport: true, ReporterID: reporter.ID,
		Credits: []*advisory_model.Credit{{UserID: credited.ID, Type: "finder"}},
	}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, a))
	a.Credits = nil
	_, err := advisory_model.AddCollaborator(ctx, &advisory_model.Collaborator{AdvisoryID: a.ID, UserID: collaborator.ID})
	require.NoError(t, err)

	// only repository admins get the report, not the advisory collaborator
	MailSecurityAdvisoryReported(ctx, reporter, a)
	assertNoticeOnly()
	rootID := sent[0].Headers["Message-ID"][0]
	assert.Equal(t, []string{admin.EmailTo()}, popSentTos())

	MailSecurityAdvisoryCollaboratorAdded(ctx, admin, a, collaborator, nil)
	assertNoticeOnly()
	assert.NotEqual(t, rootID, sent[0].Headers["Message-ID"][0])
	assert.Equal(t, []string{rootID}, sent[0].Headers["In-Reply-To"])
	assert.Equal(t, []string{collaborator.EmailTo()}, popSentTos())

	comment := &advisory_model.Comment{AdvisoryID: a.ID, PosterID: admin.ID, Content: "confirmed"}
	require.NoError(t, db.Insert(ctx, comment))
	MailSecurityAdvisoryComment(ctx, admin, a, comment)
	assertNoticeOnly()
	assert.ElementsMatch(t, []string{reporter.EmailTo(), collaborator.EmailTo()}, popSentTos())

	a.State = advisory_model.StatePublished
	MailSecurityAdvisoryPublished(ctx, admin, a)
	assert.Contains(t, sent[0].Subject, "XSS", "the summary of a published advisory is public")
	assert.ElementsMatch(t, []string{reporter.EmailTo(), collaborator.EmailTo(), credited.EmailTo()}, popSentTos())

	// participants who lost access to the repository are not notified anymore, the owner still is
	_, err = db.GetEngine(ctx).ID(1).Cols("is_private").Update(&repo_model.Repository{IsPrivate: true})
	require.NoError(t, err)
	a.Repo = nil
	MailSecurityAdvisoryComment(ctx, reporter, a, comment)
	assert.Equal(t, []string{admin.EmailTo()}, popSentTos())
}
