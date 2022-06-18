# Federation

Gitea is federated using [ActivityPub](https://www.w3.org/TR/activitypub/) and the [ForgeFed extension](https://forgefed.org/) so you can interact with users and repositories from other instances as if they were on your own instance. By using the standardized ActivityPub protocol, users on any fediverse software such as [Mastodon](https://joinmastodon.org/) can follow Gitea users, receive activity updates, and comment on issues.

Currently, only S2S ActivityPub is supported.

## Actors

Following the ForgeFed specification, users (`Person` type), organizations (`Group` type), and repositories (`Repository` type) are the ActivityPub actors in Gitea.

### Users

Users are represented using the `Person` type and can be found at `/api/v1/activitypub/user/{username}`.

### Organizations

Organizations are represented using the `Group` type and can be found at `/api/v1/activitypub/user/{orgname}`.

### Repositories

Repositories are represented using the `Repository` type and can be found at `/api/v1/activitypub/repo/{username}/{reponame}`.

## Changing your username, organization name, or repository name

Do we want to support this? If so, Gitea will send out a `Move` activity.
