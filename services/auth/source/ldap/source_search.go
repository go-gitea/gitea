// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"

	"github.com/go-ldap/ldap/v3"
)

// SearchResult : user data
type SearchResult struct {
	Username     string   // Username
	Name         string   // Name
	Surname      string   // Surname
	Mail         string   // E-mail address
	SSHPublicKey []string // SSH Public Key
	IsAdmin      bool     // if user is administrator
	IsRestricted bool     // if user is restricted
	LowerName    string   // LowerName
	Avatar       []byte
	Groups       container.Set[string]
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
		return ldap.DialTLS("tcp", net.JoinHostPort(source.Host, strconv.Itoa(source.Port)), tlsConfig) //nolint:staticcheck
	}

	conn, err := ldap.Dial("tcp", net.JoinHostPort(source.Host, strconv.Itoa(source.Port))) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("error during Dial: %w", err)
	}

	if source.SecurityProtocol == SecurityProtocolStartTLS {
		if err = conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("error during StartTLS: %w", err)
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
	if ls.AdminFilter == "" {
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
	if ls.RestrictedFilter == "" {
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
func (source *Source) listLdapGroupMemberships(l *ldap.Conn, uid string, applyGroupFilter bool) container.Set[string] {
	ldapGroups := make(container.Set[string])

	groupFilter, ok := source.sanitizedGroupFilter(source.GroupFilter)
	if !ok {
		return ldapGroups
	}

	groupDN, ok := source.sanitizedGroupDN(source.GroupDN)
	if !ok {
		return ldapGroups
	}

	var searchFilter string
	if applyGroupFilter && groupFilter != "" {
		searchFilter = fmt.Sprintf("(&(%s)(%s=%s))", groupFilter, source.GroupMemberUID, ldap.EscapeFilter(uid))
	} else {
		searchFilter = fmt.Sprintf("(%s=%s)", source.GroupMemberUID, ldap.EscapeFilter(uid))
	}
	result, err := l.Search(ldap.NewSearchRequest(
		groupDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		searchFilter,
		[]string{},
		nil,
	))
	if err != nil {
		log.Error("Failed group search in LDAP with filter [%s]: %v", searchFilter, err)
		return ldapGroups
	}

	for _, entry := range result.Entries {
		if entry.DN == "" {
			log.Error("LDAP search was successful, but found no DN!")
			continue
		}
		ldapGroups.Add(entry.DN)
	}

	return ldapGroups
}

func (source *Source) getUserAttributeListedInGroup(entry *ldap.Entry) string {
	if strings.ToLower(source.UserUID) == "dn" {
		return entry.DN
	}

	return entry.GetAttributeValue(source.UserUID)
}

// SearchEntry : search an LDAP source if an entry (name, passwd) is valid and in the specific filter
func (source *Source) SearchEntry(name, passwd string, directBind bool) *SearchResult {
	if MockedSearchEntry != nil {
		return MockedSearchEntry(source, name, passwd, directBind)
	}
	return realSearchEntry(source, name, passwd, directBind)
}

var MockedSearchEntry func(source *Source, name, passwd string, directBind bool) *SearchResult

func realSearchEntry(source *Source, name, passwd string, directBind bool) *SearchResult {
	// See https://tools.ietf.org/search/rfc4513#section-5.1.2
	if passwd == "" {
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

	isAttributeSSHPublicKeySet := strings.TrimSpace(source.AttributeSSHPublicKey) != ""
	isAttributeAvatarSet := strings.TrimSpace(source.AttributeAvatar) != ""

	attribs := []string{source.AttributeUsername, source.AttributeName, source.AttributeSurname, source.AttributeMail}
	if strings.TrimSpace(source.UserUID) != "" {
		attribs = append(attribs, source.UserUID)
	}
	if isAttributeSSHPublicKeySet {
		attribs = append(attribs, source.AttributeSSHPublicKey)
	}
	if isAttributeAvatarSet {
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

	if isAttributeSSHPublicKeySet {
		sshPublicKey = sr.Entries[0].GetAttributeValues(source.AttributeSSHPublicKey)
	}

	isAdmin := checkAdmin(l, source, userDN)

	var isRestricted bool
	if !isAdmin {
		isRestricted = checkRestricted(l, source, userDN)
	}

	if isAttributeAvatarSet {
		Avatar = sr.Entries[0].GetRawAttributeValue(source.AttributeAvatar)
	}

	// Check group membership
	var usersLdapGroups container.Set[string]
	if source.GroupsEnabled {
		userAttributeListedInGroup := source.getUserAttributeListedInGroup(sr.Entries[0])
		usersLdapGroups = source.listLdapGroupMemberships(l, userAttributeListedInGroup, true)

		if source.GroupFilter != "" && len(usersLdapGroups) == 0 {
			return nil
		}
	}

	if !directBind && source.AttributesInBind {
		// binds user (checking password) after looking-up attributes in BindDN context
		err = bindUser(l, userDN, passwd)
		if err != nil {
			return nil
		}
	}

	return &SearchResult{
		LowerName:    strings.ToLower(username),
		Username:     username,
		Name:         firstname,
		Surname:      surname,
		Mail:         mail,
		SSHPublicKey: sshPublicKey,
		IsAdmin:      isAdmin,
		IsRestricted: isRestricted,
		Avatar:       Avatar,
		Groups:       usersLdapGroups,
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

	isAttributeSSHPublicKeySet := strings.TrimSpace(source.AttributeSSHPublicKey) != ""
	isAttributeAvatarSet := strings.TrimSpace(source.AttributeAvatar) != ""

	attribs := []string{source.AttributeUsername, source.AttributeName, source.AttributeSurname, source.AttributeMail, source.UserUID}
	if isAttributeSSHPublicKeySet {
		attribs = append(attribs, source.AttributeSSHPublicKey)
	}
	if isAttributeAvatarSet {
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

	result := make([]*SearchResult, 0, len(sr.Entries))

	for _, v := range sr.Entries {
		var usersLdapGroups container.Set[string]
		if source.GroupsEnabled {
			userAttributeListedInGroup := source.getUserAttributeListedInGroup(v)

			if source.GroupFilter != "" {
				usersLdapGroups = source.listLdapGroupMemberships(l, userAttributeListedInGroup, true)
				if len(usersLdapGroups) == 0 {
					continue
				}
			}

			if source.GroupTeamMap != "" || source.GroupTeamMapRemoval {
				usersLdapGroups = source.listLdapGroupMemberships(l, userAttributeListedInGroup, false)
			}
		}

		user := &SearchResult{
			Username: v.GetAttributeValue(source.AttributeUsername),
			Name:     v.GetAttributeValue(source.AttributeName),
			Surname:  v.GetAttributeValue(source.AttributeSurname),
			Mail:     v.GetAttributeValue(source.AttributeMail),
			IsAdmin:  checkAdmin(l, source, v.DN),
			Groups:   usersLdapGroups,
		}

		if !user.IsAdmin {
			user.IsRestricted = checkRestricted(l, source, v.DN)
		}

		if isAttributeSSHPublicKeySet {
			user.SSHPublicKey = v.GetAttributeValues(source.AttributeSSHPublicKey)
		}

		if isAttributeAvatarSet {
			user.Avatar = v.GetRawAttributeValue(source.AttributeAvatar)
		}

		user.LowerName = strings.ToLower(user.Username)

		result = append(result, user)
	}

	return result, nil
}
