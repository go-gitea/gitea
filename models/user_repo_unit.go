// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
)

const (

	// UserRepoUnitLoggedInUser is a special user ID used in the UserRepoUnit table
	// for permissions that apply to all logged in users when no specific record
	// (userid + repoid) is present. It's intended for public repositories.
	UserRepoUnitLoggedInUser = int64(-1)

	// UserRepoUnitAnyUser is a special user ID used in the UserRepoUnit table
	// for permissions that apply to all users when no specific record
	// (userid + repoid) is present. It's intended for public repositories.
	UserRepoUnitAnyUser = int64(-2)
)

// UserRepoUnit is an explosion (cartesian product) of all user permissions
// on all repositories, with one record for each combination of user+repo+unittype,
// except unspecific permissions on public repos. General permissions for public
// repos shared among all users (e.g. UnitTypeCode:AccessModeRead) are set for
// UserID == UserRepoUnitLoggedInUser and UserID == UserRepoUnitAnyUser
// in order to reduce the number of records on the table. Special permissions on public
// repos (e.g. writers, owners) are exploded to specific users accordingly.
// This means that to check whether a given user has any permissions on a
// particular repository, both UserID == user's and UserID == UserRepoUnitLoggedInUser
// must be checked (the highest permission must prevail). When a UserID is available,
// checking UserID == UserRepoUnitAnyUser on top of that is redundant.
// Anonymous user permissions (i.e. for users not logged in) must be checked
// with UserID == UserRepoUnitAnyUser.

// Except for the special users UserRepoUnitLoggedInUser and UserRepoUnitAnyUser,
// only real users have records in the table (i.e. organizations don't have their
// own records).

// Users that are not active yet or have prohibited login do not have any records.

// Restricted users are users that require explicit permissions to access a repository;
// these users are solved by checking records that match their UserID but _ignoring_
// records for UserRepoUnitLoggedInUser and UserRepoUnitAnyUser.

// If all permissions result in AccessModeNone, the whole record is omitted,
// so checking against the UserRepoUnit table will result in a quick check for
// whether the user can see the repository at all (e.g. at the explore page).

// Input considered for calculating the records includes:
//
// * public/private status of the repository
// * public/limited/private status of the repository's owner (user or org)
// * user team membership (if repository owner is an org)
// * team settings (whether the team has an explicit repository list or they can access all of them)
// * user collaboration status
// * user ownership of the repository
// * user site admin status
// * which repository units are enabled (e.g. issues, PRs, etc.)

// UserRepoUnit is an explosion (cartesian product) of all user permissions
// on all repositories, and all types of relevant unit types.
type UserRepoUnit struct {
	UserID int64      `xorm:"pk"`
	RepoID int64      `xorm:"pk INDEX"`
	Type   UnitType   `xorm:"pk"`
	Mode   AccessMode `xorm:"NOT NULL"`
}

// UserRepoUnitWork is a table used for temporarily accumulate all the work performed
// while processing a batch. Ideally, this would be a temporary (no storage) table.
// Records are grouped by BatchID in order to prevent any kind of collision.
// Lack of primary key is intentional; this table is not intended for replication
// as it should never contain rows after the transaction is completed.
type UserRepoUnitWork struct {
	BatchID int64      `xorm:"NOT NULL INDEX"`
	UserID  int64      `xorm:"NOT NULL"`
	RepoID  int64      `xorm:"NOT NULL"`
	Type    UnitType   `xorm:"NOT NULL"`
	Mode    AccessMode `xorm:"NOT NULL"`
}

// UserRepoUnitBatchNumber provides in a safe way unique ID values
// for the batch number in case we are in a multi-server environment.
// It's a 63-bit number, so good luck reaching the maximum value
// (300 million years at 1000 requests per second, if you want to know).
// It's a makeshift replacement for an actual database sequence.
type UserRepoUnitBatchNumber struct {
	ID int64 `xorm:"pk autoincr"`
}

var (
	// This list may be updated as needed. Requisites are:
	// - New units need to abide by the same rules as the others
	// - Units from all repos must be rebuilt after this change
	supportedUnits = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeWiki,
	}

	// Pre-built SQL condition to filter-out unsupported unit types
	// "repo_unit.`type` IN (UnitTypeCode, UnitTypeIssues, etc.)"
	userRepoUnitSupported string
)

