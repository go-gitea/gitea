// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ________                            .__                __  .__
// \_____  \_______  _________    ____ |__|____________ _/  |_|__| ____   ____
//  /   |   \_  __ \/ ___\__  \  /    \|  \___   /\__  \\   __\  |/  _ \ /    \
// /    |    \  | \/ /_/  > __ \|   |  \  |/    /  / __ \|  | |  (  <_> )   |  \
// \_______  /__|  \___  (____  /___|  /__/_____ \(____  /__| |__|\____/|___|  /
//         \/     /_____/     \/     \/         \/     \/                    \/

// ErrOrgNotExist represents a "OrgNotExist" kind of error.
type ErrOrgNotExist struct {
	ID   int64
	Name string
}

// IsErrOrgNotExist checks if an error is a ErrOrgNotExist.
func IsErrOrgNotExist(err error) bool {
	_, ok := err.(ErrOrgNotExist)
	return ok
}

func (err ErrOrgNotExist) Error() string {
	return fmt.Sprintf("org does not exist [id: %d, name: %s]", err.ID, err.Name)
}

func (err ErrOrgNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrLastOrgOwner represents a "LastOrgOwner" kind of error.
type ErrLastOrgOwner struct {
	UID int64
}

// IsErrLastOrgOwner checks if an error is a ErrLastOrgOwner.
func IsErrLastOrgOwner(err error) bool {
	_, ok := err.(ErrLastOrgOwner)
	return ok
}

func (err ErrLastOrgOwner) Error() string {
	return fmt.Sprintf("user is the last member of owner team [uid: %d]", err.UID)
}

// ErrUserNotAllowedCreateOrg represents a "UserNotAllowedCreateOrg" kind of error.
type ErrUserNotAllowedCreateOrg struct{}

// IsErrUserNotAllowedCreateOrg checks if an error is an ErrUserNotAllowedCreateOrg.
func IsErrUserNotAllowedCreateOrg(err error) bool {
	_, ok := err.(ErrUserNotAllowedCreateOrg)
	return ok
}

func (err ErrUserNotAllowedCreateOrg) Error() string {
	return "user is not allowed to create organizations"
}

func (err ErrUserNotAllowedCreateOrg) Unwrap() error {
	return util.ErrPermissionDenied
}

// Organization represents an organization
type Organization user_model.User

// OrgFromUser converts user to organization
func OrgFromUser(user *user_model.User) *Organization {
	return (*Organization)(user)
}

// TableName represents the real table name of Organization
func (Organization) TableName() string {
	return "user"
}

// IsOwnedBy returns true if given user is in the owner team.
func (org *Organization) IsOwnedBy(uid int64) (bool, error) {
	return IsOrganizationOwner(db.DefaultContext, org.ID, uid)
}

// IsOrgAdmin returns true if given user is in the owner team or an admin team.
func (org *Organization) IsOrgAdmin(uid int64) (bool, error) {
	return IsOrganizationAdmin(db.DefaultContext, org.ID, uid)
}

// IsOrgMember returns true if given user is member of organization.
func (org *Organization) IsOrgMember(uid int64) (bool, error) {
	return IsOrganizationMember(db.DefaultContext, org.ID, uid)
}

// CanCreateOrgRepo returns true if given user can create repo in organization
func (org *Organization) CanCreateOrgRepo(uid int64) (bool, error) {
	return CanCreateOrgRepo(db.DefaultContext, org.ID, uid)
}

// GetTeam returns named team of organization.
func (org *Organization) GetTeam(ctx context.Context, name string) (*Team, error) {
	return GetTeam(ctx, org.ID, name)
}

// GetOwnerTeam returns owner team of organization.
func (org *Organization) GetOwnerTeam(ctx context.Context) (*Team, error) {
	return org.GetTeam(ctx, OwnerTeamName)
}

// FindOrgTeams returns all teams of a given organization
func FindOrgTeams(ctx context.Context, orgID int64) ([]*Team, error) {
	var teams []*Team
	return teams, db.GetEngine(ctx).
		Where("org_id=?", orgID).
		OrderBy("CASE WHEN name LIKE '" + OwnerTeamName + "' THEN '' ELSE name END").
		Find(&teams)
}

// LoadTeams load teams if not loaded.
func (org *Organization) LoadTeams() ([]*Team, error) {
	return FindOrgTeams(db.DefaultContext, org.ID)
}

// GetMembers returns all members of organization.
func (org *Organization) GetMembers() (user_model.UserList, map[int64]bool, error) {
	return FindOrgMembers(&FindOrgMembersOpts{
		OrgID: org.ID,
	})
}

// HasMemberWithUserID returns true if user with userID is part of the u organisation.
func (org *Organization) HasMemberWithUserID(userID int64) bool {
	return org.hasMemberWithUserID(db.DefaultContext, userID)
}

func (org *Organization) hasMemberWithUserID(ctx context.Context, userID int64) bool {
	isMember, err := IsOrganizationMember(ctx, org.ID, userID)
	if err != nil {
		log.Error("IsOrganizationMember: %v", err)
		return false
	}
	return isMember
}

// AvatarLink returns the full avatar link with http host
func (org *Organization) AvatarLink(ctx context.Context) string {
	return org.AsUser().AvatarLink(ctx)
}

// HTMLURL returns the organization's full link.
func (org *Organization) HTMLURL() string {
	return org.AsUser().HTMLURL()
}

// OrganisationLink returns the organization sub page link.
func (org *Organization) OrganisationLink() string {
	return org.AsUser().OrganisationLink()
}

// ShortName ellipses username to length
func (org *Organization) ShortName(length int) string {
	return org.AsUser().ShortName(length)
}

// HomeLink returns the user or organization home page link.
func (org *Organization) HomeLink() string {
	return org.AsUser().HomeLink()
}

// CanCreateRepo returns if user login can create a repository
// NOTE: functions calling this assume a failure due to repository count limit; if new checks are added, those functions should be revised
func (org *Organization) CanCreateRepo() bool {
	return org.AsUser().CanCreateRepo()
}

// FindOrgMembersOpts represensts find org members conditions
type FindOrgMembersOpts struct {
	db.ListOptions
	OrgID      int64
	PublicOnly bool
}

// CountOrgMembers counts the organization's members
func CountOrgMembers(opts *FindOrgMembersOpts) (int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where("org_id=?", opts.OrgID)
	if opts.PublicOnly {
		sess.And("is_public = ?", true)
	}
	return sess.Count(new(OrgUser))
}

// FindOrgMembers loads organization members according conditions
func FindOrgMembers(opts *FindOrgMembersOpts) (user_model.UserList, map[int64]bool, error) {
	ous, err := GetOrgUsersByOrgID(db.DefaultContext, opts)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]int64, len(ous))
	idsIsPublic := make(map[int64]bool, len(ous))
	for i, ou := range ous {
		ids[i] = ou.UID
		idsIsPublic[ou.UID] = ou.IsPublic
	}

	users, err := user_model.GetUsersByIDs(ids)
	if err != nil {
		return nil, nil, err
	}
	return users, idsIsPublic, nil
}

