// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"strconv"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// env keys for git hooks need
const (
	EnvRepoName      = "GITEA_REPO_NAME"
	EnvRepoUsername  = "GITEA_REPO_USER_NAME"
	EnvRepoID        = "GITEA_REPO_ID"
	EnvRepoIsWiki    = "GITEA_REPO_IS_WIKI"
	EnvPusherName    = "GITEA_PUSHER_NAME"
	EnvPusherEmail   = "GITEA_PUSHER_EMAIL"
	EnvPusherID      = "GITEA_PUSHER_ID"
	EnvKeyID         = "GITEA_KEY_ID" // public key ID
	EnvDeployKeyID   = "GITEA_DEPLOY_KEY_ID"
	EnvPRID          = "GITEA_PR_ID"
	EnvPRIndex       = "GITEA_PR_INDEX" // not used by Gitea at the moment, it is for custom git hooks
	EnvPushTrigger   = "GITEA_PUSH_TRIGGER"
	EnvIsInternal    = "GITEA_INTERNAL_PUSH"
	EnvAppURL        = "GITEA_ROOT_URL"
	EnvActionsTaskID = "GITEA_ACTIONS_TASK_ID"
)

type PushTrigger string

const (
	PushTriggerPRMergeToBase    PushTrigger = "pr-merge-to-base"
	PushTriggerPRUpdateWithBase PushTrigger = "pr-update-with-base"
)

// InternalPushingEnvironment returns an os environment to switch off hooks on push
// It is recommended to avoid using this unless you are pushing within a transaction
// or if you absolutely are sure that post-receive and pre-receive will do nothing
// We provide the full pushing-environment for other hook providers
func InternalPushingEnvironment(doer *user_model.User, repo *repo_model.Repository) []string {
	return append(PushingEnvironment(doer, repo),
		EnvIsInternal+"=true",
	)
}

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
	if taskID, isActionsUser := user_model.GetActionsUserTaskID(doer); isActionsUser {
		env = append(env, EnvActionsTaskID+"="+strconv.FormatInt(taskID, 10))
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
		"SSH_ORIGINAL_COMMAND=gitea-internal",
	)
	environ = append(environ, DoerPushingEnvironment(committer, repo, isWiki)...)
	return environ
}
