// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
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
	"code.gitea.io/gitea/modules/uri"
	"code.gitea.io/gitea/services/pull"

	"github.com/google/uuid"
)

var _ base.Uploader = &GiteaLocalUploader{}

// GiteaLocalUploader implements an Uploader to gitea sites
type GiteaLocalUploader struct {
	ctx            context.Context
	doer           *user_model.User
	repoOwner      string
	repoName       string
	repo           *repo_model.Repository
	labels         map[string]*issues_model.Label
	projects       map[string]*project_model.Project
	milestones     map[string]int64
	issues         map[int64]*issues_model.Issue
	gitRepo        *git.Repository
	prHeadCache    map[string]string
	sameApp        bool
	userMap        map[int64]int64 // external user id mapping to user id
	prCache        map[int64]*issues_model.PullRequest
	gitServiceType structs.GitServiceType
}

// NewGiteaLocalUploader creates an gitea Uploader via gitea API v1
func NewGiteaLocalUploader(ctx context.Context, doer *user_model.User, repoOwner, repoName string) *GiteaLocalUploader {
	return &GiteaLocalUploader{
		ctx:         ctx,
		doer:        doer,
		repoOwner:   repoOwner,
		repoName:    repoName,
		labels:      make(map[string]*issues_model.Label),
		projects:    make(map[string]*project_model.Project),
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
	case "project":
		return db.MaxBatchInsertSize(new(project_model.Project))
	case "release":
		return db.MaxBatchInsertSize(new(repo_model.Release))
	case "pullrequest":
		return db.MaxBatchInsertSize(new(issues_model.PullRequest))
	}
	return 10
}

// CreateRepo creates a repository
func (g *GiteaLocalUploader) CreateRepo(repo *base.Repository, opts base.MigrateOptions) error {
	owner, err := user_model.GetUserByName(g.ctx, g.repoOwner)
	if err != nil {
		return err
	}

	var r *repo_model.Repository
	if opts.MigrateToRepoID <= 0 {
		r, err = repo_module.CreateRepository(g.doer, owner, repo_module.CreateRepoOptions{
			Name:           g.repoName,
			Description:    repo.Description,
			OriginalURL:    repo.OriginalURL,
			GitServiceType: opts.GitServiceType,
			IsPrivate:      opts.Private,
			IsMirror:       opts.Mirror,
			Status:         repo_model.RepositoryBeingMigrated,
		})
	} else {
		r, err = repo_model.GetRepositoryByID(g.ctx, opts.MigrateToRepoID)
	}
	if err != nil {
		return err
	}
	r.DefaultBranch = repo.DefaultBranch
	r.Description = repo.Description

	r, err = repo_module.MigrateRepositoryGitData(g.ctx, owner, r, base.MigrateOptions{
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
	}, NewMigrationHTTPTransport())

	g.sameApp = strings.HasPrefix(repo.OriginalURL, setting.AppURL)
	g.repo = r
	if err != nil {
		return err
	}
	g.gitRepo, err = git.OpenRepository(g.ctx, r.RepoPath())
	return err
}

// Close closes this uploader
func (g *GiteaLocalUploader) Close() {
	if g.gitRepo != nil {
		g.gitRepo.Close()
	}
}

// CreateTopics creates topics
func (g *GiteaLocalUploader) CreateTopics(topics ...string) error {
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
	return repo_model.SaveTopics(g.repo.ID, topics...)
}

// CreateMilestones creates milestones
func (g *GiteaLocalUploader) CreateMilestones(milestones ...*base.Milestone) error {
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
		return err
	}

	for _, ms := range mss {
		g.milestones[ms.Name] = ms.ID
	}
	return nil
}

// CreateLabels creates labels
func (g *GiteaLocalUploader) CreateLabels(labels ...*base.Label) error {
	lbs := make([]*issues_model.Label, 0, len(labels))
	for _, label := range labels {
		// We must validate color here:
		if !issues_model.LabelColorPattern.MatchString("#" + label.Color) {
			log.Warn("Invalid label color: #%s for label: %s in migration to %s/%s", label.Color, label.Name, g.repoOwner, g.repoName)
			label.Color = "ffffff"
		}

		lbs = append(lbs, &issues_model.Label{
			RepoID:      g.repo.ID,
			Name:        label.Name,
			Description: label.Description,
			Color:       "#" + label.Color,
		})
	}

	err := issues_model.NewLabels(lbs...)
	if err != nil {
		return err
	}
	for _, lb := range lbs {
		g.labels[lb.Name] = lb
	}
	return nil
}