func init() {
	// Build some data that we will use quite often
	uts := make([]string, len(supportedUnits))
	for i, su := range supportedUnits {
		uts[i] = fmt.Sprintf("%d", su)
	}
	userRepoUnitSupported = "repo_unit.`type` IN (" + strings.Join(uts, ",") + ")"
}

// RebuildRepoUnits will rebuild all permissions to a given repository for all users
func RebuildRepoUnits(e Engine, repo *Repository) error {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildUserUnits: rebuilding permissions for repository %d on batch %d", repo.ID, batchID)

	// Make sure we start from scratch; we intend to recreate all pairs
	_, err = e.Delete(&UserRepoUnit{RepoID: repo.ID})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit (batch:%d): %v", batchID, err)
	}

	if err = buildRepoUnits(e, batchID, repo, -1); err != nil {
		return fmt.Errorf("buildRepoUnits(batch:%d): %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = batchConsolidateWorkData(e, batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData(batch:%d): %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch(batch:%d): %v", batchID, err)
	}

	log.Trace("RebuildRepoUnits: permissions for repository %d on batch %d rebuilt.", repo.ID, batchID)

	return nil
}

// RebuildUserUnits will rebuild all permissions for a given (real) user
func RebuildUserUnits(e Engine, user *User) error {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildUserUnits: rebuilding permissions for user %d on batch %d", user.ID, batchID)

	// Make sure we start from scratch; we intend to recreate all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: user.ID})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit (batch:%d): %v", batchID, err)
	}

	if err = buildUserUnits(e, batchID, user); err != nil {
		return fmt.Errorf("buildUserUnits(batch:%d): %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = batchConsolidateWorkData(e, batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData(batch:%d): %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch(batch:%d): %v", batchID, err)
	}

	log.Trace("RebuildUserUnits: permissions for user %d on batch %d rebuilt.", user.ID, batchID)

	return nil
}

// RebuildLoggedInUnits will rebuild all permissions for generic logged in users
func RebuildLoggedInUnits(e Engine) error {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildLoggedInUnits: rebuilding permissions for logged in users on batch %d", batchID)

	// Make sure we start from scratch; we intend to recreate all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: UserRepoUnitLoggedInUser})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit (batch:%d): %v", batchID, err)
	}

	if err = buildLoggedInUnits(e, batchID); err != nil {
		return fmt.Errorf("buildLoggedInUnits(batch:%d): %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = batchConsolidateWorkData(e, batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData(batch:%d): %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch(batch:%d): %v", batchID, err)
	}

	log.Trace("RebuildLoggedInUnits: permissions for logged in users on batch %d rebuilt.", batchID)

	return nil
}

// RebuildAnonymousUnits will rebuild all permissions for unidentified users
func RebuildAnonymousUnits(e Engine) error {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildAnonymousUnits: rebuilding permissions for unidentified users on batch %d", batchID)

	// Make sure we start from scratch; we intend to recreate all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: UserRepoUnitAnyUser})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit (batch:%d): %v", batchID, err)
	}

	if err = buildAnonymousUnits(e, batchID); err != nil {
		return fmt.Errorf("buildAnonymousUnits(batch:%d): %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = batchConsolidateWorkData(e, batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData(batch:%d): %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch(batch:%d): %v", batchID, err)
	}

	log.Trace("RebuildAnonymousUnits: permissions for logged unidentified on batch %d rebuilt.", batchID)

	return nil
}

