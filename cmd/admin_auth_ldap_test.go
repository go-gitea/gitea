// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/services/auth/source/ldap"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func TestAddLdapBindDn(t *testing.T) {
	// Mock cli functions to do not exit on error
	osExiter := cli.OsExiter
	defer func() { cli.OsExiter = osExiter }()
	cli.OsExiter = func(code int) {}

	// Test cases
	cases := []struct {
		args   []string
		source *auth.Source
		errMsg string
	}{
		// case 0
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source full",
				"--not-active",
				"--security-protocol", "ldaps",
				"--skip-tls-verify",
				"--host", "ldap-bind-server full",
				"--port", "9876",
				"--user-search-base", "ou=Users,dc=full-domain-bind,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=full-domain-bind,dc=org)",
				"--admin-filter", "(memberOf=cn=admin-group,ou=example,dc=full-domain-bind,dc=org)",
				"--restricted-filter", "(memberOf=cn=restricted-group,ou=example,dc=full-domain-bind,dc=org)",
				"--username-attribute", "uid-bind full",
				"--firstname-attribute", "givenName-bind full",
				"--surname-attribute", "sn-bind full",
				"--email-attribute", "mail-bind full",
				"--public-ssh-key-attribute", "publickey-bind full",
				"--avatar-attribute", "avatar-bind full",
				"--bind-dn", "cn=readonly,dc=full-domain-bind,dc=org",
				"--bind-password", "secret-bind-full",
				"--attributes-in-bind",
				"--synchronize-users",
				"--page-size", "99",
				"--enable-groups",
				"--group-search-base-dn", "ou=group,dc=full-domain-bind,dc=org",
				"--group-member-attribute", "memberUid",
				"--group-user-attribute", "uid",
				"--group-filter", "(|(cn=gitea_users)(cn=admins))",
				"--group-team-map", `{"cn=my-group,cn=groups,dc=example,dc=org": {"MyGiteaOrganization": ["MyGiteaTeam1", "MyGiteaTeam2"]}}`,
				"--group-team-map-removal",
			},
			source: &auth.Source{
				Type:          auth.LDAP,
				Name:          "ldap (via Bind DN) source full",
				IsActive:      false,
				IsSyncEnabled: true,
				Cfg: &ldap.Source{
					Name:                  "ldap (via Bind DN) source full",
					Host:                  "ldap-bind-server full",
					Port:                  9876,
					SecurityProtocol:      ldap.SecurityProtocol(1),
					SkipVerify:            true,
					BindDN:                "cn=readonly,dc=full-domain-bind,dc=org",
					BindPassword:          "secret-bind-full",
					UserBase:              "ou=Users,dc=full-domain-bind,dc=org",
					AttributeUsername:     "uid-bind full",
					AttributeName:         "givenName-bind full",
					AttributeSurname:      "sn-bind full",
					AttributeMail:         "mail-bind full",
					AttributesInBind:      true,
					AttributeSSHPublicKey: "publickey-bind full",
					AttributeAvatar:       "avatar-bind full",
					SearchPageSize:        99,
					Filter:                "(memberOf=cn=user-group,ou=example,dc=full-domain-bind,dc=org)",
					AdminFilter:           "(memberOf=cn=admin-group,ou=example,dc=full-domain-bind,dc=org)",
					RestrictedFilter:      "(memberOf=cn=restricted-group,ou=example,dc=full-domain-bind,dc=org)",
					Enabled:               true,
					GroupsEnabled:         true,
					GroupDN:               "ou=group,dc=full-domain-bind,dc=org",
					GroupMemberUID:        "memberUid",
					UserUID:               "uid",
					GroupFilter:           "(|(cn=gitea_users)(cn=admins))",
					GroupTeamMap:          `{"cn=my-group,cn=groups,dc=example,dc=org": {"MyGiteaOrganization": ["MyGiteaTeam1", "MyGiteaTeam2"]}}`,
					GroupTeamMapRemoval:   true,
				},
			},
		},
		// case 1
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source min",
				"--security-protocol", "unencrypted",
				"--host", "ldap-bind-server min",
				"--port", "1234",
				"--user-search-base", "ou=Users,dc=min-domain-bind,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=min-domain-bind,dc=org)",
				"--email-attribute", "mail-bind min",
			},
			source: &auth.Source{
				Type:     auth.LDAP,
				Name:     "ldap (via Bind DN) source min",
				IsActive: true,
				Cfg: &ldap.Source{
					Name:             "ldap (via Bind DN) source min",
					Host:             "ldap-bind-server min",
					Port:             1234,
					SecurityProtocol: ldap.SecurityProtocol(0),
					UserBase:         "ou=Users,dc=min-domain-bind,dc=org",
					AttributeMail:    "mail-bind min",
					Filter:           "(memberOf=cn=user-group,ou=example,dc=min-domain-bind,dc=org)",
					Enabled:          true,
				},
			},
		},
		// case 2
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source",
				"--security-protocol", "zzzzz",
				"--host", "ldap-server",
				"--port", "1234",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
				"--email-attribute", "mail",
			},
			errMsg: "Unknown security protocol name: zzzzz",
		},
		// case 3
		{
			args: []string{
				"ldap-test",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--port", "1234",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
				"--email-attribute", "mail",
			},
			errMsg: "name is not set",
		},
		// case 4
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source",
				"--host", "ldap-server",
				"--port", "1234",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
				"--email-attribute", "mail",
			},
			errMsg: "security-protocol is not set",
		},
		// case 5
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source",
				"--security-protocol", "unencrypted",
				"--port", "1234",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
				"--email-attribute", "mail",
			},
			errMsg: "host is not set",
		},
		// case 6
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
				"--email-attribute", "mail",
			},
			errMsg: "port is not set",
		},
		// case 7
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--port", "1234",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
				"--email-attribute", "mail",
			},
			errMsg: "user-filter is not set",
		},
		// case 8
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (via Bind DN) source",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--port", "1234",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
			},
			errMsg: "email-attribute is not set",
		},
	}

	for n, c := range cases {
		// Mock functions.
		var createdAuthSource *auth.Source
		service := &authService{
			initDB: func(context.Context) error {
				return nil
			},
			createAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				createdAuthSource = authSource
				return nil
			},
			updateAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				assert.FailNow(t, "case %d: should not call updateAuthSource", n)
				return nil
			},
			getAuthSourceByID: func(ctx context.Context, id int64) (*auth.Source, error) {
				assert.FailNow(t, "case %d: should not call getAuthSourceByID", n)
				return nil, nil
			},
		}

		// Create a copy of command to test
		app := cli.NewApp()
		app.Flags = microcmdAuthAddLdapBindDn.Flags
		app.Action = service.addLdapBindDn

		// Run it
		err := app.Run(c.args)
		if c.errMsg != "" {
			assert.EqualError(t, err, c.errMsg, "case %d: error should match", n)
		} else {
			assert.NoError(t, err, "case %d: should have no errors", n)
			assert.Equal(t, c.source, createdAuthSource, "case %d: wrong authSource", n)
		}
	}
}

