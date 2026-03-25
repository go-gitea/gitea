// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	actionsOIDCPath        = "/api/actions/oidc"
	actionsOIDCTokenPath   = actionsOIDCPath + "/token"
	actionsOIDCTokenExpiry = time.Hour
)

type actionsOIDCClaims struct {
	jwt.RegisteredClaims
	Actor                string `json:"actor"`
	ActorID              int64  `json:"actor_id"`
	Repository           string `json:"repository"`
	RepositoryID         int64  `json:"repository_id"`
	RepositoryOwner      string `json:"repository_owner"`
	RepositoryOwnerID    int64  `json:"repository_owner_id"`
	RunID                int64  `json:"run_id"`
	RunNumber            int64  `json:"run_number"`
	RunAttempt           int64  `json:"run_attempt"`
	Workflow             string `json:"workflow"`
	WorkflowRef          string `json:"workflow_ref,omitempty"`
	WorkflowSHA          string `json:"workflow_sha,omitempty"`
	JobWorkflowRef       string `json:"job_workflow_ref,omitempty"`
	JobWorkflowSHA       string `json:"job_workflow_sha,omitempty"`
	RepositoryVisibility string `json:"repository_visibility,omitempty"`
	EventName            string `json:"event_name"`
	Ref                  string `json:"ref"`
	RefType              string `json:"ref_type"`
	Sha                  string `json:"sha"`
	JobID                string `json:"job_id"`
	JobAttempt           int64  `json:"job_attempt"`
	BaseRef              string `json:"base_ref,omitempty"`
	HeadRef              string `json:"head_ref,omitempty"`
	RunnerEnvironment    string `json:"runner_environment,omitempty"`
	Environment          string `json:"environment,omitempty"`
}

// OIDCIssuer returns the issuer URL for Gitea Actions OIDC tokens.
func OIDCIssuer() string {
	return strings.TrimSuffix(setting.AppURL, "/") + actionsOIDCPath
}

// OIDCTokenRequestURL returns the URL for requesting an OIDC token.
func OIDCTokenRequestURL(task *actions_model.ActionTask) string {
	base := strings.TrimSuffix(setting.AppURL, "/")
	if task == nil || task.Job == nil {
		return base + actionsOIDCTokenPath
	}
	return fmt.Sprintf("%s%s?job_id=%d&run_id=%d", base, actionsOIDCTokenPath, task.Job.ID, task.Job.RunID)
}

// DefaultOIDCAudience returns the default audience used by OIDC tokens.
func DefaultOIDCAudience() string {
	return strings.TrimSuffix(setting.AppURL, "/")
}

// OIDCTokenExpiry returns the duration of issued OIDC tokens.
func OIDCTokenExpiry() time.Duration {
	return actionsOIDCTokenExpiry
}

// TaskAllowsOIDCToken reports whether a task is allowed to request an OIDC token.
func TaskAllowsOIDCToken(ctx context.Context, task *actions_model.ActionTask) (bool, error) {
	if err := task.LoadJob(ctx); err != nil {
		return false, err
	}
	if err := task.Job.LoadRepo(ctx); err != nil {
		return false, err
	}
	if err := task.Job.Repo.LoadOwner(ctx); err != nil {
		return false, err
	}

	repoActionsCfg := task.Job.Repo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	ownerActionsCfg, err := actions_model.GetOwnerActionsConfig(ctx, task.Job.Repo.OwnerID)
	if err != nil {
		return false, err
	}

	var jobDeclaredPerms repo_model.ActionsTokenPermissions
	if task.Job.TokenPermissions != nil {
		jobDeclaredPerms = *task.Job.TokenPermissions
	} else if repoActionsCfg.OverrideOwnerConfig {
		jobDeclaredPerms = repoActionsCfg.GetDefaultTokenPermissions()
	} else {
		jobDeclaredPerms = ownerActionsCfg.GetDefaultTokenPermissions()
	}

	if repoActionsCfg.OverrideOwnerConfig {
		jobDeclaredPerms = repoActionsCfg.ClampPermissions(jobDeclaredPerms)
	} else {
		jobDeclaredPerms = ownerActionsCfg.ClampPermissions(jobDeclaredPerms)
	}

	return jobDeclaredPerms.IDTokenAccessMode >= perm.AccessModeWrite, nil
}

