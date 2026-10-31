// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// TestConnectionResult contains the outcome of testing an LDAP source.
type TestConnectionResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Details []string `json:"details"`

	detailEntries []testConnectionDetail
}

type testConnectionDetail struct {
	key string
	raw string
}

// TestConnection checks the connection, bind, and configured search bases.
func (source *Source) TestConnection(isDLDAP bool) *TestConnectionResult {
	result := &TestConnectionResult{OK: true}
	if strings.TrimSpace(source.Host) == "" || source.Port <= 0 {
		return &TestConnectionResult{Message: "admin.auths.test_connection.host_port_required"}
	}
	if isDLDAP {
		if strings.TrimSpace(source.UserDN) == "" || !hasUserPlaceholder(source.UserDN) {
			result.OK = false
			result.addTranslatedDetail("admin.auths.test_connection.user_dn_required")
		}
	} else if strings.TrimSpace(source.UserBase) == "" {
		result.OK = false
		result.addTranslatedDetail("admin.auths.test_connection.user_base_required")
	}
	if !isDLDAP && !hasUserPlaceholder(source.Filter) {
		result.OK = false
		result.addTranslatedDetail("admin.auths.test_connection.user_filter_required")
	}
	if !result.OK {
		result.Message = "admin.auths.test_connection.invalid_config"
		return result
	}

	conn, err := dial(source)
	if err != nil {
		return failedTestResult("admin.auths.test_connection.connection_failed", err)
	}
	defer conn.Close()

	if source.BindDN != "" {
		if err := bindUser(conn, source.BindDN, source.BindPassword); err != nil {
			return failedTestResult("admin.auths.test_connection.bind_failed", err)
		}
		result.addTranslatedDetail("admin.auths.test_connection.bind_succeeded")
	} else {
		if isDLDAP {
			result.addTranslatedDetail("admin.auths.test_connection.dldap_user_dn_not_verified")
		} else {
			result.addTranslatedDetail("admin.auths.test_connection.no_bind_dn")
		}
	}

	if !isDLDAP {
		_, err := conn.Search(newTestSearchRequest(source.UserBase, fmt.Sprintf(source.Filter, "*")))
		if err != nil && !isSizeLimitExceeded(err) {
			return failedTestResult("admin.auths.test_connection.user_search_failed", err)
		}
		result.addTranslatedDetail("admin.auths.test_connection.user_search_succeeded")
	}

	if source.GroupsEnabled {
		if strings.TrimSpace(source.GroupDN) == "" || strings.TrimSpace(source.GroupFilter) == "" {
			return failedTestResult("admin.auths.test_connection.group_config_required", nil)
		}
		_, err := conn.Search(newTestSearchRequest(source.GroupDN, source.GroupFilter))
		if err != nil && !isSizeLimitExceeded(err) {
			return failedTestResult("admin.auths.test_connection.group_search_failed", err)
		}
		result.addTranslatedDetail("admin.auths.test_connection.group_search_succeeded")
	}
	if source.SkipVerify {
		result.addTranslatedDetail("admin.auths.test_connection.tls_verification_disabled")
	}
	result.Message = "admin.auths.test_connection.succeeded"
	return result
}

func newTestSearchRequest(base, filter string) *ldap.SearchRequest {
	return ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 10, false, filter, []string{}, nil)
}

func hasUserPlaceholder(value string) bool {
	return strings.Contains(value, "%s") || strings.Contains(value, "%[1]s")
}

func isSizeLimitExceeded(err error) bool {
	var ldapErr *ldap.Error
	return errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultSizeLimitExceeded
}

func failedTestResult(message string, err error) *TestConnectionResult {
	result := &TestConnectionResult{Message: message}
	if err != nil {
		result.detailEntries = []testConnectionDetail{{raw: err.Error()}}
	}
	return result
}

func (result *TestConnectionResult) addTranslatedDetail(key string) {
	result.detailEntries = append(result.detailEntries, testConnectionDetail{key: key})
}

// TranslateDetails resolves localized detail keys while preserving raw errors.
func (result *TestConnectionResult) TranslateDetails(translate func(string) string) {
	result.Details = make([]string, 0, len(result.detailEntries))
	for _, detail := range result.detailEntries {
		if detail.key != "" {
			result.Details = append(result.Details, translate(detail.key))
		} else {
			result.Details = append(result.Details, detail.raw)
		}
	}
}
