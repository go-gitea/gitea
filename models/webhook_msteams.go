// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/sdk/gitea"
)

type (
	// MSTeamsFact for Fact Structure
	MSTeamsFact struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	// MSTeamsSection is a MessageCard section
	MSTeamsSection struct {
		ActivityTitle    string        `json:"activityTitle"`
		ActivitySubtitle string        `json:"activitySubtitle"`
		ActivityImage    string        `json:"activityImage"`
		Facts            []MSTeamsFact `json:"facts"`
		Text             string        `json:"text"`
	}

	// MSTeamsAction is an action (creates buttons, links etc)
	MSTeamsAction struct {
		Type    string                `json:"@type"`
		Name    string                `json:"name"`
		Targets []MSTeamsActionTarget `json:"targets,omitempty"`
	}

	// MSTeamsActionTarget is the actual link to follow, etc
	MSTeamsActionTarget struct {
		Os  string `json:"os"`
		URI string `json:"uri"`
	}

	// MSTeamsPayload is the parent object
	MSTeamsPayload struct {
		Type            string           `json:"@type"`
		Context         string           `json:"@context"`
		ThemeColor      string           `json:"themeColor"`
		Title           string           `json:"title"`
		Sections        []MSTeamsSection `json:"sections"`
		PotentialAction []MSTeamsAction  `json:"potentialAction"`
	}

	// MSTeamsMeta contains the MSTeams metadata
	MSTeamsMeta struct {
		Username string `json:"username"`
		IconURL  string `json:"icon_url"`
	}
)

// SetSecret sets the MSTeams secret
func (p *MSTeamsPayload) SetSecret(_ string) {}