// RebuildUserRepoUnits will rebuild permissions for a given (real) user on a given repository
func RebuildUserRepoUnits(e Engine, user *User, repo *Repository) error {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RebuildUserUnits: rebuilding permissions for user %d, repo %d on batch %d", user.ID, repo.ID, batchID)

	// Make sure we start from scratch; we intend to recreate all pairs
	_, err = e.Delete(&UserRepoUnit{UserID: user.ID, RepoID: repo.ID})
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit (batch:%d): %v", batchID, err)
	}

	if err = buildUserRepoUnits(e, batchID, user, repo); err != nil {
		return fmt.Errorf("buildUserRepoUnits(batch:%d): %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for the user into the user_repo_unit table
	// ***********************************************************************************

	if err = batchConsolidateWorkData(e, batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData(batch:%d): %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch(batch:%d): %v", batchID, err)
	}

	log.Trace("RebuildUserUnits: permissions for user %d, repo %d on batch %d rebuilt.", user.ID, repo.ID, batchID)

	return nil
}

// RemoveTeamUnits will rebuild all the user/repo permission pairs known by a team
// while removing the permissions provided by the team itself.
func RemoveTeamUnits(e Engine, team *Team) error {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("RemoveTeamUnits: removing permissions for team %d on batch %d", team.ID, batchID)

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
			if err := x.Asc("id").Limit(batchSize, start).
				Where("repository.owner_id = ?", team.OrgID).
				Find(&repos); err != nil {
				return err
			}
			if len(repos) == 0 {
				break
			}
			for _, repo := range repos {
				// Make sure we start from scratch; we intend to recreate all pairs
				log.Trace("RemoveTeamUnits: rebuilding permissions for repository %d on batch %d", repo.ID, batchID)
				_, err = e.Delete(&UserRepoUnit{RepoID: repo.ID})
				if err != nil {
					return fmt.Errorf("DELETE user_repo_unit (batch:%d): %v", batchID, err)
				}
				// buildRepoUnits will exclude team.ID from the generated records
				if err = buildRepoUnits(e, batchID, repo, team.ID); err != nil {
					return fmt.Errorf("buildRepoUnits(batch:%d): %v", batchID, err)
				}
				log.Trace("RemoveTeamUnits: permissions for repository %d on batch %d rebuilt", repo.ID, batchID)
			}
		}

	} else {

		if err = team.getRepositories(e); err != nil {
			return fmt.Errorf("team.getRepositories(batch:%d): %v", batchID, err)
		}

		for _, repo := range team.Repos {
			// Make sure we start from scratch; we intend to recreate all pairs
			log.Trace("RemoveTeamUnits: rebuilding permissions for repository %d on batch %d", repo.ID, batchID)
			_, err = e.Delete(&UserRepoUnit{RepoID: repo.ID})
			if err != nil {
				return fmt.Errorf("DELETE user_repo_unit (batch:%d): %v", batchID, err)
			}
			// buildRepoUnits will exclude team.ID from the generated records
			if err = buildRepoUnits(e, batchID, repo, team.ID); err != nil {
				return fmt.Errorf("buildRepoUnits(batch:%d): %v", batchID, err)
			}
			log.Trace("RemoveTeamUnits: permissions for repository %d on batch %d rebuilt", repo.ID, batchID)
		}
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = batchConsolidateWorkData(e, batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData(batch:%d): %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch(batch:%d): %v", batchID, err)
	}

	log.Trace("RemoveTeamUnits: permissions for team %d on batch %d removed.", team.ID, batchID)

	return nil
}

// AddTeamUnits can only extend or upgrade units for a given team;
// it cannot be used to replace, delete or downgrade units.
func AddTeamUnits(e Engine, team *Team) error {

	batchID, err := userRepoUnitStartBatch(e)
	if err != nil {
		return fmt.Errorf("userRepoUnitStartBatch: %v", err)
	}

	log.Trace("AddTeamUnits: adding permissions for team %d on batch %d", team.ID, batchID)

	// Adding permissions is easier than removing; we just need to honor the previous
	// set of permissions.

	// *******************************************************************************************
	// Insert pre-existing permissions the team users may have (e.g. as admins or collaborators)
	// *******************************************************************************************

	_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, type, mode) "+
		"SELECT ?, user_id, repo_id, type, mode "+
		"FROM user_repo_unit "+
		"INNER JOIN team_user ON team_user.uid = user_repo_unit.user_id "+
		"WHERE team_user.team_id = ?",
		batchID, team.ID)
	if err != nil {
		return fmt.Errorf("AddTeamUnits (INSERT): %v", err)
	}

	// ***********************************************************************************
	// Add permissions that come from the team itself
	// ***********************************************************************************

	if err = buildTeamUnits(e, batchID, team); err != nil {
		return fmt.Errorf("buildTeamUnits(batch:%d): %v", batchID, err)
	}

	// ***********************************************************************************
	// Remove pre-existing permissions that we are replacing
	// ***********************************************************************************

	// Since we will insert records from user_repo_unit_work into user_repo_unit,
	// we must remove any records from user_repo_unit first. Otherwise we will
	// get collisions.
	if err = userRepoUnitRemoveWorking(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitRemoveWorking(batch:%d): %v", batchID, err)
	}

	// ***********************************************************************************
	// Consolidate the best permissions for each user into the user_repo_unit table
	// ***********************************************************************************

	if err = batchConsolidateWorkData(e, batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData(batch:%d): %v", batchID, err)
	}

	if err = userRepoUnitsFinishBatch(e, batchID); err != nil {
		return fmt.Errorf("userRepoUnitsFinishBatch(batch:%d): %v", batchID, err)
	}

	log.Trace("AddTeamUnits: permissions for team %d on batch %d added.", team.ID, batchID)

	return nil
}

