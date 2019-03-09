// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func assertMilestoneEqual(t *testing.T, title, dueOn, state string, ms *base.Milestone) {
	var tmPtr *time.Time
	if dueOn != "" {
		tm, err := time.Parse("2006-01-02 15:04:05 -0700 MST", dueOn)
		assert.NoError(t, err)
		tmPtr = &tm
	}

	assert.EqualValues(t, &base.Milestone{
		Title:    title,
		Deadline: tmPtr,
		State:    state,
	}, ms)
}

func assertLabelEqual(t *testing.T, name, color string, label *base.Label) {
	assert.EqualValues(t, &base.Label{
		Name:  name,
		Color: color,
	}, label)
}

func TestGitHubDownloadRepo(t *testing.T) {
	downloader := NewGithubDownloaderV3("", "go-gitea", "gitea")
	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:        "gitea",
		Owner:       "go-gitea",
		Description: "Git with a cup of tea, painless self-hosted git service",
		CloneURL:    "https://github.com/go-gitea/gitea.git",
	}, repo)

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	// before this tool release, we have 39 milestones on github.com/go-gitea/gitea
	assert.True(t, len(milestones) >= 39)

	for _, milestone := range milestones {
		switch milestone.Title {
		case "1.0.0":
			assertMilestoneEqual(t, "1.0.0", "2016-12-23 08:00:00 +0000 UTC", "closed", milestone)
		case "1.1.0":
			assertMilestoneEqual(t, "1.1.0", "2017-02-24 08:00:00 +0000 UTC", "closed", milestone)
		case "1.2.0":
			assertMilestoneEqual(t, "1.2.0", "2017-04-24 07:00:00 +0000 UTC", "closed", milestone)
		case "1.3.0":
			assertMilestoneEqual(t, "1.3.0", "2017-11-29 08:00:00 +0000 UTC", "closed", milestone)
		case "1.4.0":
			assertMilestoneEqual(t, "1.4.0", "2018-01-25 08:00:00 +0000 UTC", "closed", milestone)
		case "1.5.0":
			assertMilestoneEqual(t, "1.5.0", "2018-06-15 07:00:00 +0000 UTC", "closed", milestone)
		case "1.6.0":
			assertMilestoneEqual(t, "1.6.0", "2018-09-25 07:00:00 +0000 UTC", "closed", milestone)
		case "1.7.0":
			assertMilestoneEqual(t, "1.7.0", "2018-12-25 08:00:00 +0000 UTC", "closed", milestone)
		case "1.x.x":
			assertMilestoneEqual(t, "1.x.x", "", "open", milestone)
		}
	}

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.True(t, len(labels) >= 48)
	for _, l := range labels {
		switch l.Name {
		case "backport/v1.7":
			assertLabelEqual(t, "backport/v1.7", "fbca04", l)
		case "backport/v1.8":
			assertLabelEqual(t, "backport/v1.8", "fbca04", l)
		case "kind/api":
			assertLabelEqual(t, "kind/api", "5319e7", l)
		case "kind/breaking":
			assertLabelEqual(t, "kind/breaking", "fbca04", l)
		case "kind/bug":
			assertLabelEqual(t, "kind/bug", "ee0701", l)
		case "kind/docs":
			assertLabelEqual(t, "kind/docs", "c2e0c6", l)
		case "kind/enhancement":
			assertLabelEqual(t, "kind/enhancement", "84b6eb", l)
		case "kind/feature":
			assertLabelEqual(t, "kind/feature", "006b75", l)
		}
	}

	// downloader.GetIssues()
	issues, err := downloader.GetIssues(0, 3)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(issues))
	assert.EqualValues(t, []*base.Issue{
		{
			Number:     6,
			Title:      "Contribution system: History heatmap for user",
			Content:    "Hi guys,\r\n\r\nI think that is a possible feature, a history heatmap similar to github or gitlab.\r\nActually exists a plugin called Calendar HeatMap. I used this on mine project to heat application log and worked fine here.\r\nThen, is only a idea, what you think? :)\r\n\r\nhttp://cal-heatmap.com/\r\nhttps://github.com/wa0x6e/cal-heatmap\r\n\r\nReference: https://github.com/gogits/gogs/issues/1640",
			Milestone:  "1.7.0",
			PosterName: "joubertredrat",
			State:      "closed",
			Created:    time.Date(2016, 11, 02, 18, 51, 55, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "kind/feature",
					Color: "006b75",
				},
				{
					Name:  "kind/ui",
					Color: "fef2c0",
				},
			},
			Reactions: &base.Reactions{
				TotalCount: 0,
				PlusOne:    0,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      0,
				Hooray:     0,
			},
		},
		{
			Number:     7,
			Title:      "display page revisions on wiki",
			Content:    "Hi guys,\r\n\r\nWiki on Gogs is very fine, I liked a lot, but I think that is good idea to be possible see other revisions from page as a page history.\r\n\r\nWhat you think?\r\n\r\nReference: https://github.com/gogits/gogs/issues/2991",
			Milestone:  "1.x.x",
			PosterName: "joubertredrat",
			State:      "open",
			Created:    time.Date(2016, 11, 02, 18, 57, 32, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "kind/feature",
					Color: "006b75",
				},
				{
					Name:        "reviewed/confirmed",
					Color:       "8d9b12",
					Description: "Issue has been reviewed and confirmed to be present or accepted to be implemented",
				},
			},
			Reactions: &base.Reactions{
				TotalCount: 5,
				PlusOne:    4,
				MinusOne:   0,
				Laugh:      0,
				Confused:   1,
				Heart:      0,
				Hooray:     0,
			},
		},
		{
			Number:     8,
			Title:      "audit logs",
			Content:    "Hi,\r\n\r\nI think that is good idea to have user operation log to admin see what the user is doing at Gogs. Similar to example below\r\n\r\n| user | operation | information |\r\n| --- | --- | --- |\r\n| joubertredrat | repo.create | Create repo MyProjectData |\r\n| joubertredrat | user.settings | Edit settings |\r\n| tboerger | repo.fork | Create Fork from MyProjectData to ForkMyProjectData |\r\n| bkcsoft | repo.remove | Remove repo MySource |\r\n| tboerger | admin.auth | Edit auth LDAP org-connection |\r\n\r\nThis resource can be used on user page too, as user activity, set that log row is public (repo._) or private (user._, admin.*) and display only public activity.\r\n\r\nWhat you think?\r\n\r\n[Chat summary from March 14, 2017](https://github.com/go-gitea/gitea/issues/8#issuecomment-286463807)\r\n\r\nReferences:\r\nhttps://github.com/gogits/gogs/issues/3016",
			Milestone:  "1.x.x",
			PosterName: "joubertredrat",
			State:      "open",
			Created:    time.Date(2016, 11, 02, 18, 59, 20, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "kind/feature",
					Color: "006b75",
				},
				{
					Name:  "kind/proposal",
					Color: "5319e7",
				},
			},
			Reactions: &base.Reactions{
				TotalCount: 9,
				PlusOne:    8,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      1,
				Hooray:     0,
			},
		},
	}, issues)

	// downloader.GetComments()
	comments, err := downloader.GetComments(6)
	assert.NoError(t, err)
	assert.EqualValues(t, 35, len(comments))
	assert.EqualValues(t, []*base.Comment{
		{
			PosterName: "bkcsoft",
			Created:    time.Date(2016, 11, 02, 18, 59, 48, 0, time.UTC),
			Content: `I would prefer a solution that is in the backend, unless it's required to have it update without reloading. Unfortunately I can't seem to find anything that does that :unamused: 

Also this would _require_ caching, since it will fetch huge amounts of data from disk...
`,
			Reactions: &base.Reactions{
				TotalCount: 2,
				PlusOne:    2,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      0,
				Hooray:     0,
			},
		},
		{
			PosterName: "joubertredrat",
			Created:    time.Date(2016, 11, 02, 19, 16, 56, 0, time.UTC),
			Content: `Yes, this plugin build on front-end, with backend I don't know too, but we can consider make component for this.

In my case I use ajax to get data, but build on frontend anyway
`,
			Reactions: &base.Reactions{
				TotalCount: 0,
				PlusOne:    0,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      0,
				Hooray:     0,
			},
		},
		{
			PosterName: "xinity",
			Created:    time.Date(2016, 11, 03, 13, 04, 56, 0, time.UTC),
			Content: `following  @bkcsoft retention strategy in cache is a must if we don't want gitea to waste ressources.
something like in the latest 15days could be enough don't you think ?
`,
			Reactions: &base.Reactions{
				TotalCount: 2,
				PlusOne:    2,
				MinusOne:   0,
				Laugh:      0,
				Confused:   0,
				Heart:      0,
				Hooray:     0,
			},
		},
	}, comments[:3])

	// downloader.GetPullRequests()
	prs, err := downloader.GetPullRequests(0, 3)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(prs))
	assert.EqualValues(t, []*base.PullRequest{
		{
			Number:     1,
			Title:      "Rename import paths: \"github.com/gogits/gogs\" -> \"github.com/go-gitea/gitea\"",
			Content:    "",
			Milestone:  "1.0.0",
			PosterName: "andreynering",
			State:      "closed",
			Created:    time.Date(2016, 11, 02, 17, 01, 19, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "kind/enhancement",
					Color: "84b6eb",
				},
				{
					Name:  "lgtm/done",
					Color: "0e8a16",
				},
			},
			PatchURL: "https://github.com/go-gitea/gitea/pull/1.patch",
			Head: base.PullRequestBranch{
				Ref:       "import-paths",
				SHA:       "1b0ec3208db8501acba44a137c009a5a126ebaa9",
				OwnerName: "andreynering",
			},
			Base: base.PullRequestBranch{
				Ref:       "master",
				SHA:       "6bcff7828f117af8d51285ce3acba01a7e40a867",
				OwnerName: "go-gitea",
				RepoName:  "gitea",
			},
		},
		{
			Number:     2,
			Title:      "Fix sender of issue notifications",
			Content:    "It is the FROM field in mailer configuration that needs be used,\r\nnot the USER field, which is for authentication.\r\n\r\nMigrated from https://github.com/gogits/gogs/pull/3616\r\n",
			Milestone:  "1.0.0",
			PosterName: "strk",
			State:      "closed",
			Created:    time.Date(2016, 11, 02, 17, 24, 19, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "kind/bug",
					Color: "ee0701",
				},
				{
					Name:  "lgtm/done",
					Color: "0e8a16",
				},
			},
			PatchURL: "https://github.com/go-gitea/gitea/pull/2.patch",
			Head: base.PullRequestBranch{
				Ref:       "proper-from-on-issue-mail",
				SHA:       "af03d00780a6ee70c58e135c6679542cde4f8d50",
				RepoName:  "gogs",
				OwnerName: "strk",
			},
			Base: base.PullRequestBranch{
				Ref:       "develop",
				SHA:       "5c5424301443ffa3659737d12de48ab1dfe39a00",
				OwnerName: "go-gitea",
				RepoName:  "gitea",
			},
		},
		{
			Number:     3,
			Title:      "Use proper url for libravatar dep",
			Content:    "Fetch go-libravatar from its official source, rather than from an unmaintained fork\r\n",
			Milestone:  "1.0.0",
			PosterName: "strk",
			State:      "closed",
			Created:    time.Date(2016, 11, 02, 17, 34, 31, 0, time.UTC),
			Labels: []*base.Label{
				{
					Name:  "kind/enhancement",
					Color: "84b6eb",
				},
				{
					Name:  "lgtm/done",
					Color: "0e8a16",
				},
			},
			PatchURL: "https://github.com/go-gitea/gitea/pull/3.patch",
			Head: base.PullRequestBranch{
				Ref:       "libravatar-proper-url",
				SHA:       "d59a48a2550abd4129b96d38473941b895a4859b",
				RepoName:  "gogs",
				OwnerName: "strk",
			},
			Base: base.PullRequestBranch{
				Ref:       "develop",
				SHA:       "6bcff7828f117af8d51285ce3acba01a7e40a867",
				OwnerName: "go-gitea",
				RepoName:  "gitea",
			},
		},
	}, prs)
}
