# Changelog

## [1.0.1](https://github.com/go-gitea/gitea/releases/tag/v1.0.1) - 2017-01-05

* BUGFIXES
  * Fixed localized MIN_PASSWORD_LENGTH [#501](https://github.com/go-gitea/gitea/pull/501)
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
