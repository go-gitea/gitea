// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"

	"xorm.io/xorm"
)

// UserRepoUnit is an explosion (cartesian product) of user permissions
// on all allowed repositories for all valid and allowed unit types, with
// one record for each explicit combination of user + repo + unittype.
// General permissions for user classes (anonymous, identified, site
// administrator) are set associated to one of the special user IDs
// listed later below.

// Users that are not active yet or have prohibited login do not have any records.

// Combinations that result in no permission (AccessModeNone) are omitted to save space,
// so probing for simple existence against user_repo_unit provides a quick
// check for whether the user can see the repository at all (e.g. at the Explore page).

// Finally, permissions for units the repository does not have are also omitted.
// This means that checking for existing records with UnitTypeWiki will only
// return repositories that have a wiki.

// To check whether a given user has any permissions on a particular repository,
// a BEST_PERMISSION(set) operation must be performed (the highest permission
// must prevail):
//
//		SELECT MAX(mode) FROM user_repo_unit WHERE type = ? AND ...
//			Anonymous users:		user_repo_unit.user_id IN (UserRepoUnitAnyUser)
//			Restricted users:		user_repo_unit.user_id IN (user.id)
//			Regular users:			user_repo_unit.user_id IN (user.id, UserRepoUnitLoggedInUser)
//			Site administrators:	user_repo_unit.user_id IN (user.id, UserRepoUnitAdminUser)

// But the most common operation would be an INNER JOIN of type EXISTS;
// for example, to get a list of issues from repositories the user 99
// has UnitTypeCode access with AccessModeWrite permission or better:
//
//		SELECT issue.*
//		  FROM issue
//		 WHERE issue.repo_id IN
//			( SELECT user_repo_unit.repo_id
//			    FROM user_repo_unit
//			   WHERE user_repo_unit.user_id IN (99, UserRepoUnitLoggedInUser)
//				 AND user_repo_unit.type = UnitTypeCode
//				 AND user_repo_unit.mode >= AccessModeWrite )

// Except for the special user classes (anonymous, identified, site admin),
// only real users have records in the table; i.e. organizations don't
// have records of their own.

// Restricted users are users that require explicit permissions to access a repository;
// to get permission sets for these users, only records that match their UserID must
// be used (i.e. ignore UserRepoUnitLoggedInUser, etc).

// Input considered for calculating the records includes:
//
// * public/private status of the repository
// * public/limited/private status of the repository's owner (user or org)
// * user team membership (if repository owner is an org)
// * whether a team can access all repositories or only a specific list
// * user collaboration status
// * user ownership of the repository
// * user site admin status
// * which repository units are enabled (e.g. issues, PRs, etc.)

const (
	// These special constants represent user classes:

	// UserRepoUnitAnyUser is a special user ID used in the UserRepoUnit table
	// for permissions that apply to any user (even anonymous).
	UserRepoUnitAnyUser = int64(-1)

	// UserRepoUnitLoggedInUser is a special user ID used in the UserRepoUnit table
	// for permissions that apply to any identified user (logged in).
	// This set includes the permissions granted to UserRepoUnitAnyUser, so checking
	// both sets is not needed.
	UserRepoUnitLoggedInUser = int64(-2)

	// UserRepoUnitAdminUser is a special user ID used in the UserRepoUnit table
	// for permissions that apply to any site administrator.
	// As administrators have all possible permissions, checking UserRepoUnitAnyUser
	// and UserRepoUnitLoggedInUser is not required in their case.
	UserRepoUnitAdminUser = int64(-3)
)

// UserRepoUnit is an explosion (cartesian product) of all explicit user permissions
// on all repositories, and all types of relevant unit types.
type UserRepoUnit struct {
	UserID int64      `xorm:"pk"`
	RepoID int64      `xorm:"pk INDEX"`
	Type   UnitType   `xorm:"pk"`
	Mode   AccessMode `xorm:"NOT NULL"`
}

var (
	// This list may be updated as needed. Requisites are:
	// - New units need to abide by the same rules as the others
	// - Units from all repos must be rebuilt after this change
	UserRepoUnitSelectableTypes = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeReleases,
		UnitTypeWiki,
	}

	// Pre-built SQL condition to filter-out unsupported unit types
	// "repo_unit.`type` IN (UnitTypeCode, UnitTypeIssues, etc.)"
	userRepoUnitIsSelectable string

	// nextBatchID is a number used for easy correlation of operations
	// in the logs.
	nextBatchID int64

	// The temporary working table has different names according to the DB engine
	workTable string

	// Statements for creating and dropping the work table
	workTableCreate string
	workTableDrop   string

	repoUnitOnce sync.Once
)

func init() {
	// Build some data that we will use quite often
	uts := make([]string, len(UserRepoUnitSelectableTypes))
	for i, su := range UserRepoUnitSelectableTypes {
		uts[i] = fmt.Sprintf("%d", su)
	}
	userRepoUnitIsSelectable = "repo_unit.`type` IN (" + strings.Join(uts, ",") + ")"
}

