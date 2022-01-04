// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
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
type RepositoryList []*repo_model.Repository

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
func RepositoryListOfMap(repoMap map[int64]*repo_model.Repository) RepositoryList {
	return RepositoryList(valuesRepository(repoMap))
}

func (repos RepositoryList) loadAttributes(e db.Engine) error {
	if len(repos) == 0 {
		return nil
	}

	set := make(map[int64]struct{})
	repoIDs := make([]int64, len(repos))
	for i := range repos {
		set[repos[i].OwnerID] = struct{}{}
		repoIDs[i] = repos[i].ID
	}

	// Load owners.
	users := make(map[int64]*user_model.User, len(set))
	if err := e.
		Where("id > 0").
		In("id", keysInt64(set)).
		Find(&users); err != nil {
		return fmt.Errorf("find users: %v", err)
	}
	for i := range repos {
		repos[i].Owner = users[repos[i].OwnerID]
	}

	// Load primary language.
	stats := make(repo_model.LanguageStatList, 0, len(repos))
	if err := e.
		Where("`is_primary` = ? AND `language` != ?", true, "other").
		In("`repo_id`", repoIDs).
		Find(&stats); err != nil {
		return fmt.Errorf("find primary languages: %v", err)
	}
	stats.LoadAttributes()
	for i := range repos {
		for _, st := range stats {
			if st.RepoID == repos[i].ID {
				repos[i].PrimaryLanguage = st
				break
			}
		}
	}

	return nil
}

// LoadAttributes loads the attributes for the given RepositoryList
func (repos RepositoryList) LoadAttributes() error {
	return repos.loadAttributes(db.GetEngine(db.DefaultContext))
}

// SearchRepoOptions holds the search options
type SearchRepoOptions struct {
	db.ListOptions
	Actor           *user_model.User
	Keyword         string
	OwnerID         int64
	PriorityOwnerID int64
	TeamID          int64
	OrderBy         db.SearchOrderBy
	Private         bool // Include private repositories in results
	StarredByID     int64
	WatchedByID     int64
	AllPublic       bool // Include also all public repositories of users and public organisations
	AllLimited      bool // Include also all public repositories of limited organisations
	// None -> include public and private
	// True -> include just private
	// False -> include just public
	IsPrivate util.OptionalBool
	// None -> include collaborative AND non-collaborative
	// True -> include just collaborative
	// False -> include just non-collaborative
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
	// None -> include archived AND non-archived
	// True -> include just archived
	// False -> include just non-archived
	Archived util.OptionalBool
	// only search topic name
	TopicOnly bool
	// include description in keyword search
	IncludeDescription bool
	// None -> include has milestones AND has no milestone
	// True -> include just has milestones
	// False -> include just has no milestone
	HasMilestones util.OptionalBool
	// LowerNames represents valid lower names to restrict to
	LowerNames []string
}

// SearchOrderBy is used to sort the result
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

// userOwnedRepoCond returns user ownered repositories
func userOwnedRepoCond(userID int64) builder.Cond {
	return builder.Eq{
		"repository.owner_id": userID,
	}
}

// userAssignedRepoCond return user as assignee repositories list
func userAssignedRepoCond(id string, userID int64) builder.Cond {
	return builder.And(
		builder.Eq{
			"repository.is_private": false,
		},
		builder.In(id,
			builder.Select("issue.repo_id").From("issue_assignees").
				InnerJoin("issue", "issue.id = issue_assignees.issue_id").
				Where(builder.Eq{
					"issue_assignees.assignee_id": userID,
				}),
		),
	)
}

// userCreateIssueRepoCond return user created issues repositories list
func userCreateIssueRepoCond(id string, userID int64, isPull bool) builder.Cond {
	return builder.And(
		builder.Eq{
			"repository.is_private": false,
		},
		builder.In(id,
			builder.Select("issue.repo_id").From("issue").
				Where(builder.Eq{
					"issue.poster_id": userID,
					"issue.is_pull":   isPull,
				}),
		),
	)
}

// userMentionedRepoCond return user metinoed repositories list
func userMentionedRepoCond(id string, userID int64) builder.Cond {
	return builder.And(
		builder.Eq{
			"repository.is_private": false,
		},
		builder.In(id,
			builder.Select("issue.repo_id").From("issue_user").
				InnerJoin("issue", "issue.id = issue_user.issue_id").
				Where(builder.Eq{
					"issue_user.is_mentioned": true,
					"issue_user.uid":          userID,
				}),
		),
	)
}

