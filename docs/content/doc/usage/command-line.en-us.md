---
date: "2017-01-01T16:00:00+02:00"
title: "Usage: Command Line"
slug: "command-line"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Command Line"
    weight: 10
    identifier: "command-line"
---

## Command Line

### Usage

`gitea [global options] command [command or global options] [arguments...]`

### Global options

All global options can be placed at the command level.

- `--help`, `-h`: Show help text and exit. Optional.
- `--version`, `-v`: Show version and exit. Optional. (example: `Gitea version 1.1.0+218-g7b907ed built with: bindata, sqlite`).
- `--custom-path path`, `-C path`: Location of the Gitea custom folder. Optional. (default: `AppWorkPath`/custom or `$GITEA_CUSTOM`).
- `--config path`, `-c path`: Gitea configuration file path. Optional. (default: `custom`/conf/app.ini).
- `--work-path path`, `-w path`: Gitea `AppWorkPath`. Optional. (default: LOCATION_OF_GITEA_BINARY or `$GITEA_WORK_DIR`)

NB: The defaults custom-path, config and work-path can also be
changed at build time (if preferred).

### Commands

#### web

Starts the server:

- Options:
    - `--port number`, `-p number`: Port number. Optional. (default: 3000). Overrides configuration file.
    - `--pid path`, `-P path`: Pidfile path. Optional.
- Examples:
    - `gitea web`
    - `gitea web --port 80`
    - `gitea web --config /etc/gitea.ini --pid /var/run/gitea.pid`
- Notes:
    - Gitea should not be run as root. To bind to a port below 1024, you can use setcap on
      Linux: `sudo setcap 'cap_net_bind_service=+ep' /path/to/gitea`. This will need to be
      redone every time you update Gitea.

#### admin

Admin operations:

