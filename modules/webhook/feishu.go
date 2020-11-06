// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

type (
	// FeishuPayload represents
	FeishuPayload struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}
)

// SetSecret sets the Feishu secret
func (f *FeishuPayload) SetSecret(_ string) {}

// JSONPayload Marshals the FeishuPayload to json
func (f *FeishuPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

var (
	_ PayloadConvertor = &FeishuPayload{}
)

// Create implements PayloadConvertor Create method
func (f *FeishuPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return &FeishuPayload{
		Text:  title,
		Title: title,
	}, nil
}

// Delete implements PayloadConvertor Delete method
func (f *FeishuPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return &FeishuPayload{
		Text:  title,
		Title: title,
	}, nil
}

// Fork implements PayloadConvertor Fork method
func (f *FeishuPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return &FeishuPayload{
		Text:  title,
		Title: title,
	}, nil
}

// Push implements PayloadConvertor Push method
func (f *FeishuPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

	title := fmt.Sprintf("[%s:%s] %s", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = " - " + commit.Author.Name
		}
		text += fmt.Sprintf("[%s](%s) %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n")) + authorName
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}

	return &FeishuPayload{
		Text:  text,
		Title: title,
	}, nil
}

// Issue implements PayloadConvertor Issue method
func (f *FeishuPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, _ := getIssuesPayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text + "\r\n\r\n" + attachmentText,
		Title: issueTitle,
	}, nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (f *FeishuPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	text, issueTitle, _ := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text + "\r\n\r\n" + p.Comment.Body,
		Title: issueTitle,
	}, nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (f *FeishuPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, _ := getPullRequestPayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text + "\r\n\r\n" + attachmentText,
		Title: issueTitle,
	}, nil
}

// Review implements PayloadConvertor Review method
func (f *FeishuPayload) Review(p *api.PullRequestPayload, event models.HookEventType) (api.Payloader, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content

	}

	return &FeishuPayload{
		Text:  title + "\r\n\r\n" + text,
		Title: title,
	}, nil
}

// Repository implements PayloadConvertor Repository method
func (f *FeishuPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	var title string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return &FeishuPayload{
			Text:  title,
			Title: title,
		}, nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return &FeishuPayload{
			Title: title,
			Text:  title,
		}, nil
	}

	return nil, nil
}

// Release implements PayloadConvertor Release method
func (f *FeishuPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text,
		Title: text,
	}, nil
}

// GetFeishuPayload converts a ding talk webhook into a FeishuPayload
func GetFeishuPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error) {
	return convertPayloader(new(FeishuPayload), p, event)
}
