// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/pull"

	gouuid "github.com/google/uuid"
	gofff "lab.forgefriends.org/friendlyforgeformat/gofff"
	gofff_gitea "lab.forgefriends.org/friendlyforgeformat/gofff/forges/gitea"
	gofff_null "lab.forgefriends.org/friendlyforgeformat/gofff/forges/null"
	gofff_format "lab.forgefriends.org/friendlyforgeformat/gofff/format"
)

var _ gofff.ForgeInterface = &GiteaLocalUploader{}

// GiteaLocalUploader implements an Uploader to gitea sites
type GiteaLocalUploader struct {
	gofff_null.Null
	opts           base.MigrateOptions
	ctx            context.Context
	doer           *user_model.User
	repoOwner      string
	repoName       string
	repo           *repo_model.Repository
	labels         map[string]*issues_model.Label
	milestones     map[string]int64
	issues         map[int64]*issues_model.Issue
	gitRepo        *git.Repository
	prHeadCache    gofff_gitea.PrHeadCache
	sameApp        bool
	userMap        map[int64]int64 // external user id mapping to user id
	prCache        map[int64]*issues_model.PullRequest
	gitServiceType structs.GitServiceType
}

// NewGiteaLocalUploader creates an gitea Uploader via gitea API v1
func NewGiteaLocalUploader(ctx context.Context, doer *user_model.User, repoOwner string, opts base.MigrateOptions) *GiteaLocalUploader {
	return &GiteaLocalUploader{
		opts:        opts,
		ctx:         ctx,
		doer:        doer,
		repoOwner:   repoOwner,
		repoName:    opts.RepoName,
		labels:      make(map[string]*issues_model.Label),
		milestones:  make(map[string]int64),
		issues:      make(map[int64]*issues_model.Issue),
		prHeadCache: make(gofff_gitea.PrHeadCache),
		userMap:     make(map[int64]int64),
		prCache:     make(map[int64]*issues_model.PullRequest),
	}
}

func (g *GiteaLocalUploader) SetContext(ctx context.Context) {
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
		return db.MaxBatchInsertSize(new(models.Release))
	case "pullrequest":
		return db.MaxBatchInsertSize(new(issues_model.PullRequest))
	}
	return 10
}

// CreateRepo creates a repository
func (g *GiteaLocalUploader) CreateProject(project *gofff_format.Project) {
	owner, err := user_model.GetUserByName(g.ctx, g.repoOwner)
	if err != nil {
		panic(err)
	}

	var r *repo_model.Repository
	if g.opts.MigrateToRepoID <= 0 {
		r, err = repo_module.CreateRepository(g.doer, owner, models.CreateRepoOptions{
			Name:           g.repoName,
			Description:    project.Description,
			OriginalURL:    project.OriginalURL,
			GitServiceType: g.opts.GitServiceType,
			IsPrivate:      g.opts.Private,
			IsMirror:       g.opts.Mirror,
			Status:         repo_model.RepositoryBeingMigrated,
			DefaultBranch:  project.DefaultBranch,
		})
	} else {
		r, err = repo_model.GetRepositoryByID(g.opts.MigrateToRepoID)
	}
	if err != nil {
		panic(err)
	}
	r.Description = project.Description

	g.sameApp = strings.HasPrefix(project.OriginalURL, setting.AppURL)
}

// CreateRepo creates a repository
func (g *GiteaLocalUploader) CreateRepositories(repositories ...*gofff_format.Repository) {
	owner, err := user_model.GetUserByName(g.ctx, g.repoOwner)
	if err != nil {
		panic(err)
	}

	r, err := repo_model.GetRepositoryByOwnerAndNameCtx(g.ctx, g.repoOwner, g.repoName)
	if err != nil {
		panic(err)
	}

	for _, repository := range repositories {
		switch repository.Name {
		case gofff_format.RepositoryNameDefault:
			r, err = repo_module.MigrateRepositoryGitData(g.ctx, owner, repository.Fetch, r, g.opts, NewMigrationHTTPTransport())

			g.repo = r
			if err != nil {
				panic(err)
			}

			g.gitRepo, err = git.OpenRepository(g.ctx, r.RepoPath())
			if err != nil {
				panic(err)
			}

		case gofff_format.RepositoryNameWiki:
			if g.opts.Wiki {
				err = repo_module.MigrateRepositoryGitDataWiki(g.ctx, owner, repository.Fetch, r, g.opts)
				if err != nil {
					panic(err)
				}
			}

		default:
			panic(fmt.Errorf("unknown repository name %v", repository.Name))
		}
	}
}