// CreateProjects creates projects
func (g *GiteaLocalUploader) CreateProjects(projects ...*base.Project) error {
	for _, project := range projects {

		dbProject := &project_model.Project{
			RepoID:      g.repo.ID,
			Title:       project.Title,
			Description: project.Description,
		}

		switch project.Type {
		case "individual":
			dbProject.Type = project_model.TypeIndividual
		case "repository":
			dbProject.Type = project_model.TypeRepository
		case "organization":
			dbProject.Type = project_model.TypeOrganization
		default:
			return fmt.Errorf("project %q has unknown type %q", project.Title, project.Type)
		}

		err := project_model.NewProject(dbProject)
		if err != nil {
			return err
		}

		g.projects[dbProject.Title] = dbProject
	}

	return nil
}

// CreateReleases creates releases
func (g *GiteaLocalUploader) CreateReleases(releases ...*base.Release) error {
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

		rel := repo_model.Release{
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
			return err
		}

		// calc NumCommits if possible
		if rel.TagName != "" {
			commit, err := g.gitRepo.GetTagCommit(rel.TagName)
			if !git.IsErrNotExist(err) {
				if err != nil {
					return fmt.Errorf("GetTagCommit[%v]: %w", rel.TagName, err)
				}
				rel.Sha1 = commit.ID.String()
				rel.NumCommits, err = commit.CommitsCount()
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
				Name:          asset.Name,
				DownloadCount: int64(*asset.DownloadCount),
				Size:          int64(*asset.Size),
				CreatedUnix:   timeutil.TimeStamp(asset.Created.Unix()),
			}

			// SECURITY: We cannot check the DownloadURL and DownloadFunc are safe here
			// ... we must assume that they are safe and simply download the attachment
			err := func() error {
				// asset.DownloadURL maybe a local file
				var rc io.ReadCloser
				var err error
				if asset.DownloadFunc != nil {
					rc, err = asset.DownloadFunc()
					if err != nil {
						return err
					}
				} else if asset.DownloadURL != nil {
					rc, err = uri.Open(*asset.DownloadURL)
					if err != nil {
						return err
					}
				}
				if rc == nil {
					return nil
				}
				_, err = storage.Attachments.Save(attach.RelativePath(), rc, int64(*asset.Size))
				rc.Close()
				return err
			}()
			if err != nil {
				return err
			}

			rel.Attachments = append(rel.Attachments, &attach)
		}

		rels = append(rels, &rel)
	}

	return models.InsertReleases(rels...)
}

// SyncTags syncs releases with tags in the database
func (g *GiteaLocalUploader) SyncTags() error {
	return repo_module.SyncReleasesWithTags(g.repo, g.gitRepo)
}

// CreateIssues creates issues
func (g *GiteaLocalUploader) CreateIssues(issues ...*base.Issue) error {
	iss := make([]*issues_model.Issue, 0, len(issues))

	usernameCache := NewUsernameCache()

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
			if err := g.remapUser(reaction, &res); err != nil {
				return err
			}
			is.Reactions = append(is.Reactions, &res)
		}

		// add assignees
		for _, assigneeUsername := range issue.Assignees {
			assigneeID, err := usernameCache.FindUserIDByName(g.ctx, assigneeUsername)
			if err != nil {
				log.Debug("Assignees of issue %d: Unable to get ID of user name %s: %v", issue.Number, assigneeUsername, err)
				continue
			}
			if assigneeID == 0 {
				continue
			}

			assignee, err := usernameCache.GetUserByID(g.ctx, assigneeID)
			if err != nil {
				log.Error("Assignees of issue %d: Unable to get user with ID %d: %v", issue.Number, assigneeID, err)
				continue
			}
			if assignee == nil {
				continue
			}

			is.Assignees = append(is.Assignees, assignee)
		}

		iss = append(iss, &is)
	}

	if len(iss) > 0 {
		if err := models.InsertIssues(iss...); err != nil {
			return err
		}

		for _, is := range iss {
			g.issues[is.Index] = is
		}
	}

	// Subscriber.
	// Only do it after the insert to the database, so that the issue ID is known.
	for _, issue := range issues {
		issueID := g.issues[issue.Number].ID
		issueWatchers := make([]interface{}, 0)

		for _, subscriberUsername := range issue.Subscribers {
			subscriberID, err := usernameCache.FindUserIDByName(g.ctx, subscriberUsername)
			if err != nil {
				log.Debug("Subscribers of issue %d: Unable to get ID of user name %s: %v", issue.Number, subscriberUsername, err)
				continue
			}

			if subscriberID == 0 {
				continue
			}

			issueWatch := issues_model.IssueWatch{
				UserID:     subscriberID,
				IssueID:    issueID,
				IsWatching: true,
			}
			issueWatchers = append(issueWatchers, &issueWatch)
		}

		if err := db.Insert(g.ctx, issueWatchers...); err != nil {
			return err
		}
	}

	return nil
}