- Commands:
    - `create-user`
        - Options:
            - `--name value`: Username. Required. As of gitea 1.9.0, use the `--username` flag instead.
            - `--username value`: Username. Required. New in gitea 1.9.0.
            - `--password value`: Password. Required.
            - `--email value`: Email. Required.
            - `--admin`: If provided, this makes the user an admin. Optional.
            - `--access-token`: If provided, an access token will be created for the user. Optional. (default: false).
            - `--must-change-password`: If provided, the created user will be required to choose a newer password after
	    the initial login. Optional. (default: true).
            - ``--random-password``: If provided, a randomly generated password will be used as the password of
	    the created user. The value of `--password` will be discarded. Optional.
            - `--random-password-length`: If provided, it will be used to configure the length of the randomly
	    generated password. Optional. (default: 12)
        - Examples:
            - `gitea admin create-user --username myname --password asecurepassword --email me@example.com`
    - `change-password`
        - Options:
            - `--username value`, `-u value`: Username. Required.
            - `--password value`, `-p value`: New password. Required.
        - Examples:
            - `gitea admin change-password --username myname --password asecurepassword`
    - `regenerate`
        - Options:
            - `hooks`: Regenerate git-hooks for all repositories
            - `keys`: Regenerate authorized_keys file
        - Examples:
            - `gitea admin regenerate hooks`
            - `gitea admin regenerate keys`
    - `auth`:
        - `list`:
            - Description: lists all external authentication sources that exist
            - Examples:
                - `gitea admin auth list`
        - `delete`:
            - Options:
                - `--id`: ID of source to be deleted. Required.
            - Examples:
                - `gitea admin auth delete --id 1`
        - `add-oauth`:
            - Options:
                - `--name`: Application Name.
                - `--provider`: OAuth2 Provider.
                - `--key`: Client ID (Key).
                - `--secret`: Client Secret.
                - `--auto-discover-url`: OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider).
                - `--use-custom-urls`: Use custom URLs for GitLab/GitHub OAuth endpoints.
                - `--custom-auth-url`: Use a custom Authorization URL (option for GitLab/GitHub).
                - `--custom-token-url`: Use a custom Token URL (option for GitLab/GitHub).
                - `--custom-profile-url`: Use a custom Profile URL (option for GitLab/GitHub).
                - `--custom-email-url`: Use a custom Email URL (option for GitHub).
            - Examples:
                - `gitea admin auth add-oauth --name external-github --provider github --key OBTAIN_FROM_SOURCE --secret OBTAIN_FROM_SOURCE`
        - `update-oauth`:
            - Options:
                - `--id`: ID of source to be updated. Required.
                - `--name`: Application Name.
                - `--provider`: OAuth2 Provider.
                - `--key`: Client ID (Key).
                - `--secret`: Client Secret.
                - `--auto-discover-url`: OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider).
                - `--use-custom-urls`: Use custom URLs for GitLab/GitHub OAuth endpoints.
                - `--custom-auth-url`: Use a custom Authorization URL (option for GitLab/GitHub).
                - `--custom-token-url`: Use a custom Token URL (option for GitLab/GitHub).
                - `--custom-profile-url`: Use a custom Profile URL (option for GitLab/GitHub).
                - `--custom-email-url`: Use a custom Email URL (option for GitHub).
            - Examples:
                - `gitea admin auth update-oauth --id 1 --name external-github-updated`
        - `add-ldap`: Add new LDAP (via Bind DN) authentication source
            - Options:
                - `--name value`: Authentication name. Required.
                - `--not-active`: Deactivate the authentication source.
                - `--security-protocol value`: Security protocol name. Required.
                - `--skip-tls-verify`: Disable TLS verification.
                - `--host value`: The address where the LDAP server can be reached. Required.
                - `--port value`: The port to use when connecting to the LDAP server. Required.
                - `--user-search-base value`: The LDAP base at which user accounts will be searched for. Required.
                - `--user-filter value`: An LDAP filter declaring how to find the user record that is attempting to authenticate. Required.
                - `--admin-filter value`: An LDAP filter specifying if a user should be given administrator privileges.
                - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
                - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
                - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
                - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address. Required.
                - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
                - `--bind-dn value`: The DN to bind to the LDAP server with when searching for the user.
                - `--bind-password value`: The password for the Bind DN, if any.
                - `--attributes-in-bind`: Fetch attributes in bind DN context.
                - `--synchronize-users`: Enable user synchronization.
                - `--page-size value`: Search page size.
            - Examples:
                - `gitea admin auth add-ldap --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-search-base "ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(uid=%s))" --email-attribute mail`
        - `update-ldap`: Update existing LDAP (via Bind DN) authentication source
            - Options:
                - `--id value`: ID of authentication source. Required.
                - `--name value`: Authentication name.
                - `--not-active`: Deactivate the authentication source.
                - `--security-protocol value`: Security protocol name.
                - `--skip-tls-verify`: Disable TLS verification.
                - `--host value`: The address where the LDAP server can be reached.
                - `--port value`: The port to use when connecting to the LDAP server.
                - `--user-search-base value`: The LDAP base at which user accounts will be searched for.
                - `--user-filter value`: An LDAP filter declaring how to find the user record that is attempting to authenticate.
                - `--admin-filter value`: An LDAP filter specifying if a user should be given administrator privileges.
                - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
                - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
                - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
                - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address.
                - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
                - `--bind-dn value`: The DN to bind to the LDAP server with when searching for the user.
                - `--bind-password value`: The password for the Bind DN, if any.
                - `--attributes-in-bind`: Fetch attributes in bind DN context.
                - `--synchronize-users`: Enable user synchronization.
                - `--page-size value`: Search page size.
            - Examples:
                - `gitea admin auth update-ldap --id 1 --name "my ldap auth source"`
                - `gitea admin auth update-ldap --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn`
        - `add-ldap-simple`: Add new LDAP (simple auth) authentication source
            - Options:
                - `--name value`: Authentication name. Required.
                - `--not-active`: Deactivate the authentication source.
                - `--security-protocol value`: Security protocol name. Required.
                - `--skip-tls-verify`: Disable TLS verification.
                - `--host value`: The address where the LDAP server can be reached. Required.
                - `--port value`: The port to use when connecting to the LDAP server. Required.
                - `--user-search-base value`: The LDAP base at which user accounts will be searched for.
                - `--user-filter value`: An LDAP filter declaring how to find the user record that is attempting to authenticate. Required.
                - `--admin-filter value`: An LDAP filter specifying if a user should be given administrator privileges.
                - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
                - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
                - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
                - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address. Required.
                - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
                - `--user-dn value`: The user’s DN. Required.
            - Examples:
                - `gitea admin auth add-ldap-simple --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-dn "cn=%s,ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(cn=%s))" --email-attribute mail`
        - `update-ldap-simple`: Update existing LDAP (simple auth) authentication source
            - Options:
                - `--id value`: ID of authentication source. Required.
                - `--name value`: Authentication name.
                - `--not-active`: Deactivate the authentication source.
                - `--security-protocol value`: Security protocol name.
                - `--skip-tls-verify`: Disable TLS verification.
                - `--host value`: The address where the LDAP server can be reached.
                - `--port value`: The port to use when connecting to the LDAP server.
                - `--user-search-base value`: The LDAP base at which user accounts will be searched for.
                - `--user-filter value`: An LDAP filter declaring how to find the user record that is attempting to authenticate.
                - `--admin-filter value`: An LDAP filter specifying if a user should be given administrator privileges.
                - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
                - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
                - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
                - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address.
                - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
                - `--user-dn value`: The user’s DN.
            - Examples:
                - `gitea admin auth update-ldap-simple --id 1 --name "my ldap auth source"`
                - `gitea admin auth update-ldap-simple --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn`

