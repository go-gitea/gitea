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

	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{RepoID: 1, Type: unit.TypeSecurityAdvisories}))
	user := func(id int64) *user_model.User { return unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: id}) }
	admin, reporter, collaborator, credited := user(2), user(5), user(8), user(10)

	advisory := &advisory_model.Advisory{
		RepoID: 1, Summary: "XSS", Description: "details", State: advisory_model.StateTriage, IsReport: true, ReporterID: reporter.ID,
		Credits: []*advisory_model.Credit{{UserID: credited.ID, Type: "finder"}},
	}
	popMail := func(recipients ...*user_model.User) []*sender_service.Message {
		t.Helper()
		var want, got []string
		for _, user := range recipients {
			want = append(want, user.EmailTo())
		}
		for _, msg := range sent {
			got = append(got, msg.To)
			if !advisory.IsPublic() {
				for _, private := range []string{"XSS", "details", "confirmed"} {
					assert.NotContains(t, msg.Subject+msg.Body, private)
				}
			}
		}
		require.ElementsMatch(t, want, got)
		messages := sent
		sent = nil
		return messages
	}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, advisory))
	advisory.Credits = nil
	_, err := advisory_model.AddCollaborator(ctx, &advisory_model.Collaborator{AdvisoryID: advisory.ID, UserID: collaborator.ID})
	require.NoError(t, err)

	MailSecurityAdvisoryReported(ctx, reporter, advisory)
	rootID := popMail(admin)[0].Headers["Message-ID"]
	require.Len(t, rootID, 1)

	MailSecurityAdvisoryCollaboratorAdded(ctx, admin, advisory, collaborator, nil)
	invitation := popMail(collaborator)[0]
	assert.NotEqual(t, rootID, invitation.Headers["Message-ID"])
	assert.Equal(t, rootID, invitation.Headers["In-Reply-To"])

	comment := &advisory_model.Comment{ID: 1, Content: "confirmed"}
	MailSecurityAdvisoryComment(ctx, admin, advisory, comment)
	popMail(reporter, collaborator)
	comment.IsConfidential = true
	MailSecurityAdvisoryComment(ctx, admin, advisory, comment)
	popMail()

	advisory.State = advisory_model.StateClosed
	MailSecurityAdvisoryStateChanged(ctx, admin, advisory, advisory_model.StateTriage)
	assert.Contains(t, popMail(reporter, collaborator)[0].Subject, "closed")

	advisory.State = advisory_model.StatePublished
	MailSecurityAdvisoryPublished(ctx, admin, advisory)
	assert.Contains(t, popMail(reporter, collaborator, credited)[0].Subject, "XSS")

	_, err = db.GetEngine(ctx).ID(1).Cols("is_private").Update(&repo_model.Repository{IsPrivate: true})
	require.NoError(t, err)
	advisory.Repo = nil
	MailSecurityAdvisoryComment(ctx, reporter, advisory, comment)
	popMail(admin)
}
