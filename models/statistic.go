// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/login"
)

// Statistic contains the database statistics
type Statistic struct {
	Counter struct {
		User, Org, PublicKey,
		Repo, Watch, Star, Action, Access,
		Issue, IssueClosed, IssueOpen,
		Comment, Oauth, Follow,
		Mirror, Release, LoginSource, Webhook,
		Milestone, Label, HookTask,
		Team, UpdateTask, Attachment int64
	}
}

// GetStatistic returns the database statistics
func GetStatistic() (stats Statistic) {
	stats.Counter.User = CountUsers()
	stats.Counter.Org = CountOrganizations()
	stats.Counter.PublicKey, _ = db.GetEngine(db.DefaultContext).Count(new(PublicKey))
	stats.Counter.Repo = CountRepositories(true)
	stats.Counter.Watch, _ = db.GetEngine(db.DefaultContext).Count(new(Watch))
	stats.Counter.Star, _ = db.GetEngine(db.DefaultContext).Count(new(Star))
	stats.Counter.Action, _ = db.GetEngine(db.DefaultContext).Count(new(Action))
	stats.Counter.Access, _ = db.GetEngine(db.DefaultContext).Count(new(Access))

	type IssueCount struct {
		Count    int64
		IsClosed bool
	}
	issueCounts := []IssueCount{}

	_ = db.GetEngine(db.DefaultContext).Select("COUNT(*) AS count, is_closed").Table("issue").GroupBy("is_closed").Find(&issueCounts)
	for _, c := range issueCounts {
		if c.IsClosed {
			stats.Counter.IssueClosed = c.Count
		} else {
			stats.Counter.IssueOpen = c.Count
		}
	}

	stats.Counter.Issue = stats.Counter.IssueClosed + stats.Counter.IssueOpen

	stats.Counter.Comment, _ = db.GetEngine(db.DefaultContext).Count(new(Comment))
	stats.Counter.Oauth = 0
	stats.Counter.Follow, _ = db.GetEngine(db.DefaultContext).Count(new(Follow))
	stats.Counter.Mirror, _ = db.GetEngine(db.DefaultContext).Count(new(Mirror))
	stats.Counter.Release, _ = db.GetEngine(db.DefaultContext).Count(new(Release))
	stats.Counter.LoginSource = login.CountSources()
	stats.Counter.Webhook, _ = db.GetEngine(db.DefaultContext).Count(new(Webhook))
	stats.Counter.Milestone, _ = db.GetEngine(db.DefaultContext).Count(new(Milestone))
	stats.Counter.Label, _ = db.GetEngine(db.DefaultContext).Count(new(Label))
	stats.Counter.HookTask, _ = db.GetEngine(db.DefaultContext).Count(new(HookTask))
	stats.Counter.Team, _ = db.GetEngine(db.DefaultContext).Count(new(Team))
	stats.Counter.Attachment, _ = db.GetEngine(db.DefaultContext).Count(new(Attachment))
	return
}
