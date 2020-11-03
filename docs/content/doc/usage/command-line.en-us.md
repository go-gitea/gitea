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

**You can get an up to date help on all commands at any time using `gitea --help`**

Global options such as `--help`, `--version`, `--work-path`, `--config`, `--custom-path`... can be used as `[global options]` or `[command options]`.

### Usage

```
USAGE:
   gitea [global options] command [command options] [arguments...]

DESCRIPTION:
   By default, gitea will start serving using the webserver with no
arguments - which can alternatively be run by running the subcommand web.

COMMANDS:
     web       Start Gitea web server
     serv      This command should only be called by SSH shell
     hook      Delegate commands to corresponding Git hooks
     dump      Dump Gitea files and database
     cert      Generate self-signed certificate
     admin     Command line interface to perform common administrative operations
     generate  Command line interface for running generators
     migrate   Migrate the database
     keys      This command queries the Gitea database to get the authorized command for a given ssh key fingerprint
     convert   Convert the database
     doctor    Diagnose problems
     manager   Manage the running gitea process
     embedded  Extract embedded resources
     help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --port value, -p value         Temporary port number to prevent conflict (default: "3000")
   --pid value, -P value          Custom pid file path (default: "/var/run/gitea.pid")
   --custom-path value, -C value  Custom path file path (default: "/usr/local/bin/custom")
   --config value, -c value       Custom configuration file path (default: "/usr/local/bin/custom/conf/app.ini")
   --version, -v                  print the version
   --work-path value, -w value    Set the gitea working path (default: "/usr/local/bin")
   --help, -h                     show help


DEFAULT CONFIGURATION:
     CustomPath:  /usr/local/bin/custom
     CustomConf:  /usr/local/bin/custom/conf/app.ini
     AppPath:     /usr/local/bin/gitea
     AppWorkPath: /usr/local/bin
```


#### gitea web

```
NAME:
   gitea web - Start Gitea web server

USAGE:
   gitea web [command options] [arguments...]

DESCRIPTION:
   Gitea web server is the only thing you need to run,
and it takes care of all the other things for you

OPTIONS:
   --port value, -p value         Temporary port number to prevent conflict (default: "3000")
   --pid value, -P value          Custom pid file path (default: "/var/run/gitea.pid")


DEFAULT CONFIGURATION:
     CustomPath:  /usr/local/bin/custom
     CustomConf:  /usr/local/bin/custom/conf/app.ini
     AppPath:     /usr/local/bin/gitea
     AppWorkPath: /usr/local/bin
```


#### gitea dump

```
NAME:
   gitea dump - Dump Gitea files and database

USAGE:
   gitea dump [command options] [arguments...]

DESCRIPTION:
   Dump compresses all related files and database into zip file.
It can be used for backup and capture Gitea server image to send to maintainer

OPTIONS:
   --file value, -f value         Name of the dump file which will be created. (default: "gitea-dump-1600192450.zip")
   --verbose, -V                  Show process details
   --tempdir value, -t value      Temporary dir path (default: "/tmp")
   --database value, -d value     Specify the database SQL syntax
   --skip-repository, -R          Skip the repository dumping
   --skip-log, -L                 Skip the log dumping

```


#### gitea cert

```
NAME:
   gitea cert - Generate self-signed certificate

USAGE:
   gitea cert [command options] [arguments...]

DESCRIPTION:
   Generate a self-signed X.509 certificate for a TLS server.
Outputs to 'cert.pem' and 'key.pem' and will overwrite existing files.

OPTIONS:
   --host value                   Comma-separated hostnames and IPs to generate a certificate for
   --ecdsa-curve value            ECDSA curve to use to generate a key. Valid values are P224, P256, P384, P521
   --rsa-bits value               Size of RSA key to generate. Ignored if --ecdsa-curve is set (default: 2048)
   --start-date value             Creation date formatted as Jan 1 15:04:05 2011
   --duration value               Duration that certificate is valid for (default: 8760h0m0s)
   --ca                           whether this cert should be its own Certificate Authority


EXAMPLES:
   gitea cert --host git.example.com,example.com,www.example.com --ca
```

#### gitea admin

