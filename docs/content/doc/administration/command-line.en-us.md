---
date: "2017-01-01T16:00:00+02:00"
title: "Gitea Command Line"
slug: "command-line"
weight: 1
toc: false
draft: false
menu:
  sidebar:
    parent: "administration"
    name: "Command Line"
    weight: 1
    identifier: "command-line"
---

# Command Line

**Table of Contents**

{{< toc >}}

## Usage

`gitea [global options] command [command or global options] [arguments...]`

## Global options

All global options can be placed at the command level.

- `--help`, `-h`: Show help text and exit. Optional.
- `--version`, `-v`: Show version and exit. Optional. (example: `Gitea version 1.1.0+218-g7b907ed built with: bindata, sqlite`).
- `--custom-path path`, `-C path`: Location of the Gitea custom folder. Optional. (default: `AppWorkPath`/custom or `$GITEA_CUSTOM`).
- `--config path`, `-c path`: Gitea configuration file path. Optional. (default: `custom`/conf/app.ini).
- `--work-path path`, `-w path`: Gitea `AppWorkPath`. Optional. (default: LOCATION_OF_GITEA_BINARY or `$GITEA_WORK_DIR`)

NB: The defaults custom-path, config and work-path can also be
changed at build time (if preferred).

## Commands

### web

Starts the server:

- Options:
  - `--port number`, `-p number`: Port number. Optional. (default: 3000). Overrides configuration file.
  - `--install-port number`: Port number to run the install page on. Optional. (default: 3000). Overrides configuration file.
  - `--pid path`, `-P path`: Pidfile path. Optional.
  - `--quiet`, `-q`: Only emit Fatal logs on the console for logs emitted before logging set up.
  - `--verbose`: Emit tracing logs on the console for logs emitted before logging is set-up.
- Examples:
  - `gitea web`
  - `gitea web --port 80`
  - `gitea web --config /etc/gitea.ini --pid /some/custom/gitea.pid`
- Notes:
  - Gitea should not be run as root. To bind to a port below 1024, you can use setcap on
    Linux: `sudo setcap 'cap_net_bind_service=+ep' /path/to/gitea`. This will need to be
    redone every time you update Gitea.

### admin

Admin operations:

