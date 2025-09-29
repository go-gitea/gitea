// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/services/convert"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
)

const tplWorkflowRun templates.TplName = "repo/actions/workflow_run"

type convertedWorkflowJob struct {
	HTMLURL  string
	Name     string
	Status   actions_model.Status
	Attempt  int64
	Duration time.Duration
}

func generateMessageIDForActionsWorkflowRunStatusEmail(repo *repo_model.Repository, run *actions_model.ActionRun) string {
	return fmt.Sprintf("<%s/actions/runs/%d@%s>", repo.FullName(), run.Index, setting.Domain)
}

func composeAndSendActionsWorkflowRunStatusEmail(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, sender *user_model.User, recipients []*user_model.User) error {
	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if !job.Status.IsDone() {
			log.Debug("composeAndSendActionsWorkflowRunStatusEmail: A job is not done. Will not compose and send actions email.")
			return nil
		}
	}

	var subjectTrString string
	switch run.Status {
	case actions_model.StatusFailure:
		subjectTrString = "mail.repo.actions.run.failed"
	case actions_model.StatusCancelled:
		subjectTrString = "mail.repo.actions.run.cancelled"
	case actions_model.StatusSuccess:
		subjectTrString = "mail.repo.actions.run.succeeded"
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

	convertedJobs := make([]convertedWorkflowJob, 0, len(jobs))
	for _, job := range jobs {
		converted0, err := convert.ToActionWorkflowJob(ctx, repo, nil, job)
		if err != nil {
			log.Error("convert.ToActionWorkflowJob: %v", err)
			continue
		}
		convertedJobs = append(convertedJobs, convertedWorkflowJob{
			HTMLURL:  converted0.HTMLURL,
			Name:     converted0.Name,
			Status:   job.Status,
			Attempt:  converted0.RunAttempt,
			Duration: job.Duration(),
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
		subject := fmt.Sprintf("%s: %s (%s)", locale.TrString(subjectTrString), run.WorkflowID, base.ShortSha(run.CommitSHA))
		var mailBody bytes.Buffer
		if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, string(tplWorkflowRun), map[string]any{
			"Subject":       subject,
			"Repo":          repo,
			"Run":           run,
			"RunStatusText": locale.TrString(runStatusTrString),
			"Jobs":          convertedJobs,
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

func MailActionsTrigger(ctx context.Context, sender *user_model.User, repo *repo_model.Repository, run *actions_model.ActionRun) error {
	if setting.MailService == nil {
		return nil
	}
	if !run.Status.IsDone() || run.Status.IsSkipped() {
		return nil
	}

	recipients := make([]*user_model.User, 0)

	if !sender.IsGiteaActions() && !sender.IsGhost() && sender.IsMailable() {
		notifyPref, err := user_model.GetUserSetting(ctx, sender.ID,
			user_model.SettingsKeyEmailNotificationGiteaActions, user_model.SettingEmailNotificationGiteaActionsFailureOnly)
		if err != nil {
			return err
		}
		if notifyPref == user_model.SettingEmailNotificationGiteaActionsAll || !run.Status.IsSuccess() && notifyPref != user_model.SettingEmailNotificationGiteaActionsDisabled {
			recipients = append(recipients, sender)
		}
	}

	if len(recipients) > 0 {
		log.Debug("MailActionsTrigger: Initiate email composition")
		return composeAndSendActionsWorkflowRunStatusEmail(ctx, repo, run, sender, recipients)
	}
	return nil
}