// RebuildRepoUnits will rebuild all permissions to a given repository for all users
// excluding a specific team (use excludeTeamID == -1 to exclude no teams)
func RebuildRepoUnits(e Engine, repo *Repository, excludeTeamID int64) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildRepoUnits[%d]: rebuilding permissions for repository %d (excluding team %d)",
		batchID, repo.ID, excludeTeamID)

	if err = batchBuildRepoUnits(e, repo, excludeTeamID); err != nil {
		return fmt.Errorf("batchBuildRepoUnits[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	// Delete current data; we intend to replace all pairs
	_, err = e.Delete(&UserRepoUnit{RepoID: repo.ID})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("RebuildRepoUnits[%d]: permissions for repository %d (excluding team %d) rebuilt.",
		batchID, repo.ID, excludeTeamID)

	return nil
}

// RebuildUserUnits will rebuild all permissions for a given (real) user
func RebuildUserUnits(e Engine, user *User) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildUserUnits[%d]: rebuilding permissions for user %d", batchID, user.ID)

	if err = batchBuildUserUnits(e, user); err != nil {
		return fmt.Errorf("batchBuildUserUnits[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	// Delete current data; we intend to replace all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: user.ID})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("RebuildUserUnits[%d]: permissions for user %d rebuilt.", batchID, user.ID)

	return nil
}

// RebuildUserIDUnits will rebuild all permissions for a given (real) user
func RebuildUserIDUnits(e Engine, userID int64) (err error) {
	if user, err := getUserByID(e, userID); err != nil {
		return err
	} else {
		return RebuildUserUnits(e, user)
	}
}

// RebuildAdminUnits will rebuild all permissions for the site admin user class
func RebuildAdminUnits(e Engine) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildAdminUnits[%d]: rebuilding permissions for site administrators", batchID)

	if err = batchBuildAdminUnits(e); err != nil {
		return fmt.Errorf("batchBuildAdminUnits[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	// Delete current data; we intend to replace all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: UserRepoUnitAdminUser})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("RebuildAdminUnits[%d]: permissions for site administrators rebuilt.", batchID)

	return nil
}

// RebuildLoggedInUnits will rebuild all permissions for generic logged in users
func RebuildLoggedInUnits(e Engine) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildLoggedInUnits[%d]: rebuilding permissions for logged in users", batchID)

	if err = batchBuildLoggedInUnits(e); err != nil {
		return fmt.Errorf("batchBuildLoggedInUnits[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	// Delete current data; we intend to replace all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: UserRepoUnitLoggedInUser})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("RebuildLoggedInUnits[%d]: permissions for logged in users rebuilt.", batchID)

	return nil
}

// RebuildAnonymousUnits will rebuild all permissions for unidentified users
func RebuildAnonymousUnits(e Engine) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildAnonymousUnits[%d]: rebuilding permissions for unidentified users", batchID)

	if err = batchBuildAnonymousUnits(e); err != nil {
		return fmt.Errorf("batchBuildAnonymousUnits[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	// Delete current data; we intend to replace all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: UserRepoUnitAnyUser})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("RebuildAnonymousUnits[%d]: permissions for unidentified users rebuilt.", batchID)

	return nil
}

// RebuildUserRepoUnits will rebuild permissions for a given (real) user on a given repository
func RebuildUserRepoUnits(e Engine, user *User, repo *Repository) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildUserUnits[%d]: rebuilding permissions for user %d, repo %d", batchID, user.ID, repo.ID)

	if err = batchBuildUserRepoUnits(e, user, repo); err != nil {
		return fmt.Errorf("batchBuildUserRepoUnits[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for the user into the user_repo_unit table
	// ***********************************************************************************

	// Delete current data; we intend to replace all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: user.ID, RepoID: repo.ID})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("RebuildUserUnits[%d]: permissions for user %d, repo %d rebuilt.", batchID, user.ID, repo.ID)

	return nil
}

// RebuildUserIDRepoUnits will rebuild permissions for a given (real) user on a given repository
func RebuildUserIDRepoUnits(e Engine, userID int64, repo *Repository) (err error) {
	if user, err := getUserByID(e, userID); err != nil {
		return err
	} else {
		return RebuildUserRepoUnits(e, user, repo)
	}
}

// RebuildTeamUnits will rebuild all the user/repo permission pairs known by a team
// while optionally removing the permissions provided by the team itself.
func RebuildTeamUnits(e Engine, team *Team, exclude bool) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildTeamUnits[%d]: rebuilding permissions for team %d (exclude:%v)", batchID, team.ID, exclude)

	excludeID := int64(0)
	if exclude {
		excludeID = team.ID
	}

	// Since there's no specific set of records that we can identify as belonging
	// to the team (they're associated to a user and a repository instead) we have
	// no basis on how to remove any permissions that may come from this team.
	// That's why we will make a list of the affected repositories and rebuild
	// from there.

	if team.IncludesAllRepositories {
		// All repositories from the organization are affected.
		// Must rebuild all of them.
		const batchSize = 32
		for start := 0; ; start += batchSize {
			repos := make([]*Repository, 0, batchSize)
			if err = x.Asc("id").Limit(batchSize, start).
				Where("repository.owner_id = ?", team.OrgID).
				Find(&repos); err != nil {
				return err
			}
			if len(repos) == 0 {
				break
			}
			for _, repo := range repos {
				// Make sure we start from scratch; we intend to recreate all pairs
				log.Trace("RebuildTeamUnits[%d]: rebuilding permissions for repository %d", batchID, repo.ID)
				_, err = e.Delete(&UserRepoUnit{RepoID: repo.ID})
				if err != nil {
					return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
				}
				// batchBuildRepoUnits will exclude team.ID from the generated records if exclude == true
				if err = batchBuildRepoUnits(e, repo, excludeID); err != nil {
					return fmt.Errorf("batchBuildRepoUnits[%d]: %v", batchID, err)
				}
				log.Trace("RebuildTeamUnits[%d]: permissions for repository %d rebuilt", batchID, repo.ID)
			}
		}

	} else {

		if err = team.getRepositories(e); err != nil {
			return fmt.Errorf("team.getRepositories[%d]: %v", batchID, err)
		}

		for _, repo := range team.Repos {
			// Make sure we start from scratch; we intend to recreate all pairs
			log.Trace("RebuildTeamUnits[%d]: rebuilding permissions for repository %d", batchID, repo.ID)
			_, err = e.Delete(&UserRepoUnit{RepoID: repo.ID})
			if err != nil {
				return fmt.Errorf("DELETE user_repo_unit [%d]: %v", batchID, err)
			}
			// batchBuildRepoUnits will exclude team.ID from the generated records if exclude == true
			if err = batchBuildRepoUnits(e, repo, excludeID); err != nil {
				return fmt.Errorf("batchBuildRepoUnits[%d]: %v", batchID, err)
			}
			log.Trace("RebuildTeamUnits[%d]: permissions for repository %d rebuilt", batchID, repo.ID)
		}
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("RebuildTeamUnits[%d]: permissions for team %d (exclude:%v) rebuilt.", batchID, team.ID, exclude)

	return nil
}

func AddTeamRepoUnits(e Engine, team *Team, repo *Repository) (err error) {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("AddTeamRepoUnits[%d]: adding permissions on repo %d for team %d", batchID, repo.ID, team.ID)

	// Adding permissions is easier than removing; we just need to honor the previous
	// set of permissions.

	// *******************************************************************************************
	// Insert pre-existing permissions the team users may have (e.g. as admins or collaborators)
	// *******************************************************************************************

	err = batchInsertWork(e,
		"SELECT user_id, repo_id, type, mode "+
			"FROM user_repo_unit "+
			"INNER JOIN team_user ON team_user.uid = user_repo_unit.user_id "+
			"WHERE team_user.team_id = ? AND user_repo_unit.repo_id = ?",
		team.ID, repo.ID)
	if err != nil {
		return fmt.Errorf("AddTeamRepoUnits (INSERT): %v", err)
	}

	// ***********************************************************************************
	// Add permissions that come from the team itself
	// ***********************************************************************************

	if err = batchAddTeamRepoUnits(e, team, repo); err != nil {
		return fmt.Errorf("batchAddTeamRepoUnits[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Remove pre-existing permissions that we are replacing
	// ***********************************************************************************

	// Since we will insert records from user_repo_unit_work into user_repo_unit,
	// we must remove any records from user_repo_unit first. Otherwise we will
	// get collisions.
	if err = userRepoUnitRemoveWorking(e); err != nil {
		return fmt.Errorf("userRepoUnitRemoveWorking[%d]: %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = userRepoUnitsFinishBatch(e); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch[%d]: %v", batchID, err)
	}

	log.Trace("AddTeamRepoUnits[%d]: permissions on repo %d for team %d added.", batchID, repo.ID, team.ID)

	return nil
}

// batchBuildRepoUnits will build batch data for all users on a given repository, excluding one team
func batchBuildRepoUnits(e Engine, repo *Repository, excludeTeamID int64) (err error) {

	// user_repo_unit_work is expected to contain no records related to this repository
	// for the current batch (i.e. this function will only _add_ permissions).

	// excludeTeamID may indicate a team ID to exclude from calculations; this is used
	// when a team is deleted or downgraded (e.g. given less permissions).
	// If no team is to be excluded, excludeTeamID must be -1.

	//	== Usage scenarios ==
	//
	//	- A repository is created or deleted
	//	- Repository visibility is modified (public/private)
	//	- Owner's visibility is modified (public/limited/private)
	//	- Repository units are modified (e.g. add/remove wiki)
	//	- A repository is added/removed from a team
	//	- Access level changed for a team that has access to this repository
	//	- A repository changes owners
	//	- All permissions in the system are rebuilt

	// ****************************************************************************
	// Insert permissions for site admins
	// ****************************************************************************

	// Important: these admin permissions are unspecific; "prohibit_login" and "active"
	// need to be handled separately for admins where the context is appropriate.

	err = batchInsertWork(e,
		"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repo_unit "+
			"WHERE repo_unit.repo_id = ? AND "+userRepoUnitIsSelectable,
		UserRepoUnitAdminUser, AccessModeAdmin, repo.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (repo admins): %v", err)
	}

	if err = repo.getOwner(e); err != nil {
		if IsErrUserNotExist(err) {
			// Allow abnormalities like a missing owner to avoid breaking batches
			log.Warn("batchBuildRepoUnits: missing owner %d for repository %d", repo.OwnerID, repo.ID)
			// Since the repository has no owner, nobody but the admins should have permissions
			return nil
		}
		return fmt.Errorf("getOwner: %v", err)
	}

	if repo.Owner.IsOrganization() {

		// ****************************************************************************
		// Insert permissions for the members of teams that have access to this repo
		// ****************************************************************************

		// This query will cover all teams with <includes_all_repositories = false>.
		// "Find all users belonging to teams to which this repository is explicitly assigned;
		//  cross with all relevant units enabled for this repo. Assign the access configured
		//  in the team."
		err = batchInsertWork(e,
			"SELECT `user`.id, repo_unit.repo_id, team_unit.`type`, team.`authorize` "+
				"FROM team_repo "+
				"INNER JOIN team ON team.id = team_repo.team_id "+
				"INNER JOIN team_unit ON team_unit.team_id = team.id "+
				"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
				"INNER JOIN team_user ON team_user.team_id = team.id "+
				"INNER JOIN `user` ON `user`.id = team_user.uid "+
				"WHERE team_repo.repo_id = ? AND team.includes_all_repositories = ? "+
				"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
				"AND "+userRepoUnitIsSelectable+" "+
				"AND team.id <> ? ", // excludeTeamID will be -1 if no team is to be excluded
			repo.ID, false, true, false, UserTypeIndividual, excludeTeamID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (repo teams, !include_all): %v", err)
		}

		// This query will cover all teams with <includes_all_repositories = true>
		// "Find all users belonging to teams of the same organization as the repository owner;
		//  cross with all relevant units enabled for this repo. Assign the access configured
		//  in the team."
		err = batchInsertWork(e,
			"SELECT `user`.id, repo_unit.repo_id, team_unit.`type`, team.`authorize` "+
				"FROM team "+
				"INNER JOIN team_unit ON team_unit.team_id = team.id "+
				"INNER JOIN team_user ON team_user.team_id = team.id "+
				"INNER JOIN repo_unit ON repo_unit.repo_id = ? AND repo_unit.`type` = team_unit.`type` "+
				"INNER JOIN `user` ON `user`.id = team_user.uid "+
				"WHERE team.org_id = ? AND team.includes_all_repositories = ? "+
				"AND "+userRepoUnitIsSelectable+" "+
				"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
				"AND team.id <> ?", // excludeTeamID will be -1 if no team is to be excluded
			repo.ID, repo.OwnerID, true, true, false, UserTypeIndividual, excludeTeamID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (repo teams, include_all): %v", err)
		}

	} else if repo.Owner.IsActive && !repo.Owner.ProhibitLogin {

		// ****************************************************************************
		// Insert permissions for the owner (if not inhibited)
		// ****************************************************************************

		// "Find all relevant units for this repository. Assign AccessModeOwner access to repo.OwnerID."
		err = batchInsertWork(e,
			"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
				"FROM repo_unit "+
				"WHERE repo_unit.repo_id = ? AND "+userRepoUnitIsSelectable,
			repo.OwnerID, AccessModeOwner, repo.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (repo owner): %v", err)
		}
	}

	// ****************************************************************************
	// Insert permissions for collaborators
	// ****************************************************************************

	// "Find all users collaborating on this repository; cross with all relevant units
	//  enabled for this repo. Assign access specified by the collaboration."
	err = batchInsertWork(e,
		"SELECT `user`.id, repo_unit.repo_id, repo_unit.`type`, collaboration.`mode` "+
			"FROM collaboration "+
			"INNER JOIN `user` ON `user`.id = collaboration.user_id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = collaboration.repo_id "+
			"WHERE collaboration.repo_id = ? "+
			"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
			"AND "+userRepoUnitIsSelectable,
		repo.ID, true, false, UserTypeIndividual)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (repo collaborators): %v", err)
	}

	if !repo.IsPrivate {

		// ****************************************************************************
		// Process repositories not marked as 'private'
		// ****************************************************************************

		// Public repositories (IsPrivate == false) give read access to "everybody",
		// but what "everybody" actually means depends on the visibility of the repository owner.

		if repo.Owner.Visibility == structs.VisibleTypePrivate {

			if repo.Owner.IsOrganization() {

				// ****************************************************************************
				// Public repository of a hidden (private) organization
				// ****************************************************************************

				// All members of the organization get at least read access to the repository.
				// Better permissions could be granted via teams.
				// "Find all valid users belonging to the same organization as the repo;
				//  cross with all relevant units enabled for this repo. Assign AccessModeRead
				//  to each user."
				err = batchInsertWork(e,
					"SELECT `user`.id, repo_unit.repo_id, repo_unit.`type`, ? "+
						"FROM `user` "+
						"INNER JOIN repo_unit ON repo_unit.repo_id = ? "+
						"WHERE `user`.id IN ("+
						"  SELECT team_user.uid "+
						"  FROM team_user "+
						"  WHERE team_user.org_id = ? AND team_user.team_id <> ? "+ // excludeTeamID will be -1 if no team is to be excluded
						") "+
						"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
						"AND "+userRepoUnitIsSelectable,
					AccessModeRead, repo.ID, repo.OwnerID, excludeTeamID, true, false, UserTypeIndividual)
				if err != nil {
					return fmt.Errorf("INSERT INTO user_repo_unit_work (repo public for org): %v", err)
				}
			}
			/* } else {		 - empty else (intended) not allowed by linter

				// ****************************************************************************
				// Public repository for a "hidden" (private) user
				// ****************************************************************************

				// Currently, only organizations can have visibility == "private",
				// but we can support the same for plain users as well by simply doing nothing here,
				// preventing the creation of permissions for UserRepoUnitLoggedInUser and
				// UserRepoUnitAnyUser.
			} */

		} else {

			// *************************************************************************************
			// "Public" repository for a visible or limited user or organization (logged in users)
			// *************************************************************************************

			// There's one record representing "logged in users" (UserRepoUnitLoggedInUser);
			// this simplifies the queries for permission verification later.

			// This query covers organizations with Visibility == structs.VisibleTypeLimited
			// "Find all relevant units for this repository. Assign AccessModeRead
			//  to UserRepoUnitLoggedInUser".
			err = batchInsertWork(e,
				"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
					"FROM repo_unit "+
					"WHERE repo_unit.repo_id = ? AND "+userRepoUnitIsSelectable,
				UserRepoUnitLoggedInUser, AccessModeRead, repo.ID)
			if err != nil {
				return fmt.Errorf("INSERT user_repo_unit_work (repo, logged in): %v", err)
			}

			if repo.Owner.Visibility == structs.VisibleTypePublic {

				// *******************************************************************************
				// "Public" repository for a fully-visible user or organization (anonymous users)
				// *******************************************************************************

				// Records for users that are not logged in.
				// Whether the site requires all users to be logged in to access the data
				// must be considered separately.
				// "Find all relevant units for this repository. Assign AccessModeRead
				//  to UserRepoUnitAnyUser".
				err = batchInsertWork(e,
					"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
						"FROM repo_unit "+
						"WHERE repo_unit.repo_id = ? AND "+userRepoUnitIsSelectable,
					UserRepoUnitAnyUser, AccessModeRead, repo.ID)
				if err != nil {
					return fmt.Errorf("INSERT INTO user_repo_unit_work (repo public, anonymous): %v", err)
				}
			}
		}
	}

	return nil
}

// batchBuildUserUnits will build batch data for a given user on any explicitly
func batchBuildUserUnits(e Engine, user *User) (err error) {

	// user_repo_unit is expected to contain no records related to this user
	// for the current batch (i.e. this function will only _add_ permissions).

	// Important: these function add permissions specific to each user;
	// user classes (site administrator, identified users, anonymous users)
	// are not handled here but at the repository level.

	//	== Usage scenarios ==
	//
	//	- A user is created or deleted
	//	- A user's login permissions has changed (IsActive, ProhibitLogin)
	//	- A user is added/removed from a team (*)
	//
	//	(*) For adding a user to a team, batchAddTeamUserUnits is more efficient

	if !user.IsActive || user.ProhibitLogin || user.IsOrganization() {
		// No specific permissions for inactive users.
		// Organizations have no permissions themselves; only their members do.
		return nil
	}

	// ****************************************************************************
	// Normal user, owned repositories
	// ****************************************************************************

	// "Find and create records for each relevant unit type that any repository owned
	//  by the user has configured. Assign AccessModeOwner access to the user.
	err = batchInsertWork(e,
		"SELECT repository.owner_id, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repository "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id "+
			"WHERE repository.owner_id = ? AND "+userRepoUnitIsSelectable,
		AccessModeOwner, user.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user: owner): %v", err)
	}

	// ****************************************************************************
	// Normal user, collaborations on repositories
	// ****************************************************************************

	// "Find all collaborations for the user; cross with all relevant units enabled enabled
	//  for their repositories. Assign access specified by the collaboration to the user."
	err = batchInsertWork(e,
		"SELECT collaboration.user_id, collaboration.repo_id, repo_unit.`type`, collaboration.`mode` "+
			"FROM collaboration "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = collaboration.repo_id "+
			"WHERE collaboration.user_id = ? "+
			"AND "+userRepoUnitIsSelectable,
		user.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user collaborator): %v", err)
	}

	// ****************************************************************************
	// Normal user, teams they belong to
	// ****************************************************************************

	// This query will cover all teams with <includes_all_repositories = false> the user belongs to.
	// "Find all repos assigned to teams this user belongs to. Assign access specified by the
	//  team to the user."
	err = batchInsertWork(e,
		"SELECT team_user.uid, team_repo.repo_id, team_unit.`type`, team.authorize "+
			"FROM team_user "+
			"INNER JOIN team ON team.id = team_user.team_id "+
			"INNER JOIN team_repo ON team_repo.team_id = team.id "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
			"WHERE team_user.uid = ? AND team.includes_all_repositories = ? "+
			"AND "+userRepoUnitIsSelectable,
		user.ID, false)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user teams, !include_all): %v", err)
	}

	// This query will cover all teams with <includes_all_repositories = true> the user belongs to.
	// "Find all repositories belonging to organizations with teams this user belongs to.
	//  Assign access specified by the team to the user."
	err = batchInsertWork(e,
		"SELECT team_user.uid, repository.id, repo_unit.`type`, team.authorize "+
			"FROM team_user "+
			"INNER JOIN team ON team.id = team_user.team_id "+
			"INNER JOIN repository ON repository.owner_id = team.org_id "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id AND repo_unit.`type` = team_unit.`type` "+
			"WHERE team_user.uid = ? AND team.includes_all_repositories = ? "+
			"AND "+userRepoUnitIsSelectable,
		user.ID, true)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user teams, include_all): %v", err)
	}

	return nil
}

// batchBuildAdminUnits will build batch data for the site administrator user class
// on all repositories
func batchBuildAdminUnits(e Engine) (err error) {

	// ****************************************************************************
	// Process all repositories
	// ****************************************************************************

	//	== Usage scenarios ==
	//
	//	- TBD

	// "Find all relevant units for all repositories"
	err = batchInsertWork(e,
		"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repo_unit "+
			"WHERE "+userRepoUnitIsSelectable,
		UserRepoUnitAdminUser, AccessModeAdmin)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (admin): %v", err)
	}

	return nil
}