- Commands:
  - `user`:
    - `list`:
      - Options:
        - `--admin`: List only admin users. Optional.
      - Description: lists all users that exist
      - Examples:
        - `gitea admin user list`
    - `delete`:
      - Options:
        - `--email`: Email of the user to be deleted.
        - `--username`: Username of user to be deleted.
        - `--id`: ID of user to be deleted.
        - One of `--id`, `--username` or `--email` is required. If more than one is provided then all have to match.
      - Examples:
        - `gitea admin user delete --id 1`
    - `create`:
      - Options:
        - `--name value`: Username. Required. As of Gitea 1.9.0, use the `--username` flag instead.
        - `--username value`: Username. Required. New in Gitea 1.9.0.
        - `--password value`: Password. Required.
        - `--email value`: Email. Required.
        - `--admin`: If provided, this makes the user an admin. Optional.
        - `--access-token`: If provided, an access token will be created for the user. Optional. (default: false).
        - `--must-change-password`: If provided, the created user will be required to choose a newer password after the
          initial login. Optional. (default: true).
        - `--random-password`: If provided, a randomly generated password will be used as the password of the created
          user. The value of `--password` will be discarded. Optional.
        - `--random-password-length`: If provided, it will be used to configure the length of the randomly generated
          password. Optional. (default: 12)
      - Examples:
        - `gitea admin user create --username myname --password asecurepassword --email me@example.com`
    - `change-password`:
      - Options:
        - `--username value`, `-u value`: Username. Required.
        - `--password value`, `-p value`: New password. Required.
      - Examples:
        - `gitea admin user change-password --username myname --password asecurepassword`
    - `must-change-password`:
      - Args:
        - `[username...]`: Users that must change their passwords
      - Options:
        - `--all`, `-A`: Force a password change for all users
        - `--exclude username`, `-e username`: Exclude the given user. Can be set multiple times.
        - `--unset`: Revoke forced password change for the given users
  - `regenerate`
    - Options:
      - `hooks`: Regenerate Git Hooks for all repositories
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
        - `--custom-tenant-id`: Use custom Tenant ID for OAuth endpoints.
        - `--custom-auth-url`: Use a custom Authorization URL (option for GitLab/GitHub).
        - `--custom-token-url`: Use a custom Token URL (option for GitLab/GitHub).
        - `--custom-profile-url`: Use a custom Profile URL (option for GitLab/GitHub).
        - `--custom-email-url`: Use a custom Email URL (option for GitHub).
        - `--icon-url`: Custom icon URL for OAuth2 login source.
        - `--skip-local-2fa`: Allow source to override local 2FA. (Optional)
        - `--scopes`: Additional scopes to request for this OAuth2 source. (Optional)
        - `--required-claim-name`: Claim name that has to be set to allow users to login with this source. (Optional)
        - `--required-claim-value`: Claim value that has to be set to allow users to login with this source. (Optional)
        - `--group-claim-name`: Claim name providing group names for this source. (Optional)
        - `--admin-group`: Group Claim value for administrator users. (Optional)
        - `--restricted-group`: Group Claim value for restricted users. (Optional)
        - `--group-team-map`: JSON mapping between groups and org teams. (Optional)
        - `--group-team-map-removal`: Activate automatic team membership removal depending on groups. (Optional)
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
        - `--custom-tenant-id`: Use custom Tenant ID for OAuth endpoints.
        - `--custom-auth-url`: Use a custom Authorization URL (option for GitLab/GitHub).
        - `--custom-token-url`: Use a custom Token URL (option for GitLab/GitHub).
        - `--custom-profile-url`: Use a custom Profile URL (option for GitLab/GitHub).
        - `--custom-email-url`: Use a custom Email URL (option for GitHub).
        - `--icon-url`: Custom icon URL for OAuth2 login source.
        - `--skip-local-2fa`: Allow source to override local 2FA. (Optional)
        - `--scopes`: Additional scopes to request for this OAuth2 source.
        - `--required-claim-name`: Claim name that has to be set to allow users to login with this source. (Optional)
        - `--required-claim-value`: Claim value that has to be set to allow users to login with this source. (Optional)
        - `--group-claim-name`: Claim name providing group names for this source. (Optional)
        - `--admin-group`: Group Claim value for administrator users. (Optional)
        - `--restricted-group`: Group Claim value for restricted users. (Optional)
      - Examples:
        - `gitea admin auth update-oauth --id 1 --name external-github-updated`
    - `add-smtp`:
      - Options:
        - `--name`: Application Name. Required.
        - `--auth-type`: SMTP Authentication Type (PLAIN/LOGIN/CRAM-MD5). Default to PLAIN.
        - `--host`: SMTP host. Required.
        - `--port`: SMTP port. Required.
        - `--force-smtps`: SMTPS is always used on port 465. Set this to force SMTPS on other ports.
        - `--skip-verify`: Skip TLS verify.
        - `--helo-hostname`: Hostname sent with HELO. Leave blank to send current hostname.
        - `--disable-helo`: Disable SMTP helo.
        - `--allowed-domains`: Leave empty to allow all domains. Separate multiple domains with a comma (',').
        - `--skip-local-2fa`: Skip 2FA to log on.
        - `--active`: This Authentication Source is Activated.
        Remarks:
        `--force-smtps`, `--skip-verify`, `--disable-helo`, `--skip-loca-2fs` and `--active` options can be used in form:
        - `--option`, `--option=true` to enable
        - `--option=false` to disable
        If those options are not specified value would not be changed in `update-smtp` or would use default `false` value in `add-smtp`
      - Examples:
        - `gitea admin auth add-smtp --name ldap --host smtp.mydomain.org --port 587 --skip-verify --active`
    - `update-smtp`:
      - Options:
        - `--id`: ID of source to be updated. Required.
        - other options are shared with `add-smtp`
      - Examples:
        - `gitea admin auth update-smtp --id 1 --host smtp.mydomain.org --port 587 --skip-verify=false`
        - `gitea admin auth update-smtp --id 1 --active=false`
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
        - `--restricted-filter value`: An LDAP filter specifying if a user should be given restricted status.
        - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
        - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
        - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
        - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address. Required.
        - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
        - `--avatar-attribute value`: The attribute of the user’s LDAP record containing the user’s avatar.
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
        - `--restricted-filter value`: An LDAP filter specifying if a user should be given restricted status.
        - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
        - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
        - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
        - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address.
        - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
        - `--avatar-attribute value`: The attribute of the user’s LDAP record containing the user’s avatar.
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
        - `--restricted-filter value`: An LDAP filter specifying if a user should be given restricted status.
        - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
        - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
        - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
        - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address. Required.
        - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
        - `--avatar-attribute value`: The attribute of the user’s LDAP record containing the user’s avatar.
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
        - `--restricted-filter value`: An LDAP filter specifying if a user should be given restricted status.
        - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
        - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
        - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
        - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address.
        - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
        - `--avatar-attribute value`: The attribute of the user’s LDAP record containing the user’s avatar.
        - `--user-dn value`: The user’s DN.
      - Examples:
        - `gitea admin auth update-ldap-simple --id 1 --name "my ldap auth source"`
        - `gitea admin auth update-ldap-simple --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn`

