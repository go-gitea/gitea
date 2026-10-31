// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"uuid"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/repostats"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	issue_indexer "gitea.dev/modules/indexer/issues"
	"gitea.dev/modules/label"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/uri"
	"gitea.dev/modules/util"
	"gitea.dev/services/gitdiff"
	"gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"
)

var _ base.Uploader = &GiteaLocalUploader{}

// GiteaLocalUploader implements an Uploader to gitea sites
type GiteaLocalUploader struct {
	doer           *user_model.User
	repoOwner      string
	repoName       string
	cloneURL       string // clone address of the source, may carry credentials
	repo           *repo_model.Repository
	labels         map[string]*issues_model.Label
	milestones     map[string]int64
	issues         map[int64]*issues_model.Issue
	gitRepo        *git.Repository
	prHeadCache    map[string]string
	sameApp        bool
	userMap        map[int64]int64 // external user id mapping to user id
	prCache        map[int64]*issues_model.PullRequest
	gitServiceType structs.GitServiceType
	syncMode       bool // SyncRepository: create missing labels and milestones instead of skipping them
}

// NewGiteaLocalUploader creates a gitea Uploader via gitea API v1
func NewGiteaLocalUploader(_ context.Context, doer *user_model.User, repoOwner, repoName string) *GiteaLocalUploader {
	return &GiteaLocalUploader{
		doer:        doer,
		repoOwner:   repoOwner,
		repoName:    repoName,
		labels:      make(map[string]*issues_model.Label),
		milestones:  make(map[string]int64),
		issues:      make(map[int64]*issues_model.Issue),
		prHeadCache: make(map[string]string),
		userMap:     make(map[int64]int64),
		prCache:     make(map[int64]*issues_model.PullRequest),
	}
}

// MaxBatchInsertSize returns the table's max batch insert size
func (g *GiteaLocalUploader) MaxBatchInsertSize(tp string) int {
	switch tp {
	case "issue":
		return db.MaxBatchInsertSize(new(issues_model.Issue))
	case "comment":
		return db.MaxBatchInsertSize(new(issues_model.Comment))
	case "milestone":
		return db.MaxBatchInsertSize(new(issues_model.Milestone))
	case "label":
		return db.MaxBatchInsertSize(new(issues_model.Label))
	case "release":
		return db.MaxBatchInsertSize(new(repo_model.Release))
	case "pullrequest":
		return db.MaxBatchInsertSize(new(issues_model.PullRequest))
	}
	return 10
}

// CreateRepo creates a repository
func (g *GiteaLocalUploader) CreateRepo(ctx context.Context, repo *base.Repository, opts base.MigrateOptions) error {
	owner, err := user_model.GetUserByName(ctx, g.repoOwner)
	if err != nil {
		return err
	}

	var r *repo_model.Repository
	if opts.MigrateToRepoID <= 0 {
		r, err = repo_service.CreateRepositoryDirectly(ctx, g.doer, owner, repo_service.CreateRepoOptions{
			Name:           g.repoName,
			Description:    repo.Description,
			Website:        repo.Website,
			OriginalURL:    repo.OriginalURL,
			GitServiceType: opts.GitServiceType,
			IsPrivate:      opts.Private || setting.Repository.ForcePrivate,
			IsMirror:       opts.Mirror,
			Status:         repo_model.RepositoryBeingMigrated,
		}, false)
	} else {
		r, err = repo_model.GetRepositoryByID(ctx, opts.MigrateToRepoID)
	}
	if err != nil {
		return err
	}
	r.DefaultBranch = repo.DefaultBranch
	r.Description = repo.Description
	r.Website = repo.Website

	g.cloneURL = repo.CloneURL
	r, err = repo_service.MigrateRepositoryGitData(ctx, owner, r, base.MigrateOptions{
		RepoName:       g.repoName,
		Description:    repo.Description,
		OriginalURL:    repo.OriginalURL,
		GitServiceType: opts.GitServiceType,
		Mirror:         repo.IsMirror,
		LFS:            opts.LFS,
		LFSEndpoint:    opts.LFSEndpoint,
		CloneAddr:      repo.CloneURL, // SECURITY: we will assume that this has already been checked
		Private:        repo.IsPrivate,
		Wiki:           opts.Wiki,
		Releases:       opts.Releases, // if didn't get releases, then sync them from tags
		MirrorInterval: opts.MirrorInterval,
	}, getMigrationHTTPClient().Transport)

	g.sameApp = strings.HasPrefix(repo.OriginalURL, setting.AppURL)
	g.repo = r
	if err != nil {
		return err
	}
	g.gitRepo, err = git.OpenRepository(ctx, g.repo)
	if err != nil {
		return err
	}

	// detect object format from git repository and update to database
	objectFormat, err := g.gitRepo.GetObjectFormat(ctx)
	if err != nil {
		return err
	}
	g.repo.ObjectFormatName = objectFormat.Name()
	return repo_model.UpdateRepositoryColsNoAutoTime(ctx, g.repo, "object_format_name")
}

// Close closes this uploader
func (g *GiteaLocalUploader) Close() {
	if g.gitRepo != nil {
		g.gitRepo.Close()
	}
}

// CreateTopics creates topics
func (g *GiteaLocalUploader) CreateTopics(ctx context.Context, topics ...string) error {
	// Ignore topics too long for the db
	c := 0
	for _, topic := range topics {
		if len(topic) > 50 {
			continue
		}

		topics[c] = topic
		c++
	}
	topics = topics[:c]
	return repo_model.SaveTopics(ctx, g.repo.ID, topics...)
}

// CreateMilestones creates milestones
func (g *GiteaLocalUploader) CreateMilestones(ctx context.Context, milestones ...*base.Milestone) error {
	mss := make([]*issues_model.Milestone, 0, len(milestones))
	for _, milestone := range milestones {
		var deadline timeutil.TimeStamp
		if milestone.Deadline != nil {
			deadline = timeutil.TimeStamp(milestone.Deadline.Unix())
		}
		if deadline == 0 {
			deadline = timeutil.TimeStamp(time.Date(9999, 1, 1, 0, 0, 0, 0, setting.DefaultUILocation).Unix())
		}

		if milestone.Created.IsZero() {
			if milestone.Updated != nil {
				milestone.Created = *milestone.Updated
			} else if milestone.Deadline != nil {
				milestone.Created = *milestone.Deadline
			} else {
				milestone.Created = time.Now()
			}
		}
		if milestone.Updated == nil || milestone.Updated.IsZero() {
			milestone.Updated = &milestone.Created
		}

		ms := issues_model.Milestone{
			RepoID:       g.repo.ID,
			Name:         milestone.Title,
			Content:      milestone.Description,
			IsClosed:     milestone.State == "closed",
			CreatedUnix:  timeutil.TimeStamp(milestone.Created.Unix()),
			UpdatedUnix:  timeutil.TimeStamp(milestone.Updated.Unix()),
			DeadlineUnix: deadline,
		}
		if ms.IsClosed && milestone.Closed != nil {
			ms.ClosedDateUnix = timeutil.TimeStamp(milestone.Closed.Unix())
		}
		mss = append(mss, &ms)
	}

	err := issues_model.InsertMilestones(ctx, mss...)
	if err != nil {
		return err
	}

	for _, ms := range mss {
		g.milestones[ms.Name] = ms.ID
	}
	return nil
}