// batchBuildLoggedInUnits will build batch data for the generic logged in user
// on all non-private repositories
func batchBuildLoggedInUnits(e Engine) (err error) {

	// ****************************************************************************
	// Process repositories not marked as 'private'
	// ****************************************************************************

	// Public repositories (IsPrivate == false) give read access to all identified users
	// as long as their owner is public or limited (regular users are always public).

	//	== Usage scenarios ==
	//
	//	- TBD

	// "Find all relevant units for all repositories that are not marked as private
	//  and whos owner's visibility is public or limited"
	err = batchInsertWork(e,
		"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repository "+
			"INNER JOIN `user` ON `user`.id = repository.owner_id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id "+
			"WHERE repository.is_private = ? "+
			"AND `user`.visibility IN (?,?) AND "+userRepoUnitIsSelectable,
		UserRepoUnitLoggedInUser, AccessModeRead,
		false, structs.VisibleTypePublic, structs.VisibleTypeLimited)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (logged in): %v", err)
	}

	return nil
}

// batchBuildAnonymousUnits will build batch data for users that have not identified
// themselves on all public repositories
func batchBuildAnonymousUnits(e Engine) (err error) {

	// ****************************************************************************
	// Process repositories marked as 'public'
	// ****************************************************************************

	// Public repositories (IsPrivate == false) give read access to all users
	// (identified or not) as long as their owner (regular or organization)
	// is also public.

	//	== Usage scenarios ==
	//
	//	- TBD

	// "Find all relevant units for all repositories that are not marked as private
	//  and whos owner's visibility is public"
	err = batchInsertWork(e,
		"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repository "+
			"INNER JOIN `user` ON `user`.id = repository.owner_id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id "+
			"WHERE repository.is_private = ? "+
			"AND `user`.visibility = ? AND "+userRepoUnitIsSelectable,
		UserRepoUnitAnyUser, AccessModeRead,
		false, structs.VisibleTypePublic)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (logged in): %v", err)
	}

	return nil
}

