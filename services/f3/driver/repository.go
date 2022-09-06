// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	repo_model "code.gitea.io/gitea/models/repo"
	base "code.gitea.io/gitea/modules/migration"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/services/migrations"

	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Repository struct {
	format.Repository
}

func (o *Repository) Equals(other *Repository) bool {
	return false // it is costly to figure that out, mirroring is as fast
}

func (o *Repository) ToFormat() *format.Repository {
	return &o.Repository
}

func (o *Repository) FromFormat(repository *format.Repository) {
	o.Repository = *repository
}

type RepositoryProvider struct {
	g *Gitea
}

func (o *RepositoryProvider) ToFormat(repository *Repository) *format.Repository {
	return repository.ToFormat()
}

func (o *RepositoryProvider) FromFormat(p *format.Repository) *Repository {
	var repository Repository
	repository.FromFormat(p)
	return &repository
}

func (o *RepositoryProvider) GetObjects(user *User, project *Project, page int) []*Repository {
	if page > 0 {
		return make([]*Repository, 0)
	}
	repositories := make([]*Repository, len(format.RepositoryNames))
	for _, name := range format.RepositoryNames {
		repositories = append(repositories, o.Get(user, project, &Repository{
			Repository: format.Repository{
				Name: name,
			},
		}))
	}
	return repositories
}

func (o *RepositoryProvider) ProcessObject(user *User, project *Project, repository *Repository) {
}

func (o *RepositoryProvider) Get(user *User, project *Project, exemplar *Repository) *Repository {
	repoPath := repo_model.RepoPath(user.Name, project.Name) + exemplar.Name
	return &Repository{
		Repository: format.Repository{
			Name: exemplar.Name,
			FetchFunc: func(destination string) {
				util.Command(o.g.ctx, "git", "clone", "--bare", repoPath, destination)
			},
		},
	}
}

func (o *RepositoryProvider) Put(user *User, project *Project, repository *Repository) *Repository {
	if repository.FetchFunc != nil {
		directory, delete := format.RepositoryDefaultDirectory()
		defer delete()
		repository.FetchFunc(directory)

		_, err := repo_module.MigrateRepositoryGitData(o.g.ctx, &user.User, &project.Repository, base.MigrateOptions{
			RepoName:       project.Name,
			Mirror:         false,
			MirrorInterval: "",
			LFS:            false,
			LFSEndpoint:    "",
			CloneAddr:      directory,
			Wiki:           o.g.GetOptions().GetFeatures().Wiki,
			Releases:       o.g.GetOptions().GetFeatures().Releases,
		}, migrations.NewMigrationHTTPTransport())
		if err != nil {
			panic(err)
		}
	}
	return o.Get(user, project, repository)
}

func (o *RepositoryProvider) Delete(user *User, project *Project, repository *Repository) *Repository {
	panic("It is not possible to delete a repository")
}
