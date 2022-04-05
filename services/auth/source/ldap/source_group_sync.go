// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ldap

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

// SyncLdapGroupsToTeams maps LDAP groups to organization and team memberships
func (source *Source) SyncLdapGroupsToTeams(user *user_model.User, ldapTeamAdd, ldapTeamRemove map[string][]string, orgCache map[string]*organization.Organization, teamCache map[string]*organization.Team) {
	var err error
	if source.GroupsEnabled && source.GroupTeamMapRemoval {
		// when the user is not a member of configs LDAP group, remove mapped organizations/teams memberships
		removeMappedMemberships(user, ldapTeamRemove, orgCache, teamCache)
	}
	for orgName, teamNames := range ldapTeamAdd {
		org, ok := orgCache[orgName]
		if !ok {
			org, err = organization.GetOrgByName(orgName)
			if err != nil {
				// organization must be created before LDAP group sync
				log.Warn("LDAP group sync: Could not find organisation %s: %v", orgName, err)
				continue
			}
			orgCache[orgName] = org
		}

		for _, teamName := range teamNames {
			team, ok := teamCache[orgName+teamName]
			if !ok {
				team, err = org.GetTeam(teamName)
				if err != nil {
					// team must be created before LDAP group sync
					log.Warn("LDAP group sync: Could not find team %s: %v", teamName, err)
					continue
				}
				teamCache[orgName+teamName] = team
			}
			if isMember, err := organization.IsTeamMember(db.DefaultContext, org.ID, team.ID, user.ID); !isMember && err == nil {
				log.Trace("LDAP group sync: adding user [%s] to team [%s]", user.Name, org.Name)
			} else {
				continue
			}
			err := models.AddTeamMember(team, user.ID)
			if err != nil {
				log.Error("LDAP group sync: Could not add user to team: %v", err)
			}
		}
	}
}

// remove membership to organizations/teams if user is not member of corresponding LDAP group
// e.g. lets assume user is member of LDAP group "x", but LDAP group team map contains LDAP groups "x" and "y"
// then users membership gets removed for all organizations/teams mapped by LDAP group "y"
func removeMappedMemberships(user *user_model.User, ldapTeamRemove map[string][]string, orgCache map[string]*organization.Organization, teamCache map[string]*organization.Team) {
	var err error
	for orgName, teamNames := range ldapTeamRemove {
		org, ok := orgCache[orgName]
		if !ok {
			org, err = organization.GetOrgByName(orgName)
			if err != nil {
				// organization must be created before LDAP group sync
				log.Warn("LDAP group sync: Could not find organisation %s: %v", orgName, err)
				continue
			}
			orgCache[orgName] = org
		}
		for _, teamName := range teamNames {
			team, ok := teamCache[orgName+teamName]
			if !ok {
				team, err = org.GetTeam(teamName)
				if err != nil {
					// team must must be created before LDAP group sync
					log.Warn("LDAP group sync: Could not find team %s: %v", teamName, err)
					continue
				}
			}
			if isMember, err := organization.IsTeamMember(db.DefaultContext, org.ID, team.ID, user.ID); isMember && err == nil {
				log.Trace("LDAP group sync: removing user [%s] from team [%s]", user.Name, org.Name)
			} else {
				continue
			}
			err = models.RemoveTeamMember(team, user.ID)
			if err != nil {
				log.Error("LDAP group sync: Could not remove user from team: %v", err)
			}
		}
	}
}