// Close closes this uploader
func (g *GiteaLocalUploader) Close() {
	if g.gitRepo != nil {
		g.gitRepo.Close()
	}
}

// CreateTopics creates topics
func (g *GiteaLocalUploader) CreateTopics(topics ...*gofff_format.Topic) {
	// ignore topics too long for the db
	trimmedTopics := make([]string, 0, len(topics))
	for i := range topics {
		if len(topics[i].Name) <= 50 {
			trimmedTopics = append(trimmedTopics, topics[i].Name)
		}
	}
	if err := repo_model.SaveTopics(g.repo.ID, trimmedTopics...); err != nil {
		panic(err)
	}
}

// CreateMilestones creates milestones
func (g *GiteaLocalUploader) CreateMilestones(milestones ...*gofff_format.Milestone) {
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

	err := models.InsertMilestones(mss...)
	if err != nil {
		panic(err)
	}

	for _, ms := range mss {
		g.milestones[ms.Name] = ms.ID
	}
}

// CreateLabels creates labels
func (g *GiteaLocalUploader) CreateLabels(labels ...*gofff_format.Label) {
	lbs := make([]*issues_model.Label, 0, len(labels))
	for _, label := range labels {
		lbs = append(lbs, &issues_model.Label{
			RepoID:      g.repo.ID,
			Name:        label.Name,
			Description: label.Description,
			Color:       fmt.Sprintf("#%s", label.Color),
		})
	}

	err := issues_model.NewLabels(lbs...)
	if err != nil {
		panic(err)
	}
	for _, lb := range lbs {
		g.labels[lb.Name] = lb
	}
}