func TestAddLdapSimpleAuth(t *testing.T) {
	// Mock cli functions to do not exit on error
	osExiter := cli.OsExiter
	defer func() { cli.OsExiter = osExiter }()
	cli.OsExiter = func(code int) {}

	// Test cases
	cases := []struct {
		args       []string
		authSource *auth.Source
		errMsg     string
	}{
		// case 0
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source full",
				"--not-active",
				"--security-protocol", "starttls",
				"--skip-tls-verify",
				"--host", "ldap-simple-server full",
				"--port", "987",
				"--user-search-base", "ou=Users,dc=full-domain-simple,dc=org",
				"--user-filter", "(&(objectClass=posixAccount)(full-simple-cn=%s))",
				"--admin-filter", "(memberOf=cn=admin-group,ou=example,dc=full-domain-simple,dc=org)",
				"--restricted-filter", "(memberOf=cn=restricted-group,ou=example,dc=full-domain-simple,dc=org)",
				"--username-attribute", "uid-simple full",
				"--firstname-attribute", "givenName-simple full",
				"--surname-attribute", "sn-simple full",
				"--email-attribute", "mail-simple full",
				"--public-ssh-key-attribute", "publickey-simple full",
				"--avatar-attribute", "avatar-simple full",
				"--user-dn", "cn=%s,ou=Users,dc=full-domain-simple,dc=org",
			},
			authSource: &auth.Source{
				Type:     auth.DLDAP,
				Name:     "ldap (simple auth) source full",
				IsActive: false,
				Cfg: &ldap.Source{
					Name:                  "ldap (simple auth) source full",
					Host:                  "ldap-simple-server full",
					Port:                  987,
					SecurityProtocol:      ldap.SecurityProtocol(2),
					SkipVerify:            true,
					UserDN:                "cn=%s,ou=Users,dc=full-domain-simple,dc=org",
					UserBase:              "ou=Users,dc=full-domain-simple,dc=org",
					AttributeUsername:     "uid-simple full",
					AttributeName:         "givenName-simple full",
					AttributeSurname:      "sn-simple full",
					AttributeMail:         "mail-simple full",
					AttributeSSHPublicKey: "publickey-simple full",
					AttributeAvatar:       "avatar-simple full",
					Filter:                "(&(objectClass=posixAccount)(full-simple-cn=%s))",
					AdminFilter:           "(memberOf=cn=admin-group,ou=example,dc=full-domain-simple,dc=org)",
					RestrictedFilter:      "(memberOf=cn=restricted-group,ou=example,dc=full-domain-simple,dc=org)",
					Enabled:               true,
				},
			},
		},
		// case 1
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source min",
				"--security-protocol", "unencrypted",
				"--host", "ldap-simple-server min",
				"--port", "123",
				"--user-filter", "(&(objectClass=posixAccount)(min-simple-cn=%s))",
				"--email-attribute", "mail-simple min",
				"--user-dn", "cn=%s,ou=Users,dc=min-domain-simple,dc=org",
			},
			authSource: &auth.Source{
				Type:     auth.DLDAP,
				Name:     "ldap (simple auth) source min",
				IsActive: true,
				Cfg: &ldap.Source{
					Name:             "ldap (simple auth) source min",
					Host:             "ldap-simple-server min",
					Port:             123,
					SecurityProtocol: ldap.SecurityProtocol(0),
					UserDN:           "cn=%s,ou=Users,dc=min-domain-simple,dc=org",
					AttributeMail:    "mail-simple min",
					Filter:           "(&(objectClass=posixAccount)(min-simple-cn=%s))",
					Enabled:          true,
				},
			},
		},
		// case 2
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source",
				"--security-protocol", "zzzzz",
				"--host", "ldap-server",
				"--port", "123",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
				"--email-attribute", "mail",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			errMsg: "Unknown security protocol name: zzzzz",
		},
		// case 3
		{
			args: []string{
				"ldap-test",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--port", "123",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
				"--email-attribute", "mail",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			errMsg: "name is not set",
		},
		// case 4
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source",
				"--host", "ldap-server",
				"--port", "123",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
				"--email-attribute", "mail",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			errMsg: "security-protocol is not set",
		},
		// case 5
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source",
				"--security-protocol", "unencrypted",
				"--port", "123",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
				"--email-attribute", "mail",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			errMsg: "host is not set",
		},
		// case 6
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
				"--email-attribute", "mail",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			errMsg: "port is not set",
		},
		// case 7
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--port", "123",
				"--email-attribute", "mail",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			errMsg: "user-filter is not set",
		},
		// case 8
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--port", "123",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			errMsg: "email-attribute is not set",
		},
		// case 9
		{
			args: []string{
				"ldap-test",
				"--name", "ldap (simple auth) source",
				"--security-protocol", "unencrypted",
				"--host", "ldap-server",
				"--port", "123",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
				"--email-attribute", "mail",
			},
			errMsg: "user-dn is not set",
		},
	}

	for n, c := range cases {
		// Mock functions.
		var createdAuthSource *auth.Source
		service := &authService{
			initDB: func(context.Context) error {
				return nil
			},
			createAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				createdAuthSource = authSource
				return nil
			},
			updateAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				assert.FailNow(t, "case %d: should not call updateAuthSource", n)
				return nil
			},
			getAuthSourceByID: func(ctx context.Context, id int64) (*auth.Source, error) {
				assert.FailNow(t, "case %d: should not call getAuthSourceByID", n)
				return nil, nil
			},
		}

		// Create a copy of command to test
		app := cli.NewApp()
		app.Flags = microcmdAuthAddLdapSimpleAuth.Flags
		app.Action = service.addLdapSimpleAuth

		// Run it
		err := app.Run(c.args)
		if c.errMsg != "" {
			assert.EqualError(t, err, c.errMsg, "case %d: error should match", n)
		} else {
			assert.NoError(t, err, "case %d: should have no errors", n)
			assert.Equal(t, c.authSource, createdAuthSource, "case %d: wrong authSource", n)
		}
	}
}

