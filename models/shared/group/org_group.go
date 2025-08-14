package group

import (
	repo_model "code.gitea.io/gitea/models/repo"
	"context"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	organization_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// FindGroupMembers finds all users who have access to a group via team membership
func FindGroupMembers(ctx context.Context, groupID int64, opts *organization_model.FindOrgMembersOpts) (user_model.UserList, error) {
	cond := builder.
		Select("`team_user`.uid").
		From("team_user").
		InnerJoin("org_user", "`org_user`.uid = team_user.uid").
		InnerJoin("repo_group_team", "`repo_group_team`.team_id = team_user.team_id").
		Where(builder.Eq{"`org_user`.org_id": opts.OrgID}).
		And(group_model.ParentGroupCond(context.TODO(), "`repo_group_team`.group_id", groupID))
	if opts.PublicOnly() {
		cond = cond.And(builder.Eq{"`org_user`.is_public": true})
	}
	sess := db.GetEngine(ctx).Where(builder.In("`user`.id", cond))
	if opts.ListOptions.PageSize > 0 {
		sess = db.SetSessionPagination(sess, opts)
		users := make([]*user_model.User, 0, opts.ListOptions.PageSize)
		return users, sess.Find(&users)
	}

	var users []*user_model.User
	err := sess.Find(&users)
	return users, err
}

func GetGroupTeams(ctx context.Context, groupID int64) ([]*organization_model.Team, error) {
	var teams []*organization_model.Team
	return teams, db.GetEngine(ctx).
		Where("`repo_group_team`.group_id = ?", groupID).
		Join("INNER", "repo_group_team", "`repo_group_team`.team_id = `team`.id").
		Asc("`team`.name").
		Find(&teams)
}

func IsGroupMember(ctx context.Context, groupID int64, user *user_model.User) (bool, error) {
	if user == nil {
		return false, nil
	}
	return db.GetEngine(ctx).
		Where("`repo_group_team`.group_id = ?", groupID).
		Join("INNER", "repo_group_team", "`repo_group_team`.team_id = `team_user`.team_id").
		And("`team_user`.uid = ?", user.ID).
		Table("team_user").
		Exist()
}

func GetGroupRepos(ctx context.Context, groupID int64, doer *user_model.User) ([]*repo_model.Repository, error) {
	sess := db.GetEngine(ctx)
	repos := make([]*repo_model.Repository, 0)
	return repos, sess.Table("repository").
		Where("group_id = ?", groupID).
		And(builder.In("id", repo_model.AccessibleRepoIDsQuery(doer))).
		OrderBy("group_sort_order").
		Find(&repos)
}
