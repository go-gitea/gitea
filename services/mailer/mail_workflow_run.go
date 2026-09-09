// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"cmp"
	"context"
	"embed"
	"fmt"
	"slices"
	"time"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	sender_service "gitea.dev/services/mailer/sender"
)

const tplWorkflowRun templates.TplName = "mail/repo/actions/workflow_run"

//go:embed icons/*.png
var iconsFS embed.FS

// LoadMailIcon returns an embedded mail icon.
func LoadMailIcon(name string) ([]byte, error) {
	return iconsFS.ReadFile("icons/" + name)
}

type workflowRunMailJob struct {
	HTMLURL       string
	Name          string
	Status        actions_model.Status
	StatusIconCID string
	StatusIconAlt string
	StatusClass   string
	Attempt       int64
	Duration      time.Duration
}

func workflowRunJobStatusPresentation(status actions_model.Status) (icon, class string) {
	switch {
	case status.IsSuccess():
		return "status-success.png", "status-success"
	case status.IsCancelled():
		return "status-cancelled.png", "status-neutral"
	case status.IsSkipped():
		return "status-skipped.png", "status-neutral"
	default:
		return "status-failure.png", "status-failure"
	}
}

func composeAndSendActionsWorkflowRunStatusEmail(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, recipient *user_model.User) error {
	jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, repo.ID, run.ID)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if !job.Status.IsDone() {
			log.Debug("composeAndSendActionsWorkflowRunStatusEmail: A job is not done. Will not compose and send actions email.")
			return nil
		}
	}

	locale := translation.NewLocale(recipient.Language)

	slices.SortStableFunc(jobs, func(a, b *actions_model.ActionRunJob) int {
		if a.Status.IsSuccess() != b.Status.IsSuccess() {
			return util.Iif(a.Status.IsSuccess(), 1, -1)
		}
		return cmp.Compare(a.Status, b.Status)
	})

	mailJobs := make([]workflowRunMailJob, 0, len(jobs))
	var embeds []sender_service.EmbeddedFile
	embedded := make(container.Set[string])
	for _, job := range jobs {
		icon, class := workflowRunJobStatusPresentation(job.Status)
		contentID := fmt.Sprintf("%s.actions-run-%d@%s", icon, run.ID, setting.Domain)
		mailJobs = append(mailJobs, workflowRunMailJob{
			HTMLURL:       fmt.Sprintf("%s/actions/runs/%d/jobs/%d", repo.HTMLURL(ctx), run.ID, job.ID),
			Name:          job.Name,
			Status:        job.Status,
			StatusIconCID: contentID,
			StatusIconAlt: job.Status.LocaleString(locale),
			StatusClass:   class,
			Attempt:       job.Attempt,
			Duration:      job.Duration(),
		})
		if !embedded.Add(icon) {
			continue
		}
		content, err := LoadMailIcon(icon)
		if err != nil {
			return err
		}
		embeds = append(embeds, sender_service.EmbeddedFile{Name: icon, ContentID: contentID, Content: content})
	}

	var runStatusTrString string
	switch run.Status {
	case actions_model.StatusSuccess:
		runStatusTrString = "mail.repo.actions.jobs.all_succeeded"
	case actions_model.StatusFailure:
		runStatusTrString = "mail.repo.actions.jobs.all_failed"
		for _, job := range jobs {
			if !job.Status.IsFailure() {
				runStatusTrString = "mail.repo.actions.jobs.some_not_successful"
				break
			}
		}
	case actions_model.StatusCancelled:
		runStatusTrString = "mail.repo.actions.jobs.all_cancelled"
	}
	subject := fmt.Sprintf("[%s] %s: %s (%s - %s)", repo.FullName(), run.Status.LocaleString(locale), run.WorkflowID, run.PrettyRef(), base.ShortSha(run.CommitSHA))
	var mailBody bytes.Buffer
	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, string(tplWorkflowRun), map[string]any{
		"Subject":       subject,
		"Repo":          repo,
		"Run":           run,
		"RunStatusText": locale.TrString(runStatusTrString),
		"Jobs":          mailJobs,
		"locale":        locale,
	}); err != nil {
		return err
	}
	log.Trace("Sending actions email to %s (UID: %d)", recipient.Name, recipient.ID)
	msg := sender_service.NewMessageFrom(recipient.Email, fromDisplayName(recipient), setting.MailService.FromEmail, subject, mailBody.String())
	msg.Info = subject
	msg.Embeds = embeds
	for key, value := range generateSenderRecipientHeaders(recipient, recipient) {
		msg.SetHeader(key, value)
	}
	for key, value := range generateMetadataHeaders(repo) {
		msg.SetHeader(key, value)
	}
	msg.SetHeader("Message-ID", fmt.Sprintf("<%s/actions/runs/%d@%s>", repo.FullName(), run.Index, setting.Domain))
	SendAsync(msg)

	return nil
}

func MailActionsTrigger(ctx context.Context, recipient *user_model.User, repo *repo_model.Repository, run *actions_model.ActionRun) error {
	if setting.MailService == nil {
		return nil
	}
	if !run.Status.IsDone() || run.Status.IsSkipped() {
		return nil
	}
	if !recipient.IsMailable() {
		return nil
	}

	notifyPref, err := user_model.GetUserSetting(ctx, recipient.ID,
		user_model.SettingsKeyEmailNotificationGiteaActions, user_model.SettingEmailNotificationGiteaActionsFailureOnly)
	if err != nil {
		return err
	}
	// "disabled" never sends
	if notifyPref == user_model.SettingEmailNotificationGiteaActionsDisabled {
		return nil
	}
	// "failure-only" skips non-failure runs
	if notifyPref != user_model.SettingEmailNotificationGiteaActionsAll && !run.Status.IsFailure() {
		return nil
	}

	log.Debug("MailActionsTrigger: Initiate email composition")
	return composeAndSendActionsWorkflowRunStatusEmail(ctx, repo, run, recipient)
}