// AsUser returns the org as user object
func (org *Organization) AsUser() *user_model.User {
	return (*user_model.User)(org)
}

// DisplayName returns full name if it's not empty,
// returns username otherwise.
func (org *Organization) DisplayName() string {
	return org.AsUser().DisplayName()
}

// CustomAvatarRelativePath returns user custom avatar relative path.
func (org *Organization) CustomAvatarRelativePath() string {
	return org.Avatar
}

// UnitPermission returns unit permission
func (org *Organization) UnitPermission(ctx context.Context, doer *user_model.User, unitType unit.Type) perm.AccessMode {
	if doer != nil {
		teams, err := GetUserOrgTeams(ctx, org.ID, doer.ID)
		if err != nil {
			log.Error("GetUserOrgTeams: %v", err)
			return perm.AccessModeNone
		}

		if err := teams.LoadUnits(ctx); err != nil {
			log.Error("LoadUnits: %v", err)
			return perm.AccessModeNone
		}

		if len(teams) > 0 {
			return teams.UnitMaxAccess(unitType)
		}
	}

	if org.Visibility.IsPublic() {
		return perm.AccessModeRead
	}

	return perm.AccessModeNone
}

// CreateOrganization creates record of a new organization.
func CreateOrganization(org *Organization, owner *user_model.User) (err error) {
	if !owner.CanCreateOrganization() {
		return ErrUserNotAllowedCreateOrg{}
	}

	if err = user_model.IsUsableUsername(org.Name); err != nil {
		return err
	}

	isExist, err := user_model.IsUserExist(db.DefaultContext, 0, org.Name)
	if err != nil {
		return err
	} else if isExist {
		return user_model.ErrUserAlreadyExist{Name: org.Name}
	}

	org.LowerName = strings.ToLower(org.Name)
	if org.Rands, err = user_model.GetUserSalt(); err != nil {
		return err
	}
	if org.Salt, err = user_model.GetUserSalt(); err != nil {
		return err
	}
	org.UseCustomAvatar = true
	org.MaxRepoCreation = -1
	org.NumTeams = 1
	org.NumMembers = 1
	org.Type = user_model.UserTypeOrganization

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = user_model.DeleteUserRedirect(ctx, org.Name); err != nil {
		return err
	}

	if err = db.Insert(ctx, org); err != nil {
		return fmt.Errorf("insert organization: %w", err)
	}
	if err = user_model.GenerateRandomAvatar(ctx, org.AsUser()); err != nil {
		return fmt.Errorf("generate random avatar: %w", err)
	}

	// Add initial creator to organization and owner team.
	if err = db.Insert(ctx, &OrgUser{
		UID:   owner.ID,
		OrgID: org.ID,
	}); err != nil {
		return fmt.Errorf("insert org-user relation: %w", err)
	}

	// Create default owner team.
	t := &Team{
		OrgID:                   org.ID,
		LowerName:               strings.ToLower(OwnerTeamName),
		Name:                    OwnerTeamName,
		AccessMode:              perm.AccessModeOwner,
		NumMembers:              1,
		IncludesAllRepositories: true,
		CanCreateOrgRepo:        true,
	}
	if err = db.Insert(ctx, t); err != nil {
		return fmt.Errorf("insert owner team: %w", err)
	}

	// insert units for team
	units := make([]TeamUnit, 0, len(unit.AllRepoUnitTypes))
	for _, tp := range unit.AllRepoUnitTypes {
		up := perm.AccessModeOwner
		if tp == unit.TypeExternalTracker || tp == unit.TypeExternalWiki {
			up = perm.AccessModeRead
		}
		units = append(units, TeamUnit{
			OrgID:      org.ID,
			TeamID:     t.ID,
			Type:       tp,
			AccessMode: up,
		})
	}

	if err = db.Insert(ctx, &units); err != nil {
		return err
	}

	if err = db.Insert(ctx, &TeamUser{
		UID:    owner.ID,
		OrgID:  org.ID,
		TeamID: t.ID,
	}); err != nil {
		return fmt.Errorf("insert team-user relation: %w", err)
	}

	return committer.Commit()
}