// batchBuildUserRepoUnits will build batch data for a given user on a specific repository
// (e.g. if added/removed as a collaborator, from a team, etc.)
func batchBuildUserRepoUnits(e Engine, user *User, repo *Repository) (err error) {

	// user_repo_unit is expected to contain no records related to this user
	// for the current batch (i.e. this function will only _add_ permissions).

	// Important: these function add permissions specific to each user;
	// user classes (site administrator, identified users, anonymous users)
	// are not handled here but at the repository level.

	//	== Usage scenarios ==
	//
	//	- A user is added/removed from a collaboration
	//	- A user collaboration is modified

	if !user.IsActive || user.ProhibitLogin || user.IsOrganization() {
		// No permissions for inactive users
		// Organizations have no permissions themselves; only their members do
		return nil
	}

	if err = repo.getOwner(e); err != nil {
		return fmt.Errorf("getOwner: %v", err)
	}

	if user.ID == repo.OwnerID {

		// ****************************************************************************
		// Regular user, owner of the repository
		// ****************************************************************************

		err = batchInsertWork(e,
			"SELECT ?, repo_unit.repo_id, repo_unit.`type`, ? "+
				"FROM repo_unit "+
				"WHERE repo_unit.repo_id = ? AND "+userRepoUnitIsSelectable,
			user.ID, AccessModeOwner, repo.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user/repo: owner): %v", err)
		}

		// Owner permission on a 1:1 relationship is the best the user can get
		// for any repository
		return nil
	}

	// User can get owner access from teams, for instance

	// ****************************************************************************
	// Regular user, access granted by being a collaborator to the repository
	// ****************************************************************************

	err = batchInsertWork(e,
		"SELECT collaboration.user_id, collaboration.repo_id, repo_unit.`type`, collaboration.`mode` "+
			"FROM collaboration "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = collaboration.repo_id "+
			"WHERE collaboration.user_id = ? AND collaboration.repo_id = ? "+
			"AND "+userRepoUnitIsSelectable,
		user.ID, repo.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user/repo collaborator): %v", err)
	}

	// ****************************************************************************
	// Regular user, access granted by belonging to teams that own the repository
	// ****************************************************************************

	if repo.Owner.IsOrganization() {

		// This query will cover all teams with <includes_all_repositories = false> the user belongs to.
		// "Find teams from the repository's owner this user belongs to that have access to this repository.
		//  Assign access specified by the team to the user."
		err = batchInsertWork(e,
			"SELECT team_user.uid, team_repo.repo_id, team_unit.`type`, team.authorize "+
				"FROM team_repo "+
				"INNER JOIN team_user ON team_user.team_id = team_repo.team_id "+
				"INNER JOIN team ON team.id = team_repo.team_id "+
				"INNER JOIN team_unit ON team_unit.team_id = team.id "+
				"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
				"WHERE team_repo.repo_id = ? AND team.includes_all_repositories = ? "+
				"AND team_user.uid = ? AND "+userRepoUnitIsSelectable,
			repo.ID, false, user.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user/repo teams, !include_all): %v", err)
		}

		// This query will cover all teams with <includes_all_repositories = true> the user belongs to.
		// "Find teams from this repository's owner organization this user belongs to which have access to
		//  all repositories in the organization. Assign access specified by the team to the user."
		err = batchInsertWork(e,
			"SELECT team_user.uid, repo_unit.repo_id, repo_unit.`type`, team.authorize "+
				"FROM team "+
				"INNER JOIN team_user ON team_user.team_id = team.id "+
				"INNER JOIN team_unit ON team_unit.team_id = team.id "+
				"INNER JOIN repo_unit ON repo_unit.`type` = team_unit.`type` "+
				"WHERE repo_unit.repo_id = ? AND team.org_id = ? AND team.includes_all_repositories = ? "+
				"AND team_user.uid = ? AND "+userRepoUnitIsSelectable,
			repo.ID, repo.OwnerID, true, user.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user/repo teams, include_all): %v", err)
		}
	}

	return nil
}

