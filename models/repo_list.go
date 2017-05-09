// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"
)

// RepositoryList contains a list of repositories
type RepositoryList []*Repository

func (repos RepositoryList) loadAttributes(e Engine) error {
	if len(repos) == 0 {
		return nil
	}

	// Load owners.
	set := make(map[int64]struct{})
	for i := range repos {
		set[repos[i].OwnerID] = struct{}{}
	}
	users := make(map[int64]*User, len(set))
	if err := e.
		Where("id > 0").
		In("id", keysInt64(set)).
		Find(&users); err != nil {
		return fmt.Errorf("find users: %v", err)
	}
	for i := range repos {
		repos[i].Owner = users[repos[i].OwnerID]
	}
	return nil
}

// LoadAttributes loads the attributes for the given RepositoryList
func (repos RepositoryList) LoadAttributes() error {
	return repos.loadAttributes(x)
}

// MirrorRepositoryList contains the mirror repositories
type MirrorRepositoryList []*Repository

func (repos MirrorRepositoryList) loadAttributes(e Engine) error {
	if len(repos) == 0 {
		return nil
	}

	// Load mirrors.
	repoIDs := make([]int64, 0, len(repos))
	for i := range repos {
		if !repos[i].IsMirror {
			continue
		}

		repoIDs = append(repoIDs, repos[i].ID)
	}
	mirrors := make([]*Mirror, 0, len(repoIDs))
	if err := e.
		Where("id > 0").
		In("repo_id", repoIDs).
		Find(&mirrors); err != nil {
		return fmt.Errorf("find mirrors: %v", err)
	}

	set := make(map[int64]*Mirror)
	for i := range mirrors {
		set[mirrors[i].RepoID] = mirrors[i]
	}
	for i := range repos {
		repos[i].Mirror = set[repos[i].ID]
	}
	return nil
}

// LoadAttributes loads the attributes for the given MirrorRepositoryList
func (repos MirrorRepositoryList) LoadAttributes() error {
	return repos.loadAttributes(x)
}

// SearchRepoOptions holds the search options
// swagger:parameters repoSearch
type SearchRepoOptions struct {
	// Keyword to search
	//
	// in: query
	Keyword string `json:"q"`
	// Owner in we search search
	//
	// in: query
	OwnerID   int64  `json:"uid"`
	Searcher  *User  `json:"-"` //ID of the person who's seeking
	OrderBy   string `json:"-"`
	Private   bool   `json:"-"` // Include private repositories in results
	Starred   bool   `json:"-"`
	Page      int    `json:"-"`
	IsProfile bool   `json:"-"`
	// Limit of result
	//
	// maximum: setting.ExplorePagingNum
	// in: query
	PageSize int `json:"limit"` // Can be smaller than or equal to setting.ExplorePagingNum
}

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(opts *SearchRepoOptions) (repos RepositoryList, count int64, err error) {
	var (
		sess *xorm.Session
		cond = builder.NewCond()
	)

	opts.Keyword = strings.ToLower(opts.Keyword)

	if opts.Page <= 0 {
		opts.Page = 1
	}

	repos = make([]*Repository, 0, opts.PageSize)

	if opts.Starred && opts.OwnerID > 0 {
		cond = builder.Eq{
			"star.uid": opts.OwnerID,
		}
	}
	cond = cond.And(builder.Like{"lower_name", opts.Keyword})

	// Append conditions
	if !opts.Starred && opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if !opts.Private {
		cond = cond.And(builder.Eq{"is_private": false})
	}

	if opts.Searcher != nil {
		var ownerIds []int64

		ownerIds = append(ownerIds, opts.Searcher.ID)
		err = opts.Searcher.GetOrganizations(true)

		if err != nil {
			return nil, 0, fmt.Errorf("Organization: %v", err)
		}

		for _, org := range opts.Searcher.Orgs {
			ownerIds = append(ownerIds, org.ID)
		}

		cond = cond.Or(builder.And(builder.Like{"lower_name", opts.Keyword}, builder.In("owner_id", ownerIds)))
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "name ASC"
	}

	if opts.Starred && opts.OwnerID > 0 {
		sess = x.
			Join("INNER", "star", "star.repo_id = repository.id").
			Where(cond)
		count, err = x.
			Join("INNER", "star", "star.repo_id = repository.id").
			Where(cond).
			Count(new(Repository))
		if err != nil {
			return nil, 0, fmt.Errorf("Count: %v", err)
		}
	} else {
		sess = x.Where(cond)
		count, err = x.
			Where(cond).
			Count(new(Repository))
		if err != nil {
			return nil, 0, fmt.Errorf("Count: %v", err)
		}
	}

	if err = sess.
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		OrderBy(opts.OrderBy).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if !opts.IsProfile {
		if err = repos.loadAttributes(x); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
		}
	}

	return
}

// Repositories returns all repositories
func Repositories(opts *SearchRepoOptions) (_ RepositoryList, count int64, err error) {
	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "id ASC"
	}

	repos := make(RepositoryList, 0, opts.PageSize)

	if err = x.
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		OrderBy(opts.OrderBy).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if err = repos.loadAttributes(x); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
	}

	count = countRepositories(-1, opts.Private)

	return repos, count, nil
}

// GetRecentUpdatedRepositories returns the list of repositories that are recently updated.
func GetRecentUpdatedRepositories(opts *SearchRepoOptions) (repos RepositoryList, _ int64, _ error) {
	var cond = builder.NewCond()

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "updated_unix DESC"
	}

	if !opts.Private {
		cond = builder.Eq{
			"is_private": false,
		}
	}

	if opts.Searcher != nil && !opts.Searcher.IsAdmin {
		var ownerIds []int64

		ownerIds = append(ownerIds, opts.Searcher.ID)
		err := opts.Searcher.GetOrganizations(true)

		if err != nil {
			return nil, 0, fmt.Errorf("Organization: %v", err)
		}

		for _, org := range opts.Searcher.Orgs {
			ownerIds = append(ownerIds, org.ID)
		}

		cond = cond.Or(builder.In("owner_id", ownerIds))
	}

	count, err := x.Where(cond).Count(new(Repository))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	if err = x.Where(cond).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		Limit(opts.PageSize).
		OrderBy(opts.OrderBy).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if err = repos.loadAttributes(x); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
	}

	return repos, count, nil
}
