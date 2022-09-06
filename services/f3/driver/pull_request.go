// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	issue_service "code.gitea.io/gitea/services/issue"
	f3_gitea "lab.forgefriends.org/friendlyforgeformat/gof3/forges/gitea"
	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type PullRequest struct {
	issues_model.PullRequest
	FetchFunc func(repository string) string
}

func PullRequestConverter(f *issues_model.PullRequest) *PullRequest {
	return &PullRequest{
		PullRequest: *f,
	}
}

func (o PullRequest) GetID() int64 {
	return o.Index
}

func (o *PullRequest) SetID(id int64) {
	o.Index = id
}

func (o *PullRequest) IsNil() bool {
	return o.Index == 0
}

func (o *PullRequest) Equals(other *PullRequest) bool {
	return o.Issue.Title == other.Issue.Title
}

func (o PullRequest) IsForkPullRequest() bool {
	return o.HeadRepoID != o.BaseRepoID
}

func (o *PullRequest) ToFormat() *format.PullRequest {
	var milestone string
	if o.Issue.Milestone != nil {
		milestone = o.Issue.Milestone.Name
	}

	labels := make([]string, 0, len(o.Issue.Labels))
	for _, label := range o.Issue.Labels {
		labels = append(labels, label.Name)
	}

	var mergedTime *time.Time
	if o.HasMerged {
		mergedTime = o.MergedUnix.AsTimePtr()
	}

	getSHA := func(repo *repo_model.Repository, branch string) string {
		r, err := git.OpenRepository(context.Background(), repo.RepoPath())
		if err != nil {
			panic(err)
		}
		defer r.Close()

		b, err := r.GetBranch(branch)
		if err != nil {
			panic(err)
		}

		c, err := b.GetCommit()
		if err != nil {
			panic(err)
		}
		return c.ID.String()
	}

	head := format.PullRequestBranch{
		CloneURL:  o.HeadRepo.CloneLink().HTTPS,
		Ref:       o.HeadBranch,
		SHA:       getSHA(o.HeadRepo, o.HeadBranch),
		RepoName:  o.HeadRepo.Name,
		OwnerName: o.HeadRepo.OwnerName,
	}

	base := format.PullRequestBranch{
		CloneURL:  o.BaseRepo.CloneLink().HTTPS,
		Ref:       o.BaseBranch,
		SHA:       getSHA(o.BaseRepo, o.BaseBranch),
		RepoName:  o.BaseRepo.Name,
		OwnerName: o.BaseRepo.OwnerName,
	}

	return &format.PullRequest{
		Common:         format.Common{Index: o.Index},
		PosterID:       o.Issue.Poster.ID,
		PosterName:     o.Issue.Poster.Name,
		PosterEmail:    o.Issue.Poster.Email,
		Title:          o.Issue.Title,
		Content:        o.Issue.Content,
		Milestone:      milestone,
		State:          string(o.Issue.State()),
		IsLocked:       o.Issue.IsLocked,
		Created:        o.Issue.CreatedUnix.AsTime(),
		Updated:        o.Issue.UpdatedUnix.AsTime(),
		Closed:         o.Issue.ClosedUnix.AsTimePtr(),
		Labels:         labels,
		PatchURL:       o.Issue.PatchURL(),
		Merged:         o.HasMerged,
		MergedTime:     mergedTime,
		MergeCommitSHA: o.MergedCommitID,
		Head:           head,
		Base:           base,
	}
}

func (o *PullRequest) FromFormat(pullRequest *format.PullRequest) {
	labels := make([]*issues_model.Label, 0, len(pullRequest.Labels))
	for _, label := range pullRequest.Labels {
		labels = append(labels, &issues_model.Label{Name: label})
	}

	if pullRequest.Created.IsZero() {
		if pullRequest.Closed != nil {
			pullRequest.Created = *pullRequest.Closed
		} else if pullRequest.MergedTime != nil {
			pullRequest.Created = *pullRequest.MergedTime
		} else {
			pullRequest.Created = time.Now()
		}
	}
	if pullRequest.Updated.IsZero() {
		pullRequest.Updated = pullRequest.Created
	}

	base, err := repo_model.GetRepositoryByOwnerAndName(pullRequest.Base.OwnerName, pullRequest.Base.RepoName)
	if err != nil {
		panic(err)
	}
	var head *repo_model.Repository
	if pullRequest.Head.RepoName == "" {
		head = base
	} else {
		head, err = repo_model.GetRepositoryByOwnerAndName(pullRequest.Head.OwnerName, pullRequest.Head.RepoName)
		if err != nil {
			panic(err)
		}
	}

	issue := issues_model.Issue{
		RepoID:      base.ID,
		Repo:        base,
		Title:       pullRequest.Title,
		Index:       pullRequest.Index,
		Content:     pullRequest.Content,
		IsPull:      true,
		IsClosed:    pullRequest.State == "closed",
		IsLocked:    pullRequest.IsLocked,
		Labels:      labels,
		CreatedUnix: timeutil.TimeStamp(pullRequest.Created.Unix()),
		UpdatedUnix: timeutil.TimeStamp(pullRequest.Updated.Unix()),
	}

	pr := issues_model.PullRequest{
		HeadRepoID: head.ID,
		HeadBranch: pullRequest.Head.Ref,
		BaseRepoID: base.ID,
		BaseBranch: pullRequest.Base.Ref,
		MergeBase:  pullRequest.Base.SHA,
		Index:      pullRequest.Index,
		HasMerged:  pullRequest.Merged,

		Issue: &issue,
	}

	if pr.Issue.IsClosed && pullRequest.Closed != nil {
		pr.Issue.ClosedUnix = timeutil.TimeStamp(pullRequest.Closed.Unix())
	}
	if pr.HasMerged && pullRequest.MergedTime != nil {
		pr.MergedUnix = timeutil.TimeStamp(pullRequest.MergedTime.Unix())
		pr.MergedCommitID = pullRequest.MergeCommitSHA
	}

	*o = PullRequest{
		PullRequest: pr,
	}
}