// batchAddTeamUserUnits will add batch data for a given user on a team's repositories
func batchAddTeamUserUnits(e Engine, user *User, team *Team) (err error) {

	// This function will add permissions given to a user by a team membership

	//	== Usage scenarios ==
	//
	//	- A user is added to a team

	if team.IncludesAllRepositories {
		// This query will give the user access to all repositories in the organization.
		// "Find all repositories belonging to this team's organization.
		//  Assign access specified by the team to the user."
		err = batchInsertWork(e,
			"SELECT ?, repository.id, repo_unit.`type`, ? "+
				"FROM repository "+
				"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id "+
				"INNER JOIN team_unit ON team_unit.`type` = repo_unit.`type` "+
				"WHERE repository.owner_id = ? AND team_unit.team_id = ? "+
				"AND "+userRepoUnitIsSelectable,
			user.ID, team.Authorize, team.OrgID, team.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user team, include_all): %v", err)
		}
	} else {
		// This query will give the user access to the repos assigned to the team.
		// "Find all repos assigned to this team. Assign access specified by the
		//  team to the user."
		err = batchInsertWork(e,
			"SELECT ?, team_repo.repo_id, team_unit.`type`, team.authorize "+
				"FROM team_repo "+
				"INNER JOIN team_unit ON team_unit.team_id = team_repo.team_id "+
				"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
				"WHERE team_repo.team_id = ? "+
				"AND "+userRepoUnitIsSelectable,
			user.ID, team.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user team, !include_all): %v", err)
		}
	}

	return nil
}