// GetOrgByName returns organization by given name.
func GetOrgByName(ctx context.Context, name string) (*Organization, error) {
	if len(name) == 0 {
		return nil, ErrOrgNotExist{0, name}
	}
	u := &Organization{
		LowerName: strings.ToLower(name),
		Type:      user_model.UserTypeOrganization,
	}
	has, err := db.GetEngine(ctx).Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrOrgNotExist{0, name}
	}
	return u, nil
}

// DeleteOrganization deletes models associated to an organization.
func DeleteOrganization(ctx context.Context, org *Organization) error {
	if org.Type != user_model.UserTypeOrganization {
		return fmt.Errorf("%s is a user not an organization", org.Name)
	}

	if err := db.DeleteBeans(ctx,
		&Team{OrgID: org.ID},
		&OrgUser{OrgID: org.ID},
		&TeamUser{OrgID: org.ID},
		&TeamUnit{OrgID: org.ID},
		&TeamInvite{OrgID: org.ID},
		&secret_model.Secret{OwnerID: org.ID},
	); err != nil {
		return fmt.Errorf("DeleteBeans: %w", err)
	}

	if _, err := db.GetEngine(ctx).ID(org.ID).Delete(new(user_model.User)); err != nil {
		return fmt.Errorf("Delete: %w", err)
	}

	return nil
}