// buildRepoUnits will build batch data for all users on a given repository, excluding one team
func buildRepoUnits(e Engine, batchID int64, repo *Repository, excludeTeamID int64) error {

	// user_repo_unit_work is expected to contain no records related to this repository
	// for the current batch (i.e. this function will only _add_ permissions).
	// excludeTeamID may indicate a team ID to exclude from calculations; this is used
	// when a team is deleted or downgraded (e.g. given less permissions).
	// If no team is to be excluded, excludeTeamID must be -1.

	// ****************************************************************************
	// Insert permissions for site admins
	// ****************************************************************************

	// An INNER JOIN with a condition that doesn't specify a relationship between tables
	// will create a cartesian product of them (i.e. all combinations).
	// "Find all valid admin users and create records for each relevant
	//  unit type the repository has configured. Assign AccessModeAdmin access.
	_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, `user`.id, repo_unit.repo_id, repo_unit.`type`, ? "+
		"FROM `user` "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = ? "+
		"WHERE `user`.is_admin = ? AND `user`.is_active = ? AND `user`.prohibit_login = ? "+
		"AND `user`.type = ? "+
		"AND "+userRepoUnitSupported,
		batchID, AccessModeAdmin, repo.ID, true, true, false, UserTypeIndividual)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (repo admins): %v", err)
	}

	if err = repo.getOwner(e); err != nil {
		if IsErrUserNotExist(err) {
			// Allow abnormalities like a missing owner to avoid breaking batches
			log.Warn("buildRepoUnits: missing owner %d for repository %d", repo.OwnerID, repo.ID)
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
		_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, `user`.id, repo_unit.repo_id, repo_unit.`type`, team.`authorize` "+
			"FROM team_repo "+
			"INNER JOIN team ON team.id = team_repo.team_id "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
			"INNER JOIN team_user ON team_user.team_id = team.id "+
			"INNER JOIN `user` ON `user`.id = team_user.uid "+
			"WHERE team_repo.repo_id = ? AND team.includes_all_repositories = ? "+
			"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
			"AND "+userRepoUnitSupported+" "+
			"AND team.id <> ? "+ // excludeTeamID will be -1 if no team is to be excluded
			"AND team.org_id = ?", // Sanity check, just in case
			batchID, repo.ID, false, true, false, UserTypeIndividual, excludeTeamID, repo.OwnerID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (repo teams, !include_all): %v", err)
		}

		// This query will cover all teams with <includes_all_repositories = true>
		// "Find all users belonging to teams of the same organization as the repository owner;
		//  cross with all relevant units enabled for this repo. Assign the access configured
		//  in the team."
		_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, `user`.id, repo_unit.repo_id, team_unit.`type`, team.`authorize` "+
			"FROM team "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN team_user ON team_user.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = ? AND repo_unit.`type` = team_unit.`type` "+
			"INNER JOIN `user` ON `user`.id = team_user.uid "+
			"WHERE team.org_id = ? AND team.includes_all_repositories = ? "+
			"AND "+userRepoUnitSupported+" "+
			"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
			"AND team.id <> ?", // excludeTeamID will be -1 if no team is to be excluded
			batchID, repo.ID, repo.OwnerID, true, true, false, UserTypeIndividual, excludeTeamID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (repo teams, include_all): %v", err)
		}

	} else if repo.Owner.IsActive && !repo.Owner.ProhibitLogin {

		// ****************************************************************************
		// Insert permissions for the owner (if not inhibited)
		// ****************************************************************************

		// Owners require AccessModeOwner even if they are site admins, as this permission
		// is higher than AccessModeAdmin.

		// "Find all relevant units for this repository. Assign AccessModeOwner access to repo.OwnerID."
		_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repo_unit "+
			"WHERE repo_unit.repo_id = ? AND "+userRepoUnitSupported,
			batchID, repo.OwnerID, AccessModeOwner, repo.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (repo owner): %v", err)
		}
	}

	// ****************************************************************************
	// Insert permissions for collaborators
	// ****************************************************************************

	// "Find all users collaborating on this repository; cross with all relevant units
	//  enabled for this repo. Assign access specified by the collaboration."
	_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, `user`.id, repo_unit.repo_id, repo_unit.`type`, collaboration.`mode` "+
		"FROM collaboration "+
		"INNER JOIN `user` ON `user`.id = collaboration.user_id "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = collaboration.repo_id "+
		"WHERE collaboration.repo_id = ? "+
		"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
		"AND "+userRepoUnitSupported,
		batchID, repo.ID, true, false, UserTypeIndividual)
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

				// All members of the organization get at least read access to the repository
				// "Find all valid users belonging to the same organization as the repo;
				//  cross with all relevant units enabled for this repo. Assign AccessModeRead
				//  to each user."
				_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
					"SELECT ?, `user`.id, repo_unit.repo_id, repo_unit.`type`, ? "+
					"FROM `user` "+
					"INNER JOIN repo_unit ON repo_unit.repo_id = ? "+
					"WHERE `user`.id IN ("+
					"  SELECT team_user.uid "+
					"  FROM team_user "+
					"  INNER JOIN team ON team.org_id = ? "+
					"  WHERE team.id <> ? "+ // excludeTeamID will be -1 if no team is to be excluded
					") "+
					"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
					"AND "+userRepoUnitSupported,
					batchID, AccessModeRead, repo.ID, repo.OwnerID, excludeTeamID, true, false, UserTypeIndividual)
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
				// which will prevent the creation of permissions for UserRepoUnitLoggedInUser and
				// UserRepoUnitAnyUser.
			} */

		} else {

			// *************************************************************************************
			// "Public" repository for a visible or limited user or organization (logged in users)
			// *************************************************************************************

			// There's one record representing "logged in users" (UserRepoUnitLoggedInUser);
			// this simplifies the queries for permission verification later.

			// This query covers organizations with Visibility == structs.VisibleTypeLimited
			// "Find all relevant units for this repository"
			_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
				"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
				"FROM repo_unit "+
				"WHERE repo_unit.repo_id = ? AND "+userRepoUnitSupported,
				batchID, UserRepoUnitLoggedInUser, AccessModeRead, repo.ID)
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
				_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
					"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
					"FROM repo_unit "+
					"WHERE repo_unit.repo_id = ? AND "+userRepoUnitSupported,
					batchID, UserRepoUnitAnyUser, AccessModeRead, repo.ID)
				if err != nil {
					return fmt.Errorf("INSERT INTO user_repo_unit_work (repo public, anonymous): %v", err)
				}
			}
		}
	}

	return nil
}