func TestUpdateLdapBindDn(t *testing.T) {
	// Mock cli functions to do not exit on error
	osExiter := cli.OsExiter
	defer func() { cli.OsExiter = osExiter }()
	cli.OsExiter = func(code int) {}

	// Test cases
	cases := []struct {
		args               []string
		id                 int64
		existingAuthSource *auth.Source
		authSource         *auth.Source
		errMsg             string
	}{
		// case 0
		{
			args: []string{
				"ldap-test",
				"--id", "23",
				"--name", "ldap (via Bind DN) source full",
				"--not-active",
				"--security-protocol", "LDAPS",
				"--skip-tls-verify",
				"--host", "ldap-bind-server full",
				"--port", "9876",
				"--user-search-base", "ou=Users,dc=full-domain-bind,dc=org",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=full-domain-bind,dc=org)",
				"--admin-filter", "(memberOf=cn=admin-group,ou=example,dc=full-domain-bind,dc=org)",
				"--restricted-filter", "(memberOf=cn=restricted-group,ou=example,dc=full-domain-bind,dc=org)",
				"--username-attribute", "uid-bind full",
				"--firstname-attribute", "givenName-bind full",
				"--surname-attribute", "sn-bind full",
				"--email-attribute", "mail-bind full",
				"--public-ssh-key-attribute", "publickey-bind full",
				"--avatar-attribute", "avatar-bind full",
				"--bind-dn", "cn=readonly,dc=full-domain-bind,dc=org",
				"--bind-password", "secret-bind-full",
				"--synchronize-users",
				"--page-size", "99",
				"--enable-groups",
				"--group-search-base-dn", "ou=group,dc=full-domain-bind,dc=org",
				"--group-member-attribute", "memberUid",
				"--group-user-attribute", "uid",
				"--group-filter", "(|(cn=gitea_users)(cn=admins))",
				"--group-team-map", `{"cn=my-group,cn=groups,dc=example,dc=org": {"MyGiteaOrganization": ["MyGiteaTeam1", "MyGiteaTeam2"]}}`,
				"--group-team-map-removal",
			},
			id: 23,
			existingAuthSource: &auth.Source{
				Type:     auth.LDAP,
				IsActive: true,
				Cfg: &ldap.Source{
					Enabled: true,
				},
			},
			authSource: &auth.Source{
				Type:          auth.LDAP,
				Name:          "ldap (via Bind DN) source full",
				IsActive:      false,
				IsSyncEnabled: true,
				Cfg: &ldap.Source{
					Name:                  "ldap (via Bind DN) source full",
					Host:                  "ldap-bind-server full",
					Port:                  9876,
					SecurityProtocol:      ldap.SecurityProtocol(1),
					SkipVerify:            true,
					BindDN:                "cn=readonly,dc=full-domain-bind,dc=org",
					BindPassword:          "secret-bind-full",
					UserBase:              "ou=Users,dc=full-domain-bind,dc=org",
					AttributeUsername:     "uid-bind full",
					AttributeName:         "givenName-bind full",
					AttributeSurname:      "sn-bind full",
					AttributeMail:         "mail-bind full",
					AttributesInBind:      false,
					AttributeSSHPublicKey: "publickey-bind full",
					AttributeAvatar:       "avatar-bind full",
					SearchPageSize:        99,
					Filter:                "(memberOf=cn=user-group,ou=example,dc=full-domain-bind,dc=org)",
					AdminFilter:           "(memberOf=cn=admin-group,ou=example,dc=full-domain-bind,dc=org)",
					RestrictedFilter:      "(memberOf=cn=restricted-group,ou=example,dc=full-domain-bind,dc=org)",
					Enabled:               true,
					GroupsEnabled:         true,
					GroupDN:               "ou=group,dc=full-domain-bind,dc=org",
					GroupMemberUID:        "memberUid",
					UserUID:               "uid",
					GroupFilter:           "(|(cn=gitea_users)(cn=admins))",
					GroupTeamMap:          `{"cn=my-group,cn=groups,dc=example,dc=org": {"MyGiteaOrganization": ["MyGiteaTeam1", "MyGiteaTeam2"]}}`,
					GroupTeamMapRemoval:   true,
				},
			},
		},
		// case 1
		{
			args: []string{
				"ldap-test",
				"--id", "1",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg:  &ldap.Source{},
			},
		},
		// case 2
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--name", "ldap (via Bind DN) source",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Name: "ldap (via Bind DN) source",
				Cfg: &ldap.Source{
					Name: "ldap (via Bind DN) source",
				},
			},
		},
		// case 3
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--not-active",
			},
			existingAuthSource: &auth.Source{
				Type:     auth.LDAP,
				IsActive: true,
				Cfg:      &ldap.Source{},
			},
			authSource: &auth.Source{
				Type:     auth.LDAP,
				IsActive: false,
				Cfg:      &ldap.Source{},
			},
		},
		// case 4
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--security-protocol", "LDAPS",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					SecurityProtocol: ldap.SecurityProtocol(1),
				},
			},
		},
		// case 5
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--skip-tls-verify",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					SkipVerify: true,
				},
			},
		},
		// case 6
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--host", "ldap-server",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					Host: "ldap-server",
				},
			},
		},
		// case 7
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--port", "389",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					Port: 389,
				},
			},
		},
		// case 8
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					UserBase: "ou=Users,dc=domain,dc=org",
				},
			},
		},
		// case 9
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--user-filter", "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					Filter: "(memberOf=cn=user-group,ou=example,dc=domain,dc=org)",
				},
			},
		},
		// case 10
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--admin-filter", "(memberOf=cn=admin-group,ou=example,dc=domain,dc=org)",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					AdminFilter: "(memberOf=cn=admin-group,ou=example,dc=domain,dc=org)",
				},
			},
		},
		// case 11
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--username-attribute", "uid",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					AttributeUsername: "uid",
				},
			},
		},
		// case 12
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--firstname-attribute", "givenName",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					AttributeName: "givenName",
				},
			},
		},
		// case 13
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--surname-attribute", "sn",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					AttributeSurname: "sn",
				},
			},
		},
		// case 14
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--email-attribute", "mail",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					AttributeMail: "mail",
				},
			},
		},
		// case 15
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--attributes-in-bind",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					AttributesInBind: true,
				},
			},
		},
		// case 16
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--public-ssh-key-attribute", "publickey",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					AttributeSSHPublicKey: "publickey",
				},
			},
		},
		// case 17
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--bind-dn", "cn=readonly,dc=domain,dc=org",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					BindDN: "cn=readonly,dc=domain,dc=org",
				},
			},
		},
		// case 18
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--bind-password", "secret",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					BindPassword: "secret",
				},
			},
		},
		// case 19
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--synchronize-users",
			},
			authSource: &auth.Source{
				Type:          auth.LDAP,
				IsSyncEnabled: true,
				Cfg:           &ldap.Source{},
			},
		},
		// case 20
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--page-size", "12",
			},
			authSource: &auth.Source{
				Type: auth.LDAP,
				Cfg: &ldap.Source{
					SearchPageSize: 12,
				},
			},
		},
		// case 21
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--security-protocol", "xxxxx",
			},
			errMsg: "Unknown security protocol name: xxxxx",
		},
		// case 22
		{
			args: []string{
				"ldap-test",
			},
			errMsg: "id is not set",
		},
		// case 23
		{
			args: []string{
				"ldap-test",
				"--id", "1",
			},
			existingAuthSource: &auth.Source{
				Type: auth.OAuth2,
				Cfg:  &ldap.Source{},
			},
			errMsg: "Invalid authentication type. expected: LDAP (via BindDN), actual: OAuth2",
		},
		// case 24
		{
			args: []string{
				"ldap-test",
				"--id", "24",
				"--name", "ldap (via Bind DN) flip 'active' and 'user sync' attributes",
				"--active",
				"--disable-synchronize-users",
			},
			id: 24,
			existingAuthSource: &auth.Source{
				Type:          auth.LDAP,
				IsActive:      false,
				IsSyncEnabled: true,
				Cfg: &ldap.Source{
					Name:    "ldap (via Bind DN) flip 'active' and 'user sync' attributes",
					Enabled: true,
				},
			},
			authSource: &auth.Source{
				Type:          auth.LDAP,
				Name:          "ldap (via Bind DN) flip 'active' and 'user sync' attributes",
				IsActive:      true,
				IsSyncEnabled: false,
				Cfg: &ldap.Source{
					Name:    "ldap (via Bind DN) flip 'active' and 'user sync' attributes",
					Enabled: true,
				},
			},
		},
	}

	for n, c := range cases {
		// Mock functions.
		var updatedAuthSource *auth.Source
		service := &authService{
			initDB: func(context.Context) error {
				return nil
			},
			createAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				assert.FailNow(t, "case %d: should not call createAuthSource", n)
				return nil
			},
			updateAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				updatedAuthSource = authSource
				return nil
			},
			getAuthSourceByID: func(ctx context.Context, id int64) (*auth.Source, error) {
				if c.id != 0 {
					assert.Equal(t, c.id, id, "case %d: wrong id", n)
				}
				if c.existingAuthSource != nil {
					return c.existingAuthSource, nil
				}
				return &auth.Source{
					Type: auth.LDAP,
					Cfg:  &ldap.Source{},
				}, nil
			},
		}

		// Create a copy of command to test
		app := cli.NewApp()
		app.Flags = microcmdAuthUpdateLdapBindDn.Flags
		app.Action = service.updateLdapBindDn

		// Run it
		err := app.Run(c.args)
		if c.errMsg != "" {
			assert.EqualError(t, err, c.errMsg, "case %d: error should match", n)
		} else {
			assert.NoError(t, err, "case %d: should have no errors", n)
			assert.Equal(t, c.authSource, updatedAuthSource, "case %d: wrong authSource", n)
		}
	}
}

