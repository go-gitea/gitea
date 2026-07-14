// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/translation"
	"gitea.dev/services/convert"
	sender_service "gitea.dev/services/mailer/sender"
)

const tplWorkflowRun templates.TplName = "mail/repo/actions/workflow_run"

type workflowRunMailJob struct {
	HTMLURL     string
	Name        string
	Status      actions_model.Status
	StatusIcon  string
	StatusClass string
	StatusText  string
	Attempt     int64
}

// mail clients strip svg, use unicode instead
func workflowRunJobStatusPresentation(status actions_model.Status) (icon, class string) {
	switch {
	case status.IsSuccess():
		return "✔", "status-success"
	case status.IsCancelled():
		return "⊘", ""
	case status.IsSkipped():
		return "–", ""
	default:
		return "×", "status-failure"
	}
}

func generateMessageIDForActionsWorkflowRunStatusEmail(repo *repo_model.Repository, run *actions_model.ActionRun) string {
	return fmt.Sprintf("<%s/actions/runs/%d@%s>", repo.FullName(), run.Index, setting.Domain)
}

func composeAndSendActionsWorkflowRunStatusEmail(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, sender *user_model.User, recipients []*user_model.User) error {
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

	displayName := fromDisplayName(sender)
	messageID := generateMessageIDForActionsWorkflowRunStatusEmail(repo, run)
	metadataHeaders := generateMetadataHeaders(repo)

	sort.SliceStable(jobs, func(i, j int) bool {
		si, sj := jobs[i].Status, jobs[j].Status
		/*
			If both i and j are/are not success, leave it to si < sj.
			If i is success and j is not, since the desired is j goes "smaller" and i goes "bigger", this func should return false.
			If j is success and i is not, since the desired is i goes "smaller" and j goes "bigger", this func should return true.
		*/
		if si.IsSuccess() != sj.IsSuccess() {
			return !si.IsSuccess()
		}
		return si < sj
	})

	// StatusText is filled per recipient language below, the rest is locale-independent
	mailJobs := make([]workflowRunMailJob, 0, len(jobs))
	for _, job := range jobs {
		converted, err := convert.ToActionWorkflowJob(ctx, repo, nil, job)
		if err != nil {
			log.Error("convert.ToActionWorkflowJob: %v", err)
			continue
		}
		icon, class := workflowRunJobStatusPresentation(job.Status)
		mailJobs = append(mailJobs, workflowRunMailJob{
			HTMLURL:     converted.HTMLURL,
			Name:        converted.Name,
			Status:      job.Status,
			StatusIcon:  icon,
			StatusClass: class,
			Attempt:     converted.RunAttempt,
		})
	}

	langMap := make(map[string][]*user_model.User)
	for _, user := range recipients {
		langMap[user.Language] = append(langMap[user.Language], user)
	}
	for lang, tos := range langMap {
		locale := translation.NewLocale(lang)
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
		for i := range mailJobs {
			mailJobs[i].StatusText = mailJobs[i].Status.LocaleString(locale)
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
		msgs := make([]*sender_service.Message, 0, len(tos))
		for _, rec := range tos {
			log.Trace("Sending actions email to %s (UID: %d)", rec.Name, rec.ID)
			msg := sender_service.NewMessageFrom(
				rec.Email,
				displayName,
				setting.MailService.FromEmail,
				subject,
				mailBody.String(),
			)
			msg.Info = subject
			for k, v := range generateSenderRecipientHeaders(sender, rec) {
				msg.SetHeader(k, v)
			}
			for k, v := range metadataHeaders {
				msg.SetHeader(k, v)
			}
			msg.SetHeader("Message-ID", messageID)
			msgs = append(msgs, msg)
		}
		SendAsync(msgs...)
	}

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
	return composeAndSendActionsWorkflowRunStatusEmail(ctx, repo, run, recipient, []*user_model.User{recipient})
}