### cert

Generates a self-signed SSL certificate. Outputs to `cert.pem` and `key.pem` in the current
directory and will overwrite any existing files.

- Options:
  - `--host value`: Comma separated hostnames and ips which this certificate is valid for.
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

### dump

Dumps all files and databases into a zip file. Outputs into a file like `gitea-dump-1482906742.zip`
in the current directory.

- Options:
  - `--file name`, `-f name`: Name of the dump file with will be created. Optional. (default: gitea-dump-[timestamp].zip).
  - `--tempdir path`, `-t path`: Path to the temporary directory used. Optional. (default: /tmp).
  - `--skip-repository`, `-R`: Skip the repository dumping. Optional.
  - `--skip-custom-dir`: Skip dumping of the custom dir. Optional.
  - `--skip-lfs-data`: Skip dumping of LFS data. Optional.
  - `--skip-attachment-data`: Skip dumping of attachment data. Optional.
  - `--skip-package-data`: Skip dumping of package data. Optional.
  - `--skip-log`: Skip dumping of log data. Optional.
  - `--database`, `-d`: Specify the database SQL syntax. Optional.
  - `--verbose`, `-V`: If provided, shows additional details. Optional.
  - `--type`: Set the dump output format. Optional. (default: zip)
- Examples:
  - `gitea dump`
  - `gitea dump --verbose`

### generate

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

### keys

Provides an SSHD AuthorizedKeysCommand. Needs to be configured in the sshd config file:

```ini
...
# The value of -e and the AuthorizedKeysCommandUser should match the
# username running Gitea
AuthorizedKeysCommandUser git
AuthorizedKeysCommand /path/to/gitea keys -e git -u %u -t %t -k %k
```

The command will return the appropriate authorized_keys line for the
provided key. You should also set the value
`SSH_CREATE_AUTHORIZED_KEYS_FILE=false` in the `[server]` section of
`app.ini`.

NB: opensshd requires the Gitea program to be owned by root and not
writable by group or others. The program must be specified by an absolute
path.
NB: Gitea must be running for this command to succeed.

### migrate

Migrates the database. This command can be used to run other commands before starting the server for the first time.
This command is idempotent.

### convert

Converts an existing MySQL database from utf8 to utf8mb4.

### doctor

Diagnose the problems of current Gitea instance according the given configuration.
Currently there are a check list below:

- Check if OpenSSH authorized_keys file id correct
  When your Gitea instance support OpenSSH, your Gitea instance binary path will be written to `authorized_keys`
  when there is any public key added or changed on your Gitea instance.
  Sometimes if you moved or renamed your Gitea binary when upgrade and you haven't run `Update the '.ssh/authorized_keys' file with Gitea SSH keys. (Not needed for the built-in SSH server.)` on your Admin Panel. Then all pull/push via SSH will not be work.
  This check will help you to check if it works well.