// GetOrgUserMaxAuthorizeLevel returns highest authorize level of user in an organization
func (org *Organization) GetOrgUserMaxAuthorizeLevel(uid int64) (perm.AccessMode, error) {
	var authorize perm.AccessMode
	_, err := db.GetEngine(db.DefaultContext).
		Select("max(team.authorize)").
		Table("team").
		Join("INNER", "team_user", "team_user.team_id = team.id").
		Where("team_user.uid = ?", uid).
		And("team_user.org_id = ?", org.ID).
		Get(&authorize)
	return authorize, err
}

// GetUsersWhoCanCreateOrgRepo returns users which are able to create repo in organization
func GetUsersWhoCanCreateOrgRepo(ctx context.Context, orgID int64) (map[int64]*user_model.User, error) {
	// Use a map, in order to de-duplicate users.
	users := make(map[int64]*user_model.User)
	return users, db.GetEngine(ctx).
		Join("INNER", "`team_user`", "`team_user`.uid=`user`.id").
		Join("INNER", "`team`", "`team`.id=`team_user`.team_id").
		Where(builder.Eq{"team.can_create_org_repo": true}.Or(builder.Eq{"team.authorize": perm.AccessModeOwner})).
		And("team_user.org_id = ?", orgID).Find(&users)
}

// SearchOrganizationsOptions options to filter organizations
type SearchOrganizationsOptions struct {
	db.ListOptions
	All bool
}

// FindOrgOptions finds orgs options
type FindOrgOptions struct {
	db.ListOptions
	UserID         int64
	IncludePrivate bool
}

func queryUserOrgIDs(userID int64, includePrivate bool) *builder.Builder {
	cond := builder.Eq{"uid": userID}
	if !includePrivate {
		cond["is_public"] = true
	}
	return builder.Select("org_id").From("org_user").Where(cond)
}

func (opts FindOrgOptions) toConds() builder.Cond {
	var cond builder.Cond = builder.Eq{"`user`.`type`": user_model.UserTypeOrganization}
	if opts.UserID > 0 {
		cond = cond.And(builder.In("`user`.`id`", queryUserOrgIDs(opts.UserID, opts.IncludePrivate)))
	}
	if !opts.IncludePrivate {
		cond = cond.And(builder.Eq{"`user`.visibility": structs.VisibleTypePublic})
	}
	return cond
}

// FindOrgs returns a list of organizations according given conditions
func FindOrgs(opts FindOrgOptions) ([]*Organization, error) {
	orgs := make([]*Organization, 0, 10)
	sess := db.GetEngine(db.DefaultContext).
		Where(opts.toConds()).
		Asc("`user`.name")
	if opts.Page > 0 && opts.PageSize > 0 {
		sess.Limit(opts.PageSize, opts.PageSize*(opts.Page-1))
	}
	return orgs, sess.Find(&orgs)
}

// CountOrgs returns total count organizations according options
func CountOrgs(opts FindOrgOptions) (int64, error) {
	return db.GetEngine(db.DefaultContext).
		Where(opts.toConds()).
		Count(new(Organization))
}

// HasOrgOrUserVisible tells if the given user can see the given org or user
func HasOrgOrUserVisible(ctx context.Context, orgOrUser, user *user_model.User) bool {
	// If user is nil, it's an anonymous user/request.
	// The Ghost user is handled like an anonymous user.
	if user == nil || user.IsGhost() {
		return orgOrUser.Visibility == structs.VisibleTypePublic
	}

	if user.IsAdmin || orgOrUser.ID == user.ID {
		return true
	}

	if (orgOrUser.Visibility == structs.VisibleTypePrivate || user.IsRestricted) && !OrgFromUser(orgOrUser).hasMemberWithUserID(ctx, user.ID) {
		return false
	}
	return true
}