// teamUnitsRepoCond returns query condition for those repo id in the special org team with special units access
func teamUnitsRepoCond(id string, userID, orgID, teamID int64, units ...unit.Type) builder.Cond {
	return builder.In(id,
		builder.Select("repo_id").From("team_repo").Where(
			builder.Eq{
				"team_id": teamID,
			}.And(
				builder.In(
					"team_id", builder.Select("team_id").From("team_user").Where(
						builder.Eq{
							"uid": userID,
						},
					),
				)).And(
				builder.In(
					"team_id", builder.Select("team_id").From("team_unit").Where(
						builder.Eq{
							"org_id": orgID,
						}.And(
							builder.In("type", units),
						),
					),
				),
			),
		))
}

// userCollaborationRepoCond returns user as collabrators repositories list
func userCollaborationRepoCond(idStr string, userID int64) builder.Cond {
	return builder.In(idStr, builder.Select("repo_id").
		From("`access`").
		Where(builder.And(
			builder.Eq{"user_id": userID},
			builder.Gt{"mode": int(perm.AccessModeNone)},
		)),
	)
}

// userOrgTeamRepoCond selects repos that the given user has access to through team membership
func userOrgTeamRepoCond(idStr string, userID int64) builder.Cond {
	return builder.In(idStr, userOrgTeamRepoBuilder(userID))
}

// userOrgTeamRepoBuilder returns repo ids where user's teams can access.
func userOrgTeamRepoBuilder(userID int64) *builder.Builder {
	return builder.Select("`team_repo`.repo_id").
		From("team_repo").
		Join("INNER", "team_user", "`team_user`.team_id = `team_repo`.team_id").
		Where(builder.Eq{"`team_user`.uid": userID})
}

// userOrgTeamUnitRepoBuilder returns repo ids where user's teams can access the special unit.
func userOrgTeamUnitRepoBuilder(userID int64, unitType unit.Type) *builder.Builder {
	return userOrgTeamRepoBuilder(userID).
		Join("INNER", "team_unit", "`team_unit`.team_id = `team_repo`.team_id").
		Where(builder.Eq{"`team_unit`.`type`": unitType})
}

// userOrgUnitRepoCond selects repos that the given user has access to through org and the special unit
func userOrgUnitRepoCond(idStr string, userID, orgID int64, unitType unit.Type) builder.Cond {
	return builder.In(idStr,
		userOrgTeamUnitRepoBuilder(userID, unitType).
			And(builder.Eq{"org_id": orgID}),
	)
}

// userOrgPublicRepoCond returns the condition that one user could access all public repositories in organizations
func userOrgPublicRepoCond(userID int64) builder.Cond {
	return builder.And(
		builder.Eq{"`repository`.is_private": false},
		builder.In("`repository`.owner_id",
			builder.Select("`org_user`.org_id").
				From("org_user").
				Where(builder.Eq{"`org_user`.uid": userID}),
		),
	)
}

// userOrgPublicRepoCondPrivate returns the condition that one user could access all public repositories in private organizations
func userOrgPublicRepoCondPrivate(userID int64) builder.Cond {
	return builder.And(
		builder.Eq{"`repository`.is_private": false},
		builder.In("`repository`.owner_id",
			builder.Select("`org_user`.org_id").
				From("org_user").
				Join("INNER", "`user`", "`user`.id = `org_user`.org_id").
				Where(builder.Eq{
					"`org_user`.uid":    userID,
					"`user`.`type`":     user_model.UserTypeOrganization,
					"`user`.visibility": structs.VisibleTypePrivate,
				}),
		),
	)
}

// userOrgPublicUnitRepoCond returns the condition that one user could access all public repositories in the special organization
func userOrgPublicUnitRepoCond(userID, orgID int64) builder.Cond {
	return userOrgPublicRepoCond(userID).
		And(builder.Eq{"`repository`.owner_id": orgID})
}

