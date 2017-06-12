# Changelog

## Unreleased

* BREAKING
  * Password reset URL changed from `/user/forget_password` to `/user/forgot_password`
  * SSH keys management URL changed from `/user/settings/ssh` to `/user/settings/keys`

## [1.1.2](https://github.com/go-gitea/gitea/releases/tag/v1.1.2) - 2017-06-13

* BUGFIXES
  * Enforce netgo build tag while cross-compilation (Backport of #1690) (#1731)
  * fix update avatar
  * fix delete user failed on sqlite (#1321)
  * fix bug not to trim space of login username (#1806)
  * Backport bugfixes #1220 and #1393 to v1.1 (#1758)

## [1.1.1](https://github.com/go-gitea/gitea/releases/tag/v1.1.1) - 2017-05-04

* BUGFIXES
  * Markdown Sanitation Fix [#1646](https://github.com/go-gitea/gitea/pull/1646)
  * Fix broken hooks [#1376](https://github.com/go-gitea/gitea/pull/1376)
  * Fix migration issue [#1375](https://github.com/go-gitea/gitea/pull/1375)
  * Fix Wiki Issues [#1338](https://github.com/go-gitea/gitea/pull/1338)
  * Forgotten migration for wiki githooks [#1237](https://github.com/go-gitea/gitea/pull/1237)
  * Commit messages can contain pipes [#1218](https://github.com/go-gitea/gitea/pull/1218)
  * Verify external tracker URLs [#1236](https://github.com/go-gitea/gitea/pull/1236)
  * Allow upgrade after downgrade [#1197](https://github.com/go-gitea/gitea/pull/1197)
  * 500 on delete repo with issue [#1195](https://github.com/go-gitea/gitea/pull/1195)
  * INI compat with CrowdIn [#1192](https://github.com/go-gitea/gitea/pull/1192)

## [1.1.0](https://github.com/go-gitea/gitea/releases/tag/v1.1.0) - 2017-03-09

* BREAKING
  * The SSH keys can potentially break, make sure to regenerate the authorized keys
* FEATURE
  * Git LFSv2 support [#122](https://github.com/go-gitea/gitea/pull/122)
  * API endpoints for repo watching [#191](https://github.com/go-gitea/gitea/pull/191)
  * Search within private repos [#222](https://github.com/go-gitea/gitea/pull/222)
  * Hide user email address on explore page [#336](https://github.com/go-gitea/gitea/pull/336)
  * Protected branch system [#339](https://github.com/go-gitea/gitea/pull/339)
  * Sendmail for mail delivery [#355](https://github.com/go-gitea/gitea/pull/355)
  * API endpoints for org webhooks [#372](https://github.com/go-gitea/gitea/pull/372)
  * Enabled MSSQL support [#383](https://github.com/go-gitea/gitea/pull/383)
  * API endpoints for org teams [#370](https://github.com/go-gitea/gitea/pull/370)
  * API endpoints for collaborators [#375](https://github.com/go-gitea/gitea/pull/375)
  * Graceful server restart [#416](https://github.com/go-gitea/gitea/pull/416)
  * Commitgraph / timeline on commits page [#428](https://github.com/go-gitea/gitea/pull/428)
  * API endpoints for repo forks [#509](https://github.com/go-gitea/gitea/pull/509)
  * API endpoints for releases [#510](https://github.com/go-gitea/gitea/pull/510)
  * Folder jumping [#511](https://github.com/go-gitea/gitea/pull/511)
  * Stars tab on profile page [#519](https://github.com/go-gitea/gitea/pull/519)
  * Notification system [#523](https://github.com/go-gitea/gitea/pull/523)
  * Push and pull through reverse proxy basic auth [#524](https://github.com/go-gitea/gitea/pull/524)
  * Search for issues and pull requests [#530](https://github.com/go-gitea/gitea/pull/530)
  * API endpoint for stargazers [#597](https://github.com/go-gitea/gitea/pull/597)
  * API endpoints for subscribers [#598](https://github.com/go-gitea/gitea/pull/598)
  * PID file support [#610](https://github.com/go-gitea/gitea/pull/610)
  * Two factor authentication (2FA) [#630](https://github.com/go-gitea/gitea/pull/630)
  * API endpoints for org users [#645](https://github.com/go-gitea/gitea/pull/645)
  * Release attachments [#673](https://github.com/go-gitea/gitea/pull/673)
  * OAuth2 consumer [#679](https://github.com/go-gitea/gitea/pull/679)
  * Add ability to fork your own repos [#761](https://github.com/go-gitea/gitea/pull/761)
  * Search repository on dashboard [#773](https://github.com/go-gitea/gitea/pull/773)
  * Search bar on user profile [#787](https://github.com/go-gitea/gitea/pull/787)
  * Track label changes on issue view [#788](https://github.com/go-gitea/gitea/pull/788)
  * Allow using custom time format [#798](https://github.com/go-gitea/gitea/pull/798)
  * Redirects for renamed repos [#807](https://github.com/go-gitea/gitea/pull/807)
  * Track assignee changes on issue view [#808](https://github.com/go-gitea/gitea/pull/808)
  * Track title changes on issue view [#841](https://github.com/go-gitea/gitea/pull/841)
  * Archive cleanup action [#885](https://github.com/go-gitea/gitea/pull/885)
  * Basic Open Graph support [#901](https://github.com/go-gitea/gitea/pull/901)
  * Take back control of Git hooks [#1006](https://github.com/go-gitea/gitea/pull/1006)
  * API endpoints for user repos [#1059](https://github.com/go-gitea/gitea/pull/1059)
* BUGFIXES
  * Fixed counting issues for issue filters [#413](https://github.com/go-gitea/gitea/pull/413)
  * Added back default settings for SSH [#500](https://github.com/go-gitea/gitea/pull/500)
  * Fixed repo permissions [#513](https://github.com/go-gitea/gitea/pull/513)
  * Issues cannot be created with labels [#622](https://github.com/go-gitea/gitea/pull/622)
  * Add a reserved wiki paths check to the wiki [#720](https://github.com/go-gitea/gitea/pull/720)
  * Update website binding MaxSize to 255 [#722](https://github.com/go-gitea/gitea/pull/722)
  * User can see the private activity on public history [#818](https://github.com/go-gitea/gitea/pull/818)
  * Wrong pages number which includes private repositories [#844](https://github.com/go-gitea/gitea/pull/844)
  * Trim whitespaces for search keyword [#893](https://github.com/go-gitea/gitea/pull/893)
  * Don't rewrite non-gitea public keys [#906](https://github.com/go-gitea/gitea/pull/906)
  * Use fingerprint to check instead content for public key [#911](https://github.com/go-gitea/gitea/pull/911)
  * Fix random avatars [#1147](https://github.com/go-gitea/gitea/pull/1147)
* ENHANCEMENT
  * Refactored process manager [#75](https://github.com/go-gitea/gitea/pull/75)
  * Restrict rights to create new orgs [#193](https://github.com/go-gitea/gitea/pull/193)
  * Added label and milestone sorting [#199](https://github.com/go-gitea/gitea/pull/199)
  * Make minimum password length configurable [#223](https://github.com/go-gitea/gitea/pull/223)
  * Speedup conflict checking on pull requests [#276](https://github.com/go-gitea/gitea/pull/276)
  * Added button to delete merged pull request branches [#441](https://github.com/go-gitea/gitea/pull/441)
  * Improved issue references within markdown [#471](https://github.com/go-gitea/gitea/pull/471)
  * Dutch translation for the landingpage [#487](https://github.com/go-gitea/gitea/pull/487)
  * Added Gogs migration script [#532](https://github.com/go-gitea/gitea/pull/532)
  * Support a .gitea folder for issue templates [#582](https://github.com/go-gitea/gitea/pull/582)
  * Enhanced diff-view coloring [#584](https://github.com/go-gitea/gitea/pull/584)
  * Added ETag header to avatars [#721](https://github.com/go-gitea/gitea/pull/721)
  * Added option to config to disable local path imports [#724](https://github.com/go-gitea/gitea/pull/724)
  * Allow custom public files [#782](https://github.com/go-gitea/gitea/pull/782)
  * Added pprof endpoint for debugging [#801](https://github.com/go-gitea/gitea/pull/801)
  * Added `X-GitHub-*` headers [#809](https://github.com/go-gitea/gitea/pull/809)
  * Fill SSH key title automatically [#863](https://github.com/go-gitea/gitea/pull/863)
  * Display Git version on admin panel [#921](https://github.com/go-gitea/gitea/pull/921)
  * Expose URL field on issue API [#982](https://github.com/go-gitea/gitea/pull/982)
  * Statically compile the binaries [#985](https://github.com/go-gitea/gitea/pull/985)
  * Embed build tags into version string [#1051](https://github.com/go-gitea/gitea/pull/1051)
  * Gitignore support for FSharp and Clojure [#1072](https://github.com/go-gitea/gitea/pull/1072)
  * Custom templates for static builds [#1087](https://github.com/go-gitea/gitea/pull/1087)
  * Add ProxyFromEnvironment if none set [#1096](https://github.com/go-gitea/gitea/pull/1096)
* MISC
  * Replaced remaining Gogs references
  * Added more tests on various packages
  * Use Crowdin for translations again
  * Resolved some XSS attack vectors
  * Optimized and reduced number of database queries

## [1.0.2](https://github.com/go-gitea/gitea/releases/tag/v1.0.2) - 2017-02-21

* BUGFIXES
  * Fixed issue counter [#882](https://github.com/go-gitea/gitea/pull/882)
  * Fixed XSS vulnerability on wiki page [#955](https://github.com/go-gitea/gitea/pull/955)
  * Add data dir without session to dump [#587](https://github.com/go-gitea/gitea/pull/587)
  * Fixed wiki page renaming [#958](https://github.com/go-gitea/gitea/pull/958)
  * Drop default console logger if not required [#960](https://github.com/go-gitea/gitea/pull/960)
  * Fixed docker docs link on install page [#972](https://github.com/go-gitea/gitea/pull/972)
  * Handle SetModel errors [#957](https://github.com/go-gitea/gitea/pull/957)
  * Fixed XSS vulnerability on milestones [#977](https://github.com/go-gitea/gitea/pull/977)
  * Fixed XSS vulnerability on alerts [#981](https://github.com/go-gitea/gitea/pull/981)

## [1.0.1](https://github.com/go-gitea/gitea/releases/tag/v1.0.1) - 2017-01-05

* BUGFIXES
  * Fixed localized `MIN_PASSWORD_LENGTH` [#501](https://github.com/go-gitea/gitea/pull/501)
  * Fixed 500 error on organization delete [#507](https://github.com/go-gitea/gitea/pull/507)
  * Ignore empty wiki repo on migrate [#544](https://github.com/go-gitea/gitea/pull/544)
  * Proper check access for forking [#563](https://github.com/go-gitea/gitea/pull/563)
  * Fix SSH domain on installer [#506](https://github.com/go-gitea/gitea/pull/506)
  * Fix missing data rows on admin UI [#580](https://github.com/go-gitea/gitea/pull/580)
  * Do not delete tags with releases by default [#579](https://github.com/go-gitea/gitea/pull/579)
  * Fix missing session config data on admin UI [#578](https://github.com/go-gitea/gitea/pull/578)
  * Properly show the version within footer on the UI [#593](https://github.com/go-gitea/gitea/pull/593)

## [1.0.0](https://github.com/go-gitea/gitea/releases/tag/v1.0.0) - 2016-12-23

* BREAKING
  * We have various changes on the API, scripting against API must be updated
* FEATURE
  * Show last login for admins [#121](https://github.com/go-gitea/gitea/pull/121)
* BUGFIXES
  * Fixed sender of notifications [#2](https://github.com/go-gitea/gitea/pull/2)
  * Fixed keyword hijacking vulnerability [#20](https://github.com/go-gitea/gitea/pull/20)
  * Fixed non-markdown readme rendering [#95](https://github.com/go-gitea/gitea/pull/95)
  * Allow updating draft releases [#169](https://github.com/go-gitea/gitea/pull/169)
  * GitHub API compliance [#227](https://github.com/go-gitea/gitea/pull/227)
  * Added commit SHA to tag webhook [#286](https://github.com/go-gitea/gitea/issues/286)
  * Secured links via noopener [#315](https://github.com/go-gitea/gitea/issues/315)
  * Replace tabs with spaces on wiki title [#371](https://github.com/go-gitea/gitea/pull/371)
  * Fixed vulnerability on labels and releases [#409](https://github.com/go-gitea/gitea/pull/409)
  * Fixed issue comment API [#449](https://github.com/go-gitea/gitea/pull/449)
* ENHANCEMENT
  * Use proper import path for libravatar [#3](https://github.com/go-gitea/gitea/pull/3)
  * Integrated DroneCI for tests and builds [#24](https://github.com/go-gitea/gitea/issues/24)
  * Integrated dependency manager [#29](https://github.com/go-gitea/gitea/issues/29)
  * Embedded bindata optionally [#30](https://github.com/go-gitea/gitea/issues/30)
  * Integrated pagination for releases [#73](https://github.com/go-gitea/gitea/pull/73)
  * Autogenerate version on every build [#91](https://github.com/go-gitea/gitea/issues/91)
  * Refactored Docker container [#104](https://github.com/go-gitea/gitea/issues/104)
  * Added short-hash support for downloads [#211](https://github.com/go-gitea/gitea/issues/211)
  * Display tooltip for downloads [#221](https://github.com/go-gitea/gitea/issues/221)
  * Improved HTTP headers for issue attachments [#270](https://github.com/go-gitea/gitea/pull/270)
  * Integrate public as bindata optionally [#293](https://github.com/go-gitea/gitea/pull/293)
  * Integrate templates as bindata optionally [#314](https://github.com/go-gitea/gitea/pull/314)
  * Inject more ENV variables into custom hooks [#316](https://github.com/go-gitea/gitea/issues/316)
  * Correct LDAP login validation [#342](https://github.com/go-gitea/gitea/pull/342)
  * Integrate conf as bindata optionally [#354](https://github.com/go-gitea/gitea/pull/354)
  * Serve video files in browser [#418](https://github.com/go-gitea/gitea/pull/418)
  * Configurable SSH host binding [#431](https://github.com/go-gitea/gitea/issues/431)
* MISC
  * Forked from Gogs and renamed to Gitea
  * Catching more errors with logs
  * Fixed all linting errors
  * Made the go linter entirely happy
  * Really integrated vendoring
