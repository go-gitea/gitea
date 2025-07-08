// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/services/convert"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
)

const tplWorkflowRun = "notify/workflow_run"

type convertedWorkflowJob struct {
	HTMLURL string
	Status  actions_model.Status
	Name    string
	Attempt int64
}

func generateMessageIDForActionsWorkflowRunStatusEmail(repo *repo_model.Repository, run *actions_model.ActionRun) string {
	return fmt.Sprintf("<%s/actions/runs/%d@%s>", repo.FullName(), run.Index, setting.Domain)
}

func sendActionsWorkflowRunStatusEmail(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun, sender *user_model.User, recipients []*user_model.User) {
	subject := "Run"
	switch run.Status {
	case actions_model.StatusFailure:
		subject += " failed"
	case actions_model.StatusCancelled:
		subject += " cancelled"
	case actions_model.StatusSuccess:
		subject += " succeeded"
	}
	subject = fmt.Sprintf("%s: %s (%s)", subject, run.WorkflowID, base.ShortSha(run.CommitSHA))
	displayName := fromDisplayName(sender)
	messageID := generateMessageIDForActionsWorkflowRunStatusEmail(repo, run)
	metadataHeaders := generateMetadataHeaders(repo)

	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		log.Error("GetRunJobsByRunID: %v", err)
		return
	} else {
		sort.SliceStable(jobs, func(i, j int) bool {
			si := jobs[i].Status
			sj := jobs[j].Status
			if si == sj || si.IsSuccess() {
				return false
			}
			return si < sj
		})
	}
	convertedJobs := make([]convertedWorkflowJob, 0, len(jobs))
	for _, job := range jobs {
		converted0, err := convert.ToActionWorkflowJob(ctx, repo, nil, job)
		if err != nil {
			log.Error("convert.ToActionWorkflowJob: %v", err)
			continue
		}
		convertedJobs = append(convertedJobs, convertedWorkflowJob{
			HTMLURL: converted0.HTMLURL,
			Name:    converted0.Name,
			Status:  job.Status,
			Attempt: converted0.RunAttempt,
		})
	}

	langMap := make(map[string][]*user_model.User)
	for _, user := range recipients {
		langMap[user.Language] = append(langMap[user.Language], user)
	}
	for lang, tos := range langMap {
		locale := translation.NewLocale(lang)
		var runStatusText string
		switch run.Status {
		case actions_model.StatusSuccess:
			runStatusText = "All jobs have succeeded"
		case actions_model.StatusFailure:
			runStatusText = "All jobs have failed"
			for _, job := range jobs {
				if !job.Status.IsFailure() {
					runStatusText = "Some jobs were not successful"
					break
				}
			}
		case actions_model.StatusCancelled:
			runStatusText = "All jobs have been cancelled"
		}
		var mailBody bytes.Buffer
		if err := bodyTemplates.ExecuteTemplate(&mailBody, tplWorkflowRun, map[string]any{
			"Subject":       subject,
			"Repo":          repo,
			"Run":           run,
			"RunStatusText": runStatusText,
			"Jobs":          convertedJobs,
			"locale":        locale,
			"Language":      locale.Language(),
		}); err != nil {
			log.Error("ExecuteTemplate [%s]: %v", tplWorkflowRun, err)
			return
		}
		msgs := make([]*sender_service.Message, 0, len(tos))
		for _, rec := range tos {
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
}

func SendActionsWorkflowRunStatusEmail(ctx context.Context, sender *user_model.User, repo *repo_model.Repository, run *actions_model.ActionRun) {
	if setting.MailService == nil {
		return
	}
	if run.Status.IsSkipped() {
		return
	}

	recipients := make([]*user_model.User, 0)

	if !sender.IsGiteaActions() && !sender.IsGhost() && sender.IsMailable() {
		if run.Status.IsSuccess() {
			if sender.EmailNotificationsPreference == user_model.EmailNotificationsAndYourOwn {
				recipients = append(recipients, sender)
			}
			sendActionsWorkflowRunStatusEmail(ctx, repo, run, sender, recipients)
			return
		} else if sender.EmailNotificationsPreference != user_model.EmailNotificationsOnMention &&
			sender.EmailNotificationsPreference != user_model.EmailNotificationsDisabled {
			recipients = append(recipients, sender)
		}
	}

	watchers, err := repo_model.GetRepoWatchers(ctx, repo.ID, db.ListOptionsAll)
	if err != nil {
		log.Error("GetWatchers: %v", err)
	}
	for _, watcher := range watchers {
		if watcher.ID == sender.ID {
			continue
		}
		if watcher.IsMailable() && watcher.EmailNotificationsPreference != user_model.EmailNotificationsOnMention &&
			watcher.EmailNotificationsPreference != user_model.EmailNotificationsDisabled {
			recipients = append(recipients, watcher)
		}
	}
	sendActionsWorkflowRunStatusEmail(ctx, repo, run, sender, recipients)
}