// batchAddTeamRepoUnits will add batch data for a team's users on a given repository
func batchAddTeamRepoUnits(e Engine, team *Team, repo *Repository) (err error) {

	// This function will add permissions on a repository to users on the specified team

	//	== Usage scenarios ==
	//
	//	- A repository is added to a team

	// Only teams that have a specific list of repositories need to be considered

	if team.IncludesAllRepositories {
		return nil
	}

	// This query will give the team members access to the repository.
	// "Find all users that are members of the team. Assign access specified by the
	//  team to the repository."
	err = batchInsertWork(e,
		"SELECT team_user.uid, ?, team_unit.`type`, team.authorize "+
			"FROM team_repo "+
			"INNER JOIN team_user ON team_user.team_id = team_repo.team_id "+
			"INNER JOIN team_unit ON team_unit.team_id = team_repo.team_id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
			"WHERE team_repo.team_id = ? "+
			"AND "+userRepoUnitIsSelectable,
		repo.ID, team.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (repo team): %v", err)
	}

	return nil
}

// RebuildAllUserRepoUnits will rebuild the whole user_repo_unit table from scratch
func RebuildAllUserRepoUnits(xe *xorm.Engine) (err error) {
	// Don't get too greedy on the batches
	const repoBatchCount = 20

	if _, err = xe.Exec("DELETE FROM user_repo_unit"); err != nil {
		return fmt.Errorf("addUserRepoUnit: DELETE old data: %v", err)
	}

	var maxid int64
	if _, err = xe.Table("repository").Select("MAX(id)").Get(&maxid); err != nil {
		return fmt.Errorf("addUserRepoUnit: get MAX(repo_id): %v", err)
	}

	// Create access data for the first time
	for i := int64(1); i <= maxid; i += repoBatchCount {
		if err = rangeBuildRepoUnits(xe, i, repoBatchCount); err != nil {
			return fmt.Errorf("rangeBuildRepoUnits(%d,%d): %v", i, repoBatchCount, err)
		}
	}

	return nil
}

