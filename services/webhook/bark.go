// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

type (
	// BarkPayload represents the payload for Bark notifications
	BarkPayload struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		URL   string `json:"url,omitempty"`
		Group string `json:"group,omitempty"`
		Sound string `json:"sound,omitempty"`
		Icon  string `json:"icon,omitempty"`
	}

	// BarkMeta contains the metadata for the webhook
	BarkMeta struct {
		Sound string `json:"sound"`
		Group string `json:"group"`
	}

	barkConvertor struct {
		Sound string
		Group string
	}
)

// GetBarkHook returns bark metadata
func GetBarkHook(w *webhook_model.Webhook) *BarkMeta {
	s := &BarkMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetBarkHook(%d): %v", w.ID, err)
	}
	return s
}

func (bc barkConvertor) getGroup(defaultGroup string) string {
	if bc.Group != "" {
		return bc.Group
	}
	return defaultGroup
}

// Create implements PayloadConvertor Create method
func (bc barkConvertor) Create(p *api.CreatePayload) (BarkPayload, error) {
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)
	body := fmt.Sprintf("%s created %s %s", p.Sender.UserName, p.RefType, refName)
	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Repo.HTMLURL + "/src/" + util.PathEscapeSegments(refName),
		Group: bc.getGroup(p.Repo.FullName),
		Sound: bc.Sound,
	}, nil
}

// Delete implements PayloadConvertor Delete method
func (bc barkConvertor) Delete(p *api.DeletePayload) (BarkPayload, error) {
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)
	body := fmt.Sprintf("%s deleted %s %s", p.Sender.UserName, p.RefType, refName)
	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Repo.HTMLURL,
		Group: bc.getGroup(p.Repo.FullName),
		Sound: bc.Sound,
	}, nil
}

// Fork implements PayloadConvertor Fork method
func (bc barkConvertor) Fork(p *api.ForkPayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] Repository forked", p.Forkee.FullName)
	body := fmt.Sprintf("%s forked %s to %s", p.Sender.UserName, p.Forkee.FullName, p.Repo.FullName)
	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Repo.HTMLURL,
		Group: bc.getGroup(p.Forkee.FullName),
		Sound: bc.Sound,
	}, nil
}

// Push implements PayloadConvertor Push method
func (bc barkConvertor) Push(p *api.PushPayload) (BarkPayload, error) {
	branchName := git.RefName(p.Ref).ShortName()

	var titleLink string
	if p.TotalCommits == 1 {
		titleLink = p.Commits[0].URL
	} else {
		titleLink = p.CompareURL
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + util.PathEscapeSegments(branchName)
	}

	title := fmt.Sprintf("[%s:%s] %d new commit(s)", p.Repo.FullName, branchName, p.TotalCommits)

	var body strings.Builder
	body.WriteString(fmt.Sprintf("%s pushed to %s\n", p.Pusher.UserName, branchName))
	for i, commit := range p.Commits {
		body.WriteString(fmt.Sprintf("%s: %s", commit.ID[:7], strings.TrimRight(commit.Message, "\r\n")))
		if commit.Author != nil {
			body.WriteString(" - " + commit.Author.Name)
		}
		if i < len(p.Commits)-1 {
			body.WriteString("\n")
		}
	}

	return BarkPayload{
		Title: title,
		Body:  body.String(),
		URL:   titleLink,
		Group: bc.getGroup(p.Repo.FullName),
		Sound: bc.Sound,
	}, nil
}

// Issue implements PayloadConvertor Issue method
func (bc barkConvertor) Issue(p *api.IssuePayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] Issue #%d: %s", p.Repository.FullName, p.Index, p.Action)
	body := fmt.Sprintf("%s %s issue #%d: %s", p.Sender.UserName, p.Action, p.Index, p.Issue.Title)

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Issue.HTMLURL,
		Group: bc.getGroup(p.Repository.FullName),
		Sound: bc.Sound,
	}, nil
}

// Wiki implements PayloadConvertor Wiki method
func (bc barkConvertor) Wiki(p *api.WikiPayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] Wiki %s", p.Repository.FullName, p.Action)
	body := fmt.Sprintf("%s %s wiki page: %s", p.Sender.UserName, p.Action, p.Page)
	wikiURL := p.Repository.HTMLURL + "/wiki/" + url.PathEscape(p.Page)

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   wikiURL,
		Group: bc.getGroup(p.Repository.FullName),
		Sound: bc.Sound,
	}, nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (bc barkConvertor) IssueComment(p *api.IssueCommentPayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] New comment on #%d", p.Repository.FullName, p.Issue.Index)
	body := fmt.Sprintf("%s commented on issue #%d: %s\n%s",
		p.Sender.UserName, p.Issue.Index, p.Issue.Title,
		truncateString(p.Comment.Body, 100))

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Comment.HTMLURL,
		Group: bc.getGroup(p.Repository.FullName),
		Sound: bc.Sound,
	}, nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (bc barkConvertor) PullRequest(p *api.PullRequestPayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] PR #%d: %s", p.Repository.FullName, p.Index, p.Action)
	body := fmt.Sprintf("%s %s pull request #%d: %s",
		p.Sender.UserName, p.Action, p.Index, p.PullRequest.Title)

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.PullRequest.HTMLURL,
		Group: bc.getGroup(p.Repository.FullName),
		Sound: bc.Sound,
	}, nil
}

