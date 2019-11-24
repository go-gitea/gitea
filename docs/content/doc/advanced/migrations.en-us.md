---
date: "2019-04-15T17:29:00+08:00"
title: "Advanced: Migrations Interfaces"
slug: "migrations-interfaces"
weight: 30
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Migrations Interfaces"
    weight: 55
    identifier: "migrations-interfaces"
---

# Migration Features

The new migration features were introduced in Gitea 1.9.0. It defines two interfaces to support migrating 
repositories data from other git host platforms to gitea or, in the future migrating gitea data to other 
git host platforms. Currently, only the migrations from github via APIv3 to Gitea is implemented.

First of all, Gitea defines some standard objects in packages `modules/migrations/base`. They are
 `Repository`, `Milestone`, `Release`, `Label`, `Issue`, `Comment`, `PullRequest`.

## Downloader Interfaces

To migrate from a new git host platform, there are two steps to be updated.

- You should implement a `Downloader` which will get all kinds of repository informations.
- You should implement a `DownloaderFactory` which is used to detect if the URL matches and 
create a Downloader.
- You'll need to register the `DownloaderFactory` via `RegisterDownloaderFactory` on init.

```Go
type Downloader interface {
	GetRepoInfo() (*Repository, error)
	GetTopics() ([]string, error)
	GetMilestones() ([]*Milestone, error)
	GetReleases() ([]*Release, error)
	GetLabels() ([]*Label, error)
	GetIssues(start, limit int) ([]*Issue, error)
	GetComments(issueNumber int64) ([]*Comment, error)
	GetPullRequests(start, limit int) ([]*PullRequest, error)
}
```

```Go
type DownloaderFactory interface {
	Match(opts MigrateOptions) (bool, error)
	New(opts MigrateOptions) (Downloader, error)
}
```

## Uploader Interface

Currently, only a `GiteaLocalUploader` is implemented, so we only save downloaded 
data via this `Uploader` on the local Gitea instance. Other uploaders are not supported
and will be implemented in future.

```Go
// Uploader uploads all the informations
type Uploader interface {
	CreateRepo(repo *Repository, includeWiki bool) error
	CreateMilestone(milestone *Milestone) error
	CreateRelease(release *Release) error
	CreateLabel(label *Label) error
	CreateIssue(issue *Issue) error
	CreateComment(issueNumber int64, comment *Comment) error
	CreatePullRequest(pr *PullRequest) error
	Rollback() error
	Close()
}

```