For contributors, if you want to add more checks, you can write a new function like `func(ctx *cli.Context) ([]string, error)` and
append it to `doctor.go`.

```go
var checklist = []check{
	{
		title: "Check if OpenSSH authorized_keys file id correct",
		f:     runDoctorLocationMoved,
    },
    // more checks please append here
}
```

This function will receive a command line context and return a list of details about the problems or error.

#### doctor recreate-table

Sometimes when there are migrations the old columns and default values may be left
unchanged in the database schema. This may lead to warning such as:

```
2020/08/02 11:32:29 ...rm/session_schema.go:360:Sync2() [W] Table user Column keep_activity_private db default is , struct default is 0
```

You can cause Gitea to recreate these tables and copy the old data into the new table
with the defaults set appropriately by using:

```
gitea doctor recreate-table user
```

You can ask Gitea to recreate multiple tables using:

```
gitea doctor recreate-table table1 table2 ...
```

And if you would like Gitea to recreate all tables simply call:

```
gitea doctor recreate-table
```

It is highly recommended to back-up your database before running these commands.

### manager

Manage running server operations:

- Commands:
  - `shutdown`: Gracefully shutdown the running process
  - `restart`: Gracefully restart the running process - (not implemented for windows servers)
  - `flush-queues`: Flush queues in the running process
    - Options:
      - `--timeout value`: Timeout for the flushing process (default: 1m0s)
      - `--non-blocking`: Set to true to not wait for flush to complete before returning
  - `logging`: Adjust logging commands
    - Commands:
      - `pause`: Pause logging
        - Notes:
          - The logging level will be raised to INFO temporarily if it is below this level.
          - Gitea will buffer logs up to a certain point and will drop them after that point.
      - `resume`: Resume logging
      - `release-and-reopen`: Cause Gitea to release and re-open files and connections used for logging (Equivalent to sending SIGUSR1 to Gitea.)
      - `remove name`: Remove the named logger
        - Options:
          - `--group group`, `-g group`: Set the group to remove the sublogger from. (defaults to `default`)
      - `add`: Add a logger
        - Commands:
          - `console`: Add a console logger
            - Options:
              - `--group value`, `-g value`: Group to add logger to - will default to "default"
              - `--name value`, `-n value`: Name of the new logger - will default to mode
              - `--level value`, `-l value`: Logging level for the new logger
              - `--stacktrace-level value`, `-L value`: Stacktrace logging level
              - `--flags value`, `-F value`: Flags for the logger
              - `--expression value`, `-e value`: Matching expression for the logger
              - `--prefix value`, `-p value`: Prefix for the logger
              - `--color`: Use color in the logs
              - `--stderr`: Output console logs to stderr - only relevant for console
          - `file`: Add a file logger
            - Options:
              - `--group value`, `-g value`: Group to add logger to - will default to "default"
              - `--name value`, `-n value`: Name of the new logger - will default to mode
              - `--level value`, `-l value`: Logging level for the new logger
              - `--stacktrace-level value`, `-L value`: Stacktrace logging level
              - `--flags value`, `-F value`: Flags for the logger
              - `--expression value`, `-e value`: Matching expression for the logger
              - `--prefix value`, `-p value`: Prefix for the logger
              - `--color`: Use color in the logs
              - `--filename value`, `-f value`: Filename for the logger -
              - `--rotate`, `-r`: Rotate logs
              - `--max-size value`, `-s value`: Maximum size in bytes before rotation
              - `--daily`, `-d`: Rotate logs daily
              - `--max-days value`, `-D value`: Maximum number of daily logs to keep
              - `--compress`, `-z`: Compress rotated logs
              - `--compression-level value`, `-Z value`: Compression level to use
          - `conn`: Add a network connection logger
            - Options:
              - `--group value`, `-g value`: Group to add logger to - will default to "default"
              - `--name value`, `-n value`: Name of the new logger - will default to mode
              - `--level value`, `-l value`: Logging level for the new logger
              - `--stacktrace-level value`, `-L value`: Stacktrace logging level
              - `--flags value`, `-F value`: Flags for the logger
              - `--expression value`, `-e value`: Matching expression for the logger
              - `--prefix value`, `-p value`: Prefix for the logger
              - `--color`: Use color in the logs
              - `--reconnect-on-message`, `-R`: Reconnect to host for every message
              - `--reconnect`, `-r`: Reconnect to host when connection is dropped
              - `--protocol value`, `-P value`: Set protocol to use: tcp, unix, or udp (defaults to tcp)
              - `--address value`, `-a value`: Host address and port to connect to (defaults to :7020)
          - `smtp`: Add an SMTP logger
            - Options:
              - `--group value`, `-g value`: Group to add logger to - will default to "default"
              - `--name value`, `-n value`: Name of the new logger - will default to mode
              - `--level value`, `-l value`: Logging level for the new logger
              - `--stacktrace-level value`, `-L value`: Stacktrace logging level
              - `--flags value`, `-F value`: Flags for the logger
              - `--expression value`, `-e value`: Matching expression for the logger
              - `--prefix value`, `-p value`: Prefix for the logger
              - `--color`: Use color in the logs
              - `--username value`, `-u value`: Mail server username
              - `--password value`, `-P value`: Mail server password
              - `--host value`, `-H value`: Mail server host (defaults to: 127.0.0.1:25)
              - `--send-to value`, `-s value`: Email address(es) to send to
              - `--subject value`, `-S value`: Subject header of sent emails
  - `processes`: Display Gitea processes and goroutine information
    - Options:
      - `--flat`: Show processes as flat table rather than as tree
      - `--no-system`: Do not show system processes
      - `--stacktraces`: Show stacktraces for goroutines associated with processes
      - `--json`: Output as json
      - `--cancel PID`: Send cancel to process with PID. (Only for non-system processes.)