// buildUserUnits will build batch data for a given user on any explicitly
// allowed repositories (+admin status)
func buildUserUnits(e Engine, batchID int64, user *User) error {

	// user_repo_unit_work is expected to contain no records related to this user
	// for the current batch (i.e. this function will only _add_ permissions).

	if !user.IsActive || user.ProhibitLogin {
		// No permissions for inactive users
		// FIXME: should this check apply to admins as well?
		// Maybe changing the admin password from the command line should reset these flags.
		return nil
	}

	if user.IsOrganization() {
		// Organizations have no permissions themselves; only their members do
		return nil
	}

	if user.IsAdmin {
		// ****************************************************************************
		// Site admins have permissions on all repositories
		// ****************************************************************************

		// An INNER JOIN with a condition that doesn't specify a relationship between tables
		// will create a cartesian product of them (i.e. all combinations).
		// "Find and create records for each relevant unit type that any repository has configured.
		//  Assign AccessModeAdmin access to the user.
		_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repo_unit "+
			"WHERE "+userRepoUnitSupported,
			batchID, user.ID, AccessModeAdmin)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user: admin): %v", err)
		}
	}

	// Even if the user might be a site admin, AccessModeOwner is of higher importance
	// than AccessModeAdmin, so we process the rest of the options regardless of the admin
	// status of the user. We will select the best permissions for the user
	// in the consolidation step.

	// ****************************************************************************
	// Normal user, owned repositories
	// ****************************************************************************

	// "Find and create records for each relevant unit type that any repository owned
	//  by the user has configured. Assign AccessModeOwner access to the user.
	_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, repository.owner_id, repo_unit.repo_id, repo_unit.`type`, ? "+
		"FROM repository "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id "+
		"WHERE repository.owner_id = ? AND "+userRepoUnitSupported,
		batchID, AccessModeOwner, user.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user: owner): %v", err)
	}

	// ****************************************************************************
	// Normal user, collaborations on repositories
	// ****************************************************************************

	// "Find all repositories where the user is collaborating; cross with all relevant units
	//  enabled for them. Assign access specified by the collaboration to the user."
	_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, collaboration.user_id, collaboration.repo_id, repo_unit.`type`, collaboration.`mode` "+
		"FROM collaboration "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = collaboration.repo_id "+
		"WHERE collaboration.user_id = ? "+
		"AND "+userRepoUnitSupported,
		batchID, user.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user collaborator): %v", err)
	}

	// ****************************************************************************
	// Normal user, teams they belong to
	// ****************************************************************************

	// This query will cover all teams with <includes_all_repositories = false> the user belongs to.
	// "Find all repos assigned to teams this user belongs to. Assign access specified by the
	//  team to the user."
	_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, team_user.uid, team_repo.repo_id, team_unit.`type`, team.authorize "+
		"FROM team_user "+
		"INNER JOIN team ON team.id = team_user.team_id "+
		"INNER JOIN team_repo ON team_repo.team_id = team.id "+
		"INNER JOIN team_unit ON team_unit.team_id = team.id "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
		"WHERE team_user.uid = ? AND team.includes_all_repositories = ? "+
		"AND "+userRepoUnitSupported,
		batchID, user.ID, false)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user teams, !include_all): %v", err)
	}

	// This query will cover all teams with <includes_all_repositories = true> the user belongs to.
	// "Find all repositories belonging to organizations with teams this user belongs to.
	//  Assign access specified by the team to the user."
	_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, team_user.uid, repository.id, repo_unit.`type`, team.authorize "+
		"FROM team_user "+
		"INNER JOIN team ON team.id = team_user.team_id "+
		"INNER JOIN repository ON repository.owner_id = team.org_id "+
		"INNER JOIN team_unit ON team_unit.team_id = team.id "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id AND repo_unit.`type` = team_unit.`type` "+
		"WHERE team_user.uid = ? AND team.includes_all_repositories = ? "+
		"AND "+userRepoUnitSupported,
		batchID, user.ID, true)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user teams, include_all): %v", err)
	}

	return nil
}