// CreateOIDCToken creates and signs an OIDC token for the actions task.
func CreateOIDCToken(ctx context.Context, task *actions_model.ActionTask, audience string) (string, error) {
	if err := task.LoadJob(ctx); err != nil {
		return "", err
	}
	if err := task.Job.LoadAttributes(ctx); err != nil {
		return "", err
	}

	signingKey := oauth2_provider.DefaultSigningKey
	if signingKey == nil {
		return "", errors.New("missing OIDC signing key")
	}

	if audience == "" {
		audience = DefaultOIDCAudience()
	}

	if err := task.Job.Run.Repo.LoadOwner(ctx); err != nil {
		return "", err
	}

	ref, sha, refType, baseRef, headRef := resolveOIDCRefs(task.Job.Run)
	// TODO: Not supported at the moment https://github.com/go-gitea/gitea/pull/35336 will implement it
	environment := "prod"
	subject := fmt.Sprintf("repo:%s:environment:%s", task.Job.Run.Repo.FullName(), environment)
	now := time.Now()
	runAttempt := task.Job.Attempt
	if runAttempt == 0 {
		runAttempt = task.Attempt
	}
	jobAttempt := task.Attempt
	if jobAttempt == 0 {
		jobAttempt = task.Job.Attempt
	}

	claims := &actionsOIDCClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    OIDCIssuer(),
			Subject:   subject,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(actionsOIDCTokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
		Actor:                task.Job.Run.TriggerUser.Name,
		ActorID:              task.Job.Run.TriggerUser.ID,
		Repository:           task.Job.Run.Repo.FullName(),
		RepositoryID:         task.Job.Run.Repo.ID,
		RepositoryOwner:      task.Job.Run.Repo.OwnerName,
		RepositoryOwnerID:    task.Job.Run.Repo.OwnerID,
		RunID:                task.Job.Run.ID,
		RunNumber:            task.Job.Run.Index,
		RunAttempt:           runAttempt,
		Workflow:             task.Job.Run.WorkflowID,
		WorkflowRef:          buildWorkflowRef(task.Job.Run),
		WorkflowSHA:          task.Job.Run.CommitSHA,
		JobWorkflowRef:       buildWorkflowRef(task.Job.Run),
		JobWorkflowSHA:       task.Job.Run.CommitSHA,
		RepositoryVisibility: repositoryVisibility(task.Job.Run.Repo),
		EventName:            task.Job.Run.TriggerEvent,
		Ref:                  ref,
		RefType:              refType,
		Sha:                  sha,
		JobID:                task.Job.JobID,
		JobAttempt:           jobAttempt,
		BaseRef:              baseRef,
		HeadRef:              headRef,
		// TODO: Differentiate hosted vs self-hosted runners once runner metadata is available.
		RunnerEnvironment: "self-hosted",
		Environment:       environment,
	}

	token := jwt.NewWithClaims(signingKey.SigningMethod(), claims)
	signingKey.PreProcessToken(token)
	return token.SignedString(signingKey.SignKey())
}

func resolveOIDCRefs(run *actions_model.ActionRun) (ref, sha, refType, baseRef, headRef string) {
	ref = run.Ref
	sha = run.CommitSHA
	if pullPayload, err := run.GetPullRequestEventPayload(); err == nil && pullPayload.PullRequest != nil &&
		pullPayload.PullRequest.Base != nil && pullPayload.PullRequest.Head != nil {
		baseRef = pullPayload.PullRequest.Base.Ref
		headRef = pullPayload.PullRequest.Head.Ref
		if run.TriggerEvent == actions_module.GithubEventPullRequestTarget {
			ref = git.BranchPrefix + pullPayload.PullRequest.Base.Name
			sha = pullPayload.PullRequest.Base.Sha
		}
	}
	refType = string(git.RefName(ref).RefType())
	return ref, sha, refType, baseRef, headRef
}

func buildWorkflowRef(run *actions_model.ActionRun) string {
	if run == nil || run.Repo == nil || run.WorkflowID == "" {
		return ""
	}
	// TODO: When reusable workflows are supported, emit caller/callee refs separately.
	return fmt.Sprintf("%s/%s@%s", run.Repo.FullName(), run.WorkflowID, run.Ref)
}

func repositoryVisibility(repo *repo_model.Repository) string {
	if repo.IsPrivate {
		return "private"
	}
	switch repo.Owner.Visibility {
	case structs.VisibleTypeLimited:
		return "internal"
	case structs.VisibleTypePrivate:
		return "private"
	case structs.VisibleTypePublic:
		return "public"
	default:
		return "public"
	}
}
