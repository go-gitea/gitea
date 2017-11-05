// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/util"

	"github.com/go-xorm/builder"
)

// RepositoryList contains a list of repositories
type RepositoryList []*Repository

// RepositoryListOfMap make list from values of map
func RepositoryListOfMap(repoMap map[int64]*Repository) RepositoryList {
	return RepositoryList(valuesRepository(repoMap))
}

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
type SearchRepoOptions struct {
	Keyword   string
	OwnerID   int64
	OrderBy   SearchOrderBy
	Private   bool // Include private repositories in results
	Starred   bool
	Page      int
	IsProfile bool
	AllPublic bool // Include also all public repositories
	PageSize  int  // Can be smaller than or equal to setting.ExplorePagingNum
	// None -> include collaborative AND non-collaborative
	// True -> include just collaborative
	// False -> incude just non-collaborative
	Collaborate util.OptionalBool
	// None -> include forks AND non-forks
	// True -> include just forks
	// False -> include just non-forks
	Fork util.OptionalBool
	// None -> include mirrors AND non-mirrors
	// True -> include just mirrors
	// False -> include just non-mirrors
	Mirror util.OptionalBool
}

//SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse               = "name DESC"
	SearchOrderByLeastUpdated                        = "updated_unix ASC"
	SearchOrderByRecentUpdated                       = "updated_unix DESC"
	SearchOrderByOldest                              = "created_unix ASC"
	SearchOrderByNewest                              = "created_unix DESC"
	SearchOrderBySize                                = "size ASC"
	SearchOrderBySizeReverse                         = "size DESC"
	SearchOrderByID                                  = "id ASC"
	SearchOrderByIDReverse                           = "id DESC"
)

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	var cond = builder.NewCond()

	if !opts.Private {
		cond = cond.And(builder.Eq{"is_private": false})
	}

	var starred bool
	if opts.OwnerID > 0 {
		if opts.Starred {
			starred = true
			cond = builder.Eq{"star.uid": opts.OwnerID}
		} else {
			var accessCond = builder.NewCond()
			if opts.Collaborate != util.OptionalBoolTrue {
				accessCond = builder.Eq{"owner_id": opts.OwnerID}
			}

			if opts.Collaborate != util.OptionalBoolFalse {
				collaborateCond := builder.And(
					builder.Expr("id IN (SELECT repo_id FROM `access` WHERE access.user_id = ?)", opts.OwnerID),
					builder.Neq{"owner_id": opts.OwnerID})
				if !opts.Private {
					collaborateCond = collaborateCond.And(builder.Expr("owner_id NOT IN (SELECT org_id FROM org_user WHERE org_user.uid = ? AND org_user.is_public = ?)", opts.OwnerID, false))
				}

				accessCond = accessCond.Or(collaborateCond)
			}

			if opts.AllPublic {
				accessCond = accessCond.Or(builder.Eq{"is_private": false})
			}

			cond = cond.And(accessCond)
		}
	}

	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"lower_name", strings.ToLower(opts.Keyword)})
	}

	if opts.Fork != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_fork": opts.Fork == util.OptionalBoolTrue})
	}

	if opts.Mirror != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_mirror": opts.Mirror == util.OptionalBoolTrue})
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = SearchOrderByAlphabetically
	}

	sess := x.NewSession()
	defer sess.Close()

	if starred {
		sess.Join("INNER", "star", "star.repo_id = repository.id")
	}

	count, err := sess.
		Where(cond).
		Count(new(Repository))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	// Set again after reset by Count()
	if starred {
		sess.Join("INNER", "star", "star.repo_id = repository.id")
	}

	repos := make(RepositoryList, 0, opts.PageSize)
	if err = sess.
		Where(cond).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		OrderBy(opts.OrderBy.String()).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if !opts.IsProfile {
		if err = repos.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
		}
	}

	return repos, count, nil
}