// HasOrgsVisible tells if the given user can see at least one of the orgs provided
func HasOrgsVisible(orgs []*Organization, user *user_model.User) bool {
	if len(orgs) == 0 {
		return false
	}

	for _, org := range orgs {
		if HasOrgOrUserVisible(db.DefaultContext, org.AsUser(), user) {
			return true
		}
	}
	return false
}

// GetOrgsCanCreateRepoByUserID returns a list of organizations where given user ID
// are allowed to create repos.
func GetOrgsCanCreateRepoByUserID(userID int64) ([]*Organization, error) {
	orgs := make([]*Organization, 0, 10)

	return orgs, db.GetEngine(db.DefaultContext).Where(builder.In("id", builder.Select("`user`.id").From("`user`").
		Join("INNER", "`team_user`", "`team_user`.org_id = `user`.id").
		Join("INNER", "`team`", "`team`.id = `team_user`.team_id").
		Where(builder.Eq{"`team_user`.uid": userID}).
		And(builder.Eq{"`team`.authorize": perm.AccessModeOwner}.Or(builder.Eq{"`team`.can_create_org_repo": true})))).
		Asc("`user`.name").
		Find(&orgs)
}

// GetOrgUsersByUserID returns all organization-user relations by user ID.
func GetOrgUsersByUserID(uid int64, opts *SearchOrganizationsOptions) ([]*OrgUser, error) {
	ous := make([]*OrgUser, 0, 10)
	sess := db.GetEngine(db.DefaultContext).
		Join("LEFT", "`user`", "`org_user`.org_id=`user`.id").
		Where("`org_user`.uid=?", uid)
	if !opts.All {
		// Only show public organizations
		sess.And("is_public=?", true)
	}

	if opts.PageSize != 0 {
		sess = db.SetSessionPagination(sess, opts)
	}

	err := sess.
		Asc("`user`.name").
		Find(&ous)
	return ous, err
}

// GetOrgUsersByOrgID returns all organization-user relations by organization ID.
func GetOrgUsersByOrgID(ctx context.Context, opts *FindOrgMembersOpts) ([]*OrgUser, error) {
	sess := db.GetEngine(ctx).Where("org_id=?", opts.OrgID)
	if opts.PublicOnly {
		sess.And("is_public = ?", true)
	}
	if opts.ListOptions.PageSize > 0 {
		sess = db.SetSessionPagination(sess, opts)

		ous := make([]*OrgUser, 0, opts.PageSize)
		return ous, sess.Find(&ous)
	}

	var ous []*OrgUser
	return ous, sess.Find(&ous)
}

// ChangeOrgUserStatus changes public or private membership status.
func ChangeOrgUserStatus(orgID, uid int64, public bool) error {
	ou := new(OrgUser)
	has, err := db.GetEngine(db.DefaultContext).
		Where("uid=?", uid).
		And("org_id=?", orgID).
		Get(ou)
	if err != nil {
		return err
	} else if !has {
		return nil
	}

	ou.IsPublic = public
	_, err = db.GetEngine(db.DefaultContext).ID(ou.ID).Cols("is_public").Update(ou)
	return err
}

// AddOrgUser adds new user to given organization.
func AddOrgUser(orgID, uid int64) error {
	isAlreadyMember, err := IsOrganizationMember(db.DefaultContext, orgID, uid)
	if err != nil || isAlreadyMember {
		return err
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	// check in transaction
	isAlreadyMember, err = IsOrganizationMember(ctx, orgID, uid)
	if err != nil || isAlreadyMember {
		return err
	}

	ou := &OrgUser{
		UID:      uid,
		OrgID:    orgID,
		IsPublic: setting.Service.DefaultOrgMemberVisible,
	}

	if err := db.Insert(ctx, ou); err != nil {
		return err
	} else if _, err = db.Exec(ctx, "UPDATE `user` SET num_members = num_members + 1 WHERE id = ?", orgID); err != nil {
		return err
	}

	return committer.Commit()
}

// GetOrgByID returns the user object by given ID if exists.
func GetOrgByID(ctx context.Context, id int64) (*Organization, error) {
	u := new(Organization)
	has, err := db.GetEngine(ctx).ID(id).Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, user_model.ErrUserNotExist{
			UID:   id,
			Name:  "",
			KeyID: 0,
		}
	}
	return u, nil
}