// SearchRepositoryCondition creates a query condition according search repository options
func SearchRepositoryCondition(opts *SearchRepoOptions) builder.Cond {
	cond := builder.NewCond()

	if opts.Private {
		if opts.Actor != nil && !opts.Actor.IsAdmin && opts.Actor.ID != opts.OwnerID {
			// OK we're in the context of a User
			cond = cond.And(accessibleRepositoryCondition(opts.Actor))
		}
	} else {
		// Not looking at private organisations and users
		// We should be able to see all non-private repositories that
		// isn't in a private or limited organisation.
		cond = cond.And(
			builder.Eq{"is_private": false},
			builder.NotIn("owner_id", builder.Select("id").From("`user`").Where(
				builder.Or(builder.Eq{"visibility": structs.VisibleTypeLimited}, builder.Eq{"visibility": structs.VisibleTypePrivate}),
			)))
	}

	if opts.IsPrivate != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_private": opts.IsPrivate.IsTrue()})
	}

	if opts.Template != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_template": opts.Template == util.OptionalBoolTrue})
	}

	// Restrict to starred repositories
	if opts.StarredByID > 0 {
		cond = cond.And(builder.In("id", builder.Select("repo_id").From("star").Where(builder.Eq{"uid": opts.StarredByID})))
	}

	// Restrict to watched repositories
	if opts.WatchedByID > 0 {
		cond = cond.And(builder.In("id", builder.Select("repo_id").From("watch").Where(builder.Eq{"user_id": opts.WatchedByID})))
	}

	// Restrict repositories to those the OwnerID owns or contributes to as per opts.Collaborate
	if opts.OwnerID > 0 {
		accessCond := builder.NewCond()
		if opts.Collaborate != util.OptionalBoolTrue {
			accessCond = builder.Eq{"owner_id": opts.OwnerID}
		}

		if opts.Collaborate != util.OptionalBoolFalse {
			// A Collaboration is:
			collaborateCond := builder.And(
				// 1. Repository we don't own
				builder.Neq{"owner_id": opts.OwnerID},
				// 2. But we can see because of:
				builder.Or(
					// A. We have access
					userCollaborationRepoCond("`repository`.id", opts.OwnerID),
					// B. We are in a team for
					userOrgTeamRepoCond("`repository`.id", opts.OwnerID),
					// C. Public repositories in organizations that we are member of
					userOrgPublicRepoCondPrivate(opts.OwnerID),
				),
			)
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

	if opts.TeamID > 0 {
		cond = cond.And(builder.In("`repository`.id", builder.Select("`team_repo`.repo_id").From("team_repo").Where(builder.Eq{"`team_repo`.team_id": opts.TeamID})))
	}

	if opts.Keyword != "" {
		// separate keyword
		subQueryCond := builder.NewCond()
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

		keywordCond := builder.In("id", subQuery)
		if !opts.TopicOnly {
			likes := builder.NewCond()
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

	if opts.Actor != nil && opts.Actor.IsRestricted {
		cond = cond.And(accessibleRepositoryCondition(opts.Actor))
	}

	if opts.Archived != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_archived": opts.Archived == util.OptionalBoolTrue})
	}

	switch opts.HasMilestones {
	case util.OptionalBoolTrue:
		cond = cond.And(builder.Gt{"num_milestones": 0})
	case util.OptionalBoolFalse:
		cond = cond.And(builder.Eq{"num_milestones": 0}.Or(builder.IsNull{"num_milestones"}))
	}

	return cond
}

// SearchRepository returns repositories based on search options,
// it returns results in given range and number of total results.
func SearchRepository(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	cond := SearchRepositoryCondition(opts)
	return SearchRepositoryByCondition(opts, cond, true)
}

// SearchRepositoryByCondition search repositories by condition
func SearchRepositoryByCondition(opts *SearchRepoOptions, cond builder.Cond, loadAttributes bool) (RepositoryList, int64, error) {
	sess, count, err := searchRepositoryByCondition(opts, cond)
	if err != nil {
		return nil, 0, err
	}

	defaultSize := 50
	if opts.PageSize > 0 {
		defaultSize = opts.PageSize
	}
	repos := make(RepositoryList, 0, defaultSize)
	if err := sess.Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if opts.PageSize <= 0 {
		count = int64(len(repos))
	}

	if loadAttributes {
		if err := repos.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
		}
	}

	return repos, count, nil
}

func searchRepositoryByCondition(opts *SearchRepoOptions, cond builder.Cond) (db.Engine, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = db.SearchOrderByAlphabetically
	}

	if opts.PriorityOwnerID > 0 {
		opts.OrderBy = db.SearchOrderBy(fmt.Sprintf("CASE WHEN owner_id = %d THEN 0 ELSE owner_id END, %s", opts.PriorityOwnerID, opts.OrderBy))
	}

	sess := db.GetEngine(db.DefaultContext)

	var count int64
	if opts.PageSize > 0 {
		var err error
		count, err = sess.
			Where(cond).
			Count(new(repo_model.Repository))
		if err != nil {
			return nil, 0, fmt.Errorf("Count: %v", err)
		}
	}

	sess = sess.Where(cond).OrderBy(opts.OrderBy.String())
	if opts.PageSize > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	return sess, count, nil
}

