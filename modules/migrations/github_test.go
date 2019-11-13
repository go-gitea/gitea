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

func assertMilestoneEqual(t *testing.T, title, dueOn, created, updated, closed, state string, ms *base.Milestone) {
	var tmPtr *time.Time
	if dueOn != "" {
		tm, err := time.Parse("2006-01-02 15:04:05 -0700 MST", dueOn)
		assert.NoError(t, err)
		tmPtr = &tm
	}
	var (
		createdTM time.Time
		updatedTM *time.Time
		closedTM  *time.Time
	)
	if created != "" {
		var err error
		createdTM, err = time.Parse("2006-01-02 15:04:05 -0700 MST", created)
		assert.NoError(t, err)
	}
	if updated != "" {
		updatedTemp, err := time.Parse("2006-01-02 15:04:05 -0700 MST", updated)
		assert.NoError(t, err)
		updatedTM = &updatedTemp
	}
	if closed != "" {
		closedTemp, err := time.Parse("2006-01-02 15:04:05 -0700 MST", closed)
		assert.NoError(t, err)
		closedTM = &closedTemp
	}

	assert.EqualValues(t, &base.Milestone{
		Title:    title,
		Deadline: tmPtr,
		State:    state,
		Created:  createdTM,
		Updated:  updatedTM,
		Closed:   closedTM,
	}, ms)
}

func assertLabelEqual(t *testing.T, name, color string, label *base.Label) {
	assert.EqualValues(t, &base.Label{
		Name:  name,
		Color: color,
	}, label)
}

func TestGitHubDownloadRepo(t *testing.T) {
	downloader := NewGithubDownloaderV3("", "", "go-gitea", "gitea")
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
			assertMilestoneEqual(t, "1.0.0", "2016-12-23 08:00:00 +0000 UTC",
				"2016-11-02 18:06:55 +0000 UTC",
				"2016-12-29 10:26:00 +0000 UTC",
				"2016-12-24 00:40:56 +0000 UTC",
				"closed", milestone)
		case "1.1.0":
			assertMilestoneEqual(t, "1.1.0", "2017-02-24 08:00:00 +0000 UTC",
				"2016-11-03 08:40:10 +0000 UTC",
				"2017-06-15 05:04:36 +0000 UTC",
				"2017-03-09 21:22:21 +0000 UTC",
				"closed", milestone)
		case "1.2.0":
			assertMilestoneEqual(t, "1.2.0", "2017-04-24 07:00:00 +0000 UTC",
				"2016-11-03 08:40:15 +0000 UTC",
				"2017-12-10 02:43:29 +0000 UTC",
				"2017-10-12 08:24:28 +0000 UTC",
				"closed", milestone)
		case "1.3.0":
			assertMilestoneEqual(t, "1.3.0", "2017-11-29 08:00:00 +0000 UTC",
				"2017-03-03 08:08:59 +0000 UTC",
				"2017-12-04 07:48:44 +0000 UTC",
				"2017-11-29 18:39:00 +0000 UTC",
				"closed", milestone)
		case "1.4.0":
			assertMilestoneEqual(t, "1.4.0", "2018-01-25 08:00:00 +0000 UTC",
				"2017-08-23 11:02:37 +0000 UTC",
				"2018-03-25 20:01:56 +0000 UTC",
				"2018-03-25 20:01:56 +0000 UTC",
				"closed", milestone)
		case "1.5.0":
			assertMilestoneEqual(t, "1.5.0", "2018-06-15 07:00:00 +0000 UTC",
				"2017-12-30 04:21:56 +0000 UTC",
				"2018-09-05 16:34:22 +0000 UTC",
				"2018-08-11 08:45:01 +0000 UTC",
				"closed", milestone)
		case "1.7.0":
			assertMilestoneEqual(t, "1.7.0", "2018-12-25 08:00:00 +0000 UTC",
				"2018-08-28 14:20:14 +0000 UTC",
				"2019-01-27 11:30:24 +0000 UTC",
				"2019-01-23 08:58:23 +0000 UTC",
				"closed", milestone)
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

	releases, err := downloader.GetReleases()
	assert.NoError(t, err)
	assert.EqualValues(t, []*base.Release{
		{
			TagName:         "v0.9.99",
			TargetCommitish: "master",
			Name:            "fork",
			Body:            "Forked source from Gogs into Gitea\n",
			Created:         time.Date(2016, 10, 17, 02, 17, 59, 0, time.UTC),
			Published:       time.Date(2016, 11, 17, 15, 37, 0, 0, time.UTC),
		},
	}, releases[len(releases)-1:])

	// downloader.GetIssues()
	issues, isEnd, err := downloader.GetIssues(1, 8)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(issues))
	assert.False(t, isEnd)

	var (
		closed1 = time.Date(2018, 10, 23, 02, 57, 43, 0, time.UTC)
		closed7 = time.Date(2019, 7, 8, 8, 20, 23, 0, time.UTC)
	)
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
			Closed: &closed1,
		},
		{
			Number:     7,
			Title:      "display page revisions on wiki",
			Content:    "Hi guys,\r\n\r\nWiki on Gogs is very fine, I liked a lot, but I think that is good idea to be possible see other revisions from page as a page history.\r\n\r\nWhat you think?\r\n\r\nReference: https://github.com/gogits/gogs/issues/2991",
			Milestone:  "1.10.0",
			PosterName: "joubertredrat",
			State:      "closed",
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
				TotalCount: 6,
				PlusOne:    5,
				MinusOne:   0,
				Laugh:      0,
				Confused:   1,
				Heart:      0,
				Hooray:     0,
			},
			Closed: &closed7,
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
				TotalCount: 10,
				PlusOne:    9,
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
			IssueIndex: 6,
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
			IssueIndex: 6,
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
			IssueIndex: 6,
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
	prs, err := downloader.GetPullRequests(1, 3)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(prs))

	closed1 = time.Date(2016, 11, 02, 18, 22, 21, 0, time.UTC)
	var (
		closed2 = time.Date(2016, 11, 03, 8, 06, 27, 0, time.UTC)
		closed3 = time.Date(2016, 11, 02, 18, 22, 31, 0, time.UTC)
	)

	var (
		merged1 = time.Date(2016, 11, 02, 18, 22, 21, 0, time.UTC)
		merged2 = time.Date(2016, 11, 03, 8, 06, 27, 0, time.UTC)
		merged3 = time.Date(2016, 11, 02, 18, 22, 31, 0, time.UTC)
	)
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
			Closed:         &closed1,
			Merged:         true,
			MergedTime:     &merged1,
			MergeCommitSHA: "142d35e8d2baec230ddb565d1265940d59141fab",
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
				CloneURL:  "https://github.com/strk/gogs.git",
			},
			Base: base.PullRequestBranch{
				Ref:       "develop",
				SHA:       "5c5424301443ffa3659737d12de48ab1dfe39a00",
				OwnerName: "go-gitea",
				RepoName:  "gitea",
			},
			Closed:         &closed2,
			Merged:         true,
			MergedTime:     &merged2,
			MergeCommitSHA: "d8de2beb5b92d02a0597ba7c7803839380666653",
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
				CloneURL:  "https://github.com/strk/gogs.git",
			},
			Base: base.PullRequestBranch{
				Ref:       "develop",
				SHA:       "6bcff7828f117af8d51285ce3acba01a7e40a867",
				OwnerName: "go-gitea",
				RepoName:  "gitea",
			},
			Closed:         &closed3,
			Merged:         true,
			MergedTime:     &merged3,
			MergeCommitSHA: "5c5424301443ffa3659737d12de48ab1dfe39a00",
		},
	}, prs)
}
