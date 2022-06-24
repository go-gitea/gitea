// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ldap

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-ldap/ldap/v3"
)

// SearchResult : user data
type SearchResult struct {
	Username       string   // Username
	Name           string   // Name
	Surname        string   // Surname
	Mail           string   // E-mail address
	SSHPublicKey   []string // SSH Public Key
	IsAdmin        bool     // if user is administrator
	IsRestricted   bool     // if user is restricted
	LowerName      string   // LowerName
	Avatar         []byte
	LdapTeamAdd    map[string][]string // organizations teams to add
	LdapTeamRemove map[string][]string // organizations teams to remove
}

func (source *Source) sanitizedUserQuery(username string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4515
	badCharacters := "\x00()*\\"
	if strings.ContainsAny(username, badCharacters) {
		log.Debug("'%s' contains invalid query characters. Aborting.", username)
		return "", false
	}

	return fmt.Sprintf(source.Filter, username), true
}

func (source *Source) sanitizedUserDN(username string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4514: "special characters"
	badCharacters := "\x00()*\\,='\"#+;<>"
	if strings.ContainsAny(username, badCharacters) {
		log.Debug("'%s' contains invalid DN characters. Aborting.", username)
		return "", false
	}

	return fmt.Sprintf(source.UserDN, username), true
}

func (source *Source) sanitizedGroupFilter(group string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4515
	badCharacters := "\x00*\\"
	if strings.ContainsAny(group, badCharacters) {
		log.Trace("Group filter invalid query characters: %s", group)
		return "", false
	}

	return group, true
}

func (source *Source) sanitizedGroupDN(groupDn string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4514: "special characters"
	badCharacters := "\x00()*\\'\"#+;<>"
	if strings.ContainsAny(groupDn, badCharacters) || strings.HasPrefix(groupDn, " ") || strings.HasSuffix(groupDn, " ") {
		log.Trace("Group DN contains invalid query characters: %s", groupDn)
		return "", false
	}

	return groupDn, true
}

func (source *Source) findUserDN(l *ldap.Conn, name string) (string, bool) {
	log.Trace("Search for LDAP user: %s", name)

	// A search for the user.
	userFilter, ok := source.sanitizedUserQuery(name)
	if !ok {
		return "", false
	}

	log.Trace("Searching for DN using filter %s and base %s", userFilter, source.UserBase)
	search := ldap.NewSearchRequest(
		source.UserBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0,
		false, userFilter, []string{}, nil)

	// Ensure we found a user
	sr, err := l.Search(search)
	if err != nil || len(sr.Entries) < 1 {
		log.Debug("Failed search using filter[%s]: %v", userFilter, err)
		return "", false
	} else if len(sr.Entries) > 1 {
		log.Debug("Filter '%s' returned more than one user.", userFilter)
		return "", false
	}

	userDN := sr.Entries[0].DN
	if userDN == "" {
		log.Error("LDAP search was successful, but found no DN!")
		return "", false
	}

	return userDN, true
}

func dial(source *Source) (*ldap.Conn, error) {
	log.Trace("Dialing LDAP with security protocol (%v) without verifying: %v", source.SecurityProtocol, source.SkipVerify)

	tlsConfig := &tls.Config{
		ServerName:         source.Host,
		InsecureSkipVerify: source.SkipVerify,
	}

	if source.SecurityProtocol == SecurityProtocolLDAPS {
		return ldap.DialTLS("tcp", net.JoinHostPort(source.Host, strconv.Itoa(source.Port)), tlsConfig)
	}

	conn, err := ldap.Dial("tcp", net.JoinHostPort(source.Host, strconv.Itoa(source.Port)))
	if err != nil {
		return nil, fmt.Errorf("error during Dial: %v", err)
	}

	if source.SecurityProtocol == SecurityProtocolStartTLS {
		if err = conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("error during StartTLS: %v", err)
		}
	}

	return conn, nil
}

func bindUser(l *ldap.Conn, userDN, passwd string) error {
	log.Trace("Binding with userDN: %s", userDN)
	err := l.Bind(userDN, passwd)
	if err != nil {
		log.Debug("LDAP auth. failed for %s, reason: %v", userDN, err)
		return err
	}
	log.Trace("Bound successfully with userDN: %s", userDN)
	return err
}

