// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"strconv"
	"strings"

	repo_model "codenest.dev/models/repo"
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/setting"
	"codenest.dev/modules/util"
)

// env keys for git hooks need
const (
	EnvRepoName     = "CODENEST_REPO_NAME"
	EnvRepoUsername = "CODENEST_REPO_USER_NAME" // owner name
	EnvRepoID       = "CODENEST_REPO_ID"
	EnvRepoIsWiki   = "CODENEST_REPO_IS_WIKI"

	EnvKeyID = "CODENEST_KEY_ID" // public key ID

	EnvPusherName        = "CODENEST_PUSHER_NAME"
	EnvPusherEmail       = "CODENEST_PUSHER_EMAIL"
	EnvPusherID          = "CODENEST_PUSHER_ID"
	EnvPusherExtDoerData = "CODENEST_PUSHER_EXT_DOER_DATA"

	EnvPRID    = "CODENEST_PR_ID"
	EnvPRIndex = "CODENEST_PR_INDEX" // not used by CodeNest at the moment, it is for custom git hooks
	EnvAppURL  = "CODENEST_ROOT_URL"
)

// PushingEnvironment returns an os environment to allow hooks to work on push
func PushingEnvironment(doer *user_model.User, repo *repo_model.Repository) []string {
	return FullPushingEnvironment(doer, doer, repo, repo.Name, 0, 0)
}

func DoerPushingEnvironment(doer *user_model.User, repo *repo_model.Repository, isWiki bool) []string {
	env := []string{
		EnvAppURL + "=" + setting.AppURL,
		EnvRepoName + "=" + repo.Name + util.Iif(isWiki, ".wiki", ""),
		EnvRepoUsername + "=" + repo.OwnerName,
		EnvRepoID + "=" + strconv.FormatInt(repo.ID, 10),
		EnvRepoIsWiki + "=" + strconv.FormatBool(isWiki),
		EnvPusherName + "=" + doer.Name,
		EnvPusherID + "=" + strconv.FormatInt(doer.ID, 10),
	}
	if !doer.KeepEmailPrivate {
		env = append(env, EnvPusherEmail+"="+doer.Email)
	}
	if doer.ExtDoerData != nil {
		env = append(env, EnvPusherExtDoerData+"="+doer.ExtDoerData.EncodeToString())
	}
	return env
}

// FullPushingEnvironment returns an os environment to allow hooks to work on push
func FullPushingEnvironment(author, committer *user_model.User, repo *repo_model.Repository, repoName string, prID, prIndex int64) []string {
	isWiki := strings.HasSuffix(repoName, ".wiki")
	authorSig := author.NewGitSig()
	committerSig := committer.NewGitSig()
	environ := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_COMMITTER_NAME="+committerSig.Name,
		"GIT_COMMITTER_EMAIL="+committerSig.Email,
		EnvPRID+"="+strconv.FormatInt(prID, 10),
		EnvPRIndex+"="+strconv.FormatInt(prIndex, 10),
		"SSH_ORIGINAL_COMMAND=codenest-internal",
	)
	environ = append(environ, DoerPushingEnvironment(committer, repo, isWiki)...)
	return environ
}