// RemoveOrgRepo removes all team-repository relations of organization.
func RemoveOrgRepo(ctx context.Context, orgID, repoID int64) error {
	teamRepos := make([]*TeamRepo, 0, 10)
	e := db.GetEngine(ctx)
	if err := e.Find(&teamRepos, &TeamRepo{OrgID: orgID, RepoID: repoID}); err != nil {
		return err
	}

	if len(teamRepos) == 0 {
		return nil
	}

	if _, err := e.Delete(&TeamRepo{
		OrgID:  orgID,
		RepoID: repoID,
	}); err != nil {
		return err
	}

	teamIDs := make([]int64, len(teamRepos))
	for i, teamRepo := range teamRepos {
		teamIDs[i] = teamRepo.TeamID
	}

	_, err := e.Decr("num_repos").In("id", teamIDs).Update(new(Team))
	return err
}

func (org *Organization) getUserTeams(ctx context.Context, userID int64, cols ...string) ([]*Team, error) {
	teams := make([]*Team, 0, org.NumTeams)
	return teams, db.GetEngine(ctx).
		Where("`team_user`.org_id = ?", org.ID).
		Join("INNER", "team_user", "`team_user`.team_id = team.id").
		Join("INNER", "`user`", "`user`.id=team_user.uid").
		And("`team_user`.uid = ?", userID).
		Asc("`user`.name").
		Cols(cols...).
		Find(&teams)
}

func (org *Organization) getUserTeamIDs(ctx context.Context, userID int64) ([]int64, error) {
	teamIDs := make([]int64, 0, org.NumTeams)
	return teamIDs, db.GetEngine(ctx).
		Table("team").
		Cols("team.id").
		Where("`team_user`.org_id = ?", org.ID).
		Join("INNER", "team_user", "`team_user`.team_id = team.id").
		And("`team_user`.uid = ?", userID).
		Find(&teamIDs)
}

// TeamsWithAccessToRepo returns all teams that have given access level to the repository.
func (org *Organization) TeamsWithAccessToRepo(repoID int64, mode perm.AccessMode) ([]*Team, error) {
	return GetTeamsWithAccessToRepo(db.DefaultContext, org.ID, repoID, mode)
}

// GetUserTeamIDs returns of all team IDs of the organization that user is member of.
func (org *Organization) GetUserTeamIDs(userID int64) ([]int64, error) {
	return org.getUserTeamIDs(db.DefaultContext, userID)
}

// GetUserTeams returns all teams that belong to user,
// and that the user has joined.
func (org *Organization) GetUserTeams(userID int64) ([]*Team, error) {
	return org.getUserTeams(db.DefaultContext, userID)
}

// AccessibleReposEnvironment operations involving the repositories that are
// accessible to a particular user
type AccessibleReposEnvironment interface {
	CountRepos() (int64, error)
	RepoIDs(page, pageSize int) ([]int64, error)
	Repos(page, pageSize int) (repo_model.RepositoryList, error)
	MirrorRepos() (repo_model.RepositoryList, error)
	AddKeyword(keyword string)
	SetSort(db.SearchOrderBy)
}

type accessibleReposEnv struct {
	org     *Organization
	user    *user_model.User
	team    *Team
	teamIDs []int64
	ctx     context.Context
	keyword string
	orderBy db.SearchOrderBy
}

// AccessibleReposEnv builds an AccessibleReposEnvironment for the repositories in `org`
// that are accessible to the specified user.
func AccessibleReposEnv(ctx context.Context, org *Organization, userID int64) (AccessibleReposEnvironment, error) {
	var user *user_model.User

	if userID > 0 {
		u, err := user_model.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		user = u
	}

	teamIDs, err := org.getUserTeamIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &accessibleReposEnv{
		org:     org,
		user:    user,
		teamIDs: teamIDs,
		ctx:     ctx,
		orderBy: db.SearchOrderByRecentUpdated,
	}, nil
}