// CreateLabels creates labels
func (g *GiteaLocalUploader) CreateLabels(ctx context.Context, labels ...*base.Label) error {
	lbs := make([]*issues_model.Label, 0, len(labels))
	for _, l := range labels {
		if color, err := label.NormalizeColor(l.Color); err != nil {
			log.Warn("Invalid label color: #%s for label: %s in migration to %s/%s", l.Color, l.Name, g.repoOwner, g.repoName)
			l.Color = "#ffffff"
		} else {
			l.Color = color
		}

		lb := &issues_model.Label{
			RepoID:      g.repo.ID,
			Name:        l.Name,
			Exclusive:   l.Exclusive,
			Description: l.Description,
			Color:       l.Color,
		}
		lb.SetArchived(l.Archived)
		lbs = append(lbs, lb)
	}

	err := issues_model.NewLabels(ctx, lbs...)
	if err != nil {
		return err
	}
	for _, lb := range lbs {
		g.labels[lb.Name] = lb
	}
	return nil
}

// CreateReleases creates releases
func (g *GiteaLocalUploader) CreateReleases(ctx context.Context, releases ...*base.Release) error {
	rels := make([]*repo_model.Release, 0, len(releases))
	for _, release := range releases {
		if release.Created.IsZero() {
			if !release.Published.IsZero() {
				release.Created = release.Published
			} else {
				release.Created = time.Now()
			}
		}

		// SECURITY: The TagName must be a valid git ref
		if release.TagName != "" && !git.IsValidRefPattern(release.TagName) {
			release.TagName = ""
		}

		// SECURITY: The TargetCommitish must be a valid git ref
		if release.TargetCommitish != "" && !git.IsValidRefPattern(release.TargetCommitish) {
			release.TargetCommitish = ""
		}

		publishedAt := util.Iif(release.Published.IsZero(), release.Created, release.Published)

		rel := repo_model.Release{
			RepoID:        g.repo.ID,
			TagName:       release.TagName,
			LowerTagName:  strings.ToLower(release.TagName),
			Target:        release.TargetCommitish,
			Title:         release.Name,
			Note:          release.Body,
			IsDraft:       release.Draft,
			IsPrerelease:  release.Prerelease,
			IsTag:         false,
			CreatedUnix:   timeutil.TimeStamp(release.Created.Unix()),
			PublishedUnix: util.Iif(release.Draft, 0, timeutil.TimeStamp(publishedAt.Unix())),
		}

		if err := g.remapUser(ctx, release, &rel); err != nil {
			return err
		}

		// calc NumCommits if possible
		if rel.TagName != "" {
			commit, err := g.gitRepo.GetTagCommit(ctx, rel.TagName)
			if !git.IsErrNotExist(err) {
				if err != nil {
					return fmt.Errorf("GetTagCommit[%v]: %w", rel.TagName, err)
				}
				rel.Sha1 = commit.ID.String()
				rel.NumCommits, err = git.CommitsCountOfCommit(ctx, g.repo, commit.ID.String())
				if err != nil {
					return fmt.Errorf("CommitsCount: %w", err)
				}
			}
		}

		for _, asset := range release.Assets {
			if asset.Created.IsZero() {
				if !asset.Updated.IsZero() {
					asset.Created = asset.Updated
				} else {
					asset.Created = release.Created
				}
			}
			attach := repo_model.Attachment{
				UUID:          uuid.New().String(),
				RepoID:        g.repo.ID,
				Name:          asset.Name,
				DownloadCount: int64(*asset.DownloadCount),
				Size:          int64(*asset.Size),
				CreatedUnix:   timeutil.TimeStamp(asset.Created.Unix()),
			}

			// SECURITY: We cannot check the DownloadURL and DownloadFunc are safe here
			// ... we must assume that they are safe and simply download the attachment
			var rc io.ReadCloser
			var err error
			if asset.DownloadFunc != nil {
				rc, err = asset.DownloadFunc()
			} else if asset.DownloadURL != nil {
				// asset.DownloadURL maybe a local file
				// use the migration client so the fetch (including any redirect) is
				// validated against the migration host allow/block list
				rc, err = uri.OpenWithClient(*asset.DownloadURL, getMigrationHTTPClient())
			}
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				WarnAndNotice("Skipped asset %s of release %s: %v", asset.Name, release.TagName, err)
				continue
			}
			if rc != nil {
				attach.Size, err = storage.Attachments.Save(attach.RelativePath(), rc, util.Iif(*asset.Size > 0, int64(*asset.Size), -1)) // GitLab links and Forgejo external links report no size
				rc.Close()
				if err != nil {
					return fmt.Errorf("release %s asset %s: %w", release.TagName, asset.Name, err)
				}
			}

			rel.Attachments = append(rel.Attachments, &attach)
		}

		rels = append(rels, &rel)
	}

	return repo_model.InsertReleases(ctx, rels...)
}

// SyncTags syncs releases with tags in the database
func (g *GiteaLocalUploader) SyncTags(ctx context.Context) error {
	_, err := repo_module.SyncReleasesWithTags(ctx, g.repo, g.gitRepo)
	return err
}

func (g *GiteaLocalUploader) SyncBranches(ctx context.Context) error {
	_, _, err := repo_module.SyncRepoBranchesWithRepo(ctx, g.repo, g.gitRepo, g.doer.ID)
	return err
}

