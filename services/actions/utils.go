// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	secret_model "code.gitea.io/gitea/models/secret"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/log"
	secret_module "code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
)

func GetSecretsOfTask(ctx context.Context, task *actions_model.ActionTask) map[string]string {
	secrets := map[string]string{}

	secrets["GITHUB_TOKEN"] = task.Token
	secrets["GITEA_TOKEN"] = task.Token

	if task.Job.Run.IsForkPullRequest && task.Job.Run.TriggerEvent != actions_module.GithubEventPullRequestTarget {
		// ignore secrets for fork pull request, except GITHUB_TOKEN and GITEA_TOKEN which are automatically generated.
		// for the tasks triggered by pull_request_target event, they could access the secrets because they will run in the context of the base branch
		// see the documentation: https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request_target
		return secrets
	}

	ownerSecrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{OwnerID: task.Job.Run.Repo.OwnerID})
	if err != nil {
		log.Error("find secrets of owner %v: %v", task.Job.Run.Repo.OwnerID, err)
		// go on
	}
	repoSecrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{RepoID: task.Job.Run.RepoID})
	if err != nil {
		log.Error("find secrets of repo %v: %v", task.Job.Run.RepoID, err)
		// go on
	}

	for _, secret := range append(ownerSecrets, repoSecrets...) {
		if v, err := secret_module.DecryptSecret(setting.SecretKey, secret.Data); err != nil {
			log.Error("decrypt secret %v %q: %v", secret.ID, secret.Name, err)
			// go on
		} else {
			secrets[secret.Name] = v
		}
	}

	return secrets
}

func GetVariablesOfRun(ctx context.Context, run *actions_model.ActionRun) map[string]string {
	variables := map[string]string{}

	// Global
	globalVariables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{})
	if err != nil {
		log.Error("find global variables: %v", err)
	}

	// Org / User level
	ownerVariables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{OwnerID: run.Repo.OwnerID})
	if err != nil {
		log.Error("find variables of org: %d, error: %v", run.Repo.OwnerID, err)
	}

	// Repo level
	repoVariables, err := db.Find[actions_model.ActionVariable](ctx, actions_model.FindVariablesOpts{RepoID: run.RepoID})
	if err != nil {
		log.Error("find variables of repo: %d, error: %v", run.RepoID, err)
	}

	// Level precedence: Repo > Org / User > Global
	for _, v := range append(globalVariables, append(ownerVariables, repoVariables...)...) {
		variables[v.Name] = v.Data
	}

	return variables
}
