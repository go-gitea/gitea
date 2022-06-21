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

## Example

Aviva has an account on Gitea instance dev.example and Luke has an account on Gitea instance forge.example. Luke would like to create an issue on Aviva's Game of Life repository at dev.example/aviva/game-of-life. First, he clicks the `New Issue` button at the Game of Life repository. Since Luke does not have an account on dev.example, a pop-up box appears asking him to type in his instance, forge.example. He is redirected to forge.example/aviva@dev.example/game-of-life, where he can now create a new issue normally. Once he finishes creating the issue, his instance sends a `Create` activity to the dev.example/aviva/game-of-life actor, which creates the new issue on dev.example as well.

Next, Aviva replies to Luke's issue from her own instance. This sends a `Create` activity to forge.example, and the comment is created on Luke's instance.

I was writing an example scenario for the FEDERATION.md file and have ran into a problem. Let's say we have three people, Alice, Bobert, and Charlie. Alice and Bobert are on different instances and Bobert would like to create an issue on one of Alice's repositories. When he tries to create the issue on Alice's instance, he is redirected to his own instance, where the remote repo is rendered and he can create the issue. Once he finishes creating the issue, his instance sends a `Create` activity back to Alice's instance. So far no problems. However, what if Charlie then creates an issue on Alice's repository, how does Bobert's copy of the repo on his own instance know about Charlie's new issue?

If Bobert is a collaborator on Alice's repo, there's no problem because Alice's repo will send updates out to everyone in the repo's `Team` collection. However, if he is not, then when Bobert views the list of issues of the remote repo on his own instance, his instance will have to send out a request to fetch all the issues open on Alice's repo. OK, so that's not too bad either.

The real problem is supporting things like issue search on Bobert's instance. 