// CreateIssues creates issues
func (g *GiteaLocalUploader) CreateIssues(ctx context.Context, issues ...*base.Issue) error {
	iss := make([]*issues_model.Issue, 0, len(issues))
	for _, issue := range issues {
		var labels []*issues_model.Label
		for _, label := range issue.Labels {
			lb, ok := g.labels[label.Name]
			if ok {
				labels = append(labels, lb)
			}
		}

		milestoneID := g.milestones[issue.Milestone]

		if issue.Created.IsZero() {
			if issue.Closed != nil {
				issue.Created = *issue.Closed
			} else {
				issue.Created = time.Now()
			}
		}
		if issue.Updated.IsZero() {
			if issue.Closed != nil {
				issue.Updated = *issue.Closed
			} else {
				issue.Updated = time.Now()
			}
		}

		// SECURITY: issue.Ref needs to be a valid reference
		if !git.IsValidRefPattern(issue.Ref) {
			log.Warn("Invalid issue.Ref[%s] in issue #%d in %s/%s", issue.Ref, issue.Number, g.repoOwner, g.repoName)
			issue.Ref = ""
		}

		is := issues_model.Issue{
			RepoID:      g.repo.ID,
			Repo:        g.repo,
			Index:       issue.Number,
			Title:       util.TruncateRunes(issue.Title, 255),
			Content:     issue.Content,
			Ref:         issue.Ref,
			IsClosed:    issue.State == "closed",
			IsLocked:    issue.IsLocked,
			MilestoneID: milestoneID,
			Labels:      labels,
			CreatedUnix: timeutil.TimeStamp(issue.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(issue.Updated.Unix()),
		}

		if err := g.remapUser(ctx, issue, &is); err != nil {
			return err
		}

		if issue.Closed != nil {
			is.ClosedUnix = timeutil.TimeStamp(issue.Closed.Unix())
		}
		// add reactions
		for _, reaction := range issue.Reactions {
			res := issues_model.Reaction{
				Type:        reaction.Content,
				CreatedUnix: timeutil.TimeStampNow(),
			}
			if err := g.remapUser(ctx, reaction, &res); err != nil {
				return err
			}
			is.Reactions = append(is.Reactions, &res)
		}
		iss = append(iss, &is)
	}

	if len(iss) > 0 {
		if err := issues_model.InsertIssues(ctx, iss...); err != nil {
			return err
		}

		for _, is := range iss {
			g.issues[is.Index] = is
		}
	}

	return nil
}

// PatchIssues upserts issues, matched on (repo_id, index)
func (g *GiteaLocalUploader) PatchIssues(ctx context.Context, issues ...*base.Issue) error {
	iss, err := g.prepareIssues(ctx, issues...)
	if err != nil {
		return err
	}

	if len(iss) > 0 {
		if err := issues_model.UpsertIssues(ctx, iss...); err != nil {
			return err
		}

		for _, is := range iss {
			g.issues[is.Index] = is
			// the sync writes rows directly and no notifier fires, so feed the search index here
			issue_indexer.UpdateIssueIndexer(ctx, is.ID)
		}
	}

	return nil
}

// ensureLabels resolves labels by name. A sync creates missing ones (e.g. PR-only labels); a migration skips them,
// since a dump can reference org labels that must not become repository labels
func (g *GiteaLocalUploader) ensureLabels(ctx context.Context, labels []*base.Label) ([]*issues_model.Label, error) {
	result := make([]*issues_model.Label, 0, len(labels))
	for _, l := range labels {
		lb, ok := g.labels[l.Name]
		if !ok {
			if !g.syncMode {
				continue
			}
			color, err := label.NormalizeColor(l.Color)
			if err != nil {
				log.Warn("Invalid label color: #%s for label: %s in %s/%s", l.Color, l.Name, g.repoOwner, g.repoName)
				color = "#ffffff"
			}
			lb = &issues_model.Label{
				RepoID:      g.repo.ID,
				Name:        l.Name,
				Exclusive:   l.Exclusive,
				Description: l.Description,
				Color:       color,
			}
			lb.SetArchived(l.Archived)
			if err := issues_model.NewLabels(ctx, lb); err != nil {
				return nil, err
			}
			g.labels[l.Name] = lb
		}
		result = append(result, lb)
	}
	return result, nil
}

// ensureMilestone resolves a milestone id by title, creating it in sync mode
func (g *GiteaLocalUploader) ensureMilestone(ctx context.Context, title string) (int64, error) {
	if title == "" {
		return 0, nil
	}
	if id, ok := g.milestones[title]; ok {
		return id, nil
	}
	if !g.syncMode {
		return 0, nil
	}
	ms := &issues_model.Milestone{RepoID: g.repo.ID, Name: title}
	if err := issues_model.NewMilestone(ctx, ms); err != nil {
		return 0, err
	}
	g.milestones[title] = ms.ID
	return ms.ID, nil
}

func (g *GiteaLocalUploader) prepareIssues(ctx context.Context, issues ...*base.Issue) ([]*issues_model.Issue, error) {
	iss := make([]*issues_model.Issue, 0, len(issues))
	for _, issue := range issues {
		labels, err := g.ensureLabels(ctx, issue.Labels)
		if err != nil {
			return nil, err
		}

		milestoneID, err := g.ensureMilestone(ctx, issue.Milestone)
		if err != nil {
			return nil, err
		}

		if issue.Created.IsZero() {
			if issue.Closed != nil {
				issue.Created = *issue.Closed
			} else {
				issue.Created = time.Now()
			}
		}
		if issue.Updated.IsZero() {
			if issue.Closed != nil {
				issue.Updated = *issue.Closed
			} else {
				issue.Updated = time.Now()
			}
		}

		// SECURITY: issue.Ref needs to be a valid reference
		if !git.IsValidRefPattern(issue.Ref) {
			log.Warn("Invalid issue.Ref[%s] in issue #%d in %s/%s", issue.Ref, issue.Number, g.repoOwner, g.repoName)
			issue.Ref = ""
		}

		is := issues_model.Issue{
			RepoID:      g.repo.ID,
			Repo:        g.repo,
			Index:       issue.Number,
			Title:       util.TruncateRunes(issue.Title, 255),
			Content:     issue.Content,
			Ref:         issue.Ref,
			IsClosed:    issue.State == "closed",
			IsLocked:    issue.IsLocked,
			MilestoneID: milestoneID,
			Labels:      labels,
			CreatedUnix: timeutil.TimeStamp(issue.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(issue.Updated.Unix()),
		}

		if err := g.remapUser(ctx, issue, &is); err != nil {
			return nil, err
		}

		if issue.Closed != nil {
			is.ClosedUnix = timeutil.TimeStamp(issue.Closed.Unix())
		}
		// add reactions
		for _, reaction := range issue.Reactions {
			res := issues_model.Reaction{
				Type:        reaction.Content,
				CreatedUnix: timeutil.TimeStampNow(),
			}
			if err := g.remapUser(ctx, reaction, &res); err != nil {
				return nil, err
			}
			is.Reactions = append(is.Reactions, &res)
		}
		iss = append(iss, &is)
	}

	return iss, nil
}

// getIssueByIndex falls back to the database, so a sync can attach to issues an earlier run imported
func (g *GiteaLocalUploader) getIssueByIndex(ctx context.Context, index int64) (*issues_model.Issue, error) {
	if issue, ok := g.issues[index]; ok {
		return issue, nil
	}
	issue, err := issues_model.GetIssueByIndex(ctx, g.repo.ID, index)
	if err != nil {
		return nil, err
	}
	g.issues[index] = issue
	return issue, nil
}

// CreateComments creates comments of issues
func (g *GiteaLocalUploader) CreateComments(ctx context.Context, comments ...*base.Comment) error {
	cms := make([]*issues_model.Comment, 0, len(comments))
	for _, comment := range comments {
		var issue *issues_model.Issue
		issue, ok := g.issues[comment.IssueIndex]
		if !ok {
			return fmt.Errorf("comment references non existent IssueIndex %d", comment.IssueIndex)
		}

		if comment.Created.IsZero() {
			comment.Created = time.Unix(int64(issue.CreatedUnix), 0)
		}
		if comment.Updated.IsZero() {
			comment.Updated = comment.Created
		}
		if comment.CommentType == "" {
			// if type field is missing, then assume a normal comment
			comment.CommentType = issues_model.CommentTypeComment.String()
		}
		cm := issues_model.Comment{
			IssueID:     issue.ID,
			Type:        issues_model.AsCommentType(comment.CommentType),
			Content:     comment.Content,
			CreatedUnix: timeutil.TimeStamp(comment.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(comment.Updated.Unix()),
		}

		switch cm.Type {
		case issues_model.CommentTypeReopen:
			cm.Content = ""
		case issues_model.CommentTypeClose:
			cm.Content = ""
		case issues_model.CommentTypeAssignees:
			if assigneeID, ok := comment.Meta["AssigneeID"].(int); ok {
				cm.AssigneeID = int64(assigneeID)
			}
			if comment.Meta["RemovedAssigneeID"] != nil {
				cm.RemovedAssignee = true
			}
		case issues_model.CommentTypeChangeTitle:
			if comment.Meta["OldTitle"] != nil {
				cm.OldTitle = util.TruncateRunes(fmt.Sprint(comment.Meta["OldTitle"]), 255)
			}
			if comment.Meta["NewTitle"] != nil {
				cm.NewTitle = util.TruncateRunes(fmt.Sprint(comment.Meta["NewTitle"]), 255)
			}
		case issues_model.CommentTypeChangeTargetBranch:
			if comment.Meta["OldRef"] != nil && comment.Meta["NewRef"] != nil {
				cm.OldRef = util.TruncateRunes(fmt.Sprint(comment.Meta["OldRef"]), 255)
				cm.NewRef = util.TruncateRunes(fmt.Sprint(comment.Meta["NewRef"]), 255)
				cm.Content = ""
			}
		case issues_model.CommentTypeLabel:
			if name, ok := comment.Meta["LabelName"].(string); ok {
				if label, ok := g.labels[name]; ok {
					cm.LabelID = label.ID
				}
			}
			if cm.LabelID == 0 {
				continue // label_id 0 renders nothing and gitea doctor deletes it
			}
		case issues_model.CommentTypeMilestone:
			if title, ok := comment.Meta["MilestoneTitle"].(string); ok {
				if removed, _ := comment.Meta["Removed"].(bool); removed {
					cm.OldMilestoneID = g.milestones[title]
				} else {
					cm.MilestoneID = g.milestones[title]
				}
			}
			if cm.MilestoneID == 0 && cm.OldMilestoneID == 0 {
				continue
			}
		case issues_model.CommentTypeDeleteBranch:
			cm.OldRef = util.TruncateRunes(comment.Content, 255)
			cm.Content = ""
		case issues_model.CommentTypeMergePull:
			cm.Content = ""
		case issues_model.CommentTypePRScheduledToAutoMerge, issues_model.CommentTypePRUnScheduledToAutoMerge:
			cm.Content = ""
		default:
		}

		if err := g.remapUser(ctx, comment, &cm); err != nil {
			return err
		}

		// add reactions
		for _, reaction := range comment.Reactions {
			res := issues_model.Reaction{
				Type:        reaction.Content,
				CreatedUnix: timeutil.TimeStampNow(),
			}
			if err := g.remapUser(ctx, reaction, &res); err != nil {
				return err
			}
			cm.Reactions = append(cm.Reactions, &res)
		}

		cms = append(cms, &cm)
	}

	if len(cms) == 0 {
		return nil
	}
	return issues_model.InsertIssueComments(ctx, cms)
}

// PatchComments upserts comments, matched on their remote id
func (g *GiteaLocalUploader) PatchComments(ctx context.Context, comments ...*base.Comment) error {
	cms, err := g.prepareComments(ctx, comments...)
	if err != nil {
		return err
	}
	if len(cms) == 0 {
		return nil
	}
	if err := issues_model.UpsertIssueComments(ctx, cms); err != nil {
		return err
	}
	// see PatchIssues; comment bodies are part of the issue's search document
	seen := make(map[int64]struct{}, len(cms))
	for _, c := range cms {
		if _, ok := seen[c.IssueID]; ok {
			continue
		}
		seen[c.IssueID] = struct{}{}
		issue_indexer.UpdateIssueIndexer(ctx, c.IssueID)
	}
	return nil
}

func (g *GiteaLocalUploader) prepareComments(ctx context.Context, comments ...*base.Comment) ([]*issues_model.Comment, error) {
	cms := make([]*issues_model.Comment, 0, len(comments))
	for _, comment := range comments {
		issue, err := g.getIssueByIndex(ctx, comment.IssueIndex)
		if err != nil {
			return nil, err
		}

		if comment.Created.IsZero() {
			comment.Created = time.Unix(int64(issue.CreatedUnix), 0)
		}
		if comment.Updated.IsZero() {
			comment.Updated = comment.Created
		}
		if comment.CommentType == "" {
			// if type field is missing, then assume a normal comment
			comment.CommentType = issues_model.CommentTypeComment.String()
		}
		cm := issues_model.Comment{
			IssueID:     issue.ID,
			Type:        issues_model.AsCommentType(comment.CommentType),
			Content:     comment.Content,
			OriginalID:  comment.Index,
			CreatedUnix: timeutil.TimeStamp(comment.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(comment.Updated.Unix()),
		}

		switch cm.Type {
		case issues_model.CommentTypeReopen:
			cm.Content = ""
		case issues_model.CommentTypeClose:
			cm.Content = ""
		case issues_model.CommentTypeAssignees:
			if assigneeID, ok := comment.Meta["AssigneeID"].(int); ok {
				cm.AssigneeID = int64(assigneeID)
			}
			if comment.Meta["RemovedAssigneeID"] != nil {
				cm.RemovedAssignee = true
			}
		case issues_model.CommentTypeChangeTitle:
			if comment.Meta["OldTitle"] != nil {
				cm.OldTitle = fmt.Sprint(comment.Meta["OldTitle"])
			}
			if comment.Meta["NewTitle"] != nil {
				cm.NewTitle = fmt.Sprint(comment.Meta["NewTitle"])
			}
		case issues_model.CommentTypeLabel:
			// resolve the imported label by name; skip the event if the label was
			// never imported (e.g. deleted upstream) rather than leave label_id=0
			name, _ := comment.Meta["LabelName"].(string)
			lb, ok := g.labels[name]
			if !ok {
				continue
			}
			cm.LabelID = lb.ID
		case issues_model.CommentTypeMilestone:
			title, _ := comment.Meta["MilestoneTitle"].(string)
			id, ok := g.milestones[title]
			if !ok {
				continue
			}
			if removed, _ := comment.Meta["Removed"].(bool); removed {
				cm.OldMilestoneID = id
			} else {
				cm.MilestoneID = id
			}
		case issues_model.CommentTypeChangeTargetBranch:
			if comment.Meta["OldRef"] != nil && comment.Meta["NewRef"] != nil {
				cm.OldRef = fmt.Sprint(comment.Meta["OldRef"])
				cm.NewRef = fmt.Sprint(comment.Meta["NewRef"])
				cm.Content = ""
			}
		case issues_model.CommentTypeMergePull:
			cm.Content = ""
		case issues_model.CommentTypePRScheduledToAutoMerge, issues_model.CommentTypePRUnScheduledToAutoMerge:
			cm.Content = ""
		default:
		}

		if err := g.remapUser(ctx, comment, &cm); err != nil {
			return nil, err
		}

		// add reactions
		for _, reaction := range comment.Reactions {
			res := issues_model.Reaction{
				Type:        reaction.Content,
				CreatedUnix: timeutil.TimeStampNow(),
			}
			if err := g.remapUser(ctx, reaction, &res); err != nil {
				return nil, err
			}
			cm.Reactions = append(cm.Reactions, &res)
		}

		cms = append(cms, &cm)
	}

	return cms, nil
}

// CreatePullRequests creates pull requests
func (g *GiteaLocalUploader) CreatePullRequests(ctx context.Context, prs ...*base.PullRequest) error {
	gprs := make([]*issues_model.PullRequest, 0, len(prs))
	for _, pr := range prs {
		gpr, err := g.newPullRequest(ctx, pr)
		if err != nil {
			return err
		}

		if err := g.remapUser(ctx, pr, gpr.Issue); err != nil {
			return err
		}

		gprs = append(gprs, gpr)
	}
	if err := issues_model.InsertPullRequests(ctx, gprs...); err != nil {
		return err
	}
	for _, pr := range gprs {
		g.issues[pr.Issue.Index] = pr.Issue
		// see PatchIssues: keep the keyword-search index fed
		issue_indexer.UpdateIssueIndexer(ctx, pr.Issue.ID)
		pull.StartPullRequestCheckImmediately(ctx, pr)
	}
	return nil
}

func (g *GiteaLocalUploader) commitExists(ctx context.Context, sha string) bool {
	_, _, err := gitcmd.NewCommand("rev-list", "--quiet", "-1").AddDynamicArguments(sha).WithRepo(g.repo).RunStdString(ctx)
	return err == nil
}

// fetchCommit tries to get a missing head commit of a closed pull request by its SHA from the head repository,
// e.g. a squash-merged pull request whose source branch is gone and which the host does not expose as a ref
func (g *GiteaLocalUploader) fetchCommit(ctx context.Context, pr *base.PullRequest) bool {
	remote := pr.Head.CloneURL
	if !pr.IsForkPullRequest() && g.cloneURL != "" {
		remote = g.cloneURL // the source repository itself, with the migration credentials
	}
	if remote == "" {
		return false
	}
	_, _, err := gitcmd.NewCommand("fetch", "--no-tags").AddDashesAndList(remote, pr.Head.SHA).WithRepo(g.repo).RunStdString(ctx)
	if err != nil {
		log.Warn("PR #%d fetching head commit %s failed: %v", pr.Number, pr.Head.SHA, util.SanitizeCredentialURLs(err.Error()))
		return false
	}
	return g.commitExists(ctx, pr.Head.SHA)
}

// PatchPullRequests upserts pull requests, matched on (repo_id, index)
func (g *GiteaLocalUploader) PatchPullRequests(ctx context.Context, prs ...*base.PullRequest) error {
	gprs, err := g.preparePullRequests(ctx, prs...)
	if err != nil {
		return err
	}
	if err := issues_model.UpsertPullRequests(ctx, gprs...); err != nil {
		return err
	}
	for _, pr := range gprs {
		g.issues[pr.Issue.Index] = pr.Issue
		// see PatchIssues: keep the keyword-search index fed
		issue_indexer.UpdateIssueIndexer(ctx, pr.Issue.ID)
		pull.StartPullRequestCheckImmediately(ctx, pr)
	}
	return nil
}

func (g *GiteaLocalUploader) preparePullRequests(ctx context.Context, prs ...*base.PullRequest) ([]*issues_model.PullRequest, error) {
	gprs := make([]*issues_model.PullRequest, 0, len(prs))
	for _, pr := range prs {
		gpr, err := g.newPullRequest(ctx, pr)
		if err != nil {
			return nil, err
		}

		if err := g.remapUser(ctx, pr, gpr.Issue); err != nil {
			return nil, err
		}

		gprs = append(gprs, gpr)
	}
	return gprs, nil
}

// loadExistingLabelsAndMilestones fills the caches from the database, so a sync reuses what earlier runs imported
func (g *GiteaLocalUploader) loadExistingLabelsAndMilestones(ctx context.Context) error {
	labels, err := issues_model.GetLabelsByRepoID(ctx, g.repo.ID, "", db.ListOptions{})
	if err != nil {
		return err
	}
	for _, label := range labels {
		g.labels[label.Name] = label
	}

	milestones, err := db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		RepoID: g.repo.ID,
	})
	if err != nil {
		return err
	}
	for _, milestone := range milestones {
		g.milestones[milestone.Name] = milestone.ID
	}
	return nil
}

func (g *GiteaLocalUploader) updateGitForPullRequest(ctx context.Context, pr *base.PullRequest) (head string, err error) {
	// SECURITY: this pr must have been must have been ensured safe
	if !pr.EnsuredSafe {
		log.Error("PR #%d in %s/%s has not been checked for safety.", pr.Number, g.repoOwner, g.repoName)
		return "", fmt.Errorf("the PR[%d] was not checked for safety", pr.Number)
	}

	// Anonymous function to download the patch file (allows us to use defer)
	if err := func() error {
		// if the patchURL is empty there is nothing to download
		if pr.PatchURL == "" {
			return nil
		}

		// SECURITY: We will assume that the pr.PatchURL has been checked
		// pr.PatchURL maybe a local file - but note EnsureSafe should be asserting that this safe.
		// Use the migration client so an http(s) PatchURL (and any redirect it follows) is
		// validated against the migration host allow/block list at dial time.
		ret, err := uri.OpenWithClient(pr.PatchURL, getMigrationHTTPClient())
		if err != nil {
			return err
		}
		defer ret.Close()

		f, err := git.CreateRepoFile(ctx, g.repo, fmt.Sprintf("pulls/%d.patch", pr.Number))
		if err != nil {
			return err
		}
		defer f.Close()

		// TODO: Should there be limits on the size of this file?
		_, err = io.Copy(f, ret)

		return err
	}(); err != nil { // the patch is only a fallback for a missing head ref
		log.Warn("PR #%d in %s/%s: unable to download patch: %v", pr.Number, g.repoOwner, g.repoName, err)
	}

	head = "unknown repository"
	if pr.IsForkPullRequest() && pr.State != "closed" {
		// default to the branch name: a metadata mirror usually can't fetch fork branches
		if pr.Head.Ref != "" {
			head = pr.Head.Ref
		}

		// OK we want to fetch the current head as a branch from its CloneURL

		// 1. Is there a head clone URL available?
		// 2. Is there a head ref available?
		if pr.Head.CloneURL == "" || pr.Head.Ref == "" {
			return head, nil
		}

		// ... the head commit may already be in the repository (e.g. fetched by the mirror clone as a hidden
		// pull request ref), then no remote is needed and a private fork does not need credentials either
		headIsLocal := pr.Head.SHA != "" && g.commitExists(ctx, pr.Head.SHA)

		// 3. We need to create a remote for this clone url
		// ... maybe we already have a name for this remote
		remote, ok := g.prHeadCache[pr.Head.CloneURL+":"]
		if !ok && !headIsLocal {
			// ... let's try ownername as a reasonable name
			remote = pr.Head.OwnerName
			if !git.IsValidRefPattern(remote) {
				// ... let's try something less nice
				remote = "head-pr-" + strconv.FormatInt(pr.Number, 10)
			}
			// ... now add the remote
			if err := g.gitRepo.AddRemote(ctx, remote, pr.Head.CloneURL, true); err != nil {
				log.Error("PR #%d in %s/%s AddRemote[%s] failed: %v", pr.Number, g.repoOwner, g.repoName, remote, err)
				return head, nil
			}
			g.prHeadCache[pr.Head.CloneURL+":"] = remote
		}

		// 4. Check if we already have this ref?
		localRef, ok := g.prHeadCache[pr.Head.CloneURL+":"+pr.Head.Ref]
		if !ok {
			// ... We would normally name this migrated branch as <OwnerName>/<HeadRef> but we need to ensure that is safe
			localRef = git.SanitizeRefPattern(pr.Head.OwnerName + "/" + pr.Head.Ref)

			// ... Now we must assert that this does not exist
			if g.gitRepo.IsBranchExist(ctx, localRef) {
				localRef = "head-pr-" + strconv.FormatInt(pr.Number, 10) + "/" + localRef
				i := 0
				for g.gitRepo.IsBranchExist(ctx, localRef) {
					if i > 5 {
						// ... We tried, we really tried but this is just a seriously unfriendly repo
						return head, nil
					}
					// OK just try some uuids!
					localRef = git.SanitizeRefPattern("head-pr-" + strconv.FormatInt(pr.Number, 10) + uuid.New().String())
					i++
				}
			}

			if headIsLocal {
				if err = git.UpdateRef(ctx, g.repo, git.BranchPrefix+localRef, pr.Head.SHA); err != nil {
					log.Error("Creating branch %s for PR #%d in %s/%s failed: %v", localRef, pr.Number, g.repoOwner, g.repoName, err)
					return head, nil
				}
			} else {
				fetchArg := pr.Head.Ref + ":" + git.BranchPrefix + localRef
				if strings.HasPrefix(fetchArg, "-") {
					fetchArg = git.BranchPrefix + fetchArg
				}

				_, _, err = gitcmd.NewCommand("fetch", "--no-tags").AddDashesAndList(remote, fetchArg).WithRepo(g.repo).RunStdString(ctx)
				if err != nil {
					log.Error("Fetch branch from %s failed: %v", pr.Head.CloneURL, err)
					return head, nil
				}
			}
			g.prHeadCache[pr.Head.CloneURL+":"+pr.Head.Ref] = localRef
			head = localRef
		}

		// 5. Now if pr.Head.SHA == "" we should recover this to the head of this branch
		if pr.Head.SHA == "" {
			headSha, err := g.gitRepo.GetBranchCommitID(ctx, localRef)
			if err != nil {
				log.Error("unable to get head SHA of local head for PR #%d from %s in %s/%s. Error: %v", pr.Number, pr.Head.Ref, g.repoOwner, g.repoName, err)
				return head, nil
			}
			pr.Head.SHA = headSha
		}

		if err = git.UpdateRef(ctx, g.repo, pr.GetGitHeadRefName(), pr.Head.SHA); err != nil {
			return "", err
		}

		return head, nil
	}

	if pr.Head.Ref != "" {
		head = pr.Head.Ref
	}

	// Ensure the closed PR SHA still points to an existing ref
	if pr.Head.SHA == "" {
		// The SHA is empty
		log.Warn("Empty reference, no pull head for PR #%d in %s/%s", pr.Number, g.repoOwner, g.repoName)
	} else if !g.commitExists(ctx, pr.Head.SHA) && !g.fetchCommit(ctx, pr) {
		// Git update-ref remove bad references with a relative path
		log.Warn("Deprecated local head %s for PR #%d in %s/%s, removing  %s", pr.Head.SHA, pr.Number, g.repoOwner, g.repoName, pr.GetGitHeadRefName())
	} else if err = git.UpdateRef(ctx, g.repo, pr.GetGitHeadRefName(), pr.Head.SHA); err != nil {
		log.Error("unable to set %s as the local head for PR #%d from %s in %s/%s. Error: %v", pr.Head.SHA, pr.Number, pr.Head.Ref, g.repoOwner, g.repoName, err)
	}

	return head, nil
}

func (g *GiteaLocalUploader) newPullRequest(ctx context.Context, pr *base.PullRequest) (*issues_model.PullRequest, error) {
	// like the issue path, so PR-only labels and milestones are not dropped
	labels, err := g.ensureLabels(ctx, pr.Labels)
	if err != nil {
		return nil, err
	}

	milestoneID, err := g.ensureMilestone(ctx, pr.Milestone)
	if err != nil {
		return nil, err
	}

	head, err := g.updateGitForPullRequest(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("updateGitForPullRequest: %w", err)
	}

	// Now we may need to fix the mergebase
	if pr.Base.SHA == "" {
		if pr.Base.Ref != "" && pr.Head.SHA != "" {
			// A PR against a tag base does not make sense - therefore pr.Base.Ref must be a branch
			// TODO: should we be checking for the refs/heads/ prefix on the pr.Base.Ref? (i.e. are these actually branches or refs)
			pr.Base.SHA, err = git.MergeBase(ctx, g.repo, git.BranchPrefix+pr.Base.Ref, pr.Head.SHA)
			if err != nil {
				log.Error("Cannot determine the merge base for PR #%d in %s/%s. Error: %v", pr.Number, g.repoOwner, g.repoName, err)
			}
		} else {
			log.Error("Cannot determine the merge base for PR #%d in %s/%s. Not enough information", pr.Number, g.repoOwner, g.repoName)
		}
	}

	if pr.Created.IsZero() {
		if pr.Closed != nil {
			pr.Created = *pr.Closed
		} else if pr.MergedTime != nil {
			pr.Created = *pr.MergedTime
		} else {
			pr.Created = time.Now()
		}
	}
	if pr.Updated.IsZero() {
		pr.Updated = pr.Created
	}

	prTitle := pr.Title
	if pr.IsDraft && !issues_model.HasWorkInProgressPrefix(pr.Title) {
		prTitle = fmt.Sprintf("%s %s", setting.Repository.PullRequest.WorkInProgressPrefixes[0], pr.Title)
	}

	issue := issues_model.Issue{
		RepoID:      g.repo.ID,
		Repo:        g.repo,
		Title:       util.TruncateRunes(prTitle, 255),
		Index:       pr.Number,
		Content:     pr.Content,
		MilestoneID: milestoneID,
		IsPull:      true,
		IsClosed:    pr.State == "closed",
		IsLocked:    pr.IsLocked,
		Labels:      labels,
		CreatedUnix: timeutil.TimeStamp(pr.Created.Unix()),
		UpdatedUnix: timeutil.TimeStamp(pr.Updated.Unix()),
	}

	if err := g.remapUser(ctx, pr, &issue); err != nil {
		return nil, err
	}

	// add reactions
	for _, reaction := range pr.Reactions {
		res := issues_model.Reaction{
			Type:        reaction.Content,
			CreatedUnix: timeutil.TimeStampNow(),
		}
		if err := g.remapUser(ctx, reaction, &res); err != nil {
			return nil, err
		}
		issue.Reactions = append(issue.Reactions, &res)
	}

	pullRequest := issues_model.PullRequest{
		HeadRepoID:    g.repo.ID,
		HeadRepoOwner: pr.Head.OwnerName,
		HeadBranch:    head,
		BaseRepoID:    g.repo.ID,
		BaseBranch:    pr.Base.Ref,
		MergeBase:     pr.Base.SHA,
		Index:         pr.Number,
		HasMerged:     pr.Merged,

		Issue: &issue,
	}

	if pullRequest.Issue.IsClosed && pr.Closed != nil {
		pullRequest.Issue.ClosedUnix = timeutil.TimeStamp(pr.Closed.Unix())
	}
	if pullRequest.HasMerged && pr.MergedTime != nil {
		pullRequest.MergedUnix = timeutil.TimeStamp(pr.MergedTime.Unix())
		pullRequest.MergedCommitID = pr.MergeCommitSHA
		pullRequest.MergerID = g.doer.ID
	}

	// TODO: assignees

	return &pullRequest, nil
}

func convertReviewState(state string) issues_model.ReviewType {
	switch state {
	case base.ReviewStatePending:
		return issues_model.ReviewTypePending
	case base.ReviewStateApproved:
		return issues_model.ReviewTypeApprove
	case base.ReviewStateChangesRequested:
		return issues_model.ReviewTypeReject
	case base.ReviewStateCommented:
		return issues_model.ReviewTypeComment
	case base.ReviewStateRequestReview:
		return issues_model.ReviewTypeRequest
	default:
		return issues_model.ReviewTypePending
	}
}

// CreateReviews create pull request reviews of currently migrated issues
func (g *GiteaLocalUploader) CreateReviews(ctx context.Context, reviews ...*base.Review) error {
	cms := make([]*issues_model.Review, 0, len(reviews))
	var fallbackComments []*issues_model.Comment
	for _, review := range reviews {
		var issue *issues_model.Issue
		issue, ok := g.issues[review.IssueIndex]
		if !ok {
			return fmt.Errorf("review references non existent IssueIndex %d", review.IssueIndex)
		}
		if review.CreatedAt.IsZero() {
			review.CreatedAt = time.Unix(int64(issue.CreatedUnix), 0)
		}

		cm := issues_model.Review{
			Type:        convertReviewState(review.State),
			IssueID:     issue.ID,
			Content:     review.Content,
			Official:    review.Official,
			Dismissed:   review.Dismissed,
			CreatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
			UpdatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
		}

		if err := g.remapUser(ctx, review, &cm); err != nil {
			return err
		}

		cms = append(cms, &cm)

		// get pr
		pr, ok := g.prCache[issue.ID]
		if !ok {
			var err error
			pr, err = issues_model.GetPullRequestByIssueIDWithNoAttributes(ctx, issue.ID)
			if err != nil {
				return err
			}
			g.prCache[issue.ID] = pr
		}
		headCommitID := ""
		if pr.MergeBase == "" {
			log.Warn("PR #%d in %s/%s: does not have a merge base, review comments will be added as plain comments", pr.Index, g.repoOwner, g.repoName)
		} else if id, err := g.gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName()); err != nil {
			log.Warn("PR #%d GetRefCommitID[%s] in %s/%s: %v, review comments will be added as plain comments", pr.Index, pr.GetGitHeadRefName(), g.repoOwner, g.repoName, err)
		} else {
			headCommitID = id
		}

		if headCommitID == "" {
			// without a diff to anchor to, keep the text as plain comments instead of an empty review
			for _, comment := range review.Comments {
				c := issues_model.Comment{
					Type:        issues_model.CommentTypeComment,
					IssueID:     issue.ID,
					Content:     fmt.Sprintf("Comment on `%s` line %d:\n\n%s", util.PathJoinRel(comment.TreePath), max(comment.Line, -comment.Line), comment.Content),
					CreatedUnix: timeutil.TimeStamp(cmp.Or(comment.CreatedAt, review.CreatedAt).Unix()),
					UpdatedUnix: timeutil.TimeStamp(cmp.Or(comment.UpdatedAt, comment.CreatedAt, review.CreatedAt).Unix()),
				}
				if err := g.remapUser(ctx, review, &c); err != nil {
					return err
				}
				fallbackComments = append(fallbackComments, &c)
			}
			if len(review.Comments) > 0 && cm.Type == issues_model.ReviewTypeComment && cm.Content == "" {
				cms = cms[:len(cms)-1]
			}
			continue
		}

		for _, comment := range review.Comments {
			line := comment.Line
			if line != 0 {
				comment.Position = 1
			} else if comment.DiffHunk != "" {
				_, _, line, _ = git.ParseDiffHunkString(comment.DiffHunk)
			}

			// SECURITY: The TreePath must be cleaned! use relative path
			comment.TreePath = util.PathJoinRel(comment.TreePath)

			patch, _ := gitdiff.GetCodeCommentPatch(ctx, g.gitRepo, pr.MergeBase, headCommitID, comment.TreePath, int64(line+comment.Position-1))

			if comment.CreatedAt.IsZero() {
				comment.CreatedAt = review.CreatedAt
			}
			if comment.UpdatedAt.IsZero() {
				comment.UpdatedAt = comment.CreatedAt
			}

			objectFormat := git.ObjectFormatFromName(g.repo.ObjectFormatName)
			if !git.IsStringValidObjectID(objectFormat, comment.CommitID) {
				log.Warn("Invalid comment CommitID[%s] on comment[%d] in PR #%d of %s/%s replaced with %s", comment.CommitID, pr.Index, g.repoOwner, g.repoName, headCommitID)
				comment.CommitID = headCommitID
			}

			c := issues_model.Comment{
				Type:        issues_model.CommentTypeCode,
				IssueID:     issue.ID,
				Content:     comment.Content,
				Line:        int64(line + comment.Position - 1),
				TreePath:    comment.TreePath,
				CommitSHA:   comment.CommitID,
				Patch:       patch,
				CreatedUnix: timeutil.TimeStamp(comment.CreatedAt.Unix()),
				UpdatedUnix: timeutil.TimeStamp(comment.UpdatedAt.Unix()),
			}

			if comment.Resolved {
				c.ResolveDoerID = g.doer.ID // the original resolver is unknown to the migration data
			}

			if err := g.remapUser(ctx, review, &c); err != nil {
				return err
			}

			cm.Comments = append(cm.Comments, &c)
		}
	}

	if err := issues_model.InsertReviews(ctx, cms); err != nil {
		return err
	}
	return issues_model.InsertIssueComments(ctx, fallbackComments)
}

// PatchReviews upserts reviews, matched on their remote id
func (g *GiteaLocalUploader) PatchReviews(ctx context.Context, reviews ...*base.Review) error {
	cms, err := g.prepareReviews(ctx, reviews...)
	if err != nil {
		return err
	}
	return issues_model.UpsertReviews(ctx, cms)
}

func (g *GiteaLocalUploader) prepareReviews(ctx context.Context, reviews ...*base.Review) ([]*issues_model.Review, error) {
	cms := make([]*issues_model.Review, 0, len(reviews))
	for _, review := range reviews {
		issue, err := g.getIssueByIndex(ctx, review.IssueIndex)
		if err != nil {
			return nil, err
		}
		if review.CreatedAt.IsZero() {
			review.CreatedAt = time.Unix(int64(issue.CreatedUnix), 0)
		}

		cm := issues_model.Review{
			Type:        convertReviewState(review.State),
			IssueID:     issue.ID,
			Content:     review.Content,
			Official:    review.Official,
			Dismissed:   review.Dismissed,
			OriginalID:  review.ID,
			CreatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
			UpdatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
		}

		if err := g.remapUser(ctx, review, &cm); err != nil {
			return nil, err
		}

		cms = append(cms, &cm)

		// get pr
		pr, ok := g.prCache[issue.ID]
		if !ok {
			var err error
			pr, err = issues_model.GetPullRequestByIssueIDWithNoAttributes(ctx, issue.ID)
			if err != nil {
				return nil, err
			}
			g.prCache[issue.ID] = pr
		}
		if pr.MergeBase == "" {
			// No mergebase -> no basis for any patches
			log.Warn("PR #%d in %s/%s: does not have a merge base, all review comments will be ignored", pr.Index, g.repoOwner, g.repoName)
			continue
		}

		headCommitID, err := g.gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
		if err != nil {
			log.Warn("PR #%d GetRefCommitID[%s] in %s/%s: %v, all review comments will be ignored", pr.Index, pr.GetGitHeadRefName(), g.repoOwner, g.repoName, err)
			continue
		}

		for _, comment := range review.Comments {
			line := comment.Line
			if line != 0 {
				comment.Position = 1
			} else if comment.DiffHunk != "" {
				_, _, line, _ = git.ParseDiffHunkString(comment.DiffHunk)
			}

			// SECURITY: The TreePath must be cleaned! use relative path
			comment.TreePath = util.PathJoinRel(comment.TreePath)

			patch, _ := git.GetFileDiffCutAroundLine(ctx,
				g.gitRepo, pr.MergeBase, headCommitID, comment.TreePath,
				int64((&issues_model.Comment{Line: int64(line + comment.Position - 1)}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines,
			)

			if comment.CreatedAt.IsZero() {
				comment.CreatedAt = review.CreatedAt
			}
			if comment.UpdatedAt.IsZero() {
				comment.UpdatedAt = comment.CreatedAt
			}

			objectFormat := git.ObjectFormatFromName(g.repo.ObjectFormatName)
			if !git.IsStringValidObjectID(objectFormat, comment.CommitID) {
				log.Warn("Invalid comment CommitID[%s] on comment[%d] in PR #%d of %s/%s replaced with %s", comment.CommitID, pr.Index, g.repoOwner, g.repoName, headCommitID)
				comment.CommitID = headCommitID
			}

			c := issues_model.Comment{
				Type:        issues_model.CommentTypeCode,
				IssueID:     issue.ID,
				Content:     comment.Content,
				Line:        int64(line + comment.Position - 1),
				TreePath:    comment.TreePath,
				CommitSHA:   comment.CommitID,
				Patch:       patch,
				OriginalID:  comment.ID,
				CreatedUnix: timeutil.TimeStamp(comment.CreatedAt.Unix()),
				UpdatedUnix: timeutil.TimeStamp(comment.UpdatedAt.Unix()),
			}

			if comment.Resolved {
				c.ResolveDoerID = g.doer.ID // the original resolver is unknown to the migration data
			}

			if err := g.remapUser(ctx, review, &c); err != nil {
				return nil, err
			}

			cm.Comments = append(cm.Comments, &c)
		}
	}

	return cms, nil
}

// Rollback when migrating failed, this will rollback all the changes.
func (g *GiteaLocalUploader) Rollback() error {
	if g.repo != nil && g.repo.ID > 0 && g.gitRepo != nil {
		g.gitRepo.Close()

		// do not delete the repository, otherwise the end users won't be able to see the last error message
	}
	return nil
}

// Finish when migrating success, this will do some status update things.
func (g *GiteaLocalUploader) Finish(ctx context.Context) error {
	if g.repo == nil || g.repo.ID <= 0 {
		return ErrRepoNotCreated
	}

	// update issue_index
	if err := issues_model.RecalculateIssueIndexForRepo(ctx, g.repo.ID); err != nil {
		return err
	}

	if err := repostats.UpdateRepoStats(ctx, g.repo.ID); err != nil {
		return err
	}

	g.repo.Status = repo_model.RepositoryReady
	return repo_model.UpdateRepositoryColsWithAutoTime(ctx, g.repo, "status")
}

func (g *GiteaLocalUploader) remapUser(ctx context.Context, source user_model.ExternalUserMigrated, target user_model.ExternalUserRemappable) error {
	var userID int64
	var err error
	if g.sameApp {
		userID, err = g.remapLocalUser(ctx, source)
	} else {
		userID, err = g.remapExternalUser(ctx, source)
	}
	if err != nil {
		return err
	}

	if userID > 0 {
		return target.RemapExternalUser("", 0, userID)
	}
	return target.RemapExternalUser(source.GetExternalName(), source.GetExternalID(), g.doer.ID)
}

func (g *GiteaLocalUploader) remapLocalUser(ctx context.Context, source user_model.ExternalUserMigrated) (int64, error) {
	userid, ok := g.userMap[source.GetExternalID()]
	if !ok {
		user, err := user_model.GetUserByID(ctx, source.GetExternalID())
		if errors.Is(err, util.ErrNotExist) {
			g.userMap[source.GetExternalID()] = userid
			return 0, nil
		} else if err != nil {
			return 0, err
		}
		// let's not reuse an ID when the user was deleted or has a different username
		if !util.AsciiEqualFold(user.Name, source.GetExternalName()) {
			userid = 0
		} else {
			userid = source.GetExternalID()
		}
		g.userMap[source.GetExternalID()] = userid
	}
	return userid, nil
}

func (g *GiteaLocalUploader) remapExternalUser(ctx context.Context, source user_model.ExternalUserMigrated) (userid int64, err error) {
	userid, ok := g.userMap[source.GetExternalID()]
	if !ok {
		userid, err = user_model.GetUserIDByExternalUserID(ctx, g.gitServiceType.Name(), strconv.FormatInt(source.GetExternalID(), 10))
		if err != nil {
			log.Error("GetUserIDByExternalUserID: %v", err)
			return 0, err
		}
		g.userMap[source.GetExternalID()] = userid
	}
	return userid, nil
}