```
NAME:
   Gitea admin - Command line interface to perform common administrative operations

USAGE:
   Gitea admin command [command options] [arguments...]

COMMANDS:
     create-user         Create a new user in database
     change-password     Change a user's password
     repo-sync-releases  Synchronize repository releases with tags
     regenerate          Regenerate specific files
     auth                Modify external auth providers

OPTIONS:

```

##### gitea admin create-user


```
NAME:
   Gitea admin create-user - Create a new user in database

USAGE:
   Gitea admin create-user [command options] [arguments...]

OPTIONS:
   --name value                    Username. DEPRECATED: use username instead
   --username value                Username
   --password value                User password
   --email value                   User email address
   --admin                         User is an admin
   --random-password               Generate a random password for the user
   --must-change-password          Set this option to false to prevent forcing the user to change their password after initial login, (Default: true)
   --random-password-length value  Length of the random password to be generated (default: 12)
   --access-token                  Generate access token for the user
   --custom-path value, -C value   Custom path file path (default: "/usr/local/bin/custom")
   --config value, -c value        Custom configuration file path (default: "/usr/local/bin/custom/conf/app.ini")
   --version, -v                   print the version
   --work-path value, -w value     Set the gitea working path (default: "/usr/local/bin")
```


Examples:

```
gitea admin create-user --username myname --password asecurepassword --email me@example.com
```

##### gitea admin change-password

```
NAME:
   Gitea admin change-password - Change a user's password

USAGE:
   Gitea admin change-password [command options] [arguments...]

OPTIONS:
   --username value, -u value     The user to change password for
   --password value, -p value     New password to set for user

```
Examples:

```
gitea admin change-password --username myname --password asecurepassword
```

##### gitea admin repo-sync-releases

```
NAME:
   Gitea admin repo-sync-releases - Synchronize repository releases with tags

USAGE:
   Gitea admin repo-sync-releases [command options] [arguments...]

OPTIONS:

```


##### gitea admin regenerate

```
NAME:
   Gitea admin regenerate - Regenerate specific files

USAGE:
   Gitea admin regenerate command [command options] [arguments...]

COMMANDS:
     hooks  Regenerate git-hooks
     keys   Regenerate authorized_keys file

OPTIONS:

```



##### gitea admin auth

```
NAME:
   Gitea admin auth - Modify external auth providers

USAGE:
   Gitea admin auth command [command options] [arguments...]

COMMANDS:
     add-oauth           Add new Oauth authentication source
     update-oauth        Update existing Oauth authentication source
     add-ldap            Add new LDAP (via Bind DN) authentication source
     update-ldap         Update existing LDAP (via Bind DN) authentication source
     add-ldap-simple     Add new LDAP (simple auth) authentication source
     update-ldap-simple  Update existing LDAP (simple auth) authentication source
     list                List auth sources
     delete              Delete specific auth source

OPTIONS:


```

###### gitea admin auth add-oauth

```
NAME:
   Gitea admin auth add-oauth - Add new Oauth authentication source

USAGE:
   Gitea admin auth add-oauth [command options] [arguments...]

OPTIONS:
   --name value                   Application Name
   --provider value               OAuth2 Provider
   --key value                    Client ID (Key)
   --secret value                 Client Secret
   --auto-discover-url value      OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider)
   --use-custom-urls value        Use custom URLs for GitLab/GitHub OAuth endpoints (default: "false")
   --custom-auth-url value        Use a custom Authorization URL (option for GitLab/GitHub)
   --custom-token-url value       Use a custom Token URL (option for GitLab/GitHub)
   --custom-profile-url value     Use a custom Profile URL (option for GitLab/GitHub)
   --custom-email-url value       Use a custom Email URL (option for GitHub)


EXAMPLES:
   gitea admin auth add-oauth --name external-github --provider github --key OBTAIN_FROM_SOURCE --secret OBTAIN_FROM_SOURCE

```

##### gitea admin auth update-oauth

