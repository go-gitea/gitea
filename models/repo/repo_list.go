// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// FindReposMapByIDs find repos as map
func FindReposMapByIDs(ctx context.Context, repoIDs []int64, res map[int64]*Repository) error {
	return db.GetEngine(ctx).In("id", repoIDs).Find(&res)
}

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

// ValuesRepository converts a repository map to a list
// FIXME: Remove in favor of maps.values when MIN_GO_VERSION >= 1.18
func ValuesRepository(m map[int64]*Repository) []*Repository {
	values := make([]*Repository, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// RepositoryListOfMap make list from values of map
func RepositoryListOfMap(repoMap map[int64]*Repository) RepositoryList {
	return RepositoryList(ValuesRepository(repoMap))
}

func (repos RepositoryList) LoadUnits(ctx context.Context) error {
	if len(repos) == 0 {
		return nil
	}

	// Load units.
	units := make([]*RepoUnit, 0, len(repos)*6)
	if err := db.GetEngine(ctx).
		In("repo_id", repos.IDs()).
		Find(&units); err != nil {
		return fmt.Errorf("find units: %w", err)
	}

	unitsMap := make(map[int64][]*RepoUnit, len(repos))
	for _, unit := range units {
		if !unit.Type.UnitGlobalDisabled() {
			unitsMap[unit.RepoID] = append(unitsMap[unit.RepoID], unit)
		}
	}

	for _, repo := range repos {
		repo.Units = unitsMap[repo.ID]
	}

	return nil
}

func (repos RepositoryList) IDs() []int64 {
	repoIDs := make([]int64, len(repos))
	for i := range repos {
		repoIDs[i] = repos[i].ID
	}
	return repoIDs
}

// LoadAttributes loads the attributes for the given RepositoryList
func (repos RepositoryList) LoadAttributes(ctx context.Context) error {
	if len(repos) == 0 {
		return nil
	}

	userIDs := container.FilterSlice(repos, func(repo *Repository) (int64, bool) {
		return repo.OwnerID, true
	})
	repoIDs := make([]int64, len(repos))
	for i := range repos {
		repoIDs[i] = repos[i].ID
	}

	// Load owners.
	users := make(map[int64]*user_model.User, len(userIDs))
	if err := db.GetEngine(ctx).
		Where("id > 0").
		In("id", userIDs).
		Find(&users); err != nil {
		return fmt.Errorf("find users: %w", err)
	}
	for i := range repos {
		repos[i].Owner = users[repos[i].OwnerID]
	}

	// Load primary language.
	stats := make(LanguageStatList, 0, len(repos))
	if err := db.GetEngine(ctx).
		Where("`is_primary` = ? AND `language` != ?", true, "other").
		In("`repo_id`", repoIDs).
		Find(&stats); err != nil {
		return fmt.Errorf("find primary languages: %w", err)
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
	IsPrivate optional.Option[bool]
	// None -> include collaborative AND non-collaborative
	// True -> include just collaborative
	// False -> include just non-collaborative
	Collaborate optional.Option[bool]
	// What type of unit the user can be collaborative in,
	// it is ignored if Collaborate is False.
	// TypeInvalid means any unit type.
	UnitType unit.Type
	// None -> include forks AND non-forks
	// True -> include just forks
	// False -> include just non-forks
	Fork optional.Option[bool]
	// If Fork option is True, you can use this option to limit the forks of a special repo by repo id.
	ForkFrom int64
	// None -> include templates AND non-templates
	// True -> include just templates
	// False -> include just non-templates
	Template optional.Option[bool]
	// None -> include mirrors AND non-mirrors
	// True -> include just mirrors
	// False -> include just non-mirrors
	Mirror optional.Option[bool]
	// None -> include archived AND non-archived
	// True -> include just archived
	// False -> include just non-archived
	Archived optional.Option[bool]
	// only search topic name
	TopicOnly bool
	// only search repositories with specified primary language
	Language string
	// include description in keyword search
	IncludeDescription bool
	// None -> include has milestones AND has no milestone
	// True -> include just has milestones
	// False -> include just has no milestone
	HasMilestones optional.Option[bool]
	// LowerNames represents valid lower names to restrict to
	LowerNames []string
	// When specified true, apply some filters over the conditions:
	// - Don't show forks, when opts.Fork is OptionalBoolNone.
	// - Do not display repositories that don't have a description, an icon and topics.
	OnlyShowRelevant bool
}

// UserOwnedRepoCond returns user ownered repositories
func UserOwnedRepoCond(userID int64) builder.Cond {
	return builder.Eq{
		"repository.owner_id": userID,
	}
}

// UserAssignedRepoCond return user as assignee repositories list
func UserAssignedRepoCond(id string, userID int64) builder.Cond {
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

// UserCreateIssueRepoCond return user created issues repositories list
func UserCreateIssueRepoCond(id string, userID int64, isPull bool) builder.Cond {
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

// UserMentionedRepoCond return user metinoed repositories list
func UserMentionedRepoCond(id string, userID int64) builder.Cond {
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

// UserAccessRepoCond returns a condition for selecting all repositories a user has unit independent access to
func UserAccessRepoCond(idStr string, userID int64) builder.Cond {
	return builder.In(idStr, builder.Select("repo_id").
		From("`access`").
		Where(builder.And(
			builder.Eq{"`access`.user_id": userID},
			builder.Gt{"`access`.mode": int(perm.AccessModeNone)},
		)),
	)
}

// userCollaborationRepoCond returns a condition for selecting all repositories a user is collaborator in
func UserCollaborationRepoCond(idStr string, userID int64) builder.Cond {
	return builder.In(idStr, builder.Select("repo_id").
		From("`collaboration`").
		Where(builder.And(
			builder.Eq{"`collaboration`.user_id": userID},
		)),
	)
}

// UserOrgTeamRepoCond selects repos that the given user has access to through team membership
func UserOrgTeamRepoCond(idStr string, userID int64) builder.Cond {
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
		Where(builder.Eq{"`team_unit`.`type`": unitType}).
		And(builder.Gt{"`team_unit`.`access_mode`": int(perm.AccessModeNone)})
}

// userOrgTeamUnitRepoCond returns a condition to select repo ids where user's teams can access the special unit.
func userOrgTeamUnitRepoCond(idStr string, userID int64, unitType unit.Type) builder.Cond {
	return builder.In(idStr, userOrgTeamUnitRepoBuilder(userID, unitType))
}

// UserOrgUnitRepoCond selects repos that the given user has access to through org and the special unit
func UserOrgUnitRepoCond(idStr string, userID, orgID int64, unitType unit.Type) builder.Cond {
	return builder.In(idStr,
		userOrgTeamUnitRepoBuilder(userID, unitType).
			And(builder.Eq{"`team_unit`.org_id": orgID}),
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

// UserOrgPublicUnitRepoCond returns the condition that one user could access all public repositories in the special organization
func UserOrgPublicUnitRepoCond(userID, orgID int64) builder.Cond {
	return userOrgPublicRepoCond(userID).
		And(builder.Eq{"`repository`.owner_id": orgID})
}

// SearchRepositoryCondition creates a query condition according search repository options
func SearchRepositoryCondition(opts *SearchRepoOptions) builder.Cond {
	cond := builder.NewCond()

	if opts.Private {
		if opts.Actor != nil && !opts.Actor.IsAdmin && opts.Actor.ID != opts.OwnerID {
			// OK we're in the context of a User
			cond = cond.And(AccessibleRepositoryCondition(opts.Actor, unit.TypeInvalid))
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

	if opts.IsPrivate.Has() {
		cond = cond.And(builder.Eq{"is_private": opts.IsPrivate.Value()})
	}

	if opts.Template.Has() {
		cond = cond.And(builder.Eq{"is_template": opts.Template.Value()})
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
		if !opts.Collaborate.Value() {
			accessCond = builder.Eq{"owner_id": opts.OwnerID}
		}

		if opts.Collaborate.ValueOrDefault(true) {
			// A Collaboration is:

			collaborateCond := builder.NewCond()
			// 1. Repository we don't own
			collaborateCond = collaborateCond.And(builder.Neq{"owner_id": opts.OwnerID})
			// 2. But we can see because of:
			{
				userAccessCond := builder.NewCond()
				// A. We have unit independent access
				userAccessCond = userAccessCond.Or(UserAccessRepoCond("`repository`.id", opts.OwnerID))
				// B. We are in a team for
				if opts.UnitType == unit.TypeInvalid {
					userAccessCond = userAccessCond.Or(UserOrgTeamRepoCond("`repository`.id", opts.OwnerID))
				} else {
					userAccessCond = userAccessCond.Or(userOrgTeamUnitRepoCond("`repository`.id", opts.OwnerID, opts.UnitType))
				}
				// C. Public repositories in organizations that we are member of
				userAccessCond = userAccessCond.Or(userOrgPublicRepoCondPrivate(opts.OwnerID))
				collaborateCond = collaborateCond.And(userAccessCond)
			}
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

				// If the string looks like "org/repo", match against that pattern too
				if opts.TeamID == 0 && strings.Count(opts.Keyword, "/") == 1 {
					pieces := strings.Split(opts.Keyword, "/")
					ownerName := pieces[0]
					repoName := pieces[1]
					likes = likes.Or(builder.And(builder.Like{"owner_name", strings.ToLower(ownerName)}, builder.Like{"lower_name", strings.ToLower(repoName)}))
				}

				if opts.IncludeDescription {
					likes = likes.Or(builder.Like{"LOWER(description)", strings.ToLower(v)})
				}
			}
			keywordCond = keywordCond.Or(likes)
		}
		cond = cond.And(keywordCond)
	}

	if opts.Language != "" {
		cond = cond.And(builder.In("id", builder.
			Select("repo_id").
			From("language_stat").
			Where(builder.Eq{"language": opts.Language}).And(builder.Eq{"is_primary": true})))
	}

	if opts.Fork.Has() || opts.OnlyShowRelevant {
		if opts.OnlyShowRelevant && !opts.Fork.Has() {
			cond = cond.And(builder.Eq{"is_fork": false})
		} else {
			cond = cond.And(builder.Eq{"is_fork": opts.Fork.Value()})

			if opts.ForkFrom > 0 && opts.Fork.Value() {
				cond = cond.And(builder.Eq{"fork_id": opts.ForkFrom})
			}
		}
	}

	if opts.Mirror.Has() {
		cond = cond.And(builder.Eq{"is_mirror": opts.Mirror.Value()})
	}

	if opts.Actor != nil && opts.Actor.IsRestricted {
		cond = cond.And(AccessibleRepositoryCondition(opts.Actor, unit.TypeInvalid))
	}

	if opts.Archived.Has() {
		cond = cond.And(builder.Eq{"is_archived": opts.Archived.Value()})
	}

	if opts.HasMilestones.Has() {
		if opts.HasMilestones.Value() {
			cond = cond.And(builder.Gt{"num_milestones": 0})
		} else {
			cond = cond.And(builder.Eq{"num_milestones": 0}.Or(builder.IsNull{"num_milestones"}))
		}
	}

	if opts.OnlyShowRelevant {
		// Only show a repo that has at least a topic, an icon, or a description
		subQueryCond := builder.NewCond()

		// Topic checking. Topics are present.
		if setting.Database.Type.IsPostgreSQL() { // postgres stores the topics as json and not as text
			subQueryCond = subQueryCond.Or(builder.And(builder.NotNull{"topics"}, builder.Neq{"(topics)::text": "[]"}))
		} else {
			subQueryCond = subQueryCond.Or(builder.And(builder.Neq{"topics": "null"}, builder.Neq{"topics": "[]"}))
		}

		// Description checking. Description not empty
		subQueryCond = subQueryCond.Or(builder.Neq{"description": ""})

		// Repo has a avatar
		subQueryCond = subQueryCond.Or(builder.Neq{"avatar": ""})

		// Always hide repo's that are empty
		subQueryCond = subQueryCond.And(builder.Eq{"is_empty": false})

		cond = cond.And(subQueryCond)
	}

	return cond
}

// SearchRepository returns repositories based on search options,
// it returns results in given range and number of total results.
func SearchRepository(ctx context.Context, opts *SearchRepoOptions) (RepositoryList, int64, error) {
	cond := SearchRepositoryCondition(opts)
	return SearchRepositoryByCondition(ctx, opts, cond, true)
}

// CountRepository counts repositories based on search options,
func CountRepository(ctx context.Context, opts *SearchRepoOptions) (int64, error) {
	return db.GetEngine(ctx).Where(SearchRepositoryCondition(opts)).Count(new(Repository))
}

// SearchRepositoryByCondition search repositories by condition
func SearchRepositoryByCondition(ctx context.Context, opts *SearchRepoOptions, cond builder.Cond, loadAttributes bool) (RepositoryList, int64, error) {
	sess, count, err := searchRepositoryByCondition(ctx, opts, cond)
	if err != nil {
		return nil, 0, err
	}

	defaultSize := 50
	if opts.PageSize > 0 {
		defaultSize = opts.PageSize
	}
	repos := make(RepositoryList, 0, defaultSize)
	if err := sess.Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %w", err)
	}

	if opts.PageSize <= 0 {
		count = int64(len(repos))
	}

	if loadAttributes {
		if err := repos.LoadAttributes(ctx); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %w", err)
		}
	}

	return repos, count, nil
}

func searchRepositoryByCondition(ctx context.Context, opts *SearchRepoOptions, cond builder.Cond) (db.Engine, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = db.SearchOrderByAlphabetically
	}

	args := make([]any, 0)
	if opts.PriorityOwnerID > 0 {
		opts.OrderBy = db.SearchOrderBy(fmt.Sprintf("CASE WHEN owner_id = ? THEN 0 ELSE owner_id END, %s", opts.OrderBy))
		args = append(args, opts.PriorityOwnerID)
	} else if strings.Count(opts.Keyword, "/") == 1 {
		// With "owner/repo" search times, prioritise results which match the owner field
		orgName := strings.Split(opts.Keyword, "/")[0]
		opts.OrderBy = db.SearchOrderBy(fmt.Sprintf("CASE WHEN owner_name LIKE ? THEN 0 ELSE 1 END, %s", opts.OrderBy))
		args = append(args, orgName)
	}

	sess := db.GetEngine(ctx)

	var count int64
	if opts.PageSize > 0 {
		var err error
		count, err = sess.
			Where(cond).
			Count(new(Repository))
		if err != nil {
			return nil, 0, fmt.Errorf("Count: %w", err)
		}
	}

	sess = sess.Where(cond).OrderBy(opts.OrderBy.String(), args...)
	if opts.PageSize > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	return sess, count, nil
}

// SearchRepositoryIDsByCondition search repository IDs by given condition.
func SearchRepositoryIDsByCondition(ctx context.Context, cond builder.Cond) ([]int64, error) {
	repoIDs := make([]int64, 0, 10)
	return repoIDs, db.GetEngine(ctx).
		Table("repository").
		Cols("id").
		Where(cond).
		Find(&repoIDs)
}

// AccessibleRepositoryCondition takes a user a returns a condition for checking if a repository is accessible
func AccessibleRepositoryCondition(user *user_model.User, unitType unit.Type) builder.Cond {
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
		// 2. Be able to see all repositories that we have unit independent access to
		// 3. Be able to see all repositories through team membership(s)
		if unitType == unit.TypeInvalid {
			// Regardless of UnitType
			cond = cond.Or(
				UserAccessRepoCond("`repository`.id", user.ID),
				UserOrgTeamRepoCond("`repository`.id", user.ID),
			)
		} else {
			// For a specific UnitType
			cond = cond.Or(
				UserCollaborationRepoCond("`repository`.id", user.ID),
				userOrgTeamUnitRepoCond("`repository`.id", user.ID, unitType),
			)
		}
		// 4. Repositories that we directly own
		cond = cond.Or(builder.Eq{"`repository`.owner_id": user.ID})
		if !user.IsRestricted {
			// 5. Be able to see all public repos in private organizations that we are an org_user of
			cond = cond.Or(userOrgPublicRepoCond(user.ID))
		}
	}

	return cond
}

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(ctx context.Context, opts *SearchRepoOptions) (RepositoryList, int64, error) {
	opts.IncludeDescription = false
	return SearchRepository(ctx, opts)
}

// SearchRepositoryIDs takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryIDs(ctx context.Context, opts *SearchRepoOptions) ([]int64, int64, error) {
	opts.IncludeDescription = false

	cond := SearchRepositoryCondition(opts)

	sess, count, err := searchRepositoryByCondition(ctx, opts, cond)
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
	return builder.Select("id").From("repository").Where(AccessibleRepositoryCondition(user, unit.TypeInvalid))
}

// FindUserCodeAccessibleRepoIDs finds all at Code level accessible repositories' ID by the user's id
func FindUserCodeAccessibleRepoIDs(ctx context.Context, user *user_model.User) ([]int64, error) {
	return SearchRepositoryIDsByCondition(ctx, AccessibleRepositoryCondition(user, unit.TypeCode))
}

// FindUserCodeAccessibleOwnerRepoIDs finds all repository IDs for the given owner whose code the user can see.
func FindUserCodeAccessibleOwnerRepoIDs(ctx context.Context, ownerID int64, user *user_model.User) ([]int64, error) {
	return SearchRepositoryIDsByCondition(ctx, builder.NewCond().And(
		builder.Eq{"owner_id": ownerID},
		AccessibleRepositoryCondition(user, unit.TypeCode),
	))
}

// GetUserRepositories returns a list of repositories of given user.
func GetUserRepositories(ctx context.Context, opts *SearchRepoOptions) (RepositoryList, int64, error) {
	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "updated_unix DESC"
	}

	cond := builder.NewCond()
	if opts.Actor == nil {
		return nil, 0, util.NewInvalidArgumentErrorf("GetUserRepositories: Actor is needed but not given")
	}
	cond = cond.And(builder.Eq{"owner_id": opts.Actor.ID})
	if !opts.Private {
		cond = cond.And(builder.Eq{"is_private": false})
	}

	if opts.LowerNames != nil && len(opts.LowerNames) > 0 {
		cond = cond.And(builder.In("lower_name", opts.LowerNames))
	}

	sess := db.GetEngine(ctx)

	count, err := sess.Where(cond).Count(new(Repository))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %w", err)
	}

	sess = sess.Where(cond).OrderBy(opts.OrderBy.String())
	repos := make(RepositoryList, 0, opts.PageSize)
	return repos, count, db.SetSessionPagination(sess, opts).Find(&repos)
}