// rangeBuildRepoUnits will rebuild permissions for a range of repository IDs
func rangeBuildRepoUnits(xe *xorm.Engine, fromID int64, count int) (err error) {
	// Use a single transaction for the batch
	sess := xe.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	repos := make([]*Repository, 0, count)
	if err = sess.Where("id BETWEEN ? AND ?", fromID, fromID+int64(count-1)).Find(&repos); err != nil {
		return fmt.Errorf("Find repositories: %v", err)
	}

	// Some ID ranges might be empty
	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		if err = RebuildRepoUnits(sess, repo, -1); err != nil {
			return fmt.Errorf("RebuildRepoUnits(%d): %v", repo.ID, err)
		}
	}

	return sess.Commit()
}

// userRepoUnitStartBatch will create the temporary work table
// and return a unique ID for the batch transaction
func userRepoUnitStartBatch(e Engine) (int64, error) {

	// user_repo_unit_work is a table used for temporarily accumulate all the work performed
	// while processing a batch. It's created for each batch operation as a temporary table.
	// Temporary tables are invisible to other sessions (each can have their own)
	// and are automatically dropped when the connection is closed (or reused).
	// They're also usually only kept in memory.

	repoUnitOnce.Do(func() {
		// Build create/drop statements for the temporary work table
		// We can do this only after setting.Database has been set.
		switch {
		case setting.Database.UseMSSQL:
			workTable = "#user_repo_unit_work"
			workTableCreate = "SELECT * INTO #user_repo_unit_work FROM user_repo_unit WHERE 0 = 1"
			workTableDrop = "DROP TABLE #user_repo_unit_work"
		case setting.Database.UseMySQL:
			workTable = "user_repo_unit_work"
			workTableCreate = "CREATE TEMPORARY TABLE user_repo_unit_work  AS SELECT * FROM user_repo_unit WHERE 0 = 1"
			workTableDrop = "DROP TEMPORARY TABLE user_repo_unit_work"
		case setting.Database.UsePostgreSQL:
			workTable = "user_repo_unit_work"
			workTableCreate = "CREATE TEMPORARY TABLE user_repo_unit_work  AS SELECT * FROM user_repo_unit WHERE 0 = 1"
			workTableDrop = "DROP TEMPORARY TABLE user_repo_unit_work"
		case setting.Database.UseSQLite3:
			workTable = "temp.user_repo_unit_work"
			workTableCreate = "CREATE TEMPORARY TABLE temp.user_repo_unit_work AS SELECT * FROM user_repo_unit WHERE 0 = 1"
			workTableDrop = "DROP TABLE temp.user_repo_unit_work"
		}
	})

	if _, err := e.Exec(workTableCreate); err != nil {
		return 0, err
	}
	return atomic.AddInt64(&nextBatchID, 1), nil
}

