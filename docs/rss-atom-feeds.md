# RSS and Atom feeds

Gitea exposes RSS and Atom feeds for user activity, repository activity, branches, files, tags, and releases. Feeds are enabled by default. Site administrators can disable them with `ENABLE_FEED = false` under `[other]` in `app.ini`.

Every feed is available in both formats: append `.rss` for RSS 2.0 or `.atom` for Atom. You can also request a page with an `Accept: application/rss+xml` or `Accept: application/atom+xml` header where that route supports content negotiation (for example a user or organization profile).

Replace `{owner}`, `{repo}`, `{branch}`, and `{path}` with the values for your instance. Examples below use `https://gitea.example.com` as the site root.

## User and organization activity

| Feed | URL |
| ---- | --- |
| User / org activity (RSS) | `https://gitea.example.com/{owner}.rss` |
| User / org activity (Atom) | `https://gitea.example.com/{owner}.atom` |

Example: `https://gitea.example.com/alice.rss`

## Repository activity

| Feed | URL |
| ---- | --- |
| Repository activity (RSS) | `https://gitea.example.com/{owner}/{repo}.rss` |
| Repository activity (Atom) | `https://gitea.example.com/{owner}/{repo}.atom` |

Example: `https://gitea.example.com/alice/my-project.rss`

## Branch commits

| Feed | URL |
| ---- | --- |
| Branch commits (RSS) | `https://gitea.example.com/{owner}/{repo}/rss/branch/{branch}` |
| Branch commits (Atom) | `https://gitea.example.com/{owner}/{repo}/atom/branch/{branch}` |

Example: `https://gitea.example.com/alice/my-project/rss/branch/main`

## File history on a branch

| Feed | URL |
| ---- | --- |
| File on a branch (RSS) | `https://gitea.example.com/{owner}/{repo}/rss/branch/{branch}/{path}` |
| File on a branch (Atom) | `https://gitea.example.com/{owner}/{repo}/atom/branch/{branch}/{path}` |

Example: `https://gitea.example.com/alice/my-project/rss/branch/main/README.md`

## Tags and releases

| Feed | URL |
| ---- | --- |
| Tags (RSS) | `https://gitea.example.com/{owner}/{repo}/tags.rss` |
| Tags (Atom) | `https://gitea.example.com/{owner}/{repo}/tags.atom` |
| Releases (RSS) | `https://gitea.example.com/{owner}/{repo}/releases.rss` |
| Releases (Atom) | `https://gitea.example.com/{owner}/{repo}/releases.atom` |

## Private repositories and authentication

Public feeds need no authentication. For private repositories (and private user activity), feed routes accept HTTP Basic authentication with a personal access token that has **repository** read scope:

```bash
curl -u 'USERNAME:TOKEN' https://gitea.example.com/alice/private-repo.rss
```

A token without repository read scope is rejected. A public-only user token does not include private activity in a user feed.

## Related configuration

- `[other]` `ENABLE_FEED` (default `true`) — master switch for all feeds above
- `[ui]` `FEED_MAX_COMMIT_NUM` — max commits shown in one activity feed item
- `[ui]` `FEED_PAGING_NUM` — items per page in the home feed
