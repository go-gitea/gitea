// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
)

type (
	// FeishuPayload represents the payload for Feishu direct message content.
	FeishuPayload struct {
		MsgType string `json:"msg_type"` // text / post / image / share_chat / interactive / file /audio / media
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	}
)

// FeishuMeta contains the feishu webhook metadata for self-built app usage
type FeishuMeta struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// GetFeishuHook returns feishu metadata
func GetFeishuHook(w *webhook_model.Webhook) *FeishuMeta {
	m := &FeishuMeta{}
	if err := json.Unmarshal([]byte(w.Meta), m); err != nil {
		log.Error("webhook.GetFeishuHook(%d): %v", w.ID, err)
	}
	return m
}

func newFeishuTextPayload(text string) FeishuPayload {
	return FeishuPayload{
		MsgType: "text",
		Content: struct {
			Text string `json:"text"`
		}{
			Text: strings.TrimSpace(text),
		},
	}
}

type feishuConvertor struct{}

// Create implements PayloadConvertor Create method
func (fc feishuConvertor) Create(p *api.CreatePayload) (FeishuPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	text := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Delete implements PayloadConvertor Delete method
func (fc feishuConvertor) Delete(p *api.DeletePayload) (FeishuPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	text := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Fork implements PayloadConvertor Fork method
func (fc feishuConvertor) Fork(p *api.ForkPayload) (FeishuPayload, error) {
	text := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return newFeishuTextPayload(text), nil
}

// Push implements PayloadConvertor Push method
func (fc feishuConvertor) Push(p *api.PushPayload) (FeishuPayload, error) {
	var (
		branchName = git.RefName(p.Ref).ShortName()
		commitDesc string
	)

	var text strings.Builder
	fmt.Fprintf(&text, "[%s:%s] %s\r\n", p.Repo.FullName, branchName, commitDesc)
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = " - " + commit.Author.Name
		}
		text.WriteString(fmt.Sprintf("[%s](%s) %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n")) + authorName)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text.WriteString("\r\n")
		}
	}

	return newFeishuTextPayload(text.String()), nil
}

// Issue implements PayloadConvertor Issue method
func (fc feishuConvertor) Issue(p *api.IssuePayload) (FeishuPayload, error) {
	title, link, by, operator, result, assignees := getIssuesInfo(p)
	if assignees != "" {
		if p.Action == api.HookIssueAssigned || p.Action == api.HookIssueUnassigned || p.Action == api.HookIssueMilestoned {
			return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, result, assignees, p.Issue.Body)), nil
		}
		return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, assignees, p.Issue.Body)), nil
	}
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.Issue.Body)), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (fc feishuConvertor) IssueComment(p *api.IssueCommentPayload) (FeishuPayload, error) {
	title, link, by, operator := getIssuesCommentInfo(p)
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.Comment.Body)), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (fc feishuConvertor) PullRequest(p *api.PullRequestPayload) (FeishuPayload, error) {
	title, link, by, operator, result, assignees := getPullRequestInfo(p)
	if assignees != "" {
		if p.Action == api.HookIssueAssigned || p.Action == api.HookIssueUnassigned || p.Action == api.HookIssueMilestoned {
			return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, result, assignees, p.PullRequest.Body)), nil
		}
		return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, assignees, p.PullRequest.Body)), nil
	}
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.PullRequest.Body)), nil
}

// Review implements PayloadConvertor Review method
func (fc feishuConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (FeishuPayload, error) {
	action, err := parseHookPullRequestEventType(event)
	if err != nil {
		return FeishuPayload{}, err
	}

	title := fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
	text := p.Review.Content

	return newFeishuTextPayload(title + "\r\n\r\n" + text), nil
}

// Repository implements PayloadConvertor Repository method
func (fc feishuConvertor) Repository(p *api.RepositoryPayload) (FeishuPayload, error) {
	var text string
	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return newFeishuTextPayload(text), nil
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return newFeishuTextPayload(text), nil
	}

	return FeishuPayload{}, nil
}

// Wiki implements PayloadConvertor Wiki method
func (fc feishuConvertor) Wiki(p *api.WikiPayload) (FeishuPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

// Release implements PayloadConvertor Release method
func (fc feishuConvertor) Release(p *api.ReleasePayload) (FeishuPayload, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func (fc feishuConvertor) Package(p *api.PackagePayload) (FeishuPayload, error) {
	text, _ := getPackagePayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func (fc feishuConvertor) Status(p *api.CommitStatusPayload) (FeishuPayload, error) {
	text, _ := getStatusPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func (feishuConvertor) WorkflowRun(p *api.WorkflowRunPayload) (FeishuPayload, error) {
	text, _ := getWorkflowRunPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func (feishuConvertor) WorkflowJob(p *api.WorkflowJobPayload) (FeishuPayload, error) {
	text, _ := getWorkflowJobPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func newFeishuRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	payload, err := newPayload(feishuConvertor{}, []byte(t.PayloadContent), t.EventType)
	if err != nil {
		return nil, nil, err
	}
	text := payload.Content.Text

	// The recipient emails are stored directly on the HookTask by the notifier,
	// not embedded in the webhook payload JSON. This keeps the webhook API payload
	// clean and avoids adding a Feishu-specific field to the cross-cutting structs.
	recipients := t.DeliveryRecipients
	meta := GetFeishuHook(w)
	if meta.AppID == "" || meta.AppSecret == "" {
		return nil, nil, fmt.Errorf("feishu app credentials (app_id/app_secret) not configured")
	}

	// The webhook URL can be used to override the default API base URL (e.g.
	// https://open.larksuite.com for Lark Suite).
	baseURL := feishuBaseURLFromWebhook(w)

	// Deduplicate recipient emails while keeping their order. Build a fresh
	// slice so we never mutate the HookTask's stored DeliveryRecipients.
	seen := make(map[string]struct{})
	recipients = slices.DeleteFunc(slices.Clone(recipients), func(r string) bool {
		if r == "" {
			return true
		}
		if _, ok := seen[r]; ok {
			return true
		}
		seen[r] = struct{}{}
		return false
	})

	// No recipients: there is no one to notify via direct message. Validate
	// the app credentials against the token endpoint so the framework still
	// records a successful delivery (and surfaces misconfigured credentials).
	if len(recipients) == 0 {
		return newFeishuNoopRequest(ctx, baseURL, meta.AppID, meta.AppSecret)
	}

	// Obtain a tenant access token, reusing a cached one when available.
	token, err := feishuGetAccessTokenFunc(ctx, baseURL, meta.AppID, meta.AppSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("feishu get tenant_access_token: %w", err)
	}

	// Deliver direct messages to every recipient except the first one via
	// the Feishu Open API. This runs synchronously so failures are logged and
	// observable instead of being silently dropped.
	for _, email := range recipients[1:] {
		if err := feishuSendMessageFunc(ctx, baseURL, token, email, text); err != nil {
			log.Error("feishu send direct message to %s: %v", email, err)
		}
	}

	// The first recipient is delivered by the framework request itself, so
	// its delivery status is recorded and visible to the user.

	contentBytes, _ := json.Marshal(map[string]string{"text": text})
	contentStr := string(contentBytes)
	body := map[string]string{
		"receive_id": recipients[0],
		"msg_type":   "text",
		"content":    contentStr,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/open-apis/im/v1/messages?receive_id_type=email", bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return req, b, nil
}

func init() {
	RegisterWebhookRequester(webhook_module.FEISHU, newFeishuRequest)
}