// userRepoUnitsFinishBatch dumps the batch data into user_repo_unit and
// removes the temporary work table used for the batch.
func userRepoUnitsFinishBatch(e Engine) (err error) {
	// Combine all records into the best set of permissions
	// for each user and insert them into user_repo_unit.
	if _, err = e.Exec(fmt.Sprintf("INSERT INTO user_repo_unit (user_id, repo_id, `type`, `mode`) "+
		"SELECT user_id, repo_id, `type`, MAX(`mode`) "+
		"FROM %s WHERE `mode` > %d "+
		"GROUP BY user_id, repo_id, `type`",
		workTable, AccessModeNone)); err != nil {
		return fmt.Errorf("batchConsolidateWorkData (INSERT): %v", err)
	}

	// Even if the working table is dropped automatically when the connection
	// is freed, it's quite possible that it's needed again within this particular
	// session (e.g. from RebuildAllUserRepoUnits()). We need to drop the table
	// to cover that case.
	_, err = e.Exec(workTableDrop)
	return err
}

// batchInsertWork will add records to the temporary work table
// according to the given query statement.
func batchInsertWork(e Engine, stmt string, args ...interface{}) (err error) {
	eargs := []interface{}{fmt.Sprintf("INSERT INTO %s (user_id, repo_id, `type`, `mode`) ", workTable) + stmt}
	eargs = append(eargs, args...)
	_, err = e.Exec(eargs...)
	return err
}

// userRepoUnitRemoveWorking will remove any actual user_repo_unit records that have
// a matching record in the temporary work table, so they can be replaced with the new values.
func userRepoUnitRemoveWorking(e Engine) (err error) {
	// An IN clause would be better, but it's not supported by SQLite3
	// when a multicolumn key is required.
	// NOTE: we're leaving out any match for `type` because the current
	// logic doesn't require that (so the statement runs faster).
	// TODO: Replace with an UPSERT statement.
	_, err = e.Exec(fmt.Sprintf("DELETE FROM user_repo_unit WHERE EXISTS "+
		"(SELECT 1 FROM %s WHERE "+
		"%s.user_id = user_repo_unit.user_id AND "+
		"%s.repo_id = user_repo_unit.repo_id AND "+
		"%s.batch_id = ?)", workTable, workTable, workTable, workTable))
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit (existing work entries): %v", err)
	}
	return nil
}