```
NAME:
   Gitea admin auth update-oauth - Update existing Oauth authentication source

USAGE:
   Gitea admin auth update-oauth [command options] [arguments...]

OPTIONS:
   --name value                   Application Name
   --id value                     ID of authentication source (default: 0)
   --provider value               OAuth2 Provider
   --key value                    Client ID (Key)
   --secret value                 Client Secret
   --auto-discover-url value      OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider)
   --use-custom-urls value        Use custom URLs for GitLab/GitHub OAuth endpoints (default: "false")
   --custom-auth-url value        Use a custom Authorization URL (option for GitLab/GitHub)
   --custom-token-url value       Use a custom Token URL (option for GitLab/GitHub)
   --custom-profile-url value     Use a custom Profile URL (option for GitLab/GitHub)
   --custom-email-url value       Use a custom Email URL (option for GitHub)


EXAMPLES:
   gitea admin auth update-oauth --id 1 --name external-github-updated

```

##### gitea admin auth add-ldap


```
NAME:
   Gitea admin auth add-ldap - Add new LDAP (via Bind DN) authentication source

USAGE:
   Gitea admin auth add-ldap [command options] [arguments...]

OPTIONS:
   --name value                      Authentication name.
   --not-active                      Deactivate the authentication source.
   --security-protocol value         Security protocol name.
   --skip-tls-verify                 Disable TLS verification.
   --host value                      The address where the LDAP server can be reached.
   --port value                      The port to use when connecting to the LDAP server. (default: 0)
   --user-search-base value          The LDAP base at which user accounts will be searched for.
   --user-filter value               An LDAP filter declaring how to find the user record that is attempting to authenticate.
   --admin-filter value              An LDAP filter specifying if a user should be given administrator privileges.
   --restricted-filter value         An LDAP filter specifying if a user should be given restricted status.
   --allow-deactivate-all            Allow empty search results to deactivate all users.
   --username-attribute value        The attribute of the user’s LDAP record containing the user name.
   --firstname-attribute value       The attribute of the user’s LDAP record containing the user’s first name.
   --surname-attribute value         The attribute of the user’s LDAP record containing the user’s surname.
   --email-attribute value           The attribute of the user’s LDAP record containing the user’s email address.
   --public-ssh-key-attribute value  The attribute of the user’s LDAP record containing the user’s public ssh key.
   --bind-dn value                   The DN to bind to the LDAP server with when searching for the user.
   --bind-password value             The password for the Bind DN, if any.
   --attributes-in-bind              Fetch attributes in bind DN context.
   --synchronize-users               Enable user synchronization.
   --page-size value                 Search page size. (default: 0)


EXAMPLES:
   gitea admin auth add-ldap --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-search-base "ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(uid=%s))" --email-attribute mail


```

##### gitea admin auth update-ldap

```
NAME:
   Gitea admin auth update-ldap - Update existing LDAP (via Bind DN) authentication source

USAGE:
   Gitea admin auth update-ldap [command options] [arguments...]

OPTIONS:
   --id value                        ID of authentication source (default: 0)
   --name value                      Authentication name.
   --not-active                      Deactivate the authentication source.
   --security-protocol value         Security protocol name.
   --skip-tls-verify                 Disable TLS verification.
   --host value                      The address where the LDAP server can be reached.
   --port value                      The port to use when connecting to the LDAP server. (default: 0)
   --user-search-base value          The LDAP base at which user accounts will be searched for.
   --user-filter value               An LDAP filter declaring how to find the user record that is attempting to authenticate.
   --admin-filter value              An LDAP filter specifying if a user should be given administrator privileges.
   --restricted-filter value         An LDAP filter specifying if a user should be given restricted status.
   --allow-deactivate-all            Allow empty search results to deactivate all users.
   --username-attribute value        The attribute of the user’s LDAP record containing the user name.
   --firstname-attribute value       The attribute of the user’s LDAP record containing the user’s first name.
   --surname-attribute value         The attribute of the user’s LDAP record containing the user’s surname.
   --email-attribute value           The attribute of the user’s LDAP record containing the user’s email address.
   --public-ssh-key-attribute value  The attribute of the user’s LDAP record containing the user’s public ssh key.
   --bind-dn value                   The DN to bind to the LDAP server with when searching for the user.
   --bind-password value             The password for the Bind DN, if any.
   --attributes-in-bind              Fetch attributes in bind DN context.
   --synchronize-users               Enable user synchronization.
   --page-size value                 Search page size. (default: 0)


EXAMPLES:
   gitea admin auth update-ldap --id 1 --name "my ldap auth source"
   gitea admin auth update-ldap --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn

```


###### gitea admin auth add-ldap-simple

