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
	defer mockMailTemplates(string(tplSecurityAdvisory), "{{.Subject}}", "<p>{{.Summary}}</p>")()

	var sent []*sender_service.Message
	defer test.MockVariableValue(&SendAsync, func(msgs ...*sender_service.Message) { sent = append(sent, msgs...) })()
	popSent := func() (tos []string, msgs []*sender_service.Message) {
		for _, m := range sent {
			tos = append(tos, m.To)
		}
		msgs, sent = sent, nil
		return tos, msgs
	}

	require.NoError(t, db.Insert(ctx, &repo_model.RepoUnit{RepoID: 1, Type: unit.TypeSecurityAdvisories}))
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	reporter := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	collaborator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
	credited := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})

	a := &advisory_model.Advisory{
		RepoID: 1, Summary: "XSS", Description: "details", State: advisory_model.StateTriage, IsReport: true, ReporterID: reporter.ID,
		Credits: []*advisory_model.Credit{{UserID: credited.ID, Type: "finder"}},
	}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, a))
	a.Credits = nil
	_, err := advisory_model.AddCollaborator(ctx, a.ID, collaborator.ID, 0, false)
	require.NoError(t, err)

	// only repository admins get the report, not the advisory collaborator
	MailSecurityAdvisoryReported(ctx, reporter, a)
	tos, msgs := popSent()
	assert.Equal(t, []string{admin.EmailTo()}, tos)
	rootID := msgs[0].Headers["Message-ID"][0]

	MailSecurityAdvisoryCollaboratorAdded(ctx, admin, a, collaborator, nil)
	tos, msgs = popSent()
	assert.Equal(t, []string{collaborator.EmailTo()}, tos)
	assert.NotEqual(t, rootID, msgs[0].Headers["Message-ID"][0])
	assert.Equal(t, []string{rootID}, msgs[0].Headers["In-Reply-To"])

	comment := &advisory_model.Comment{AdvisoryID: a.ID, PosterID: admin.ID, Content: "confirmed"}
	require.NoError(t, advisory_model.CreateComment(ctx, comment))
	MailSecurityAdvisoryComment(ctx, admin, a, comment)
	tos, _ = popSent()
	assert.ElementsMatch(t, []string{reporter.EmailTo(), collaborator.EmailTo()}, tos)

	a.State = advisory_model.StatePublished
	MailSecurityAdvisoryPublished(ctx, admin, a)
	tos, _ = popSent()
	assert.ElementsMatch(t, []string{reporter.EmailTo(), collaborator.EmailTo(), credited.EmailTo()}, tos)

	// participants who lost access to the repository are not notified anymore, the owner still is
	_, err = db.GetEngine(ctx).ID(1).Cols("is_private").Update(&repo_model.Repository{IsPrivate: true})
	require.NoError(t, err)
	a.Repo = nil
	MailSecurityAdvisoryComment(ctx, reporter, a, comment)
	tos, _ = popSent()
	assert.Equal(t, []string{admin.EmailTo()}, tos)
}
