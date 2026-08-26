// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/translation"
	sender_service "gitea.dev/services/mailer/sender"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomail "github.com/wneessen/go-mail"
)

func TestWorkflowRunMail(t *testing.T) {
	t.Run("StatusPresentation", func(t *testing.T) {
		testCases := []struct {
			status actions_model.Status
			icon   string
			class  string
		}{
			{actions_model.StatusSuccess, "status-success.png", "status-success"},
			{actions_model.StatusFailure, "status-failure.png", "status-failure"},
			{actions_model.StatusCancelled, "status-cancelled.png", "status-neutral"},
			{actions_model.StatusSkipped, "status-skipped.png", "status-neutral"},
		}
		for _, testCase := range testCases {
			t.Run(testCase.status.String(), func(t *testing.T) {
				icon, class := workflowRunJobStatusPresentation(testCase.status)
				assert.Equal(t, testCase.icon, icon)
				assert.Equal(t, testCase.class, class)
				content, err := LoadMailIcon(icon)
				require.NoError(t, err)
				config, err := png.DecodeConfig(bytes.NewReader(content))
				require.NoError(t, err)
				assert.Equal(t, 48, config.Width)
				assert.Equal(t, 48, config.Height)
			})
		}
	})

	t.Run("Compose", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Langs, []string{"en-US"})()
		defer test.MockVariableValue(&setting.Names, []string{"English"})()
		translation.InitLocales(t.Context())
		require.NoError(t, unittest.PrepareTestDatabase())
		defer test.MockVariableValue(&setting.MailService, &setting.Mailer{FromEmail: "gitea@localhost"})()
		defer test.MockVariableValue(&setting.Domain, "localhost")()
		defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/")()

		recipient := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 795})
		run.Repo = repo
		var messages []*sender_service.Message
		defer test.MockVariableValue(&SendAsync, func(sent ...*sender_service.Message) {
			messages = append(messages, sent...)
		})()

		require.NoError(t, composeAndSendActionsWorkflowRunStatusEmail(t.Context(), repo, run, recipient))
		require.Len(t, messages, 1)
		message := messages[0]
		assert.Equal(t, "[user2/repo2] Failure: test.yaml (test - c2d72f5484)", message.Subject)
		assert.Equal(t, []string{"<user2/repo2/actions/runs/191@localhost>"}, message.Headers["Message-ID"])
		assert.NotContains(t, message.Body, "Some jobs were not successful")
		require.Contains(t, message.Body, ">job_1</a>")
		require.Contains(t, message.Body, ">job_2</a>")
		assert.Less(t, strings.Index(message.Body, ">job_2</a>"), strings.Index(message.Body, ">job_1</a>"))
		assert.Contains(t, message.Body, `<a class="status-success" href="http://localhost:3000/user2/repo2/actions/runs/795/jobs/198"`)
		assert.Contains(t, message.Body, `<a class="status-failure" href="http://localhost:3000/user2/repo2/actions/runs/795/jobs/199"`)
		assert.Equal(t, 1, strings.Count(message.Body, `<tr class="job-row">`))
		assert.Equal(t, 2, strings.Count(message.Body, "1m38s"))
		assert.Equal(t, 2, strings.Count(message.Body, `width="18" height="18"`))

		mailMessage := message.ToMessage()
		require.Len(t, mailMessage.GetParts(), 2)
		plainBody, err := mailMessage.GetParts()[0].GetContent()
		require.NoError(t, err)
		assert.Contains(t, string(plainBody), "Failure: job_2")
		assert.Contains(t, string(plainBody), "Success: job_1")

		var serialized bytes.Buffer
		_, err = mailMessage.WriteTo(&serialized)
		require.NoError(t, err)
		parsed, err := gomail.EMLToMsgFromReader(&serialized)
		require.NoError(t, err)
		require.Len(t, parsed.GetEmbeds(), 2)
		contentIDs := make([]string, 0, len(parsed.GetEmbeds()))
		for _, embed := range parsed.GetEmbeds() {
			contentID := embed.Header.Get(gomail.HeaderContentID.String())
			contentIDs = append(contentIDs, contentID)
			assert.Contains(t, message.Body, "cid:"+strings.Trim(contentID, "<>"))
		}
		assert.ElementsMatch(t, []string{
			"<status-failure.png.actions-run-795@localhost>",
			"<status-success.png.actions-run-795@localhost>",
		}, contentIDs)

		var emptyBody bytes.Buffer
		require.NoError(t, LoadedTemplates().BodyTemplates.ExecuteTemplate(&emptyBody, string(tplWorkflowRun), map[string]any{
			"Run":    run,
			"Jobs":   nil,
			"locale": translation.NewLocale("en-US"),
		}))
		assert.Contains(t, emptyBody.String(), `href="http://localhost:3000/user2/repo2/actions/runs/795">test.yaml</a>`)

		defer mockMailTemplates(string(tplWorkflowRun), "", `{{.Repo.FullName}}|{{.RunStatusText}}|{{range .Jobs}}{{.Status}} {{end}}`)()
		messages = nil
		require.NoError(t, composeAndSendActionsWorkflowRunStatusEmail(t.Context(), repo, run, recipient))
		require.Len(t, messages, 1)
		assert.Equal(t, "user2/repo2|Some jobs were not successful|failure success ", messages[0].Body)
	})
}