// AccessibleTeamReposEnv an AccessibleReposEnvironment for the repositories in `org`
// that are accessible to the specified team.
func (org *Organization) AccessibleTeamReposEnv(team *Team) AccessibleReposEnvironment {
	return &accessibleReposEnv{
		org:     org,
		team:    team,
		ctx:     db.DefaultContext,
		orderBy: db.SearchOrderByRecentUpdated,
	}
}

func (env *accessibleReposEnv) cond() builder.Cond {
	cond := builder.NewCond()
	if env.team != nil {
		cond = cond.And(builder.Eq{"team_repo.team_id": env.team.ID})
	} else {
		if env.user == nil || !env.user.IsRestricted {
			cond = cond.Or(builder.Eq{
				"`repository`.owner_id":   env.org.ID,
				"`repository`.is_private": false,
			})
		}
		if len(env.teamIDs) > 0 {
			cond = cond.Or(builder.In("team_repo.team_id", env.teamIDs))
		}
	}
	if env.keyword != "" {
		cond = cond.And(builder.Like{"`repository`.lower_name", strings.ToLower(env.keyword)})
	}
	return cond
}

func (env *accessibleReposEnv) CountRepos() (int64, error) {
	repoCount, err := db.GetEngine(env.ctx).
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id").
		Where(env.cond()).
		Distinct("`repository`.id").
		Count(&repo_model.Repository{})
	if err != nil {
		return 0, fmt.Errorf("count user repositories in organization: %w", err)
	}
	return repoCount, nil
}

func (env *accessibleReposEnv) RepoIDs(page, pageSize int) ([]int64, error) {
	if page <= 0 {
		page = 1
	}

	repoIDs := make([]int64, 0, pageSize)
	return repoIDs, db.GetEngine(env.ctx).
		Table("repository").
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id").
		Where(env.cond()).
		GroupBy("`repository`.id,`repository`."+strings.Fields(string(env.orderBy))[0]).
		OrderBy(string(env.orderBy)).
		Limit(pageSize, (page-1)*pageSize).
		Cols("`repository`.id").
		Find(&repoIDs)
}

func (env *accessibleReposEnv) Repos(page, pageSize int) (repo_model.RepositoryList, error) {
	repoIDs, err := env.RepoIDs(page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("GetUserRepositoryIDs: %w", err)
	}

	repos := make([]*repo_model.Repository, 0, len(repoIDs))
	if len(repoIDs) == 0 {
		return repos, nil
	}

	return repos, db.GetEngine(env.ctx).
		In("`repository`.id", repoIDs).
		OrderBy(string(env.orderBy)).
		Find(&repos)
}

func (env *accessibleReposEnv) MirrorRepoIDs() ([]int64, error) {
	repoIDs := make([]int64, 0, 10)
	return repoIDs, db.GetEngine(env.ctx).
		Table("repository").
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id AND `repository`.is_mirror=?", true).
		Where(env.cond()).
		GroupBy("`repository`.id, `repository`.updated_unix").
		OrderBy(string(env.orderBy)).
		Cols("`repository`.id").
		Find(&repoIDs)
}

func (env *accessibleReposEnv) MirrorRepos() (repo_model.RepositoryList, error) {
	repoIDs, err := env.MirrorRepoIDs()
	if err != nil {
		return nil, fmt.Errorf("MirrorRepoIDs: %w", err)
	}

	repos := make([]*repo_model.Repository, 0, len(repoIDs))
	if len(repoIDs) == 0 {
		return repos, nil
	}

	return repos, db.GetEngine(env.ctx).
		In("`repository`.id", repoIDs).
		Find(&repos)
}

func (env *accessibleReposEnv) AddKeyword(keyword string) {
	env.keyword = keyword
}

func (env *accessibleReposEnv) SetSort(orderBy db.SearchOrderBy) {
	env.orderBy = orderBy
}
