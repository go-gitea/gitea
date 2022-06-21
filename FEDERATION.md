# Federation

Gitea is federated using [ActivityPub](https://www.w3.org/TR/activitypub/) and the [ForgeFed extension](https://forgefed.org/) so you can interact with users and repositories from other instances as if they were on your own instance. By using the standardized ActivityPub protocol, users on any fediverse software such as [Mastodon](https://joinmastodon.org/) can follow Gitea users, receive activity updates, and comment on issues.

Currently, only S2S ActivityPub is supported.

## Following

You can use any fediverse software to follow a Gitea user. Gitea will automatically accept follow requests. The usernames of remote users are displayed as `username@instance.com`. To follow a remote user, click follow on their profile page, and a pop-up box will appear for you to type in your instance. You are redirected to your own instance, where the remote user is fetched and rendered, and you can now follow them.

When following a Gitea user, you will receive their activity updates. If you are using Mastodon or Pleroma, these will show up in your feed.

## Starring

You can star repositories on another instance. The full name of a remote repository is `username@instance.com/reponame`. Similar to following, a pop-up box appears for you to type in your instance, and you are redirected to your own instance, where the remote repository is fetched and rendered.

## Organizations

You can add users from other instances to organizations. An instance has a name and an instance, so its full name would look like `orgname@instance.com`. This indicates that the organization data resides on `instance.com`. To prevent syncronization errors, this data is only synchronized one-way to other instances.

## Collaborators

You can add users from other instances as collaborators. As mentioned previously, a repository has full name `username@instance.com/reponame`, which indicates that the repository data resides on `instance.com`.

Each collaborator's instance has a copy of the repository, but to prevent synchronization errors, the copy at `instance.com` is the main copy and it is synchronized one-way to all other instances. When a collaborator tries to modify their copy of the repository, the modification is first sent to the main copy at `instance.com` and then synchronized back to their instance.

I think rendering a remote repository without actually having a copy of the repository on your instance is a bad idea, because this will have to use the Gitea API instead of ForgeFed.

## Issues

You can create an issue on a remote repository.

## Comments

You can comment on an issue using any fediverse software. The entire issue thread is rendered on your instance, but the repository as a whole is not rendered.

## Forks

When forking a remote repository, the fork is created on your instance, not the remote instance. The maintainers of the original repository are added as implicit remote collaborators to your fork.

## Pull requests

When opening a pull request to a remote repository, the pull request can be rendered on your instance, but the repository as a whole is not rendered.

## Migrations

If you change your username or the name of repository, Gitea handles this similarly to how Mastodon does. Gitea will send a `Move` activity to your followers and update your actor to point to the new actor.