```
NAME:
   Gitea admin auth add-ldap-simple - Add new LDAP (simple auth) authentication source

USAGE:
   Gitea admin auth add-ldap-simple [command options] [arguments...]

OPTIONS:
   --name value                      Authentication name.
   --not-active                      Deactivate the authentication source.
   --security-protocol value         Security protocol name.
   --skip-tls-verify                 Disable TLS verification.
   --host value                      The address where the LDAP server can be reached.
   --port value                      The port to use when connecting to the LDAP server. (default: 0)
   --user-search-base value          The LDAP base at which user accounts will be searched for.
   --user-filter value               An LDAP filter declaring how to find the user record that is attempting to authenticate.
   --admin-filter value              An LDAP filter specifying if a user should be given administrator privileges.
   --restricted-filter value         An LDAP filter specifying if a user should be given restricted status.
   --allow-deactivate-all            Allow empty search results to deactivate all users.
   --username-attribute value        The attribute of the user’s LDAP record containing the user name.
   --firstname-attribute value       The attribute of the user’s LDAP record containing the user’s first name.
   --surname-attribute value         The attribute of the user’s LDAP record containing the user’s surname.
   --email-attribute value           The attribute of the user’s LDAP record containing the user’s email address.
   --public-ssh-key-attribute value  The attribute of the user’s LDAP record containing the user’s public ssh key.
   --user-dn value                   The user’s DN.


EXAMPLES:
   gitea admin auth add-ldap-simple --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-dn "cn=%s,ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(cn=%s))" --email-attribute mail
```


##### gitea admin auth update-ldap-simple

```
NAME:
   Gitea admin auth update-ldap-simple - Update existing LDAP (simple auth) authentication source

USAGE:
   Gitea admin auth update-ldap-simple [command options] [arguments...]

OPTIONS:
   --id value                        ID of authentication source (default: 0)
   --name value                      Authentication name.
   --not-active                      Deactivate the authentication source.
   --security-protocol value         Security protocol name.
   --skip-tls-verify                 Disable TLS verification.
   --host value                      The address where the LDAP server can be reached.
   --port value                      The port to use when connecting to the LDAP server. (default: 0)
   --user-search-base value          The LDAP base at which user accounts will be searched for.
   --user-filter value               An LDAP filter declaring how to find the user record that is attempting to authenticate.
   --admin-filter value              An LDAP filter specifying if a user should be given administrator privileges.
   --restricted-filter value         An LDAP filter specifying if a user should be given restricted status.
   --allow-deactivate-all            Allow empty search results to deactivate all users.
   --username-attribute value        The attribute of the user’s LDAP record containing the user name.
   --firstname-attribute value       The attribute of the user’s LDAP record containing the user’s first name.
   --surname-attribute value         The attribute of the user’s LDAP record containing the user’s surname.
   --email-attribute value           The attribute of the user’s LDAP record containing the user’s email address.
   --public-ssh-key-attribute value  The attribute of the user’s LDAP record containing the user’s public ssh key.
   --user-dn value                   The user’s DN.


EXAMPLES:
   gitea admin auth update-ldap-simple --id 1 --name "my ldap auth source"
   gitea admin auth update-ldap-simple --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn

```


###### gitea admin auth list

```
NAME:
   Gitea admin auth list - List auth sources

USAGE:
   Gitea admin auth list [command options] [arguments...]

OPTIONS:
   --min-width value              Minimal cell width including any padding for the formatted table (default: 0)
   --tab-width value              width of tab characters in formatted table (equivalent number of spaces) (default: 8)
   --padding value                padding added to a cell before computing its width (default: 1)
   --pad-char value               ASCII char used for padding if padchar == '\\t', the Writer will assume that the width of a '\\t' in the formatted output is tabwidth, and cells are left-aligned independent of align_left (for correct-looking results, tabwidth must correspond to the tab width in the viewer displaying the result) (default: "\t")
   --vertical-bars                Set to true to print vertical bars between columns

```


###### gitea admin auth delete

```
NAME:
   Gitea admin auth delete - Delete specific auth source

USAGE:
   Gitea admin auth delete [command options] [arguments...]

OPTIONS:
   --id value                     ID of authentication source (default: 0)

EXAMPLES:
   gitea admin auth delete --id 1

```

#### gitea generate secret