func TestUpdateLdapSimpleAuth(t *testing.T) {
	// Mock cli functions to do not exit on error
	osExiter := cli.OsExiter
	defer func() { cli.OsExiter = osExiter }()
	cli.OsExiter = func(code int) {}

	// Test cases
	cases := []struct {
		args               []string
		id                 int64
		existingAuthSource *auth.Source
		authSource         *auth.Source
		errMsg             string
	}{
		// case 0
		{
			args: []string{
				"ldap-test",
				"--id", "7",
				"--name", "ldap (simple auth) source full",
				"--not-active",
				"--security-protocol", "starttls",
				"--skip-tls-verify",
				"--host", "ldap-simple-server full",
				"--port", "987",
				"--user-search-base", "ou=Users,dc=full-domain-simple,dc=org",
				"--user-filter", "(&(objectClass=posixAccount)(full-simple-cn=%s))",
				"--admin-filter", "(memberOf=cn=admin-group,ou=example,dc=full-domain-simple,dc=org)",
				"--restricted-filter", "(memberOf=cn=restricted-group,ou=example,dc=full-domain-simple,dc=org)",
				"--username-attribute", "uid-simple full",
				"--firstname-attribute", "givenName-simple full",
				"--surname-attribute", "sn-simple full",
				"--email-attribute", "mail-simple full",
				"--public-ssh-key-attribute", "publickey-simple full",
				"--avatar-attribute", "avatar-simple full",
				"--user-dn", "cn=%s,ou=Users,dc=full-domain-simple,dc=org",
			},
			id: 7,
			authSource: &auth.Source{
				Type:     auth.DLDAP,
				Name:     "ldap (simple auth) source full",
				IsActive: false,
				Cfg: &ldap.Source{
					Name:                  "ldap (simple auth) source full",
					Host:                  "ldap-simple-server full",
					Port:                  987,
					SecurityProtocol:      ldap.SecurityProtocol(2),
					SkipVerify:            true,
					UserDN:                "cn=%s,ou=Users,dc=full-domain-simple,dc=org",
					UserBase:              "ou=Users,dc=full-domain-simple,dc=org",
					AttributeUsername:     "uid-simple full",
					AttributeName:         "givenName-simple full",
					AttributeSurname:      "sn-simple full",
					AttributeMail:         "mail-simple full",
					AttributeSSHPublicKey: "publickey-simple full",
					AttributeAvatar:       "avatar-simple full",
					Filter:                "(&(objectClass=posixAccount)(full-simple-cn=%s))",
					AdminFilter:           "(memberOf=cn=admin-group,ou=example,dc=full-domain-simple,dc=org)",
					RestrictedFilter:      "(memberOf=cn=restricted-group,ou=example,dc=full-domain-simple,dc=org)",
				},
			},
		},
		// case 1
		{
			args: []string{
				"ldap-test",
				"--id", "1",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg:  &ldap.Source{},
			},
		},
		// case 2
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--name", "ldap (simple auth) source",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Name: "ldap (simple auth) source",
				Cfg: &ldap.Source{
					Name: "ldap (simple auth) source",
				},
			},
		},
		// case 3
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--not-active",
			},
			existingAuthSource: &auth.Source{
				Type:     auth.DLDAP,
				IsActive: true,
				Cfg:      &ldap.Source{},
			},
			authSource: &auth.Source{
				Type:     auth.DLDAP,
				IsActive: false,
				Cfg:      &ldap.Source{},
			},
		},
		// case 4
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--security-protocol", "starttls",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					SecurityProtocol: ldap.SecurityProtocol(2),
				},
			},
		},
		// case 5
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--skip-tls-verify",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					SkipVerify: true,
				},
			},
		},
		// case 6
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--host", "ldap-server",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					Host: "ldap-server",
				},
			},
		},
		// case 7
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--port", "987",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					Port: 987,
				},
			},
		},
		// case 8
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--user-search-base", "ou=Users,dc=domain,dc=org",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					UserBase: "ou=Users,dc=domain,dc=org",
				},
			},
		},
		// case 9
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--user-filter", "(&(objectClass=posixAccount)(cn=%s))",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					Filter: "(&(objectClass=posixAccount)(cn=%s))",
				},
			},
		},
		// case 10
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--admin-filter", "(memberOf=cn=admin-group,ou=example,dc=domain,dc=org)",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					AdminFilter: "(memberOf=cn=admin-group,ou=example,dc=domain,dc=org)",
				},
			},
		},
		// case 11
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--username-attribute", "uid",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					AttributeUsername: "uid",
				},
			},
		},
		// case 12
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--firstname-attribute", "givenName",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					AttributeName: "givenName",
				},
			},
		},
		// case 13
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--surname-attribute", "sn",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					AttributeSurname: "sn",
				},
			},
		},
		// case 14
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--email-attribute", "mail",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					AttributeMail: "mail",
				},
			},
		},
		// case 15
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--public-ssh-key-attribute", "publickey",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					AttributeSSHPublicKey: "publickey",
				},
			},
		},
		// case 16
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--user-dn", "cn=%s,ou=Users,dc=domain,dc=org",
			},
			authSource: &auth.Source{
				Type: auth.DLDAP,
				Cfg: &ldap.Source{
					UserDN: "cn=%s,ou=Users,dc=domain,dc=org",
				},
			},
		},
		// case 17
		{
			args: []string{
				"ldap-test",
				"--id", "1",
				"--security-protocol", "xxxxx",
			},
			errMsg: "Unknown security protocol name: xxxxx",
		},
		// case 18
		{
			args: []string{
				"ldap-test",
			},
			errMsg: "id is not set",
		},
		// case 19
		{
			args: []string{
				"ldap-test",
				"--id", "1",
			},
			existingAuthSource: &auth.Source{
				Type: auth.PAM,
				Cfg:  &ldap.Source{},
			},
			errMsg: "Invalid authentication type. expected: LDAP (simple auth), actual: PAM",
		},
		// case 20
		{
			args: []string{
				"ldap-test",
				"--id", "20",
				"--name", "ldap (simple auth) flip 'active' attribute",
				"--active",
			},
			id: 20,
			existingAuthSource: &auth.Source{
				Type:     auth.DLDAP,
				IsActive: false,
				Cfg: &ldap.Source{
					Name:    "ldap (simple auth) flip 'active' attribute",
					Enabled: true,
				},
			},
			authSource: &auth.Source{
				Type:     auth.DLDAP,
				Name:     "ldap (simple auth) flip 'active' attribute",
				IsActive: true,
				Cfg: &ldap.Source{
					Name:    "ldap (simple auth) flip 'active' attribute",
					Enabled: true,
				},
			},
		},
	}

	for n, c := range cases {
		// Mock functions.
		var updatedAuthSource *auth.Source
		service := &authService{
			initDB: func(context.Context) error {
				return nil
			},
			createAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				assert.FailNow(t, "case %d: should not call createAuthSource", n)
				return nil
			},
			updateAuthSource: func(ctx context.Context, authSource *auth.Source) error {
				updatedAuthSource = authSource
				return nil
			},
			getAuthSourceByID: func(ctx context.Context, id int64) (*auth.Source, error) {
				if c.id != 0 {
					assert.Equal(t, c.id, id, "case %d: wrong id", n)
				}
				if c.existingAuthSource != nil {
					return c.existingAuthSource, nil
				}
				return &auth.Source{
					Type: auth.DLDAP,
					Cfg:  &ldap.Source{},
				}, nil
			},
		}

		// Create a copy of command to test
		app := cli.NewApp()
		app.Flags = microcmdAuthUpdateLdapSimpleAuth.Flags
		app.Action = service.updateLdapSimpleAuth

		// Run it
		err := app.Run(c.args)
		if c.errMsg != "" {
			assert.EqualError(t, err, c.errMsg, "case %d: error should match", n)
		} else {
			assert.NoError(t, err, "case %d: should have no errors", n)
			assert.Equal(t, c.authSource, updatedAuthSource, "case %d: wrong authSource", n)
		}
	}
}