func checkAdmin(l *ldap.Conn, ls *Source, userDN string) bool {
	if len(ls.AdminFilter) == 0 {
		return false
	}
	log.Trace("Checking admin with filter %s and base %s", ls.AdminFilter, userDN)
	search := ldap.NewSearchRequest(
		userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, ls.AdminFilter,
		[]string{ls.AttributeName},
		nil)

	sr, err := l.Search(search)

	if err != nil {
		log.Error("LDAP Admin Search with filter %s for %s failed unexpectedly! (%v)", ls.AdminFilter, userDN, err)
	} else if len(sr.Entries) < 1 {
		log.Trace("LDAP Admin Search found no matching entries.")
	} else {
		return true
	}
	return false
}

func checkRestricted(l *ldap.Conn, ls *Source, userDN string) bool {
	if len(ls.RestrictedFilter) == 0 {
		return false
	}
	if ls.RestrictedFilter == "*" {
		return true
	}
	log.Trace("Checking restricted with filter %s and base %s", ls.RestrictedFilter, userDN)
	search := ldap.NewSearchRequest(
		userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, ls.RestrictedFilter,
		[]string{ls.AttributeName},
		nil)

	sr, err := l.Search(search)

	if err != nil {
		log.Error("LDAP Restrictred Search with filter %s for %s failed unexpectedly! (%v)", ls.RestrictedFilter, userDN, err)
	} else if len(sr.Entries) < 1 {
		log.Trace("LDAP Restricted Search found no matching entries.")
	} else {
		return true
	}
	return false
}

// List all group memberships of a user
func (source *Source) listLdapGroupMemberships(l *ldap.Conn, uid string) []string {
	var ldapGroups []string
	groupFilter := fmt.Sprintf("(%s=%s)", source.GroupMemberUID, uid)
	result, err := l.Search(ldap.NewSearchRequest(
		source.GroupDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		groupFilter,
		[]string{},
		nil,
	))
	if err != nil {
		log.Error("Failed group search using filter[%s]: %v", groupFilter, err)
		return ldapGroups
	}

	for _, entry := range result.Entries {
		if entry.DN == "" {
			log.Error("LDAP search was successful, but found no DN!")
			continue
		}
		ldapGroups = append(ldapGroups, entry.DN)
	}

	return ldapGroups
}

// parse LDAP groups and return map of ldap groups to organizations teams
func (source *Source) mapLdapGroupsToTeams() map[string]map[string][]string {
	ldapGroupsToTeams := make(map[string]map[string][]string)
	err := json.Unmarshal([]byte(source.GroupTeamMap), &ldapGroupsToTeams)
	if err != nil {
		log.Error("Failed to unmarshall LDAP teams map: %v", err)
		return ldapGroupsToTeams
	}
	return ldapGroupsToTeams
}

// getMappedMemberships : returns the organizations and teams to modify the users membership
func (source *Source) getMappedMemberships(l *ldap.Conn, uid string) (map[string][]string, map[string][]string) {
	// get all LDAP group memberships for user
	usersLdapGroups := source.listLdapGroupMemberships(l, uid)
	// unmarshall LDAP group team map from configs
	ldapGroupsToTeams := source.mapLdapGroupsToTeams()
	membershipsToAdd := map[string][]string{}
	membershipsToRemove := map[string][]string{}
	for group, memberships := range ldapGroupsToTeams {
		isUserInGroup := util.IsStringInSlice(group, usersLdapGroups)
		if isUserInGroup {
			for org, teams := range memberships {
				membershipsToAdd[org] = teams
			}
		} else if !isUserInGroup {
			for org, teams := range memberships {
				membershipsToRemove[org] = teams
			}
		}
	}
	return membershipsToAdd, membershipsToRemove
}