// accessibleRepositoryCondition takes a user a returns a condition for checking if a repository is accessible
func accessibleRepositoryCondition(user *user_model.User) builder.Cond {
	cond := builder.NewCond()

	if user == nil || !user.IsRestricted || user.ID <= 0 {
		orgVisibilityLimit := []structs.VisibleType{structs.VisibleTypePrivate}
		if user == nil || user.ID <= 0 {
			orgVisibilityLimit = append(orgVisibilityLimit, structs.VisibleTypeLimited)
		}
		// 1. Be able to see all non-private repositories that either:
		cond = cond.Or(builder.And(
			builder.Eq{"`repository`.is_private": false},
			// 2. Aren't in an private organisation or limited organisation if we're not logged in
			builder.NotIn("`repository`.owner_id", builder.Select("id").From("`user`").Where(
				builder.And(
					builder.Eq{"type": user_model.UserTypeOrganization},
					builder.In("visibility", orgVisibilityLimit)),
			))))
	}

	if user != nil {
		cond = cond.Or(
			// 2. Be able to see all repositories that we have access to
			userCollaborationRepoCond("`repository`.id", user.ID),
			// 3. Repositories that we directly own
			builder.Eq{"`repository`.owner_id": user.ID},
			// 4. Be able to see all repositories that we are in a team
			userOrgTeamRepoCond("`repository`.id", user.ID),
			// 5. Be able to see all public repos in private organizations that we are an org_user of
			userOrgPublicRepoCond(user.ID),
		)
	}

	return cond
}

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	opts.IncludeDescription = false
	return SearchRepository(opts)
}

// SearchRepositoryIDs takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryIDs(opts *SearchRepoOptions) ([]int64, int64, error) {
	opts.IncludeDescription = false

	cond := SearchRepositoryCondition(opts)

	sess, count, err := searchRepositoryByCondition(opts, cond)
	if err != nil {
		return nil, 0, err
	}

	defaultSize := 50
	if opts.PageSize > 0 {
		defaultSize = opts.PageSize
	}

	ids := make([]int64, 0, defaultSize)
	err = sess.Select("id").Table("repository").Find(&ids)
	if opts.PageSize <= 0 {
		count = int64(len(ids))
	}

	return ids, count, err
}

// AccessibleRepoIDsQuery queries accessible repository ids. Usable as a subquery wherever repo ids need to be filtered.
func AccessibleRepoIDsQuery(user *user_model.User) *builder.Builder {
	// NB: Please note this code needs to still work if user is nil
	return builder.Select("id").From("repository").Where(accessibleRepositoryCondition(user))
}

// FindUserAccessibleRepoIDs find all accessible repositories' ID by user's id
func FindUserAccessibleRepoIDs(user *user_model.User) ([]int64, error) {
	repoIDs := make([]int64, 0, 10)
	if err := db.GetEngine(db.DefaultContext).
		Table("repository").
		Cols("id").
		Where(accessibleRepositoryCondition(user)).
		Find(&repoIDs); err != nil {
		return nil, fmt.Errorf("FindUserAccesibleRepoIDs: %v", err)
	}
	return repoIDs, nil
}

// GetUserRepositories returns a list of repositories of given user.
func GetUserRepositories(opts *SearchRepoOptions) ([]*repo_model.Repository, int64, error) {
	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "updated_unix DESC"
	}

	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"owner_id": opts.Actor.ID})
	if !opts.Private {
		cond = cond.And(builder.Eq{"is_private": false})
	}

	if opts.LowerNames != nil && len(opts.LowerNames) > 0 {
		cond = cond.And(builder.In("lower_name", opts.LowerNames))
	}

	sess := db.GetEngine(db.DefaultContext)

	count, err := sess.Where(cond).Count(new(repo_model.Repository))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	sess = sess.Where(cond).OrderBy(opts.OrderBy.String())
	repos := make([]*repo_model.Repository, 0, opts.PageSize)
	return repos, count, db.SetSessionPagination(sess, opts).Find(&repos)
}
