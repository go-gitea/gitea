// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/require"
)

func TestIsSizeLimitExceeded(t *testing.T) {
	require.True(t, isSizeLimitExceeded(&ldap.Error{ResultCode: ldap.LDAPResultSizeLimitExceeded}))
	require.False(t, isSizeLimitExceeded(&ldap.Error{ResultCode: ldap.LDAPResultOperationsError}))
}

func TestHasUserPlaceholder(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   bool
	}{
		{name: "standard", filter: "(uid=%s)", want: true},
		{name: "indexed", filter: "(uid=%[1]s)", want: true},
		{name: "wildcard", filter: "(uid=*)", want: false},
		{name: "wrong type", filter: "(uid=%d)", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, hasUserPlaceholder(test.filter))
		})
	}
}

func TestTestConnectionValidation(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		isDLDAP bool
		message string
	}{
		{
			name:    "missing host and port",
			source:  Source{},
			message: "admin.auths.test_connection.host_port_required",
		},
		{
			name:    "LDAP requires user search settings",
			source:  Source{Host: "ldap.example.test", Port: 389},
			message: "admin.auths.test_connection.invalid_config",
		},
		{
			name:    "DLDAP requires user DN",
			source:  Source{Host: "ldap.example.test", Port: 389},
			isDLDAP: true,
			message: "admin.auths.test_connection.invalid_config",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.source.TestConnection(test.isDLDAP)
			require.False(t, result.OK)
			require.Equal(t, test.message, result.Message)
		})
	}
}