### dump-repo

Dump-repo dumps repository data from Git/GitHub/Gitea/GitLab:

- Options:
  - `--git_service service` : Git service, it could be `git`, `github`, `gitea`, `gitlab`, If clone_addr could be recognized, this could be ignored.
  - `--repo_dir dir`, `-r dir`: Repository dir path to store the data
  - `--clone_addr addr`: The URL will be clone, currently could be a git/github/gitea/gitlab http/https URL. i.e. https://github.com/lunny/tango.git
  - `--auth_username lunny`: The username to visit the clone_addr
  - `--auth_password <password>`: The password to visit the clone_addr
  - `--auth_token <token>`: The personal token to visit the clone_addr
  - `--owner_name lunny`: The data will be stored on a directory with owner name if not empty
  - `--repo_name tango`: The data will be stored on a directory with repository name if not empty
  - `--units <units>`: Which items will be migrated, one or more units should be separated as comma. wiki, issues, labels, releases, release_assets, milestones, pull_requests, comments are allowed. Empty means all units.

### restore-repo

Restore-repo restore repository data from disk dir:

- Options:
  - `--repo_dir dir`, `-r dir`: Repository dir path to restore from
  - `--owner_name lunny`: Restore destination owner name
  - `--repo_name tango`: Restore destination repository name
  - `--units <units>`: Which items will be restored, one or more units should be separated as comma. wiki, issues, labels, releases, release_assets, milestones, pull_requests, comments are allowed. Empty means all units.

### actions generate-runner-token

Generate a new token for a runner to use to register with the server

- Options:
  - `--scope {owner}[/{repo}]`, `-s {owner}[/{repo}]`: To limit the scope of the runner, no scope means the runner can be used for all repos, but you can also limit it to a specific repo or owner

To register a global runner:

```
gitea actions generate-runner-token
```

To register a runner for a specific organization, in this case `org`:

```
gitea actions generate-runner-token -s org
```

To register a runner for a specific repo, in this case `username/test-repo`:

```
gitea actions generate-runner-token -s username/test-repo
```
