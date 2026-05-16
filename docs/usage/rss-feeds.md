---
title: "RSS/Atom Feeds"
slug: "rss-feeds"
weight: 45
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "RSS/Atom Feeds"
    weight: 45
    identifier: "rss-feeds"
---

# RSS/Atom Feeds

Gitea provides RSS and Atom feeds for various resources. These feeds allow you to subscribe to updates using your favorite feed reader.

## Available Feed URLs

### User Feeds

| Feed | URL Format | Description |
|------|-----------|-------------|
| User Actions | `/{username}.rss` | RSS feed of a user's recent actions |
| User Actions (Atom) | `/{username}.atom` | Atom feed of a user's recent actions |

### Repository Feeds

| Feed | URL Format | Description |
|------|-----------|-------------|
| Repository Releases | `/{owner}/{repo}/releases.rss` | RSS feed of releases |
| Repository Releases (Atom) | `/{owner}/{repo}/releases.atom` | Atom feed of releases |
| Repository Tags | `/{owner}/{repo}/tags.rss` | RSS feed of tags |
| Repository Tags (Atom) | `/{owner}/{repo}/tags.atom` | Atom feed of tags |
| Repository Commits | `/{owner}/{repo}/commits/{branch}.rss` | RSS feed of commits |
| Repository Commits (Atom) | `/{owner}/{repo}/commits/{branch}.atom` | Atom feed of commits |

### Organization Feeds

| Feed | URL Format | Description |
|------|-----------|-------------|
| Organization Actions | `/{org}.rss` | RSS feed of an organization's recent actions |
| Organization Actions (Atom) | `/{org}.atom` | Atom feed of an organization's recent actions |

## Usage

Simply append `.rss` or `.atom` to the appropriate Gitea URL to get the feed.
For example:
- `https://gitea.com/username.rss` - User's recent actions as RSS
- `https://gitea.com/owner/repo/releases.rss` - Repository releases as RSS
- `https://gitea.com/owner/repo/commits/main.atom` - Commits on main branch as Atom

## Notes

- Feeds respect the same visibility settings as their corresponding web pages
- Private repositories require authentication to access their feeds
- Feed entries are sorted by date, newest first
- Feeds include up to 20 entries by default