// JSONPayload Marshals the MSTeamsPayload to json
func (p *MSTeamsPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func getMSTeamsCreatePayload(p *api.CreatePayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", successColor),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
					MSTeamsFact{
						Name:  fmt.Sprintf("%s:", p.RefType),
						Value: refName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: p.Repo.HTMLURL + "/src/" + refName,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsDeletePayload(p *api.DeletePayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	// deleted tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", warnColor),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
					MSTeamsFact{
						Name:  fmt.Sprintf("%s:", p.RefType),
						Value: refName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: p.Repo.HTMLURL + "/src/" + refName,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsForkPayload(p *api.ForkPayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	// fork
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", successColor),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Forkee:",
						Value: p.Forkee.FullName,
					},
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: p.Repo.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsPushPayload(p *api.PushPayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

	var titleLink string
	if len(p.Commits) == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
	} else {
		commitDesc = fmt.Sprintf("%d new commits", len(p.Commits))
		titleLink = p.CompareURL
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + branchName
	}

	title := fmt.Sprintf("[%s:%s] %s", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		text += fmt.Sprintf("[%s](%s) %s - %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n"), commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", successColor),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
					MSTeamsFact{
						Name:  "Commit count:",
						Value: fmt.Sprintf("%d", len(p.Commits)),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: titleLink,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsIssuesPayload(p *api.IssuePayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	var text, title string
	var color int
	url := fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Issue.Index)
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] Issue opened: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueClosed:
		title = fmt.Sprintf("[%s] Issue closed: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		color = failedColor
		text = p.Issue.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] Issue re-opened: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] Issue edited: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueAssigned:
		title = fmt.Sprintf("[%s] Issue assigned to %s: #%d %s", p.Repository.FullName,
			p.Issue.Assignee.UserName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = successColor
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] Issue unassigned: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] Issue labels updated: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] Issue labels cleared: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] Issue synchronized: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] Issue milestone: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] Issue clear milestone: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = warnColor
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             text,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					MSTeamsFact{
						Name:  "Issue #:",
						Value: fmt.Sprintf("%d", p.Issue.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: url,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsIssueCommentPayload(p *api.IssueCommentPayload, MSTeams *MSTeamsMeta) (*MSTeamsPayload, error) {
	title := fmt.Sprintf("#%d %s", p.Issue.Index, p.Issue.Title)
	url := fmt.Sprintf("%s/issues/%d#%s", p.Repository.HTMLURL, p.Issue.Index, CommentHashTag(p.Comment.ID))
	content := ""
	var color int
	switch p.Action {
	case api.HookIssueCommentCreated:
		title = "New comment: " + title
		content = p.Comment.Body
		color = successColor
	case api.HookIssueCommentEdited:
		title = "Comment edited: " + title
		content = p.Comment.Body
		color = warnColor
	case api.HookIssueCommentDeleted:
		title = "Comment deleted: " + title
		url = fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Issue.Index)
		content = p.Comment.Body
		color = warnColor
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             content,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					MSTeamsFact{
						Name:  "Issue #:",
						Value: fmt.Sprintf("%d", p.Issue.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: url,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsPullRequestPayload(p *api.PullRequestPayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] Pull request opened: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			title = fmt.Sprintf("[%s] Pull request merged: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
			color = successColor
		} else {
			title = fmt.Sprintf("[%s] Pull request closed: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
			color = failedColor
		}
		text = p.PullRequest.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] Pull request re-opened: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] Pull request edited: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = user.UserName
		}
		title = fmt.Sprintf("[%s] Pull request assigned to %s: #%d by %s", p.Repository.FullName,
			strings.Join(list, ", "),
			p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = successColor
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] Pull request unassigned: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] Pull request labels updated: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] Pull request labels cleared: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] Pull request synchronized: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] Pull request milestone: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] Pull request clear milestone: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             text,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					MSTeamsFact{
						Name:  "Pull request #:",
						Value: fmt.Sprintf("%d", p.PullRequest.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: p.PullRequest.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsPullRequestApprovalPayload(p *api.PullRequestPayload, meta *MSTeamsMeta, event HookEventType) (*MSTeamsPayload, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             text,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					MSTeamsFact{
						Name:  "Pull request #:",
						Value: fmt.Sprintf("%d", p.PullRequest.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: p.PullRequest.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsRepositoryPayload(p *api.RepositoryPayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		color = successColor
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		color = warnColor
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: url,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsReleasePayload(p *api.ReleasePayload, meta *MSTeamsMeta) (*MSTeamsPayload, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookReleasePublished:
		title = fmt.Sprintf("[%s] Release created", p.Release.TagName)
		url = p.Release.URL
		color = successColor
	case api.HookReleaseUpdated:
		title = fmt.Sprintf("[%s] Release updated", p.Release.TagName)
		url = p.Release.URL
		color = successColor
	case api.HookReleaseDeleted:
		title = fmt.Sprintf("[%s] Release deleted", p.Release.TagName)
		url = p.Release.URL
		color = successColor
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Sections: []MSTeamsSection{
			MSTeamsSection{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             p.Release.Note,
				Facts: []MSTeamsFact{
					MSTeamsFact{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					MSTeamsFact{
						Name:  "Tag:",
						Value: p.Release.TagName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			MSTeamsAction{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					MSTeamsActionTarget{
						Os:  "default",
						URI: url,
					},
				},
			},
		},
	}, nil
}

// GetMSTeamsPayload converts a MSTeams webhook into a MSTeamsPayload
func GetMSTeamsPayload(p api.Payloader, event HookEventType, meta string) (*MSTeamsPayload, error) {
	s := new(MSTeamsPayload)

	MSTeams := &MSTeamsMeta{}
	if err := json.Unmarshal([]byte(meta), &MSTeams); err != nil {
		return s, errors.New("GetMSTeamsPayload meta json:" + err.Error())
	}

	switch event {
	case HookEventCreate:
		return getMSTeamsCreatePayload(p.(*api.CreatePayload), MSTeams)
	case HookEventDelete:
		return getMSTeamsDeletePayload(p.(*api.DeletePayload), MSTeams)
	case HookEventFork:
		return getMSTeamsForkPayload(p.(*api.ForkPayload), MSTeams)
	case HookEventIssues:
		return getMSTeamsIssuesPayload(p.(*api.IssuePayload), MSTeams)
	case HookEventIssueComment:
		return getMSTeamsIssueCommentPayload(p.(*api.IssueCommentPayload), MSTeams)
	case HookEventPush:
		return getMSTeamsPushPayload(p.(*api.PushPayload), MSTeams)
	case HookEventPullRequest:
		return getMSTeamsPullRequestPayload(p.(*api.PullRequestPayload), MSTeams)
	case HookEventPullRequestRejected, HookEventPullRequestApproved, HookEventPullRequestComment:
		return getMSTeamsPullRequestApprovalPayload(p.(*api.PullRequestPayload), MSTeams, event)
	case HookEventRepository:
		return getMSTeamsRepositoryPayload(p.(*api.RepositoryPayload), MSTeams)
	case HookEventRelease:
		return getMSTeamsReleasePayload(p.(*api.ReleasePayload), MSTeams)
	}

	return s, nil
}
