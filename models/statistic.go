// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/setting"
)

// Statistic contains the database statistics
type Statistic struct {
	Counter struct {
		User, Org, PublicKey,
		Repo, Watch, Star, Action, Access,
		Issue, IssueClosed, IssueOpen,
		Comment, Oauth, Follow,
		Mirror, Release, AuthSource, Webhook,
		Milestone, Label, HookTask,
		Team, UpdateTask, Project,
		ProjectBoard, Attachment int64
		IssueByLabel      []IssueByLabelCount
		IssueByRepository []IssueByRepositoryCount
	}
	Time time.Time
}

// IssueByLabelCount contains the number of issue group by label
type IssueByLabelCount struct {
	Count int64
	Label string
}

// IssueByRepositoryCount contains the number of issue group by repository
type IssueByRepositoryCount struct {
	Count      int64
	OwnerName  string
	Repository string
}

// GetStatistic returns the database statistics
func GetStatistic(estimate, metrics bool) (stats Statistic) {
	e := db.GetEngine(db.DefaultContext)
	stats.Counter.User = user_model.CountUsers()
	stats.Counter.Org = organization.CountOrganizations()
	stats.Counter.Repo = repo_model.CountRepositories(true)
	if estimate {
		stats.Counter.PublicKey, _ = db.EstimateTotal(new(asymkey_model.PublicKey))
		stats.Counter.Watch, _ = db.EstimateTotal(new(repo_model.Watch))
		stats.Counter.Star, _ = db.EstimateTotal(new(repo_model.Star))
		stats.Counter.Action, _ = db.EstimateTotal(new(Action))
		stats.Counter.Access, _ = db.EstimateTotal(new(Access))
	} else {
		stats.Counter.PublicKey, _ = e.Count(new(asymkey_model.PublicKey))
		stats.Counter.Watch, _ = e.Count(new(repo_model.Watch))
		stats.Counter.Star, _ = e.Count(new(repo_model.Star))
		stats.Counter.Action, _ = e.Count(new(Action))
		stats.Counter.Access, _ = e.Count(new(Access))
	}

	type IssueCount struct {
		Count    int64
		IsClosed bool
	}

	if metrics && setting.Metrics.EnabledIssueByLabel {
		stats.Counter.IssueByLabel = []IssueByLabelCount{}

		_ = e.Select("COUNT(*) AS count, l.name AS label").
			Join("LEFT", "label l", "l.id=il.label_id").
			Table("issue_label il").
			GroupBy("l.name").
			Find(&stats.Counter.IssueByLabel)
	}

	if metrics && setting.Metrics.EnabledIssueByRepository {
		stats.Counter.IssueByRepository = []IssueByRepositoryCount{}

		_ = e.Select("COUNT(*) AS count, r.owner_name, r.name AS repository").
			Join("LEFT", "repository r", "r.id=i.repo_id").
			Table("issue i").
			GroupBy("r.owner_name, r.name").
			Find(&stats.Counter.IssueByRepository)
	}

	issueCounts := []IssueCount{}

	_ = e.Select("COUNT(*) AS count, is_closed").Table("issue").GroupBy("is_closed").Find(&issueCounts)
	for _, c := range issueCounts {
		if c.IsClosed {
			stats.Counter.IssueClosed = c.Count
		} else {
			stats.Counter.IssueOpen = c.Count
		}
	}

	stats.Counter.Issue = stats.Counter.IssueClosed + stats.Counter.IssueOpen

	if estimate {
		stats.Counter.Comment, _ = db.EstimateTotal(new(Comment))
		stats.Counter.Follow, _ = db.EstimateTotal(new(user_model.Follow))
		stats.Counter.Mirror, _ = db.EstimateTotal(new(repo_model.Mirror))
		stats.Counter.Release, _ = db.EstimateTotal(new(Release))
		stats.Counter.Webhook, _ = db.EstimateTotal(new(webhook.Webhook))
		stats.Counter.Milestone, _ = db.EstimateTotal(new(issues_model.Milestone))
		stats.Counter.Label, _ = db.EstimateTotal(new(Label))
		stats.Counter.HookTask, _ = db.EstimateTotal(new(webhook.HookTask))
		stats.Counter.Team, _ = db.EstimateTotal(new(organization.Team))
		stats.Counter.Attachment, _ = db.EstimateTotal(new(repo_model.Attachment))
		stats.Counter.Project, _ = db.EstimateTotal(new(project_model.Project))
		stats.Counter.ProjectBoard, _ = db.EstimateTotal(new(project_model.Board))
	} else {
		stats.Counter.Comment, _ = e.Count(new(Comment))
		stats.Counter.Follow, _ = e.Count(new(user_model.Follow))
		stats.Counter.Mirror, _ = e.Count(new(repo_model.Mirror))
		stats.Counter.Release, _ = e.Count(new(Release))
		stats.Counter.Webhook, _ = e.Count(new(webhook.Webhook))
		stats.Counter.Milestone, _ = e.Count(new(issues_model.Milestone))
		stats.Counter.Label, _ = e.Count(new(Label))
		stats.Counter.HookTask, _ = e.Count(new(webhook.HookTask))
		stats.Counter.Team, _ = e.Count(new(organization.Team))
		stats.Counter.Attachment, _ = e.Count(new(repo_model.Attachment))
		stats.Counter.Project, _ = e.Count(new(project_model.Project))
		stats.Counter.ProjectBoard, _ = e.Count(new(project_model.Board))
	}
	stats.Counter.Oauth = 0
	stats.Counter.AuthSource = auth.CountSources()
	stats.Time = time.Now()
	return
}
