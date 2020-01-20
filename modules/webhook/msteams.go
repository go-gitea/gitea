// Copyright 2019 The Gitea Authors. All rights reserved.
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
		Summary         string           `json:"summary"`
		Sections        []MSTeamsSection `json:"sections"`
		PotentialAction []MSTeamsAction  `json:"potentialAction"`
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

func getMSTeamsCreatePayload(p *api.CreatePayload) (*MSTeamsPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", greenColor),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
					{
						Name:  fmt.Sprintf("%s:", p.RefType),
						Value: refName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.Repo.HTMLURL + "/src/" + refName,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsDeletePayload(p *api.DeletePayload) (*MSTeamsPayload, error) {
	// deleted tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", yellowColor),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
					{
						Name:  fmt.Sprintf("%s:", p.RefType),
						Value: refName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.Repo.HTMLURL + "/src/" + refName,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsForkPayload(p *api.ForkPayload) (*MSTeamsPayload, error) {
	// fork
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", greenColor),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					{
						Name:  "Forkee:",
						Value: p.Forkee.FullName,
					},
					{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.Repo.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsPushPayload(p *api.PushPayload) (*MSTeamsPayload, error) {
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
		ThemeColor: fmt.Sprintf("%x", greenColor),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             text,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repo.FullName,
					},
					{
						Name:  "Commit count:",
						Value: fmt.Sprintf("%d", len(p.Commits)),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: titleLink,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsIssuesPayload(p *api.IssuePayload) (*MSTeamsPayload, error) {
	text, _, attachmentText, color := getIssuesPayloadInfo(p, noneLinkFormatter, false)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      text,
		Summary:    text,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             attachmentText,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					{
						Name:  "Issue #:",
						Value: fmt.Sprintf("%d", p.Issue.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.Issue.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsIssueCommentPayload(p *api.IssueCommentPayload) (*MSTeamsPayload, error) {
	text, _, color := getIssueCommentPayloadInfo(p, noneLinkFormatter, false)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      text,
		Summary:    text,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             p.Comment.Body,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					{
						Name:  "Issue #:",
						Value: fmt.Sprintf("%d", p.Issue.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.Comment.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsPullRequestPayload(p *api.PullRequestPayload) (*MSTeamsPayload, error) {
	text, _, attachmentText, color := getPullRequestPayloadInfo(p, noneLinkFormatter, false)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      text,
		Summary:    text,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             attachmentText,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					{
						Name:  "Pull request #:",
						Value: fmt.Sprintf("%d", p.PullRequest.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.PullRequest.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsPullRequestApprovalPayload(p *api.PullRequestPayload, event models.HookEventType) (*MSTeamsPayload, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content

		switch event {
		case models.HookEventPullRequestApproved:
			color = greenColor
		case models.HookEventPullRequestRejected:
			color = redColor
		case models.HookEventPullRequestComment:
			color = greyColor
		default:
			color = yellowColor
		}
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             text,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					{
						Name:  "Pull request #:",
						Value: fmt.Sprintf("%d", p.PullRequest.ID),
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.PullRequest.HTMLURL,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsRepositoryPayload(p *api.RepositoryPayload) (*MSTeamsPayload, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		color = greenColor
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		color = yellowColor
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: url,
					},
				},
			},
		},
	}, nil
}

func getMSTeamsReleasePayload(p *api.ReleasePayload) (*MSTeamsPayload, error) {
	text, color := getReleasePayloadInfo(p, noneLinkFormatter, false)

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      text,
		Summary:    text,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    p.Sender.FullName,
				ActivitySubtitle: p.Sender.UserName,
				ActivityImage:    p.Sender.AvatarURL,
				Text:             p.Release.Note,
				Facts: []MSTeamsFact{
					{
						Name:  "Repository:",
						Value: p.Repository.FullName,
					},
					{
						Name:  "Tag:",
						Value: p.Release.TagName,
					},
				},
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: p.Release.URL,
					},
				},
			},
		},
	}, nil
}

// GetMSTeamsPayload converts a MSTeams webhook into a MSTeamsPayload
func GetMSTeamsPayload(p api.Payloader, event models.HookEventType, meta string) (*MSTeamsPayload, error) {
	s := new(MSTeamsPayload)

	switch event {
	case models.HookEventCreate:
		return getMSTeamsCreatePayload(p.(*api.CreatePayload))
	case models.HookEventDelete:
		return getMSTeamsDeletePayload(p.(*api.DeletePayload))
	case models.HookEventFork:
		return getMSTeamsForkPayload(p.(*api.ForkPayload))
	case models.HookEventIssues:
		return getMSTeamsIssuesPayload(p.(*api.IssuePayload))
	case models.HookEventIssueComment:
		return getMSTeamsIssueCommentPayload(p.(*api.IssueCommentPayload))
	case models.HookEventPush:
		return getMSTeamsPushPayload(p.(*api.PushPayload))
	case models.HookEventPullRequest:
		return getMSTeamsPullRequestPayload(p.(*api.PullRequestPayload))
	case models.HookEventPullRequestRejected, models.HookEventPullRequestApproved, models.HookEventPullRequestComment:
		return getMSTeamsPullRequestApprovalPayload(p.(*api.PullRequestPayload), event)
	case models.HookEventRepository:
		return getMSTeamsRepositoryPayload(p.(*api.RepositoryPayload))
	case models.HookEventRelease:
		return getMSTeamsReleasePayload(p.(*api.ReleasePayload))
	}

	return s, nil
}
