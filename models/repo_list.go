// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// RepositoryListDefaultPageSize is the default number of repositories
// to load in memory when running administrative tasks on all (or almost
// all) of them.
// The number should be low enough to avoid filling up all RAM with
// repository data...
const RepositoryListDefaultPageSize = 64

// RepositoryList contains a list of repositories
type RepositoryList []*Repository

func (repos RepositoryList) Len() int {
	return len(repos)
}

func (repos RepositoryList) Less(i, j int) bool {
	return repos[i].FullName() < repos[j].FullName()
}

func (repos RepositoryList) Swap(i, j int) {
	repos[i], repos[j] = repos[j], repos[i]
}

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
	UserID          int64
	UserIsAdmin     bool
	Keyword         string
	OwnerID         int64
	PriorityOwnerID int64
	OrderBy         SearchOrderBy
	Private         bool // Include private repositories in results
	StarredByID     int64
	Page            int
	IsProfile       bool
	AllPublic       bool // Include also all public repositories of users and public organisations
	AllLimited      bool // Include also all public repositories of limited organisations
	PageSize        int  // Can be smaller than or equal to setting.ExplorePagingNum
	// None -> include collaborative AND non-collaborative
	// True -> include just collaborative
	// False -> incude just non-collaborative
	Collaborate util.OptionalBool
	// None -> include forks AND non-forks
	// True -> include just forks
	// False -> include just non-forks
	Fork util.OptionalBool
	// None -> include templates AND non-templates
	// True -> include just templates
	// False -> include just non-templates
	Template util.OptionalBool
	// None -> include mirrors AND non-mirrors
	// True -> include just mirrors
	// False -> include just non-mirrors
	Mirror util.OptionalBool
	// only search topic name
	TopicOnly bool
	// include description in keyword search
	IncludeDescription bool
}

//SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse SearchOrderBy = "name DESC"
	SearchOrderByLeastUpdated          SearchOrderBy = "updated_unix ASC"
	SearchOrderByRecentUpdated         SearchOrderBy = "updated_unix DESC"
	SearchOrderByOldest                SearchOrderBy = "created_unix ASC"
	SearchOrderByNewest                SearchOrderBy = "created_unix DESC"
	SearchOrderBySize                  SearchOrderBy = "size ASC"
	SearchOrderBySizeReverse           SearchOrderBy = "size DESC"
	SearchOrderByID                    SearchOrderBy = "id ASC"
	SearchOrderByIDReverse             SearchOrderBy = "id DESC"
	SearchOrderByStars                 SearchOrderBy = "num_stars ASC"
	SearchOrderByStarsReverse          SearchOrderBy = "num_stars DESC"
	SearchOrderByForks                 SearchOrderBy = "num_forks ASC"
	SearchOrderByForksReverse          SearchOrderBy = "num_forks DESC"
)

