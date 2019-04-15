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

The new migration features introduced in Gitea 1.9.0. It defined two interfaces to support migrating 
repositories data from other git host platforms to gitea or in future migrating gitea data to other 
git host platform. Currently, it only implements to migrate from github via APIv3 to Gitea.

First of all, Gitea defines some standard objects, `Repository`, `Milestone`, `Release`, `Label`, `Issue`,
`Comment`, `PullRequest`.

## Downloader Interfaces

To migrate from a new git host platform, there are two steps to be updated.

- You should implement a Downloader which could get repository all kinds of informations.
- You should implement a DownloaderFactory which could detect if the url should match the 
factory and new a Downloader
- You should RegisterDownloaderFactory when init

```Go
type Downloader interface {
	GetRepoInfo() (*Repository, error)
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

Currently, we only implemented an Uploader `GiteaLocalUploader` so we only save downloaded 
data via this `Uploader` and we haven't supported a new Uploader that will be in future.

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
}

```