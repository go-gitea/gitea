// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"
	issue_service "code.gitea.io/gitea/services/issue"

	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Issue struct {
	issues_model.Issue
}

func IssueConverter(f *issues_model.Issue) *Issue {
	return &Issue{
		Issue: *f,
	}
}

func (o Issue) GetID() int64 {
	return o.Index
}

func (o *Issue) SetID(id int64) {
	o.Index = id
}

func (o *Issue) IsNil() bool {
	return o.Index == 0
}

func (o *Issue) Equals(other *Issue) bool {
	return o.Index == other.Index
}

func (o *Issue) ToFormat() *format.Issue {
	var milestone string
	if o.Milestone != nil {
		milestone = o.Milestone.Name
	}

	labels := make([]string, 0, len(o.Labels))
	for _, label := range o.Labels {
		labels = append(labels, label.Name)
	}

	var assignees []string
	for i := range o.Assignees {
		assignees = append(assignees, o.Assignees[i].Name)
	}

	return &format.Issue{
		Common:      format.Common{Index: o.Index},
		Title:       o.Title,
		PosterID:    o.Poster.ID,
		PosterName:  o.Poster.Name,
		PosterEmail: o.Poster.Email,
		Content:     o.Content,
		Milestone:   milestone,
		State:       string(o.State()),
		Created:     o.CreatedUnix.AsTime(),
		Updated:     o.UpdatedUnix.AsTime(),
		Closed:      o.ClosedUnix.AsTimePtr(),
		IsLocked:    o.IsLocked,
		Labels:      labels,
		Assignees:   assignees,
	}
}

func (o *Issue) FromFormat(issue *format.Issue) {
	labels := make([]*issues_model.Label, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, &issues_model.Label{Name: label})
	}

	assignees := make([]*user_model.User, 0, len(issue.Assignees))
	for _, a := range issue.Assignees {
		assignees = append(assignees, &user_model.User{
			Name: a,
		})
	}

	*o = Issue{
		Issue: issues_model.Issue{
			Title: issue.Title,
			Index: issue.Index,
			Poster: &user_model.User{
				ID:    issue.PosterID,
				Name:  issue.PosterName,
				Email: issue.PosterEmail,
			},
			Content: issue.Content,
			Milestone: &issues_model.Milestone{
				Name: issue.Milestone,
			},
			IsClosed:    issue.State == "closed",
			CreatedUnix: timeutil.TimeStamp(issue.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(issue.Updated.Unix()),
			ClosedUnix:  timeutil.TimeStamp(issue.Closed.Unix()),
			IsLocked:    issue.IsLocked,
			Labels:      labels,
			Assignees:   assignees,
		},
	}
}

type IssueProvider struct {
	g       *Gitea
	project *ProjectProvider
}

func (o *IssueProvider) ToFormat(issue *Issue) *format.Issue {
	return issue.ToFormat()
}

func (o *IssueProvider) FromFormat(i *format.Issue) *Issue {
	var issue Issue
	issue.FromFormat(i)
	if i.Milestone != "" {
		issue.Milestone.ID = o.project.milestones.GetID(issue.Milestone.Name)
	}
	for _, label := range issue.Labels {
		label.ID = o.project.labels.GetID(label.Name)
	}
	return &issue
}

func (o *IssueProvider) GetObjects(user *User, project *Project, page int) []*Issue {
	issues, err := issues_model.Issues(&issues_model.IssuesOptions{
		ListOptions: db.ListOptions{Page: page, PageSize: o.g.perPage},
		RepoID:      project.GetID(),
	})
	if err != nil {
		panic(fmt.Errorf("error while listing issues: %v", err))
	}

	return util.ConvertMap[*issues_model.Issue, *Issue](issues, IssueConverter)
}

func (o *IssueProvider) ProcessObject(user *User, project *Project, issue *Issue) {
	if err := (&issue.Issue).LoadAttributes(o.g.ctx); err != nil {
		panic(true)
	}
}

func (o *IssueProvider) Get(user *User, project *Project, exemplar *Issue) *Issue {
	id := exemplar.GetID()
	issue, err := issues_model.GetIssueByIndex(project.GetID(), id)
	if issues_model.IsErrIssueNotExist(err) {
		return &Issue{}
	}
	if err != nil {
		panic(err)
	}
	i := IssueConverter(issue)
	o.ProcessObject(user, project, i)
	return i
}

func (o *IssueProvider) Put(user *User, project *Project, issue *Issue) *Issue {
	i := issue.Issue
	i.RepoID = project.GetID()
	labels := make([]int64, 0, len(i.Labels))
	for _, label := range i.Labels {
		labels = append(labels, label.ID)
	}

	if err := issues_model.NewIssue(&project.Repository, &i, labels, []string{}); err != nil {
		panic(err)
	}
	return o.Get(user, project, IssueConverter(&i))
}

func (o *IssueProvider) Delete(user *User, project *Project, issue *Issue) *Issue {
	m := o.Get(user, project, issue)
	if !m.IsNil() {
		repoPath := repo_model.RepoPath(user.Name, project.Name)
		gitRepo, err := git.OpenRepository(o.g.ctx, repoPath)
		if err != nil {
			panic(err)
		}
		defer gitRepo.Close()
		if err := issue_service.DeleteIssue(o.g.GetDoer(), gitRepo, &issue.Issue); err != nil {
			panic(err)
		}
	}
	return m
}
