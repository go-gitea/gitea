// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

// Statistic contains the database statistics
type Statistic struct {
	Counter struct {
		UsersActive, UsersNotActive,
		Org, PublicKey,
		Repo, Watch, Star, Access,
		Issue, IssueClosed, IssueOpen,
		Comment, Oauth, Follow,
		Mirror, Release, AuthSource, Webhook,
		Milestone, Label, HookTask,
		Team, UpdateTask, Project,
		ProjectColumn, Attachment,
		Branches, Tags, CommitStatus int64
		IssueByLabel      []IssueByLabelCount
		IssueByRepository []IssueByRepositoryCount
	}
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
func GetStatistic(ctx context.Context) (stats Statistic) {
	e := db.GetEngine(ctx)

	// Number of active users
	usersActiveOpts := user_model.CountUserFilter{
		IsActive: optional.Some(true),
	}
	stats.Counter.UsersActive = user_model.CountUsers(ctx, &usersActiveOpts)

	// Number of inactive users
	usersNotActiveOpts := user_model.CountUserFilter{
		IsActive: optional.Some(false),
	}
	stats.Counter.UsersNotActive = user_model.CountUsers(ctx, &usersNotActiveOpts)

	stats.Counter.Org, _ = db.Count[organization.Organization](ctx, organization.FindOrgOptions{IncludeVisibility: structs.VisibleTypePrivate})
	stats.Counter.PublicKey, _ = e.Count(new(asymkey_model.PublicKey))
	stats.Counter.Repo, _ = repo_model.CountRepositories(ctx, repo_model.CountRepositoryOptions{})
	stats.Counter.Watch, _ = e.Count(new(repo_model.Watch))
	stats.Counter.Star, _ = e.Count(new(repo_model.Star))
	stats.Counter.Access, _ = e.Count(new(access_model.Access))
	stats.Counter.Branches, _ = e.Count(new(git_model.Branch))
	stats.Counter.Tags, _ = e.Where("is_draft=?", false).Count(new(repo_model.Release))
	stats.Counter.CommitStatus, _ = e.Count(new(git_model.CommitStatus))

	type IssueCount struct {
		Count    int64
		IsClosed bool
	}

	if setting.Metrics.EnabledIssueByLabel {
		stats.Counter.IssueByLabel = []IssueByLabelCount{}

		_ = e.Select("COUNT(*) AS count, l.name AS label").
			Join("LEFT", "label l", "l.id=il.label_id").
			Table("issue_label il").
			GroupBy("l.name").
			Find(&stats.Counter.IssueByLabel)
	}

	if setting.Metrics.EnabledIssueByRepository {
		stats.Counter.IssueByRepository = []IssueByRepositoryCount{}

		_ = e.Select("COUNT(*) AS count, r.owner_name, r.name AS repository").
			Join("LEFT", "repository r", "r.id=i.repo_id").
			Table("issue i").
			GroupBy("r.owner_name, r.name").
			Find(&stats.Counter.IssueByRepository)
	}

	var issueCounts []IssueCount

	_ = e.Select("COUNT(*) AS count, is_closed").Table("issue").GroupBy("is_closed").Find(&issueCounts)
	for _, c := range issueCounts {
		if c.IsClosed {
			stats.Counter.IssueClosed = c.Count
		} else {
			stats.Counter.IssueOpen = c.Count
		}
	}

	stats.Counter.Issue = stats.Counter.IssueClosed + stats.Counter.IssueOpen

	stats.Counter.Comment, _ = e.Count(new(issues_model.Comment))
	stats.Counter.Oauth = 0
	stats.Counter.Follow, _ = e.Count(new(user_model.Follow))
	stats.Counter.Mirror, _ = e.Count(new(repo_model.Mirror))
	stats.Counter.Release, _ = e.Count(new(repo_model.Release))
	stats.Counter.AuthSource, _ = db.Count[auth.Source](ctx, auth.FindSourcesOptions{})
	stats.Counter.Webhook, _ = e.Count(new(webhook.Webhook))
	stats.Counter.Milestone, _ = e.Count(new(issues_model.Milestone))
	stats.Counter.Label, _ = e.Count(new(issues_model.Label))
	stats.Counter.HookTask, _ = e.Count(new(webhook.HookTask))
	stats.Counter.Team, _ = e.Count(new(organization.Team))
	stats.Counter.Attachment, _ = e.Count(new(repo_model.Attachment))
	stats.Counter.Project, _ = e.Count(new(project_model.Project))
	stats.Counter.ProjectColumn, _ = e.Count(new(project_model.Column))
	return stats
}
