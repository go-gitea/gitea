// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

type (
	// GoogleChatMeta contains the Google Chat metadata.
	GoogleChatMeta struct {
		IconURL string `json:"icon_url"`
	}

	// GoogleChatPayload is the Google Chat incoming webhook message.
	GoogleChatPayload struct {
		Text    string             `json:"text,omitempty"`
		CardsV2 []GoogleChatCardV2 `json:"cardsV2,omitempty"`
	}

	// GoogleChatCardV2 is a card wrapper for Google Chat cards.
	GoogleChatCardV2 struct {
		CardID string         `json:"cardId,omitempty"`
		Card   GoogleChatCard `json:"card"`
	}

	// GoogleChatCard is a Google Chat card.
	GoogleChatCard struct {
		Header   GoogleChatCardHeader    `json:"header"`
		Sections []GoogleChatCardSection `json:"sections"`
	}

	// GoogleChatCardHeader is the card heading.
	GoogleChatCardHeader struct {
		Title    string `json:"title"`
		Subtitle string `json:"subtitle,omitempty"`
		ImageURL string `json:"imageUrl,omitempty"`
	}

	// GoogleChatCardSection is a group of card widgets.
	GoogleChatCardSection struct {
		Widgets []GoogleChatWidget `json:"widgets"`
	}

	// GoogleChatWidget is one card widget.
	GoogleChatWidget struct {
		TextParagraph *GoogleChatTextParagraph `json:"textParagraph,omitempty"`
	}

	// GoogleChatTextParagraph is a text paragraph widget.
	GoogleChatTextParagraph struct {
		Text string `json:"text"`
	}
)

// GetGoogleChatHook returns Google Chat metadata.
func GetGoogleChatHook(w *webhook_model.Webhook) *GoogleChatMeta {
	s := &GoogleChatMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetGoogleChatHook(%d): %v", w.ID, err)
	}
	return s
}

type googleChatConvertor struct {
	Name    string
	IconURL string
}

func googleChatTextFormatter(s string) string {
	return html.EscapeString(s)
}

func googleChatLinkFormatter(url, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(url), googleChatTextFormatter(text))
}

func googleChatLinkToRef(repoURL, ref string) string {
	refName := git.RefName(ref)
	return googleChatLinkFormatter(repoURL+"/src/"+refName.RefWebLinkPath(), refName.ShortName())
}

func googleChatUserLink(s *api.User) string {
	if s == nil {
		return ""
	}
	userURL := s.HTMLURL
	if userURL == "" {
		userURL = setting.AppURL + url.PathEscape(s.UserName)
	}
	return googleChatLinkFormatter(userURL, s.UserName)
}

// Create implements payloadConvertor Create method.
func (g googleChatConvertor) Create(p *api.CreatePayload) (GoogleChatPayload, error) {
	repoLink := googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	refLink := googleChatLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, googleChatUserLink(p.Sender))

	return g.createPayload(text), nil
}

// Delete implements payloadConvertor Delete method.
func (g googleChatConvertor) Delete(p *api.DeletePayload) (GoogleChatPayload, error) {
	refName := git.RefName(p.Ref).ShortName()
	repoLink := googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("[%s:%s] %s deleted by %s", repoLink, googleChatTextFormatter(refName), p.RefType, googleChatUserLink(p.Sender))

	return g.createPayload(text), nil
}

// Fork implements payloadConvertor Fork method.
func (g googleChatConvertor) Fork(p *api.ForkPayload) (GoogleChatPayload, error) {
	baseLink := googleChatLinkFormatter(p.Forkee.HTMLURL, p.Forkee.FullName)
	forkLink := googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("%s is forked to %s", baseLink, forkLink)

	return g.createPayload(text), nil
}

// Push implements payloadConvertor Push method.
func (g googleChatConvertor) Push(p *api.PushPayload) (GoogleChatPayload, error) {
	refName := git.RefName(p.Ref)
	branchName := refName.ShortName()
	commitDesc := fmt.Sprintf("%d new commits", p.TotalCommits)
	if p.TotalCommits == 1 {
		commitDesc = "1 new commit"
	}
	commitString := commitDesc
	if p.CompareURL != "" {
		commitString = googleChatLinkFormatter(p.CompareURL, commitDesc)
	}

	repoLink := googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	branchLink := googleChatLinkFormatter(p.Repo.HTMLURL+"/src/"+refName.RefWebLinkPath(), branchName)
	text := fmt.Sprintf("[%s:%s] %s pushed by %s", repoLink, branchLink, commitString, googleChatUserLink(p.Pusher))

	var commitText strings.Builder
	for i, commit := range p.Commits {
		fmt.Fprintf(&commitText, "%s: %s - %s",
			googleChatLinkFormatter(commit.URL, commit.ID[:7]),
			googleChatTextFormatter(strings.TrimRight(strings.SplitN(commit.Message, "\n", 2)[0], "\r")),
			googleChatTextFormatter(commit.Author.Name),
		)
		if i < len(p.Commits)-1 {
			commitText.WriteString("\n")
		}
	}

	return g.createPayload(text, commitText.String()), nil
}