// buildLoggedInUnits will build batch data for the generic logged in user
// on all non-private repositories
func buildLoggedInUnits(e Engine, batchID int64) error {

	// ****************************************************************************
	// Process repositories not marked as 'private'
	// ****************************************************************************

	// Public repositories (IsPrivate == false) give read access to any logged in users
	// as long as their owner is public or limited (regular users are always public).

	// "Find all relevant units for this repository"
	_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
		"FROM repository "+
		"INNER JOIN `user` ON `user`.id = repository.owner_id "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id "+
		"WHERE repository.is_private = ? "+
		"AND `user`.visibility IN (?,?) AND "+userRepoUnitSupported,
		batchID, UserRepoUnitLoggedInUser, AccessModeRead,
		false, structs.VisibleTypePublic, structs.VisibleTypeLimited)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (logged in): %v", err)
	}

	return nil
}

// buildAnonymousUnits will build batch data for users that have not identified
// themselves on all public repositories
func buildAnonymousUnits(e Engine, batchID int64) error {

	// ****************************************************************************
	// Process repositories marked as 'public'
	// ****************************************************************************

	// Public repositories (IsPrivate == false) give read access to all users
	// as long as their owner (regular or organization) is public.

	// "Find all relevant units for this repository"
	_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
		"FROM repository "+
		"INNER JOIN `user` ON `user`.id = repository.owner_id "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id "+
		"WHERE repository.is_private = ? "+
		"AND `user`.visibility = ? AND "+userRepoUnitSupported,
		batchID, UserRepoUnitAnyUser, AccessModeRead,
		false, structs.VisibleTypePublic)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (logged in): %v", err)
	}

	return nil
}