// SearchEntry : search an LDAP source if an entry (name, passwd) is valid and in the specific filter
func (source *Source) SearchEntry(name, passwd string, directBind bool) *SearchResult {
	// See https://tools.ietf.org/search/rfc4513#section-5.1.2
	if len(passwd) == 0 {
		log.Debug("Auth. failed for %s, password cannot be empty", name)
		return nil
	}
	l, err := dial(source)
	if err != nil {
		log.Error("LDAP Connect error, %s:%v", source.Host, err)
		source.Enabled = false
		return nil
	}
	defer l.Close()

	var userDN string
	if directBind {
		log.Trace("LDAP will bind directly via UserDN template: %s", source.UserDN)

		var ok bool
		userDN, ok = source.sanitizedUserDN(name)

		if !ok {
			return nil
		}

		err = bindUser(l, userDN, passwd)
		if err != nil {
			return nil
		}

		if source.UserBase != "" {
			// not everyone has a CN compatible with input name so we need to find
			// the real userDN in that case

			userDN, ok = source.findUserDN(l, name)
			if !ok {
				return nil
			}
		}
	} else {
		log.Trace("LDAP will use BindDN.")

		var found bool

		if source.BindDN != "" && source.BindPassword != "" {
			err := l.Bind(source.BindDN, source.BindPassword)
			if err != nil {
				log.Debug("Failed to bind as BindDN[%s]: %v", source.BindDN, err)
				return nil
			}
			log.Trace("Bound as BindDN %s", source.BindDN)
		} else {
			log.Trace("Proceeding with anonymous LDAP search.")
		}

		userDN, found = source.findUserDN(l, name)
		if !found {
			return nil
		}
	}

	if !source.AttributesInBind {
		// binds user (checking password) before looking-up attributes in user context
		err = bindUser(l, userDN, passwd)
		if err != nil {
			return nil
		}
	}

	userFilter, ok := source.sanitizedUserQuery(name)
	if !ok {
		return nil
	}

	isAttributeSSHPublicKeySet := len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0
	isAtributeAvatarSet := len(strings.TrimSpace(source.AttributeAvatar)) > 0

	attribs := []string{source.AttributeUsername, source.AttributeName, source.AttributeSurname, source.AttributeMail}
	if len(strings.TrimSpace(source.UserUID)) > 0 {
		attribs = append(attribs, source.UserUID)
	}
	if isAttributeSSHPublicKeySet {
		attribs = append(attribs, source.AttributeSSHPublicKey)
	}
	if isAtributeAvatarSet {
		attribs = append(attribs, source.AttributeAvatar)
	}

	log.Trace("Fetching attributes '%v', '%v', '%v', '%v', '%v', '%v', '%v' with filter '%s' and base '%s'", source.AttributeUsername, source.AttributeName, source.AttributeSurname, source.AttributeMail, source.AttributeSSHPublicKey, source.AttributeAvatar, source.UserUID, userFilter, userDN)
	search := ldap.NewSearchRequest(
		userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, userFilter,
		attribs, nil)

	sr, err := l.Search(search)
	if err != nil {
		log.Error("LDAP Search failed unexpectedly! (%v)", err)
		return nil
	} else if len(sr.Entries) < 1 {
		if directBind {
			log.Trace("User filter inhibited user login.")
		} else {
			log.Trace("LDAP Search found no matching entries.")
		}

		return nil
	}

	var sshPublicKey []string
	var Avatar []byte

	username := sr.Entries[0].GetAttributeValue(source.AttributeUsername)
	firstname := sr.Entries[0].GetAttributeValue(source.AttributeName)
	surname := sr.Entries[0].GetAttributeValue(source.AttributeSurname)
	mail := sr.Entries[0].GetAttributeValue(source.AttributeMail)
	uid := sr.Entries[0].GetAttributeValue(source.UserUID)
	if source.UserUID == "dn" || source.UserUID == "DN" {
		uid = sr.Entries[0].DN
	}

	// Check group membership
	if source.GroupsEnabled && source.GroupFilter != "" {
		groupFilter, ok := source.sanitizedGroupFilter(source.GroupFilter)
		if !ok {
			return nil
		}
		groupDN, ok := source.sanitizedGroupDN(source.GroupDN)
		if !ok {
			return nil
		}

		log.Trace("Fetching groups '%v' with filter '%s' and base '%s'", source.GroupMemberUID, groupFilter, groupDN)
		groupSearch := ldap.NewSearchRequest(
			groupDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, groupFilter,
			[]string{source.GroupMemberUID},
			nil)

		srg, err := l.Search(groupSearch)
		if err != nil {
			log.Error("LDAP group search failed: %v", err)
			return nil
		} else if len(srg.Entries) < 1 {
			log.Error("LDAP group search failed: 0 entries")
			return nil
		}

		isMember := false
	Entries:
		for _, group := range srg.Entries {
			for _, member := range group.GetAttributeValues(source.GroupMemberUID) {
				if (source.UserUID == "dn" && member == sr.Entries[0].DN) || member == uid {
					isMember = true
					break Entries
				}
			}
		}

		if !isMember {
			log.Error("LDAP group membership test failed")
			return nil
		}
	}

	if isAttributeSSHPublicKeySet {
		sshPublicKey = sr.Entries[0].GetAttributeValues(source.AttributeSSHPublicKey)
	}
	isAdmin := checkAdmin(l, source, userDN)
	var isRestricted bool
	if !isAdmin {
		isRestricted = checkRestricted(l, source, userDN)
	}

	if isAtributeAvatarSet {
		Avatar = sr.Entries[0].GetRawAttributeValue(source.AttributeAvatar)
	}

	teamsToAdd := make(map[string][]string)
	teamsToRemove := make(map[string][]string)
	if source.GroupsEnabled && (source.GroupTeamMap != "" || source.GroupTeamMapRemoval) {
		teamsToAdd, teamsToRemove = source.getMappedMemberships(l, uid)
	}

	if !directBind && source.AttributesInBind {
		// binds user (checking password) after looking-up attributes in BindDN context
		err = bindUser(l, userDN, passwd)
		if err != nil {
			return nil
		}
	}

	return &SearchResult{
		LowerName:      strings.ToLower(username),
		Username:       username,
		Name:           firstname,
		Surname:        surname,
		Mail:           mail,
		SSHPublicKey:   sshPublicKey,
		IsAdmin:        isAdmin,
		IsRestricted:   isRestricted,
		Avatar:         Avatar,
		LdapTeamAdd:    teamsToAdd,
		LdapTeamRemove: teamsToRemove,
	}
}