// CreateReleases creates releases
func (g *GiteaLocalUploader) CreateReleases(releases ...*gofff_format.Release) {
	rels := make([]*models.Release, 0, len(releases))
	for _, release := range releases {
		if release.Created.IsZero() {
			if !release.Published.IsZero() {
				release.Created = release.Published
			} else {
				release.Created = time.Now()
			}
		}

		rel := models.Release{
			RepoID:       g.repo.ID,
			TagName:      release.TagName,
			LowerTagName: strings.ToLower(release.TagName),
			Target:       release.TargetCommitish,
			Title:        release.Name,
			Note:         release.Body,
			IsDraft:      release.Draft,
			IsPrerelease: release.Prerelease,
			IsTag:        false,
			CreatedUnix:  timeutil.TimeStamp(release.Created.Unix()),
		}

		if err := g.remapUser(release, &rel); err != nil {
			panic(err)
		}

		// calc NumCommits if possible
		if rel.TagName != "" {
			commit, err := g.gitRepo.GetTagCommit(rel.TagName)
			if !git.IsErrNotExist(err) {
				if err != nil {
					panic(fmt.Errorf("GetTagCommit[%v]: %v", rel.TagName, err))
				}
				rel.Sha1 = commit.ID.String()
				rel.NumCommits, err = commit.CommitsCount()
				if err != nil {
					panic(fmt.Errorf("CommitsCount: %v", err))
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
				UUID:          gouuid.New().String(),
				Name:          asset.Name,
				DownloadCount: int64(*asset.DownloadCount),
				Size:          int64(*asset.Size),
				CreatedUnix:   timeutil.TimeStamp(asset.Created.Unix()),
			}

			// download attachment
			err := func() error {
				rc := asset.DownloadFunc()
				if rc == nil {
					return nil
				}
				_, err := storage.Attachments.Save(attach.RelativePath(), rc, int64(*asset.Size))
				rc.Close()
				return err
			}()
			if err != nil {
				panic(err)
			}

			rel.Attachments = append(rel.Attachments, &attach)
		}

		rels = append(rels, &rel)
	}

	if err := models.InsertReleases(rels...); err != nil {
		panic(err)
	}
}

// SyncTags syncs releases with tags in the database
func (g *GiteaLocalUploader) SyncTags() error {
	return repo_module.SyncReleasesWithTags(g.repo, g.gitRepo)
}

// CreateIssues creates issues
func (g *GiteaLocalUploader) CreateIssues(issues ...*gofff_format.Issue) {
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

		is := issues_model.Issue{
			RepoID:      g.repo.ID,
			Repo:        g.repo,
			Index:       issue.Number,
			Title:       issue.Title,
			Content:     issue.Content,
			Ref:         issue.Ref,
			IsClosed:    issue.State == "closed",
			IsLocked:    issue.IsLocked,
			MilestoneID: milestoneID,
			Labels:      labels,
			CreatedUnix: timeutil.TimeStamp(issue.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(issue.Updated.Unix()),
		}

		if err := g.remapUser(issue, &is); err != nil {
			panic(err)
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
			if err := g.remapUser(reaction, &res); err != nil {
				panic(err)
			}
			is.Reactions = append(is.Reactions, &res)
		}
		iss = append(iss, &is)
	}

	if len(iss) > 0 {
		if err := models.InsertIssues(iss...); err != nil {
			panic(err)
		}

		for _, is := range iss {
			g.issues[is.Index] = is
		}
	}
}

// CreateComments creates comments of issues
func (g *GiteaLocalUploader) CreateComments(commentable gofff_format.Commentable, comments ...*gofff_format.Comment) {
	cms := make([]*issues_model.Comment, 0, len(comments))
	for _, comment := range comments {
		var issue *issues_model.Issue
		issue, ok := g.issues[comment.IssueIndex]
		if !ok {
			panic(fmt.Errorf("comment references non existent IssueIndex %d", comment.IssueIndex))
		}

		if comment.Created.IsZero() {
			comment.Created = time.Unix(int64(issue.CreatedUnix), 0)
		}
		if comment.Updated.IsZero() {
			comment.Updated = comment.Created
		}

		cm := issues_model.Comment{
			IssueID:     issue.ID,
			Type:        issues_model.CommentTypeComment,
			Content:     comment.Content,
			CreatedUnix: timeutil.TimeStamp(comment.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(comment.Updated.Unix()),
		}

		if err := g.remapUser(comment, &cm); err != nil {
			panic(err)
		}

		// add reactions
		for _, reaction := range comment.Reactions {
			res := issues_model.Reaction{
				Type:        reaction.Content,
				CreatedUnix: timeutil.TimeStampNow(),
			}
			if err := g.remapUser(reaction, &res); err != nil {
				panic(err)
			}
			cm.Reactions = append(cm.Reactions, &res)
		}

		cms = append(cms, &cm)
	}

	if len(cms) == 0 {
		return
	}
	if err := models.InsertIssueComments(cms); err != nil {
		panic(err)
	}
}

// CreatePullRequests creates pull requests
func (g *GiteaLocalUploader) CreatePullRequests(prs ...*gofff_format.PullRequest) {
	gprs := make([]*issues_model.PullRequest, 0, len(prs))
	for _, pr := range prs {
		gpr, err := g.newPullRequest(pr)
		if err != nil {
			panic(err)
		}

		if err := g.remapUser(pr, gpr.Issue); err != nil {
			panic(err)
		}

		gprs = append(gprs, gpr)
	}
	if err := models.InsertPullRequests(gprs...); err != nil {
		panic(err)
	}
	for _, pr := range gprs {
		g.issues[pr.Issue.Index] = pr.Issue
		pull.AddToTaskQueue(pr)
	}
}

func (g *GiteaLocalUploader) newPullRequest(pr *gofff_format.PullRequest) (*issues_model.PullRequest, error) {
	var labels []*issues_model.Label
	for _, label := range pr.Labels {
		lb, ok := g.labels[label.Name]
		if ok {
			labels = append(labels, lb)
		}
	}

	milestoneID := g.milestones[pr.Milestone]

	_ = pr.Fetch(g.repo.RepoPath())
	head, messages := gofff_gitea.UpdateGitForPullRequest(g.ctx, &g.prHeadCache, pr, g.repo.RepoPath())
	for _, message := range messages {
		log.Error(message)
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

	issue := issues_model.Issue{
		RepoID:      g.repo.ID,
		Repo:        g.repo,
		Title:       pr.Title,
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

	if err := g.remapUser(pr, &issue); err != nil {
		return nil, err
	}

	// add reactions
	for _, reaction := range pr.Reactions {
		res := issues_model.Reaction{
			Type:        reaction.Content,
			CreatedUnix: timeutil.TimeStampNow(),
		}
		if err := g.remapUser(reaction, &res); err != nil {
			return nil, err
		}
		issue.Reactions = append(issue.Reactions, &res)
	}

	pullRequest := issues_model.PullRequest{
		HeadRepoID: g.repo.ID,
		HeadBranch: head,
		BaseRepoID: g.repo.ID,
		BaseBranch: pr.Base.Ref,
		MergeBase:  pr.Base.SHA,
		Index:      pr.Number,
		HasMerged:  pr.Merged,

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
	case gofff_format.ReviewStatePending:
		return issues_model.ReviewTypePending
	case gofff_format.ReviewStateApproved:
		return issues_model.ReviewTypeApprove
	case gofff_format.ReviewStateChangesRequested:
		return issues_model.ReviewTypeReject
	case gofff_format.ReviewStateCommented:
		return issues_model.ReviewTypeComment
	case gofff_format.ReviewStateRequestReview:
		return issues_model.ReviewTypeRequest
	default:
		return issues_model.ReviewTypePending
	}
}

// CreateReviews create pull request reviews of currently migrated issues
func (g *GiteaLocalUploader) CreateReviews(reviewable gofff_format.Reviewable, reviews ...*gofff_format.Review) {
	cms := make([]*issues_model.Review, 0, len(reviews))
	for _, review := range reviews {
		var issue *issues_model.Issue
		issue, ok := g.issues[review.IssueIndex]
		if !ok {
			panic(fmt.Errorf("review references non existent IssueIndex %d", review.IssueIndex))
		}
		if review.CreatedAt.IsZero() {
			review.CreatedAt = time.Unix(int64(issue.CreatedUnix), 0)
		}

		cm := issues_model.Review{
			Type:        convertReviewState(review.State),
			IssueID:     issue.ID,
			Content:     review.Content,
			Official:    review.Official,
			CreatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
			UpdatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
		}

		if err := g.remapUser(review, &cm); err != nil {
			panic(err)
		}

		// get pr
		pr, ok := g.prCache[issue.ID]
		if !ok {
			var err error
			pr, err = issues_model.GetPullRequestByIssueIDWithNoAttributes(issue.ID)
			if err != nil {
				panic(err)
			}
			g.prCache[issue.ID] = pr
		}

		for _, comment := range review.Comments {
			line := comment.Line
			if line != 0 {
				comment.Position = 1
			} else {
				_, _, line, _ = git.ParseDiffHunkString(comment.DiffHunk)
			}
			headCommitID, err := g.gitRepo.GetRefCommitID(pr.GetGitRefName())
			if err != nil {
				log.Warn("GetRefCommitID[%s]: %v, the review comment will be ignored", pr.GetGitRefName(), err)
				continue
			}

			var patch string
			reader, writer := io.Pipe()
			defer func() {
				_ = reader.Close()
				_ = writer.Close()
			}()
			go func(comment *gofff_format.ReviewComment) {
				if err := git.GetRepoRawDiffForFile(g.gitRepo, pr.MergeBase, headCommitID, git.RawDiffNormal, comment.TreePath, writer); err != nil {
					// We should ignore the error since the commit maybe removed when force push to the pull request
					log.Warn("GetRepoRawDiffForFile failed when migrating [%s, %s, %s, %s]: %v", g.gitRepo.Path, pr.MergeBase, headCommitID, comment.TreePath, err)
				}
				_ = writer.Close()
			}(comment)

			patch, _ = git.CutDiffAroundLine(reader, int64((&issues_model.Comment{Line: int64(line + comment.Position - 1)}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines)

			if comment.CreatedAt.IsZero() {
				comment.CreatedAt = review.CreatedAt
			}
			if comment.UpdatedAt.IsZero() {
				comment.UpdatedAt = comment.CreatedAt
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

			if err := g.remapUser(review, &c); err != nil {
				panic(err)
			}

			cm.Comments = append(cm.Comments, &c)
		}

		cms = append(cms, &cm)
	}

	if err := issues_model.InsertReviews(cms); err != nil {
		panic(err)
	}
}

// Rollback when migrating failed, this will rollback all the changes.
func (g *GiteaLocalUploader) Rollback() {
	if g.repo != nil && g.repo.ID > 0 {
		g.gitRepo.Close()
		if err := models.DeleteRepository(g.doer, g.repo.OwnerID, g.repo.ID); err != nil {
			panic(err)
		}
	}
}

// Finish when migrating success, this will do some status update things.
func (g *GiteaLocalUploader) Finish() {
	if g.repo == nil || g.repo.ID <= 0 {
		panic(ErrRepoNotCreated)
	}

	// update issue_index
	if err := issues_model.RecalculateIssueIndexForRepo(g.repo.ID); err != nil {
		panic(err)
	}

	if err := models.UpdateRepoStats(g.ctx, g.repo.ID); err != nil {
		panic(err)
	}

	g.repo.Status = repo_model.RepositoryReady
	if err := repo_model.UpdateRepositoryCols(g.ctx, g.repo, "status"); err != nil {
		panic(err)
	}
}

func (g *GiteaLocalUploader) remapUser(source user_model.ExternalUserMigrated, target user_model.ExternalUserRemappable) error {
	var userid int64
	var err error
	if g.sameApp {
		userid, err = g.remapLocalUser(source, target)
	} else {
		userid, err = g.remapExternalUser(source, target)
	}

	if err != nil {
		return err
	}

	if userid > 0 {
		return target.RemapExternalUser("", 0, userid)
	}
	return target.RemapExternalUser(source.GetExternalName(), source.GetExternalID(), g.doer.ID)
}

func (g *GiteaLocalUploader) remapLocalUser(source user_model.ExternalUserMigrated, target user_model.ExternalUserRemappable) (int64, error) {
	userid, ok := g.userMap[source.GetExternalID()]
	if !ok {
		name, err := user_model.GetUserNameByID(g.ctx, source.GetExternalID())
		if err != nil {
			return 0, err
		}
		// let's not reuse an ID when the user was deleted or has a different user name
		if name != source.GetExternalName() {
			userid = 0
		} else {
			userid = source.GetExternalID()
		}
		g.userMap[source.GetExternalID()] = userid
	}
	return userid, nil
}

func (g *GiteaLocalUploader) remapExternalUser(source user_model.ExternalUserMigrated, target user_model.ExternalUserRemappable) (userid int64, err error) {
	userid, ok := g.userMap[source.GetExternalID()]
	if !ok {
		userid, err = user_model.GetUserIDByExternalUserID(g.gitServiceType.Name(), fmt.Sprintf("%d", source.GetExternalID()))
		if err != nil {
			log.Error("GetUserIDByExternalUserID: %v", err)
			return 0, err
		}
		g.userMap[source.GetExternalID()] = userid
	}
	return userid, nil
}
