// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	repo_module "code.gitea.io/gitea/modules/repository"
	repo_service "code.gitea.io/gitea/services/repository"
	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	f3_util "lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Project struct {
	repo_model.Repository
}

func ProjectConverter(f *repo_model.Repository) *Project {
	return &Project{
		Repository: *f,
	}
}

func (o Project) GetID() int64 {
	return o.ID
}

func (o *Project) SetID(id int64) {
	o.ID = id
}

func (o *Project) IsNil() bool {
	return o.ID == 0
}

func (o *Project) Equals(other *Project) bool {
	return (o.Name == other.Name)
}

func (o *Project) ToFormat() *format.Project {
	return &format.Project{
		Common:        format.Common{Index: o.ID},
		Name:          o.Name,
		Owner:         o.Owner.Name,
		IsPrivate:     o.IsPrivate,
		Description:   o.Description,
		CloneURL:      repo_model.ComposeHTTPSCloneURL(o.Owner.Name, o.Name),
		OriginalURL:   o.OriginalURL,
		DefaultBranch: o.DefaultBranch,
	}
}

func (o *Project) FromFormat(project *format.Project) {
	*o = Project{
		Repository: repo_model.Repository{
			ID:   project.Index,
			Name: project.Name,
			Owner: &user_model.User{
				Name: project.Owner,
			},
			IsPrivate:     project.IsPrivate,
			Description:   project.Description,
			OriginalURL:   project.OriginalURL,
			DefaultBranch: project.DefaultBranch,
		},
	}
}

type ProjectProvider struct {
	g          *Gitea
	milestones f3_util.NameIDMap
	labels     f3_util.NameIDMap
}

func (o *ProjectProvider) ToFormat(project *Project) *format.Project {
	return project.ToFormat()
}

func (o *ProjectProvider) FromFormat(p *format.Project) *Project {
	var project Project
	project.FromFormat(p)
	return &project
}

func (o *ProjectProvider) GetObjects(user *User, page int) []*Project {
	repoList, _, err := repo_model.GetUserRepositories(&repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{Page: page, PageSize: o.g.perPage},
		Actor:       &user.User,
		Private:     true,
	})
	if err != nil {
		panic(fmt.Errorf("error while listing projects: %T %v", err, err))
	}
	if err := repoList.LoadAttributes(); err != nil {
		panic(nil)
	}
	return f3_util.ConvertMap[*repo_model.Repository, *Project](([]*repo_model.Repository)(repoList), ProjectConverter)
}

func (o *ProjectProvider) ProcessObject(user *User, project *Project) {
}

func (o *ProjectProvider) Get(user *User, exemplar *Project) *Project {
	var project *repo_model.Repository
	var err error
	if exemplar.GetID() > 0 {
		project, err = repo_model.GetRepositoryByIDCtx(o.g.ctx, exemplar.GetID())
	} else if exemplar.Name != "" {
		project, err = repo_model.GetRepositoryByName(user.GetID(), exemplar.Name)
	} else {
		panic("GetID() == 0 and ProjectName == \"\"")
	}
	if repo_model.IsErrRepoNotExist(err) {
		return &Project{}
	}
	if err != nil {
		panic(fmt.Errorf("project %v %w", exemplar, err))
	}
	if err := project.GetOwner(o.g.ctx); err != nil {
		panic(err)
	}
	return ProjectConverter(project)
}

func (o *ProjectProvider) Put(user *User, project *Project) *Project {
	repo, err := repo_module.CreateRepository(o.g.GetDoer(), &user.User, repo_module.CreateRepoOptions{
		Name:        project.Name,
		Description: project.Description,
		OriginalURL: project.OriginalURL,
		IsPrivate:   project.IsPrivate,
	})
	if err != nil {
		panic(err)
	}
	return o.Get(user, ProjectConverter(repo))
}

func (o *ProjectProvider) Delete(user *User, project *Project) *Project {
	if project.IsNil() {
		return project
	}
	if project.ID > 0 {
		project = o.Get(user, project)
	}
	if !project.IsNil() {
		err := repo_service.DeleteRepository(o.g.ctx, o.g.GetDoer(), &project.Repository, true)
		if err != nil {
			panic(err)
		}
	}
	return project
}