// SearchRepository returns repositories based on search options,
// it returns results in given range and number of total results.
func SearchRepository(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	var cond = builder.NewCond()

	if opts.Private {
		if !opts.UserIsAdmin && opts.UserID != 0 && opts.UserID != opts.OwnerID {
			// OK we're in the context of a User
			cond = cond.And(accessibleRepositoryCondition(opts.UserID))
		}
	} else {
		// Not looking at private organisations
		// We should be able to see all non-private repositories that either:
		cond = cond.And(builder.Eq{"is_private": false})
		accessCond := builder.Or(
			//   A. Aren't in organisations  __OR__
			builder.NotIn("owner_id", builder.Select("id").From("`user`").Where(builder.Eq{"type": UserTypeOrganization})),
			//   B. Isn't a private or limited organisation.
			builder.NotIn("owner_id", builder.Select("id").From("`user`").Where(builder.Or(builder.Eq{"visibility": structs.VisibleTypeLimited}, builder.Eq{"visibility": structs.VisibleTypePrivate}))))
		cond = cond.And(accessCond)
	}

	if opts.Template != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_template": opts.Template == util.OptionalBoolTrue})
	}

	// Restrict to starred repositories
	if opts.StarredByID > 0 {
		cond = cond.And(builder.In("id", builder.Select("repo_id").From("star").Where(builder.Eq{"uid": opts.StarredByID})))
	}

	// Restrict repositories to those the OwnerID owns or contributes to as per opts.Collaborate
	if opts.OwnerID > 0 {
		var accessCond = builder.NewCond()
		if opts.Collaborate != util.OptionalBoolTrue {
			accessCond = builder.Eq{"owner_id": opts.OwnerID}
		}

		if opts.Collaborate != util.OptionalBoolFalse {
			collaborateCond := builder.And(
				builder.Or(
					builder.Expr("repository.id IN (SELECT repo_id FROM `access` WHERE access.user_id = ?)", opts.OwnerID),
					builder.In("id", builder.Select("`team_repo`.repo_id").
						From("team_repo").
						Where(builder.Eq{"`team_user`.uid": opts.OwnerID}).
						Join("INNER", "team_user", "`team_user`.team_id = `team_repo`.team_id"))),
				builder.Neq{"owner_id": opts.OwnerID})
			if !opts.Private {
				collaborateCond = collaborateCond.And(builder.Expr("owner_id NOT IN (SELECT org_id FROM org_user WHERE org_user.uid = ? AND org_user.is_public = ?)", opts.OwnerID, false))
			}

			accessCond = accessCond.Or(collaborateCond)
		}

		if opts.AllPublic {
			accessCond = accessCond.Or(builder.Eq{"is_private": false}.And(builder.In("owner_id", builder.Select("`user`.id").From("`user`").Where(builder.Eq{"`user`.visibility": structs.VisibleTypePublic}))))
		}

		if opts.AllLimited {
			accessCond = accessCond.Or(builder.Eq{"is_private": false}.And(builder.In("owner_id", builder.Select("`user`.id").From("`user`").Where(builder.Eq{"`user`.visibility": structs.VisibleTypeLimited}))))
		}

		cond = cond.And(accessCond)
	}

	if opts.Keyword != "" {
		// separate keyword
		var subQueryCond = builder.NewCond()
		for _, v := range strings.Split(opts.Keyword, ",") {
			if opts.TopicOnly {
				subQueryCond = subQueryCond.Or(builder.Eq{"topic.name": strings.ToLower(v)})
			} else {
				subQueryCond = subQueryCond.Or(builder.Like{"topic.name", strings.ToLower(v)})
			}
		}
		subQuery := builder.Select("repo_topic.repo_id").From("repo_topic").
			Join("INNER", "topic", "topic.id = repo_topic.topic_id").
			Where(subQueryCond).
			GroupBy("repo_topic.repo_id")

		var keywordCond = builder.In("id", subQuery)
		if !opts.TopicOnly {
			var likes = builder.NewCond()
			for _, v := range strings.Split(opts.Keyword, ",") {
				likes = likes.Or(builder.Like{"lower_name", strings.ToLower(v)})
				if opts.IncludeDescription {
					likes = likes.Or(builder.Like{"LOWER(description)", strings.ToLower(v)})
				}
			}
			keywordCond = keywordCond.Or(likes)
		}
		cond = cond.And(keywordCond)
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

	if opts.PriorityOwnerID > 0 {
		opts.OrderBy = SearchOrderBy(fmt.Sprintf("CASE WHEN owner_id = %d THEN 0 ELSE owner_id END, %s", opts.PriorityOwnerID, opts.OrderBy))
	}

	sess := x.NewSession()
	defer sess.Close()

	count, err := sess.
		Where(cond).
		Count(new(Repository))

	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	repos := make(RepositoryList, 0, opts.PageSize)
	if err = sess.
		Where(cond).
		OrderBy(opts.OrderBy.String()).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
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

// accessibleRepositoryCondition takes a user a returns a condition for checking if a repository is accessible
func accessibleRepositoryCondition(userID int64) builder.Cond {
	if userID <= 0 {
		return builder.And(
			builder.Eq{"`repository`.is_private": false},
			builder.Or(
				//   A. Aren't in organisations  __OR__
				builder.NotIn("`repository`.owner_id", builder.Select("id").From("`user`").Where(builder.Eq{"type": UserTypeOrganization})),
				//   B. Is a public organisation.
				builder.In("`repository`.owner_id", builder.Select("id").From("`user`").Where(builder.Eq{"visibility": structs.VisibleTypePublic}))),
		)
	}

	return builder.Or(
		// 1. Be able to see all non-private repositories that either:
		builder.And(
			builder.Eq{"`repository`.is_private": false},
			builder.Or(
				//   A. Aren't in organisations  __OR__
				builder.NotIn("`repository`.owner_id", builder.Select("id").From("`user`").Where(builder.Eq{"type": UserTypeOrganization})),
				//   B. Isn't a private organisation. (Limited is OK because we're logged in)
				builder.NotIn("`repository`.owner_id", builder.Select("id").From("`user`").Where(builder.Eq{"visibility": structs.VisibleTypePrivate}))),
		),
		// 2. Be able to see all repositories that we have access to
		builder.Or(
			builder.In("`repository`.id", builder.Select("repo_id").
				From("`access`").
				Where(builder.And(
					builder.Eq{"user_id": userID},
					builder.Gt{"mode": int(AccessModeNone)}))),
			builder.In("`repository`.id", builder.Select("id").
				From("`repository`").
				Where(builder.Eq{"owner_id": userID}))),
		// 3. Be able to see all repositories that we are in a team
		builder.In("`repository`.id", builder.Select("`team_repo`.repo_id").
			From("team_repo").
			Where(builder.Eq{"`team_user`.uid": userID}).
			Join("INNER", "team_user", "`team_user`.team_id = `team_repo`.team_id")))
}

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	opts.IncludeDescription = false
	return SearchRepository(opts)
}

// AccessibleRepoIDsQuery queries accessible repository ids. Usable as a subquery wherever repo ids need to be filtered.
func AccessibleRepoIDsQuery(userID int64) *builder.Builder {
	// NB: Please note this code needs to still work if user is nil
	return builder.Select("id").From("repository").Where(accessibleRepositoryCondition(userID))
}

// FindUserAccessibleRepoIDs find all accessible repositories' ID by user's id
func FindUserAccessibleRepoIDs(userID int64) ([]int64, error) {
	var accessCond builder.Cond = builder.Eq{"is_private": false}

	if userID > 0 {
		accessCond = accessCond.Or(
			builder.Eq{"owner_id": userID},
			builder.And(
				builder.Expr("id IN (SELECT repo_id FROM `access` WHERE access.user_id = ?)", userID),
				builder.Neq{"owner_id": userID},
			),
		)
	}

	repoIDs := make([]int64, 0, 10)
	if err := x.
		Table("repository").
		Cols("id").
		Where(accessCond).
		Find(&repoIDs); err != nil {
		return nil, fmt.Errorf("FindUserAccesibleRepoIDs: %v", err)
	}
	return repoIDs, nil
}