// UsePagedSearch returns if need to use paged search
func (source *Source) UsePagedSearch() bool {
	return source.SearchPageSize > 0
}

// SearchEntries : search an LDAP source for all users matching userFilter
func (source *Source) SearchEntries() ([]*SearchResult, error) {
	l, err := dial(source)
	if err != nil {
		log.Error("LDAP Connect error, %s:%v", source.Host, err)
		source.Enabled = false
		return nil, err
	}
	defer l.Close()

	if source.BindDN != "" && source.BindPassword != "" {
		err := l.Bind(source.BindDN, source.BindPassword)
		if err != nil {
			log.Debug("Failed to bind as BindDN[%s]: %v", source.BindDN, err)
			return nil, err
		}
		log.Trace("Bound as BindDN %s", source.BindDN)
	} else {
		log.Trace("Proceeding with anonymous LDAP search.")
	}

	userFilter := fmt.Sprintf(source.Filter, "*")

	isAttributeSSHPublicKeySet := len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0
	isAtributeAvatarSet := len(strings.TrimSpace(source.AttributeAvatar)) > 0

	attribs := []string{source.AttributeUsername, source.AttributeName, source.AttributeSurname, source.AttributeMail, source.UserUID}
	if isAttributeSSHPublicKeySet {
		attribs = append(attribs, source.AttributeSSHPublicKey)
	}
	if isAtributeAvatarSet {
		attribs = append(attribs, source.AttributeAvatar)
	}

	log.Trace("Fetching attributes '%v', '%v', '%v', '%v', '%v', '%v' with filter %s and base %s", source.AttributeUsername, source.AttributeName, source.AttributeSurname, source.AttributeMail, source.AttributeSSHPublicKey, source.AttributeAvatar, userFilter, source.UserBase)
	search := ldap.NewSearchRequest(
		source.UserBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, userFilter,
		attribs, nil)

	var sr *ldap.SearchResult
	if source.UsePagedSearch() {
		sr, err = l.SearchWithPaging(search, source.SearchPageSize)
	} else {
		sr, err = l.Search(search)
	}
	if err != nil {
		log.Error("LDAP Search failed unexpectedly! (%v)", err)
		return nil, err
	}

	result := make([]*SearchResult, len(sr.Entries))

	for i, v := range sr.Entries {
		teamsToAdd := make(map[string][]string)
		teamsToRemove := make(map[string][]string)
		if source.GroupsEnabled && (source.GroupTeamMap != "" || source.GroupTeamMapRemoval) {
			userAttributeListedInGroup := v.GetAttributeValue(source.UserUID)
			if source.UserUID == "dn" || source.UserUID == "DN" {
				userAttributeListedInGroup = v.DN
			}
			teamsToAdd, teamsToRemove = source.getMappedMemberships(l, userAttributeListedInGroup)
		}
		result[i] = &SearchResult{
			Username:       v.GetAttributeValue(source.AttributeUsername),
			Name:           v.GetAttributeValue(source.AttributeName),
			Surname:        v.GetAttributeValue(source.AttributeSurname),
			Mail:           v.GetAttributeValue(source.AttributeMail),
			IsAdmin:        checkAdmin(l, source, v.DN),
			LdapTeamAdd:    teamsToAdd,
			LdapTeamRemove: teamsToRemove,
		}
		if !result[i].IsAdmin {
			result[i].IsRestricted = checkRestricted(l, source, v.DN)
		}
		if isAttributeSSHPublicKeySet {
			result[i].SSHPublicKey = v.GetAttributeValues(source.AttributeSSHPublicKey)
		}
		if isAtributeAvatarSet {
			result[i].Avatar = v.GetRawAttributeValue(source.AttributeAvatar)
		}
		result[i].LowerName = strings.ToLower(result[i].Username)
	}

	return result, nil
}