// CreateComments creates comments of issues
func (g *GiteaLocalUploader) CreateComments(comments ...*base.Comment) error {
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
		case issues_model.CommentTypeAssignees:
			if assigneeID, ok := comment.Meta["AssigneeID"].(int); ok {
				cm.AssigneeID = int64(assigneeID)
			}
			if comment.Meta["RemovedAssigneeID"] != nil {
				cm.RemovedAssignee = true
			}
		case issues_model.CommentTypeChangeTitle:
			if comment.Meta["OldTitle"] != nil {
				cm.OldTitle = fmt.Sprintf("%s", comment.Meta["OldTitle"])
			}
			if comment.Meta["NewTitle"] != nil {
				cm.NewTitle = fmt.Sprintf("%s", comment.Meta["NewTitle"])
			}
		default:
		}

		if err := g.remapUser(comment, &cm); err != nil {
			return err
		}

		// add reactions
		for _, reaction := range comment.Reactions {
			res := issues_model.Reaction{
				Type:        reaction.Content,
				CreatedUnix: timeutil.TimeStampNow(),
			}
			if err := g.remapUser(reaction, &res); err != nil {
				return err
			}
			cm.Reactions = append(cm.Reactions, &res)
		}

		cms = append(cms, &cm)
	}

	if len(cms) == 0 {
		return nil
	}
	return models.InsertIssueComments(cms)
}

// CreatePullRequests creates pull requests
func (g *GiteaLocalUploader) CreatePullRequests(prs ...*base.PullRequest) error {
	gprs := make([]*issues_model.PullRequest, 0, len(prs))
	for _, pr := range prs {
		gpr, err := g.newPullRequest(pr)
		if err != nil {
			return err
		}

		if err := g.remapUser(pr, gpr.Issue); err != nil {
			return err
		}

		gprs = append(gprs, gpr)
	}
	if err := models.InsertPullRequests(gprs...); err != nil {
		return err
	}
	for _, pr := range gprs {
		g.issues[pr.Issue.Index] = pr.Issue
		pull.AddToTaskQueue(pr)
	}
	return nil
}