#### cert

Generates a self-signed SSL certificate. Outputs to `cert.pem` and `key.pem` in the current
directory and will overwrite any existing files.

- Options:
    - `--host value`: Comma seperated hostnames and ips which this certificate is valid for.
      Wildcards are supported. Required.
    - `--ecdsa-curve value`: ECDSA curve to use to generate a key. Optional. Valid options
      are P224, P256, P384, P521.
    - `--rsa-bits value`: Size of RSA key to generate. Optional. Ignored if --ecdsa-curve is
      set. (default: 2048).
    - `--start-date value`: Creation date. Optional. (format: `Jan 1 15:04:05 2011`).
    - `--duration value`: Duration which the certificate is valid for. Optional. (default: 8760h0m0s)
    - `--ca`: If provided, this cert generates it's own certificate authority. Optional.
- Examples:
    - `gitea cert --host git.example.com,example.com,www.example.com --ca`

#### dump

Dumps all files and databases into a zip file. Outputs into a file like `gitea-dump-1482906742.zip`
in the current directory.

- Options:
    - `--file name`, `-f name`: Name of the dump file with will be created. Optional. (default: gitea-dump-[timestamp].zip).
    - `--tempdir path`, `-t path`: Path to the temporary directory used. Optional. (default: /tmp).
    - `--skip-repository`, `-R`: Skip the repository dumping. Optional.
    - `--database`, `-d`: Specify the database SQL syntax. Optional.
    - `--verbose`, `-V`: If provided, shows additional details. Optional.
- Examples:
    - `gitea dump`
    - `gitea dump --verbose`

#### generate

Generates random values and tokens for usage in configuration file. Useful for generating values
for automatic deployments.

- Commands:
    - `secret`:
        - Options:
            - `INTERNAL_TOKEN`: Token used for an internal API call authentication.
            - `JWT_SECRET`: LFS & OAUTH2 JWT authentication secret (LFS_JWT_SECRET is aliased to this option for backwards compatibility).
            - `SECRET_KEY`: Global secret key.
        - Examples:
            - `gitea generate secret INTERNAL_TOKEN`
            - `gitea generate secret JWT_SECRET`
            - `gitea generate secret SECRET_KEY`

#### keys

Provides an SSHD AuthorizedKeysCommand. Needs to be configured in the sshd config file:

```ini
...
# The value of -e and the AuthorizedKeysCommandUser should match the
# username running gitea
AuthorizedKeysCommandUser git
AuthorizedKeysCommand /path/to/gitea keys -e git -u %u -t %t -k %k
```

The command will return the appropriate authorized_keys line for the
provided key. You should also set the value
`SSH_CREATE_AUTHORIZED_KEYS_FILE=false` in the `[server]` section of
`app.ini`.

NB: opensshd requires the gitea program to be owned by root and not
writable by group or others. The program must be specified by an absolute
path.
NB: Gitea must be running for this command to succeed.

#### migrate
Migrates the database. This command can be used to run other commands before starting the server for the first time.  
This command is idempotent.

#### convert
Converts an existing MySQL database from utf8 to utf8mb4.