// Issue implements payloadConvertor Issue method.
func (g googleChatConvertor) Issue(p *api.IssuePayload) (GoogleChatPayload, error) {
	text, _, extraMarkdown, _ := getIssuesPayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text, googleChatTextFormatter(extraMarkdown)), nil
}

// IssueComment implements payloadConvertor IssueComment method.
func (g googleChatConvertor) IssueComment(p *api.IssueCommentPayload) (GoogleChatPayload, error) {
	text, _, _ := getIssueCommentPayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text, googleChatTextFormatter(p.Comment.Body)), nil
}

// PullRequest implements payloadConvertor PullRequest method.
func (g googleChatConvertor) PullRequest(p *api.PullRequestPayload) (GoogleChatPayload, error) {
	text, _, extraMarkdown, _ := getPullRequestPayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text, googleChatTextFormatter(extraMarkdown)), nil
}

// Review implements payloadConvertor Review method.
func (g googleChatConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (GoogleChatPayload, error) {
	var text string
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return GoogleChatPayload{}, err
		}
		repoLink := googleChatLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
		titleLink := googleChatLinkFormatter(fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index), fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title))
		text = fmt.Sprintf("[%s] Pull request review %s: %s by %s", repoLink, action, titleLink, googleChatUserLink(p.Sender))
	}

	return g.createPayload(text, googleChatTextFormatter(p.Review.Content)), nil
}

// Repository implements payloadConvertor Repository method.
func (g googleChatConvertor) Repository(p *api.RepositoryPayload) (GoogleChatPayload, error) {
	senderLink := googleChatUserLink(p.Sender)
	repoLink := googleChatLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string
	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", repoLink, senderLink)
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", repoLink, senderLink)
	}

	return g.createPayload(text), nil
}

// Wiki implements payloadConvertor Wiki method.
func (g googleChatConvertor) Wiki(p *api.WikiPayload) (GoogleChatPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text), nil
}

// Release implements payloadConvertor Release method.
func (g googleChatConvertor) Release(p *api.ReleasePayload) (GoogleChatPayload, error) {
	text, _ := getReleasePayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text), nil
}

func (g googleChatConvertor) Package(p *api.PackagePayload) (GoogleChatPayload, error) {
	text, _ := getPackagePayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text), nil
}

func (g googleChatConvertor) Status(p *api.CommitStatusPayload) (GoogleChatPayload, error) {
	text, _ := getStatusPayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text), nil
}

func (g googleChatConvertor) WorkflowRun(p *api.WorkflowRunPayload) (GoogleChatPayload, error) {
	text, _ := getWorkflowRunPayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text), nil
}

func (g googleChatConvertor) WorkflowJob(p *api.WorkflowJobPayload) (GoogleChatPayload, error) {
	text, _ := getWorkflowJobPayloadInfo(p, googleChatLinkFormatter, true)

	return g.createPayload(text), nil
}

func (g googleChatConvertor) createPayload(text string, details ...string) GoogleChatPayload {
	name := g.Name
	if name == "" {
		name = "Gitea"
	}
	return createGoogleChatPayload(name, g.IconURL, text, details...)
}

func createGoogleChatPayload(name, iconURL, text string, details ...string) GoogleChatPayload {
	widgets := make([]GoogleChatWidget, 0, 1+len(details))
	if text != "" {
		widgets = append(widgets, GoogleChatWidget{
			TextParagraph: &GoogleChatTextParagraph{Text: text},
		})
	}
	for _, detail := range details {
		if detail == "" {
			continue
		}
		widgets = append(widgets, GoogleChatWidget{
			TextParagraph: &GoogleChatTextParagraph{Text: detail},
		})
	}

	card := GoogleChatCard{
		Header: GoogleChatCardHeader{
			Title:    html.EscapeString(name),
			Subtitle: "Gitea Webhook",
		},
		Sections: []GoogleChatCardSection{
			{Widgets: widgets},
		},
	}
	if iconURL != "" {
		card.Header.ImageURL = iconURL
	}

	return GoogleChatPayload{
		CardsV2: []GoogleChatCardV2{
			{
				CardID: "gitea-notification",
				Card:   card,
			},
		},
	}
}

func newGoogleChatRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	meta := &GoogleChatMeta{}
	if err := json.Unmarshal([]byte(w.Meta), meta); err != nil {
		return nil, nil, fmt.Errorf("newGoogleChatRequest meta json: %w", err)
	}
	var pc payloadConvertor[GoogleChatPayload] = googleChatConvertor{
		Name:    w.Name,
		IconURL: meta.IconURL,
	}
	return newJSONRequest(pc, w, t, true)
}

func init() {
	RegisterWebhookRequester(webhook_module.GOOGLECHAT, newGoogleChatRequest)
}
