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

`gitea [global options] command [command options] [arguments...]`

### Global options
 - `--help`, `-h`: Show help text and exit. Optional. This can be used with any of the
   subcommands to see help text for it.
 - `--version`, `-v`: Show version and exit. Optional. (example: `Gitea version
   1.1.0+218-g7b907ed built with: bindata, sqlite`).

### Commands

#### web

Starts the server:

- Options:
    - `--port number`, `-p number`: Port number. Optional. (default: 3000). Overrides configuration file.
    - `--config path`, `-c path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
    - `--pid path`, `-P path`: Pidfile path. Optional.
- Examples:
    - `gitea web`
    - `gitea web --port 80`
    - `gitea web --config /etc/gitea.ini --pid /var/run/gitea.pid`
- Notes:
    - Gitea should not be run as root. To bind to a port below 1000, you can use setcap on
      Linux: `sudo setcap 'cap_net_bind_service=+ep' /path/to/gitea`. This will need to be
      redone every time you update Gitea.

#### admin

Admin operations:

- Commands:
    - `create-user`
        - Options:
            - `--name value`: Username. Required.
            - `--password value`: Password. Required.
            - `--email value`: Email. Required.
            - `--admin`: If provided, this makes the user an admin. Optional.
            - `--config path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
            - `--must-change-password`: If provided, the created user will be required to choose a newer password after
	    the initial login. Optional. (default: true).
            - ``--random-password``: If provided, a randomly generated password will be used as the password of
	    the created user. The value of `--password` will be discarded. Optional.
            - `--random-password-length`: If provided, it will be used to configure the length of the randomly
	    generated password. Optional. (default: 12)
        - Examples:
            - `gitea admin create-user --name myname --password asecurepassword --email me@example.com`
    - `change-password`
        - Options:
            - `--username value`, `-u value`: Username. Required.
            - `--password value`, `-p value`: New password. Required.
            - `--config path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
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
            - Options:
                - `--config path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
            - Examples:
                - `gitea auth list`
        - `delete`:
            - Options:
                - `--id`: ID of source to be deleted. Required.
                - `--config path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
            - Examples:
                - `gitea auth delete --id 1`
        - `add-oauth`:
            - Options:
                - `--config path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
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
                - `gitea auth add-oauth --name external-github --provider github --key OBTAIN_FROM_SOURCE --secret OBTAIN_FROM_SOURCE`
        - `update-oauth`:
            - Options:
                - `--id`: ID of source to be updated. Required.
                - `--config path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
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
                - `gitea auth update-oauth --id 1 --name external-github-updated`

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
    - `--config path`, `-c path`: Gitea configuration file path. Optional. (default: custom/conf/app.ini).
    - `--tempdir path`, `-t path`: Path to the temporary directory used. Optional. (default: /tmp).
    - `--skip-repository`, `-R`: Skip the repository dumping. Optional.
    - `--database`, `-d`: Specify the database SQL syntax. Optional.
    - `--verbose`, `-v`: If provided, shows additional details. Optional.
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
            - `LFS_JWT_SECRET`: LFS authentication secret.
            - `SECRET_KEY`: Global secret key.
        - Examples:
            - `gitea generate secret INTERNAL_TOKEN`
            - `gitea generate secret LFS_JWT_SECRET`
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