type PullRequestProvider struct {
	g           *Gitea
	project     *ProjectProvider
	prHeadCache f3_gitea.PrHeadCache
}

func (o *PullRequestProvider) ToFormat(pullRequest *PullRequest) *format.PullRequest {
	return pullRequest.ToFormat()
}

func (o *PullRequestProvider) FromFormat(pr *format.PullRequest) *PullRequest {
	var pullRequest PullRequest
	pullRequest.FromFormat(pr)
	return &pullRequest
}

func (o *PullRequestProvider) Init() *PullRequestProvider {
	o.prHeadCache = make(f3_gitea.PrHeadCache)
	return o
}

func (o *PullRequestProvider) cleanupRemotes(repository string) {
	for remote := range o.prHeadCache {
		util.Command(o.g.ctx, "git", "-C", repository, "remote", "rm", remote)
	}
	o.prHeadCache = make(f3_gitea.PrHeadCache)
}

func (o *PullRequestProvider) GetObjects(user *User, project *Project, page int) []*PullRequest {
	pullRequests, _, err := issues_model.PullRequests(project.GetID(), &issues_model.PullRequestsOptions{
		ListOptions: db.ListOptions{Page: page, PageSize: o.g.perPage},
		State:       string(api.StateAll),
	})
	if err != nil {
		panic(fmt.Errorf("error while listing pullRequests: %v", err))
	}

	return util.ConvertMap[*issues_model.PullRequest, *PullRequest](pullRequests, PullRequestConverter)
}

func (o *PullRequestProvider) ProcessObject(user *User, project *Project, pr *PullRequest) {
	if err := pr.LoadIssue(); err != nil {
		panic(err)
	}
	if err := pr.Issue.LoadRepo(o.g.ctx); err != nil {
		panic(err)
	}
	if err := pr.LoadAttributes(); err != nil {
		panic(err)
	}
	if err := pr.LoadBaseRepoCtx(o.g.ctx); err != nil {
		panic(err)
	}
	if err := pr.LoadHeadRepoCtx(o.g.ctx); err != nil {
		panic(err)
	}

	pr.FetchFunc = func(repository string) string {
		head, messages := f3_gitea.UpdateGitForPullRequest(o.g.ctx, &o.prHeadCache, pr.ToFormat(), repository)
		for _, message := range messages {
			o.g.GetLogger().Warn(message)
		}
		o.cleanupRemotes(repository)
		return head
	}
}

func (o *PullRequestProvider) Get(user *User, project *Project, pullRequest *PullRequest) *PullRequest {
	id := pullRequest.GetID()
	pr, err := issues_model.GetPullRequestByIndex(o.g.ctx, project.GetID(), id)
	if issues_model.IsErrPullRequestNotExist(err) {
		return &PullRequest{}
	}
	if err != nil {
		panic(err)
	}
	p := PullRequestConverter(pr)
	o.ProcessObject(user, project, p)
	return p
}

func (o *PullRequestProvider) Put(user *User, project *Project, pullRequest *PullRequest) *PullRequest {
	i := pullRequest.PullRequest.Issue
	i.RepoID = project.GetID()
	labels := make([]int64, 0, len(i.Labels))
	for _, label := range i.Labels {
		labels = append(labels, label.ID)
	}

	if err := issues_model.NewPullRequest(o.g.ctx, &project.Repository, i, labels, []string{}, &pullRequest.PullRequest); err != nil {
		panic(err)
	}
	return o.Get(user, project, pullRequest)
}

func (o *PullRequestProvider) Delete(user *User, project *Project, pullRequest *PullRequest) *PullRequest {
	p := o.Get(user, project, pullRequest)
	if !p.IsNil() {
		repoPath := repo_model.RepoPath(user.Name, project.Name)
		gitRepo, err := git.OpenRepository(o.g.ctx, repoPath)
		if err != nil {
			panic(err)
		}
		defer gitRepo.Close()
		if err := issue_service.DeleteIssue(o.g.GetDoer(), gitRepo, p.PullRequest.Issue); err != nil {
			panic(err)
		}
	}
	return p
}