// Review implements PayloadConvertor Review method
func (bc barkConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (BarkPayload, error) {
	var action string
	switch p.Action {
	case api.HookIssueReviewed:
		var err error
		action, err = parseHookPullRequestEventType(event)
		if err != nil {
			return BarkPayload{}, err
		}
	}

	title := fmt.Sprintf("[%s] PR #%d review %s", p.Repository.FullName, p.Index, action)
	body := fmt.Sprintf("PR #%d: %s", p.Index, p.PullRequest.Title)
	if p.Review != nil && p.Review.Content != "" {
		body += "\n" + truncateString(p.Review.Content, 100)
	}

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.PullRequest.HTMLURL,
		Group: bc.getGroup(p.Repository.FullName),
		Sound: bc.Sound,
	}, nil
}

// Repository implements PayloadConvertor Repository method
func (bc barkConvertor) Repository(p *api.RepositoryPayload) (BarkPayload, error) {
	var title, body string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		body = p.Sender.UserName + " created repository"
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		body = p.Sender.UserName + " deleted repository"
	default:
		return BarkPayload{}, nil
	}

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Repository.HTMLURL,
		Group: bc.getGroup(p.Repository.FullName),
		Sound: bc.Sound,
	}, nil
}

// Release implements PayloadConvertor Release method
func (bc barkConvertor) Release(p *api.ReleasePayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] Release %s", p.Repository.FullName, p.Action)
	body := fmt.Sprintf("%s %s release %s", p.Sender.UserName, p.Action, p.Release.TagName)
	if p.Release.Title != "" {
		body += ": " + p.Release.Title
	}

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Release.HTMLURL,
		Group: bc.getGroup(p.Repository.FullName),
		Sound: bc.Sound,
	}, nil
}

// Package implements PayloadConvertor Package method
func (bc barkConvertor) Package(p *api.PackagePayload) (BarkPayload, error) {
	repoFullName := ""
	if p.Repository != nil {
		repoFullName = p.Repository.FullName
	}

	title := fmt.Sprintf("[%s] Package %s", repoFullName, p.Action)
	body := fmt.Sprintf("%s %s package %s:%s",
		p.Sender.UserName, p.Action, p.Package.Name, p.Package.Version)

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.Package.HTMLURL,
		Group: bc.getGroup(repoFullName),
		Sound: bc.Sound,
	}, nil
}

// Status implements PayloadConvertor Status method
func (bc barkConvertor) Status(p *api.CommitStatusPayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] Commit status: %s", p.Repo.FullName, p.State)
	body := fmt.Sprintf("Commit %s: %s", base.ShortSha(p.SHA), p.Description)
	if p.Context != "" {
		body = fmt.Sprintf("%s (%s)", body, p.Context)
	}

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.TargetURL,
		Group: bc.getGroup(p.Repo.FullName),
		Sound: bc.Sound,
	}, nil
}

// WorkflowRun implements PayloadConvertor WorkflowRun method
func (bc barkConvertor) WorkflowRun(p *api.WorkflowRunPayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] Workflow %s", p.Repo.FullName, p.WorkflowRun.Status)
	body := fmt.Sprintf("Workflow '%s' %s", p.WorkflowRun.DisplayTitle, p.WorkflowRun.Status)

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.WorkflowRun.HTMLURL,
		Group: bc.getGroup(p.Repo.FullName),
		Sound: bc.Sound,
	}, nil
}

// WorkflowJob implements PayloadConvertor WorkflowJob method
func (bc barkConvertor) WorkflowJob(p *api.WorkflowJobPayload) (BarkPayload, error) {
	title := fmt.Sprintf("[%s] Job %s", p.Repo.FullName, p.WorkflowJob.Status)
	body := fmt.Sprintf("Job '%s' %s", p.WorkflowJob.Name, p.WorkflowJob.Status)

	return BarkPayload{
		Title: title,
		Body:  body,
		URL:   p.WorkflowJob.HTMLURL,
		Group: bc.getGroup(p.Repo.FullName),
		Sound: bc.Sound,
	}, nil
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func newBarkRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	meta := &BarkMeta{}
	if err := json.Unmarshal([]byte(w.Meta), meta); err != nil {
		return nil, nil, fmt.Errorf("newBarkRequest meta json: %w", err)
	}
	var pc payloadConvertor[BarkPayload] = barkConvertor{
		Sound: meta.Sound,
		Group: meta.Group,
	}
	return newJSONRequest(pc, w, t, true)
}

func init() {
	RegisterWebhookRequester(webhook_module.BARK, newBarkRequest)
}