func (g *GiteaLocalUploader) updateGitForPullRequest(pr *base.PullRequest) (head string, err error) {
	// SECURITY: this pr must have been must have been ensured safe
	if !pr.EnsuredSafe {
		log.Error("PR #%d in %s/%s has not been checked for safety.", pr.Number, g.repoOwner, g.repoName)
		return "", fmt.Errorf("the PR[%d] was not checked for safety", pr.Number)
	}

	// Anonymous function to download the patch file (allows us to use defer)
	err = func() error {
		// if the patchURL is empty there is nothing to download
		if pr.PatchURL == "" {
			return nil
		}

		// SECURITY: We will assume that the pr.PatchURL has been checked
		// pr.PatchURL maybe a local file - but note EnsureSafe should be asserting that this safe
		ret, err := uri.Open(pr.PatchURL) // TODO: This probably needs to use the downloader as there may be rate limiting issues here
		if err != nil {
			return err
		}
		defer ret.Close()

		pullDir := filepath.Join(g.repo.RepoPath(), "pulls")
		if err = os.MkdirAll(pullDir, os.ModePerm); err != nil {
			return err
		}

		f, err := os.Create(filepath.Join(pullDir, fmt.Sprintf("%d.patch", pr.Number)))
		if err != nil {
			return err
		}
		defer f.Close()

		// TODO: Should there be limits on the size of this file?
		_, err = io.Copy(f, ret)

		return err
	}()
	if err != nil {
		return "", err
	}

	head = "unknown repository"
	if pr.IsForkPullRequest() && pr.State != "closed" {
		// OK we want to fetch the current head as a branch from its CloneURL

		// 1. Is there a head clone URL available?
		// 2. Is there a head ref available?
		if pr.Head.CloneURL == "" || pr.Head.Ref == "" {
			return head, nil
		}

		// 3. We need to create a remote for this clone url
		// ... maybe we already have a name for this remote
		remote, ok := g.prHeadCache[pr.Head.CloneURL+":"]
		if !ok {
			// ... let's try ownername as a reasonable name
			remote = pr.Head.OwnerName
			if !git.IsValidRefPattern(remote) {
				// ... let's try something less nice
				remote = "head-pr-" + strconv.FormatInt(pr.Number, 10)
			}
			// ... now add the remote
			err := g.gitRepo.AddRemote(remote, pr.Head.CloneURL, true)
			if err != nil {
				log.Error("PR #%d in %s/%s AddRemote[%s] failed: %v", pr.Number, g.repoOwner, g.repoName, remote, err)
			} else {
				g.prHeadCache[pr.Head.CloneURL+":"] = remote
				ok = true
			}
		}
		if !ok {
			return head, nil
		}

		// 4. Check if we already have this ref?
		localRef, ok := g.prHeadCache[pr.Head.CloneURL+":"+pr.Head.Ref]
		if !ok {
			// ... We would normally name this migrated branch as <OwnerName>/<HeadRef> but we need to ensure that is safe
			localRef = git.SanitizeRefPattern(pr.Head.OwnerName + "/" + pr.Head.Ref)

			// ... Now we must assert that this does not exist
			if g.gitRepo.IsBranchExist(localRef) {
				localRef = "head-pr-" + strconv.FormatInt(pr.Number, 10) + "/" + localRef
				i := 0
				for g.gitRepo.IsBranchExist(localRef) {
					if i > 5 {
						// ... We tried, we really tried but this is just a seriously unfriendly repo
						return head, nil
					}
					// OK just try some uuids!
					localRef = git.SanitizeRefPattern("head-pr-" + strconv.FormatInt(pr.Number, 10) + uuid.New().String())
					i++
				}
			}

			fetchArg := pr.Head.Ref + ":" + git.BranchPrefix + localRef
			if strings.HasPrefix(fetchArg, "-") {
				fetchArg = git.BranchPrefix + fetchArg
			}

			_, _, err = git.NewCommand(g.ctx, "fetch", "--no-tags").AddDashesAndList(remote, fetchArg).RunStdString(&git.RunOpts{Dir: g.repo.RepoPath()})
			if err != nil {
				log.Error("Fetch branch from %s failed: %v", pr.Head.CloneURL, err)
				return head, nil
			}
			g.prHeadCache[pr.Head.CloneURL+":"+pr.Head.Ref] = localRef
			head = localRef
		}

		// 5. Now if pr.Head.SHA == "" we should recover this to the head of this branch
		if pr.Head.SHA == "" {
			headSha, err := g.gitRepo.GetBranchCommitID(localRef)
			if err != nil {
				log.Error("unable to get head SHA of local head for PR #%d from %s in %s/%s. Error: %v", pr.Number, pr.Head.Ref, g.repoOwner, g.repoName, err)
				return head, nil
			}
			pr.Head.SHA = headSha
		}

		_, _, err = git.NewCommand(g.ctx, "update-ref", "--no-deref").AddDynamicArguments(pr.GetGitRefName(), pr.Head.SHA).RunStdString(&git.RunOpts{Dir: g.repo.RepoPath()})
		if err != nil {
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
	} else {
		_, _, err = git.NewCommand(g.ctx, "rev-list", "--quiet", "-1").AddDynamicArguments(pr.Head.SHA).RunStdString(&git.RunOpts{Dir: g.repo.RepoPath()})
		if err != nil {
			// Git update-ref remove bad references with a relative path
			log.Warn("Deprecated local head %s for PR #%d in %s/%s, removing  %s", pr.Head.SHA, pr.Number, g.repoOwner, g.repoName, pr.GetGitRefName())
		} else {
			// set head information
			_, _, err = git.NewCommand(g.ctx, "update-ref", "--no-deref").AddDynamicArguments(pr.GetGitRefName(), pr.Head.SHA).RunStdString(&git.RunOpts{Dir: g.repo.RepoPath()})
			if err != nil {
				log.Error("unable to set %s as the local head for PR #%d from %s in %s/%s. Error: %v", pr.Head.SHA, pr.Number, pr.Head.Ref, g.repoOwner, g.repoName, err)
			}
		}
	}

	return head, nil
}

func (g *GiteaLocalUploader) newPullRequest(pr *base.PullRequest) (*issues_model.PullRequest, error) {
	var labels []*issues_model.Label
	for _, label := range pr.Labels {
		lb, ok := g.labels[label.Name]
		if ok {
			labels = append(labels, lb)
		}
	}

	milestoneID := g.milestones[pr.Milestone]

	head, err := g.updateGitForPullRequest(pr)
	if err != nil {
		return nil, fmt.Errorf("updateGitForPullRequest: %w", err)
	}

	// Now we may need to fix the mergebase
	if pr.Base.SHA == "" {
		if pr.Base.Ref != "" && pr.Head.SHA != "" {
			// A PR against a tag base does not make sense - therefore pr.Base.Ref must be a branch
			// TODO: should we be checking for the refs/heads/ prefix on the pr.Base.Ref? (i.e. are these actually branches or refs)
			pr.Base.SHA, _, err = g.gitRepo.GetMergeBase("", git.BranchPrefix+pr.Base.Ref, pr.Head.SHA)
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
func (g *GiteaLocalUploader) CreateReviews(reviews ...*base.Review) error {
	cms := make([]*issues_model.Review, 0, len(reviews))
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
			CreatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
			UpdatedUnix: timeutil.TimeStamp(review.CreatedAt.Unix()),
		}

		if err := g.remapUser(review, &cm); err != nil {
			return err
		}

		cms = append(cms, &cm)

		// get pr
		pr, ok := g.prCache[issue.ID]
		if !ok {
			var err error
			pr, err = issues_model.GetPullRequestByIssueIDWithNoAttributes(issue.ID)
			if err != nil {
				return err
			}
			g.prCache[issue.ID] = pr
		}
		if pr.MergeBase == "" {
			// No mergebase -> no basis for any patches
			log.Warn("PR #%d in %s/%s: does not have a merge base, all review comments will be ignored", pr.Index, g.repoOwner, g.repoName)
			continue
		}

		headCommitID, err := g.gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			log.Warn("PR #%d GetRefCommitID[%s] in %s/%s: %v, all review comments will be ignored", pr.Index, pr.GetGitRefName(), g.repoOwner, g.repoName, err)
			continue
		}

		for _, comment := range review.Comments {
			line := comment.Line
			if line != 0 {
				comment.Position = 1
			} else {
				_, _, line, _ = git.ParseDiffHunkString(comment.DiffHunk)
			}

			// SECURITY: The TreePath must be cleaned!
			comment.TreePath = path.Clean("/" + comment.TreePath)[1:]

			var patch string
			reader, writer := io.Pipe()
			defer func() {
				_ = reader.Close()
				_ = writer.Close()
			}()
			go func(comment *base.ReviewComment) {
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

			if !git.IsValidSHAPattern(comment.CommitID) {
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

			if err := g.remapUser(review, &c); err != nil {
				return err
			}

			cm.Comments = append(cm.Comments, &c)
		}
	}

	return issues_model.InsertReviews(cms)
}

// Rollback when migrating failed, this will rollback all the changes.
func (g *GiteaLocalUploader) Rollback() error {
	if g.repo != nil && g.repo.ID > 0 {
		g.gitRepo.Close()
		if err := models.DeleteRepository(g.doer, g.repo.OwnerID, g.repo.ID); err != nil {
			return err
		}
	}
	return nil
}

// Finish when migrating success, this will do some status update things.
func (g *GiteaLocalUploader) Finish() error {
	if g.repo == nil || g.repo.ID <= 0 {
		return ErrRepoNotCreated
	}

	// update issue_index
	if err := issues_model.RecalculateIssueIndexForRepo(g.repo.ID); err != nil {
		return err
	}

	if err := models.UpdateRepoStats(g.ctx, g.repo.ID); err != nil {
		return err
	}

	g.repo.Status = repo_model.RepositoryReady
	return repo_model.UpdateRepositoryCols(g.ctx, g.repo, "status")
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