// buildUserRepoUnits will build batch data for a given user on a specific repository
// (e.g. if added/removed as a collaborator, from a team, etc.)
func buildUserRepoUnits(e Engine, batchID int64, user *User, repo *Repository) error {

	if !user.IsActive || user.ProhibitLogin {
		// No permissions for inactive users
		// FIXME: should this check apply to admins as well?
		// Maybe changing the admin password from the command line should reset these flags.
		return nil
	}

	if user.IsOrganization() {
		// Organizations have no permissions themselves; only their members do
		return nil
	}

	if user.ID == repo.OwnerID {

		// ****************************************************************************
		// Regular user, owner of the repository
		// ****************************************************************************

		_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repo_unit "+
			"WHERE repo_unit.repo_id = ? AND "+userRepoUnitSupported,
			batchID, user.ID, AccessModeOwner, repo.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user/repo: owner): %v", err)
		}

		// Owner permission on a 1:1 relationship is the best the user can get
		return nil
	}

	if user.IsAdmin {
		// ****************************************************************************
		// Site admin user
		// ****************************************************************************

		_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, ?, repo_unit.repo_id, repo_unit.`type`, ? "+
			"FROM repo_unit "+
			"WHERE repo_unit.repo_id = ? AND "+userRepoUnitSupported,
			batchID, user.ID, AccessModeAdmin, repo.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user/repo: admin): %v", err)
		}
	}

	// User can get owner access from teams, for instance

	// ****************************************************************************
	// Regular user, access granted by being a collaborator to the repository
	// ****************************************************************************

	_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
		"SELECT ?, collaboration.user_id, collaboration.repo_id, repo_unit.`type`, collaboration.`mode` "+
		"FROM collaboration "+
		"INNER JOIN repo_unit ON repo_unit.repo_id = collaboration.repo_id "+
		"WHERE collaboration.user_id = ? AND collaboration.repo_id = ? "+
		"AND "+userRepoUnitSupported,
		batchID, user.ID, repo.ID)
	if err != nil {
		return fmt.Errorf("INSERT user_repo_unit_work (user/repo collaborator): %v", err)
	}

	// ****************************************************************************
	// Regular user, access granted by belonging to teams that own the repository
	// ****************************************************************************

	if err = repo.getOwner(e); err != nil {
		return fmt.Errorf("getOwner: %v", err)
	}

	if repo.Owner.IsOrganization() {

		// This query will cover all teams with <includes_all_repositories = false> the user belongs to.
		// "Find teams from the repository owner this user belongs to that have access to this repository.
		//  Assign access specified by the team to the user."
		_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, team_user.uid, team_repo.repo_id, team_unit.`type`, team.authorize "+
			"FROM team "+
			"INNER JOIN team_user ON team_user.team_id = team.id "+
			"INNER JOIN team_repo ON team_repo.team_id = team.id "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
			"WHERE team.org_id = ? AND team.includes_all_repositories = ? "+
			"AND team_user.uid = ? AND team_repo.repo_id = ? AND "+userRepoUnitSupported,
			batchID, repo.OwnerID, false, user.ID, repo.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user/repo teams, !include_all): %v", err)
		}

		// This query will cover all teams with <includes_all_repositories = true> the user belongs to.
		// "Find teams from this repository owner organization this user belongs to, having access to
		//  all repositories in the organization. Assign access specified by the team to the user."
		_, err = e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, team_user.uid, repo_unit.repo_id, repo_unit.`type`, team.authorize "+
			"FROM team "+
			"INNER JOIN team_user ON team_user.team_id = team.id "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.`type` = team_unit.`type` "+
			"WHERE repo_unit.repo_id = ? AND team.org_id = ? AND team.includes_all_repositories = ? "+
			"AND team_user.uid = ? AND "+userRepoUnitSupported,
			batchID, repo.ID, repo.OwnerID, true, user.ID)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (user/repo teams, include_all): %v", err)
		}
	}

	return nil
}