```
NAME:
   Gitea generate secret - Generate a secret token

USAGE:
   Gitea generate secret command [command options] [arguments...]

COMMANDS:
     INTERNAL_TOKEN              Generate a new INTERNAL_TOKEN
     JWT_SECRET, LFS_JWT_SECRET  Generate a new JWT_SECRET
     SECRET_KEY                  Generate a new SECRET_KEY
```

Useful for generating values for automatic deployments.


#### gitea migrate

```
NAME:
   gitea migrate - Migrate the database

USAGE:
   gitea migrate [command options] [arguments...]

DESCRIPTION:
   This is a command for migrating the database, so that you can run gitea admin create-user before starting the server.
```


#### gitea keys

```
NAME:
   gitea keys - This command queries the Gitea database to get the authorized command for a given ssh key fingerprint

USAGE:
   gitea keys [command options] [arguments...]

OPTIONS:
   --expected value, -e value     Expected user for whom provide key commands (default: "git")
   --username value, -u value     Username trying to log in by SSH
   --type value, -t value         Type of the SSH key provided to the SSH Server (requires content to be provided too)
   --content value, -k value      Base64 encoded content of the SSH key provided to the SSH Server (requires type to be provided too)

```

Needs to be configured in the sshd config file:

```ini
...
# The value of -e and the AuthorizedKeysCommandUser should match the
# username running gitea
AuthorizedKeysCommandUser git
AuthorizedKeysCommand /path/to/gitea keys -e git -u %u -t %t -k %k
```

You should also set the value `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` in the `[server]` section of `app.ini`.

- NB: opensshd requires the gitea program to be owned by root and not writable by group or others. The program must be specified by an absolute path.
- NB: Gitea must be running for this command to succeed.


#### gitea convert

```
NAME:
   gitea convert - Convert the database

USAGE:
   gitea convert [command options] [arguments...]

DESCRIPTION:
   A command to convert an existing MySQL database from utf8 to utf8mb4

```


#### gitea doctor

```
NAME:
   gitea doctor - Diagnose problems

USAGE:
   gitea doctor [command options] [arguments...]

DESCRIPTION:
   A command to diagnose problems with the current Gitea instance according to the given configuration.

OPTIONS:
   --list                         List the available checks
   --default                      Run the default checks (if neither --run or --all is set, this is the default behaviour)
   --run value                    Run the provided checks - (if --default is set, the default checks will also run)
   --all                          Run all the available checks
   --fix                          Automatically fix what we can
   --log-file value               Name of the log file (default: "doctor.log"). Set to "-" to output to stdout, set to "" to disable


EXAMPLES:
   gitea doctor --run hooks --fix  Check if hook files are up-to-date and executable (and automatically fix)
```

For contributors: if you want to add more checks, you can write a new function like `func(ctx *cli.Context) ([]string, error)` and append it to `doctor.go`.

```go
# This function will receive a command line context and return a list of details about the problems or error.
var checklist = []check{
	{
		title: "Check if OpenSSH authorized_keys file id correct",
		f:     runDoctorLocationMoved,
    },
    // more checks please append here
}
```


#### gitea manager

```
NAME:
   Gitea manager - This is a command for managing the running gitea process

USAGE:
   Gitea manager command [command options] [arguments...]

COMMANDS:
     shutdown      Gracefully shutdown the running process
     restart       Gracefully restart the running process - (not implemented for windows servers)
     flush-queues  Flush queues in the running process

```

`gitea manager flush-queues` supports these additional options:

```
--non-blocking                 Set to true to not wait for flush to complete before returning
--timeout value                Timeout for the flushing process (default: 1m0s)
```

#### gitea embedded

```
NAME:
   Gitea embedded - A command for extracting embedded resources, like templates and images

USAGE:
   Gitea embedded command [command options] [arguments...]

COMMANDS:
     list     List files matching the given pattern
     view     View a file matching the given pattern
     extract  Extract resources

OPTIONS:
   --include-vendored, --vendor           Include files under public/vendor as well

```

`gitea embedded extract` supports these additional options:

```
   --overwrite                            Overwrite files if they already exist
   --rename                               Rename files as {name}.bak if they already exist (overwrites previous .bak)
   --custom                               Extract to the 'custom' directory as per app.ini
   --destination value, --dest-dir value  Extract to the specified directory
```
