---
date: "2016-12-26T16:00:00+02:00"
title: "Config Cheat Sheet"
slug: "config-cheat-sheet"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Config Cheat Sheet"
    weight: 20
    identifier: "config-cheat-sheet"
---

# Configuration Cheat Sheet

This is a cheat sheet for the Gitea configuration file. It contains all settings that can configured.

Any changes to the Gitea configuration file should be made in `custom/conf/app.ini` or any corresponding location. When installing from a distribution, this will typically be found at `/etc/gitea/conf/app.ini`.

The defaults provided here are best-effort (not built automatically). They are accurately recorded in [app.ini.sample](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.ini.sample) (s/master/<tag|release\>. Any string in the format `%(X)s` is a feature powered by [ini](https://github.com/go-ini/ini/#recursive-values), for reading values recursively.

Values containing `#` or `;` must be quoted using `` ` `` or `"""`.

**Note:** A full restart is required for Gitea configuration changes to take effect.

## Overall (`DEFAULT`)

- `APP_NAME`: **Gitea: Git with a cup of tea**: Application name, used in the page title.
- `RUN_USER`: **git**: The user Gitea will run as. This should be a dedicated service (non-user) account. Setting this incorrectly will cause Gitea to not start.
- `RUN_MODE`: **dev**: For performance and other purposes, change this to `prod` when deployed to a production environment. The installation process will set this to `prod` automatically. \[prod, dev, test\]

## Repository (`repository`)

- `ROOT`: **~/gitea-repositories/**: Root path for storing all repository data. It must be an absolute path.
- `SCRIPT_TYPE`: **bash**: The script type your server supports, usually this is `bash`, but some users report that only `sh` is available.
- `ANSI_CHARSET`: **\<empty\>**: The default charset for an unrecognized charset.
- `FORCE_PRIVATE`: **false**: Force every new repository to be private.
- `DEFAULT_PRIVATE`: **last**: Default private when create a new repository. \[last, private, public\]
- `MAX_CREATION_LIMIT`: **-1**: Global maximum creation limit of repositories per user, `-1` means no limit.
- `PULL_REQUEST_QUEUE_LENGTH`: **1000**: Length of pull request patch test queue, make it as large as possible. Use caution when editing this value.
- `MIRROR_QUEUE_LENGTH`: **1000**: Patch test queue length, increase if pull request patch testing starts hanging.
- `PREFERRED_LICENSES`: **Apache License 2.0,MIT License**: Preferred Licenses to place at the top of the List. Name must match file name in conf/license or custom/conf/license.
- `DISABLE_HTTP_GIT`: **false**: Disable the ability to interact with repositories over the HTTP protocol.
- `USE_COMPAT_SSH_URI`: **false**: Force ssh:// clone url instead of scp-style uri when default SSH port is used.

## UI (`ui`)

- `EXPLORE_PAGING_NUM`: **20**: Number of repositories that are shown in one explore page.
- `ISSUE_PAGING_NUM`: **10**: Number of issues that are shown in one page (for all pages that list issues).
- `FEED_MAX_COMMIT_NUM`: **5**: Number of maximum commits shown in one activity feed.

### UI - Admin (`ui.admin`)

- `USER_PAGING_NUM`: **50**: Number of users that are shown in one page.
- `REPO_PAGING_NUM`: **50**: Number of repos that are shown in one page.
- `NOTICE_PAGING_NUM`: **25**: Number of notices that are shown in one page.
- `ORG_PAGING_NUM`: **50**: Number of organizations that are shown in one page.

## Markdown (`markdown`)

- `ENABLE_HARD_LINE_BREAK`: **false**: Whether or not to enable hard the line break extension.

## Server (`server`)

- `PROTOCOL`: **http**: Either `http`, `https`, `fcgi`, or `unix`.
- `DOMAIN`: **localhost**: Domain name of your server.
- `ROOT_URL`: **%(PROTOCOL)s://%(DOMAIN)s:%(HTTP\_PORT)s/**: Full public URL of Gitea server.
- `HTTP_ADDR`: **0.0.0.0**: HTTP listen address.
   - If `PROTOCOL` is set to `fcgi`, Gitea will listen for FastCGI requests on TCP socket defined by `HTTP_ADDR` and `HTTP_PORT` configuration settings.
   - If `PROTOCOL` is set to `unix`, this should be the name of the Unix socket file to use.
- `HTTP_PORT`: **3000**: HTTP listen port.
   - If `PROTOCOL` is set to `fcgi`, Gitea will listen for FastCGI requests on TCP socket defined by `HTTP_ADDR` and `HTTP_PORT` configuration settings.
- `UNIX_SOCKET_PERMISSION`: **666**: Permisson mode for Unix socket.
- `DISABLE_SSH`: **false**: Disables SSH feature when it's not available.
- `START_SSH_SERVER`: **false**: When enabled, uses the built-in SSH server.
- `SSH_DOMAIN`: **%(DOMAIN)s**: Domain name of your ssh server, used for displayed clone URL.
- `SSH_PORT`: **22**: SSH port displayed in clone URL.
- `SSH_LISTEN_PORT`: **%(SSH\_PORT)s**: Port for the built-in SSH server.
- `OFFLINE_MODE`: **false**: Disables use of CDN for static files and Gravatar for profile pictures.
- `DISABLE_ROUTER_LOG`: **false**: Mutes printing of the router log.
- `CERT_FILE`: **custom/https/cert.pem**: Cert file path used for HTTPS.
- `KEY_FILE`: **custom/https/key.pem**: Key file path used for HTTPS.
- `STATIC_ROOT_PATH`: **./**: Upper level of template and static files path.
- `ENABLE_GZIP`: **false**: Enables application-level GZIP support.
- `LANDING_PAGE`: **home**: Landing page for unauthenticated users, either `home` or `explore`.
- `LFS_START_SERVER`: **false**: Enables git-lfs support.
- `LFS_CONTENT_PATH`: **./data/lfs**: Where to store lfs files.
- `LFS_JWT_SECRET`: **\<empty\>**: LFS authentication secret, change this a unique string.
- `REDIRECT_OTHER_PORT`: **false**: If true and `PROTOCOL` is https, redirects http requests on another (https) port.
- `PORT_TO_REDIRECT`: **80**: Port used when `REDIRECT_OTHER_PORT` is true.

## Database (`database`)

- `DB_TYPE`: **mysql**: The database type you choose, either `mysql`, `postgres`, `mssql` or `sqlite3`.
- `HOST`: **127.0.0.1:3306**: Database host address and port.
- `NAME`: **gitea**: Database name.
- `USER`: **root**: Database username.
- `PASSWD`: **\<empty\>**: Database user password.
- `SSL_MODE`: **disable**: For PostgreSQL only.
- `PATH`: **data/gitea.db**: For SQLite3 only, the database file path.

## Security (`security`)

- `INSTALL_LOCK`: **false**: Disable to allow accessing the install page.
- `SECRET_KEY`: **random at every install**: Global secret key. This should be changed.
- `LOGIN_REMEMBER_DAYS`: **7**: Cookie lifetime, in days.
- `COOKIE_USERNAME`: **gitea\_awesome**: Name of the cookie used to store the current username.
- `COOKIE_REMEMBER_NAME`: **gitea\_incredible**: Name of cookie used to store authentication information.
- `REVERSE_PROXY_AUTHENTICATION_USER`: **X-WEBAUTH-USER**: Header name for reverse proxy authentication.
- `DISABLE_GIT_HOOKS`: **false**: Prevent all users (including admin) from creating custom git hooks.

## OpenID (`openid`)

- `ENABLE_OPENID_SIGNIN`: **false**: Allow authentication in via OpenID.
- `ENABLE_OPENID_SIGNUP`: **! DISABLE\_REGISTRATION**: Allow registering via OpenID.
- `WHITELISTED_URIS`: **\<empty\>**: If non-empty, list of POSIX regex patterns matching OpenID URI's to permit.
- `BLACKLISTED_URIS`: **\<empty\>**: If non-empty, list of POSIX regex patterns matching OpenID URI's to block.

## Service (`service`)

- `ACTIVE_CODE_LIVE_MINUTES`: **180**: Time limit (min) to confirm account/email registration.
- `RESET_PASSWD_CODE_LIVE_MINUTES`: **180**: Time limit (min) to confirm forgot password reset process.
- `REGISTER_EMAIL_CONFIRM`: **false**: Enable this to ask for mail confirmation of registration. Requires `Mailer` to be enabled.
- `DISABLE_REGISTRATION`: **false**: Disable registration, after which only admin can create accounts for users.
- `REQUIRE_SIGNIN_VIEW`: **false**: Enable this to force users to log in to view any page.
- `ENABLE_NOTIFY_MAIL`: **false**: Enable this to send e-mail to watchers of a repository when something happens, like creating issues. Requires `Mailer` to be enabled.
- `ENABLE_REVERSE_PROXY_AUTHENTICATION`: **false**: Enable this to allow reverse proxy authentication.
- `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION`: **false**: Enable this to allow auto-registration for reverse authentication.
- `ENABLE_CAPTCHA`: **true**: Enable this to use captcha validation for registration.

## Webhook (`webhook`)

- `QUEUE_LENGTH`: **1000**: Hook task queue length. Use caution when editing this value.
- `DELIVER_TIMEOUT`: **5**: Delivery timeout (sec) for shooting webhooks.
- `SKIP_TLS_VERIFY`: **false**: Allow insecure certification.
- `PAGING_NUM`: **10**: Number of webhook history events that are shown in one page.

## Mailer (`mailer`)

- `ENABLED`: **false**: Enable to use a mail service.
- `DISABLE_HELO`: **\<empty\>**: Disable HELO operation.
- `HELO_HOSTNAME`: **\<empty\>**: Custom hostname for HELO operation.
- `HOST`: **\<empty\>**: SMTP mail host address and port (example: smtp.gitea.io:587).
- `FROM`: **\<empty\>**: Mail from address, RFC 5322. This can be just an email address, or the "Name" \<email@example.com\> format.
- `USER`: **\<empty\>**: Username of mailing user (usually the sender's e-mail address).
- `PASSWD`: **\<empty\>**: Password of mailing user.
- `SKIP_VERIFY`: **\<empty\>**: Do not verify the self-signed certificates.
   - **Note:** Gitea only supports SMTP with STARTTLS.
- `USE_SENDMAIL`: **false** Use the operating system's `sendmail` command instead of SMTP. This is common on linux systems.
   - Note that enabling sendmail will ignore all other `mailer` settings except `ENABLED`, `FROM` and `SENDMAIL_PATH`.
- `SENDMAIL_PATH`: **sendmail**: The location of sendmail on the operating system. (can be command or full path)

## Cache (`cache`)

- `ADAPTER`: **memory**: Cache engine adapter, either `memory`, `redis`, or `memcache`. If you want to use `redis` or `memcache`, be sure to rebuild everything with build tags `redis` or `memcache`: `go build -tags='redis'`.
- `INTERVAL`: **60**: Garbage Collection interval (sec), for memory cache only.
- `HOST`: **\<empty\>**: Connection string for `redis` and `memcache`.
    - Redis: `network=tcp,addr=127.0.0.1:6379,password=macaron,db=0,pool_size=100,idle_timeout=180`
    - Memache: `127.0.0.1:9090;127.0.0.1:9091`

## Session (`session`)

- `PROVIDER`: **memory**: Session engine provider, either `memory`, `file`, `redis`, or `mysql`.
- `PROVIDER_CONFIG`: **data/sessions**: For file, the root path; for others, the connection string.
- `COOKIE_SECURE`: **false**: Enable this to force using HTTPS for all session access.
- `COOKIE_NAME`: **i\_like\_gitea**: The name of the cookie used for the session ID, defaults to `i_like_gitea`.
- `GC_INTERVAL_TIME`: **86400**: GC interval in seconds.

## Picture (`picture`)

- `GRAVATAR_SOURCE`: **gravatar**: Can be `gravatar`, `duoshuo` or anything like `http://cn.gravatar.com/avatar/`.
- `DISABLE_GRAVATAR`: **false**: Enable this to use local avatars only.
- `ENABLE_FEDERATED_AVATAR`: **false**: Enable support for federated avatars (see http://www.libravatar.org)
- `AVATAR_UPLOAD_PATH`: **data/avatars**: Path to store local and cached files.

## Attachment (`attachment`)

- `ENABLED`: **true**: Enable this to allow uploading attachments.
- `PATH`: **data/attachments**: Path to store attachments.
- `ALLOWED_TYPES`: **see app.ini.sample**: Allowed MIME types, e.g. `image/jpeg|image/png`, use `*/*` for all types.
- `MAX_SIZE`: **4**: Maximum size (MB).
- `MAX_FILES`: **5**: Maximum number of attachments that can be uploaded at once.

## Log (`log`)

- `ROOT_PATH`: **\<empty\>**: Root path for log files.
- `MODE`: **console**: Logging mode. For multiple modes, use a comma to separate values.
- `LEVEL`: **Trace**: General log level. \[Trace, Debug, Info, Warn, Error, Critical\]

## Cron (`cron`)

- `ENABLED`: **true**: Run cron tasks periodically.
- `RUN_AT_START`: **false**: Run cron tasks at application start-up.

### Cron - Cleanup old repository archives (`cron.archive_cleanup`)

- `ENABLED`: **true**: Enable service.
- `RUN_AT_START`: **true**: Run tasks at start up time (if ENABLED).
- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.
- `OLDER_THAN`: **24h**: Archives created more than `OLDER_THAN` ago are subject to deletion, e.g. `12h`.

### Cron - Update Mirrors (`cron.update_mirrors`)

- `SCHEDULE`: **@every 10m**: Cron syntax for scheduling update mirrors, e.g. `@every 3h`.

### Cron - Repository Health Check (`cron.repo_health_check`)

- `SCHEDULE`: **every 24h**: Cron syntax for scheduling repository health check.
- `TIMEOUT`: **60s**: Time duration syntax for health check execution timeout.
- `ARGS`: **\<empty\>**: Arguments for command `git fsck`, e.g. `--unreachable --tags`.

### Cron - Repository Statistics Check (`cron.check_repo_stats`)

- `RUN_AT_START`: **true**: Run repository statistics check at start time.
- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository statistics check.

## Git (`git`)

- `MAX_GIT_DIFF_LINES`: **100**: Max number of lines allowed of a single file in diff view.
- `MAX_GIT_DIFF_LINE_CHARACTERS`: **5000**: Max character count per line highlighted in diff view.
- `MAX_GIT_DIFF_FILES`: **100**: Max number of files shown in diff view.
- `GC_ARGS`: **\<empty\>**: Arguments for command `git gc`, e.g. `--aggressive --auto`.

## markup (`markup`)

Gitea supports external markup support, you can integrate all your favorite tool. For example,
below will add a markup named `asciidoc` which is followed `markup.` ini section. And there are some config items below.

```ini
[markup.asciidoc]
ENABLED = false
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoc --out-file=- -"
IS_INPUT_FILE = false
```

- ENABLED: Whether enable this tool. default is false.
- FILE_EXTENSIONS: List of file extensions that should be rendered by an external command.  Multiple extentions needs a comma as splitter.
- RENDER_COMMAND: External command to render all matching extensions.
- IS_INPUT_FILE: Input is not a standard input but a file param followed `RENDER_COMMAND`, default is false.

## Other (`other`)

- `SHOW_FOOTER_BRANDING`: **false**: Show Gitea branding in the footer.
- `SHOW_FOOTER_VERSION`: **true**: Show Gitea version information in the footer.
- `SHOW_FOOTER_TEMPLATE_LOAD_TIME`: **true**: Show time of template execution in the footer.