// buildTeamUnits will build batch data for a given team on associated repositories
func buildTeamUnits(e Engine, batchID int64, team *Team) error {

	// This function will _add_ permissions to users of a given team as granted
	// by the team settings. Unlike buildRepoUnits() or buildUserUnits(), this function
	// expects and deals with prior existing permissions, to which it will add its own.
	// However, it assumes that any existing permissions come from other sources
	// rather than the specified team. This means that it _will not_ remove or downgrade
	// permissions from the team users.

	if team.IncludesAllRepositories {

		// ********************************************************************************************
		// Insert permissions for all repositories that belong to the organization the team belongs to
		// ********************************************************************************************

		// "Find all users belonging to the team, and all the repositories whos owner is the team
		//  organization, and the relevant units granted to the team;
		//  Assign access specified by the team units to the users."
		_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, team_user.uid, repo_unit.repo_id, team_unit.`type`, team.authorize "+
			"FROM team "+ // Not strictly required, but simplifies the code here
			"INNER JOIN team_user ON team_user.team_id = team.id "+
			"INNER JOIN repository ON repository.owner_id = team.org_id "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = repository.id AND repo_unit.`type` = team_unit.`type` "+
			"INNER JOIN `user` ON `user`.id = team_user.uid "+
			"WHERE team.id = ? "+
			"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
			"AND "+userRepoUnitSupported,
			batchID, team.ID, true, false, UserTypeIndividual)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (team, org repos): %v", err)
		}

	} else {

		// ****************************************************************************
		// Insert permissions for specifically enabled repositories
		// ****************************************************************************

		// "Find all users belonging and repositories associated with the team,
		//  and the relevant units granted to the team. Assign access specified by the
		//  team units to the users."
		_, err := e.Exec("INSERT INTO user_repo_unit_work (batch_id, user_id, repo_id, `type`, `mode`) "+
			"SELECT ?, team_user.uid, team_repo.repo_id, team_unit.`type`, team.authorize "+
			"FROM team "+
			"INNER JOIN team_user ON team_user.team_id = team.id "+
			"INNER JOIN `user` ON `user`.id = team_user.uid "+
			"INNER JOIN team_repo ON team_repo.team_id = team.id "+
			"INNER JOIN team_unit ON team_unit.team_id = team.id "+
			"INNER JOIN repo_unit ON repo_unit.repo_id = team_repo.repo_id AND repo_unit.`type` = team_unit.`type` "+
			"WHERE team.id = ? "+
			"AND `user`.is_active = ? AND `user`.prohibit_login = ? AND `user`.type = ? "+
			"AND "+userRepoUnitSupported,
			batchID, team.ID, true, false, UserTypeIndividual)
		if err != nil {
			return fmt.Errorf("INSERT user_repo_unit_work (team, team repos): %v", err)
		}
	}

	return nil
}

// userRepoUnitStartBatch will return a unique ID for the batch transaction
func userRepoUnitStartBatch(e Engine) (int64, error) {
	var batchnum UserRepoUnitBatchNumber
	// e.Insert() will return a new ID for the batch that is unique even among
	// concurrent transactions.
	if _, err := e.Insert(&batchnum); err != nil {
		return 0, err
	}
	if batchnum.ID == 0 {
		return 0, fmt.Errorf("userRepoUnitStartBatch: unable to obtain a proper batch ID")
	}
	return batchnum.ID, nil
}

// userRepoUnitStartBatch will remove temporary data used for a batch update
func userRepoUnitsFinishBatch(e Engine, batchID int64) error {
	_, err := e.Delete(&UserRepoUnitWork{BatchID: batchID})
	if err != nil {
		return err
	}
	_, err = e.Delete(&UserRepoUnitBatchNumber{ID: batchID})
	return err
}

// userRepoUnitStartBatch dumps a user_repo_unit_work batch into user_repo_unit
func batchConsolidateWorkData(e Engine, batchID int64) error {
	// This function will combine all records into the best set of permissions
	// for each user and insert them into user_repo_unit.
	if _, err := e.Exec("INSERT INTO user_repo_unit (user_id, repo_id, type, mode) "+
		"SELECT user_id, repo_id, type, MAX(mode) "+
		"FROM user_repo_unit_work WHERE batch_id = ? "+
		"GROUP BY user_id, repo_id, type", batchID); err != nil {
		return fmt.Errorf("batchConsolidateWorkData (INSERT): %v", err)
	}
	return nil
}

// userRepoUnitRemoveWorking will remove any user_repo_unit records that are present
// in user_repo_unit_work in order to replace them. A MERGE/UPSERT statement would be better,
// but its syntax is not consistent among all databases.
func userRepoUnitRemoveWorking(e Engine, batchID int64) error {
	// An IN clause would be better, but it's not supported by SQLite3
	// when a multicolumn key is required.
	// NOTE: we're leaving out any match for `type` because the current
	// logic doesn't require that (so the statement runs faster).
	_, err := e.Exec("DELETE FROM user_repo_unit WHERE EXISTS "+
		"(SELECT 1 FROM user_repo_unit_work WHERE "+
		"user_repo_unit_work.user_id = user_repo_unit.user_id AND "+
		"user_repo_unit_work.repo_id = user_repo_unit.repo_id AND "+
		"user_repo_unit_work.batch_id = ?)", batchID)
	if err != nil {
		return fmt.Errorf("DELETE user_repo_unit (existing work entries): %v", err)
	}
	return nil
}
