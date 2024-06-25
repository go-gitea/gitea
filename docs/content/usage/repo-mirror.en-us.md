---
date: "2021-05-13T00:00:00-00:00"
title: "Repository Mirror"
slug: "repo-mirror"
sidebar_position: 45
toc: false
draft: false
aliases:
  - /en-us/repo-mirror
menu:
  sidebar:
    parent: "usage"
    name: "Repository Mirror"
    sidebar_position: 45
    identifier: "repo-mirror"
---

# Repository Mirror

Repository mirroring allows for the mirroring of repositories to and from external sources. You can use it to mirror branches, tags, and commits between repositories.

## Use cases

The following are some possible use cases for repository mirroring:

- You migrated to Gitea but still need to keep your project in another source. In that case, you can simply set it up to mirror to Gitea (pull) and all the essential history of commits, tags, and branches are available in your Gitea instance.
- You have old projects in another source that you don’t use actively anymore, but don’t want to remove for archiving purposes. In that case, you can create a push mirror so that your active Gitea repository can push its changes to the old location.

## Pulling from a remote repository

For an existing remote repository, you can set up pull mirroring as follows:

1. Select **New Migration** in the **Create...** menu on the top right.
2. Select the remote repository service.
3. Enter a repository URL.
4. If the repository needs authentication fill in your authentication information.
5. Check the box **This repository will be a mirror**.
6. Select **Migrate repository** to save the configuration.

The repository now gets mirrored periodically from the remote repository. You can force a sync by selecting **Synchronize Now** in the repository settings.

:exclamation::exclamation: **NOTE:** You can only set up pull mirroring for repos that don't exist yet on your instance. Once the repo is created, you can't convert it into a pull mirror anymore. :exclamation::exclamation:

## Pushing to a remote repository

For an existing repository, you can set up push mirroring as follows:

1. In your repository, go to **Settings** > **Repository**, and then the **Mirror Settings** section.
2. Enter a repository URL.
3. If the repository needs authentication expand the **Authorization** section and fill in your authentication information. Note that the requested **password** can also be your access token.
4. Select **Add Push Mirror** to save the configuration.

The repository now gets mirrored periodically to the remote repository. You can force a sync by selecting **Synchronize Now**. In case of an error a message displayed to help you resolve it.

:exclamation::exclamation: **NOTE:** This will force push to the remote repository. This will overwrite any changes in the remote repository! :exclamation::exclamation:

### Setting up a push mirror from Gitea to GitHub

To set up a mirror from Gitea to GitHub, you need to follow these steps:

1. Create a [GitHub personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token) with the *public_repo* box checked. Also check the **workflow** checkbox in case your repo uses GitHub Actions for continuous integration.
2. Create a repository with that name on GitHub. Unlike Gitea, GitHub does not support creating repositories by pushing to the remote. You can also use an existing remote repo if it has the same commit history as your Gitea repo.
3. In the settings of your Gitea repo, fill in the **Git Remote Repository URL**: `https://github.com/<your_github_group>/<your_github_project>.git`.
4. Fill in the **Authorization** fields with your GitHub username and the personal access token as **Password**.
5. (Optional, available on Gitea 1.18+) Select `Sync when new commits are pushed` so that the mirror will be updated as well as soon as there are changes. You can also disable the periodic sync if you like.
6. Select **Add Push Mirror** to save the configuration.

The repository pushes shortly thereafter. To force a push, select the **Synchronize Now** button.

### Setting up a push mirror from Gitea to GitLab

To set up a mirror from Gitea to GitLab, you need to follow these steps:

1. Create a [GitLab personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) with *write_repository* scope.
2. Fill in the **Git Remote Repository URL**: `https://<destination host>/<your_gitlab_group_or_name>/<your_gitlab_project>.git`.
3. Fill in the **Authorization** fields with `oauth2` as **Username** and your GitLab personal access token as **Password**.
4. Select **Add Push Mirror** to save the configuration.

The repository pushes shortly thereafter. To force a push, select the **Synchronize Now** button.

### Setting up a push mirror from Gitea to Bitbucket

To set up a mirror from Gitea to Bitbucket, you need to follow these steps:

1. Create a [Bitbucket app password](https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/) with the *Repository Write* box checked.
2. Fill in the **Git Remote Repository URL**: `https://bitbucket.org/<your_bitbucket_group_or_name>/<your_bitbucket_project>.git`.
3. Fill in the **Authorization** fields with your Bitbucket username and the app password as **Password**.
4. Select **Add Push Mirror** to save the configuration.

The repository pushes shortly thereafter. To force a push, select the **Synchronize Now** button.

### Mirror an existing ssh repository

Currently Gitea supports no ssh push mirrors. You can work around this by adding a `post-receive` hook to your Gitea repository that pushes manually.

1. Make sure the user running Gitea has access to the git repo you are trying to mirror to from shell.
2. On the web interface at the repository settings > git hooks add a post-receive hook for the mirror. I.e.

```
#!/usr/bin/env bash
git push --mirror --quiet git@github.com:username/repository.git &>/dev/null &
echo "GitHub mirror initiated .."
```
