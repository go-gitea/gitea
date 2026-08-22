// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	actionsOIDCPath              = "/api/actions/oidc"
	actionsOIDCTokenPath         = actionsOIDCPath + "/token"
	actionsOIDCTokenExpiry       = 5 * time.Minute
	actionsOIDCMaxAudienceLength = 255
)

var (
	ErrOIDCInvalidAudience  = errors.New("invalid OIDC audience")
	ErrOIDCPermissionDenied = errors.New("OIDC token permission not granted")
	ErrOIDCTaskNotRunning   = errors.New("OIDC task is not running")
)

type actionsOIDCClaims struct {
	jwt.RegisteredClaims
	Actor                   string `json:"actor"`
	ActorID                 string `json:"actor_id"`
	Repository              string `json:"repository"`
	RepositoryID            string `json:"repository_id"`
	RepositoryOwner         string `json:"repository_owner"`
	RepositoryOwnerID       string `json:"repository_owner_id"`
	RunID                   string `json:"run_id"`
	RunNumber               string `json:"run_number"`
	RunAttempt              string `json:"run_attempt"`
	Workflow                string `json:"workflow"`
	WorkflowRepository      string `json:"workflow_repository"`
	WorkflowRepositoryID    string `json:"workflow_repository_id"`
	WorkflowRef             string `json:"workflow_ref,omitempty"`
	WorkflowSHA             string `json:"workflow_sha,omitempty"`
	JobWorkflowRepository   string `json:"job_workflow_repository,omitempty"`
	JobWorkflowRepositoryID string `json:"job_workflow_repository_id,omitempty"`
	JobWorkflowRef          string `json:"job_workflow_ref,omitempty"`
	JobWorkflowSHA          string `json:"job_workflow_sha,omitempty"`
	RepositoryVisibility    string `json:"repository_visibility"`
	EventName               string `json:"event_name"`
	Ref                     string `json:"ref,omitempty"`
	RefType                 string `json:"ref_type,omitempty"`
	SHA                     string `json:"sha"`
	JobID                   string `json:"job_id"`
	BaseRef                 string `json:"base_ref,omitempty"`
	HeadRef                 string `json:"head_ref,omitempty"`
	RunnerEnvironment       string `json:"runner_environment"`
}

func OIDCEnabled() bool {
	key := oauth2_provider.DefaultSigningKey
	return setting.Actions.Enabled && key != nil && !key.IsSymmetric()
}

// OIDCIssuer returns the issuer URL for Gitea Actions OIDC tokens.
func OIDCIssuer() string {
	return strings.TrimSuffix(setting.AppURL, "/") + actionsOIDCPath
}

// OIDCTokenRequestURL returns the capability-authenticated OIDC token endpoint.
func OIDCTokenRequestURL() string {
	return strings.TrimSuffix(setting.AppURL, "/") + actionsOIDCTokenPath + "?"
}

// TaskAllowsOIDCToken reports whether the task's canonical effective permissions allow token issuance.
func TaskAllowsOIDCToken(ctx context.Context, task *actions_model.ActionTask) (bool, error) {
	if err := task.LoadJob(ctx); err != nil {
		return false, err
	}
	if err := task.Job.LoadRepo(ctx); err != nil {
		return false, err
	}
	effective, err := actions_model.ComputeTaskTokenPermissions(ctx, task, task.Job.Repo)
	if err != nil {
		return false, err
	}
	return effective.IDTokenAccessMode == perm.AccessModeWrite, nil
}

// CreateOIDCToken reloads and authorizes a running task before signing its workload identity.
func CreateOIDCToken(ctx context.Context, taskID int64, audience string) (string, error) {
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		return "", err
	}
	if task.Status != actions_model.StatusRunning {
		return "", ErrOIDCTaskNotRunning
	}
	if err := task.LoadAttributes(ctx); err != nil {
		return "", err
	}
	allowed, err := TaskAllowsOIDCToken(ctx, task)
	if err != nil {
		return "", err
	}
	if !allowed {
		return "", ErrOIDCPermissionDenied
	}
	if !OIDCEnabled() {
		return "", errors.New("OIDC signing key is not available")
	}

	if audience == "" {
		audience = defaultOIDCAudience(task.Job.Run.Repo)
	}
	if err := validateOIDCAudience(audience); err != nil {
		return "", err
	}

	claims, err := createOIDCClaims(ctx, task, audience, time.Now().UTC())
	if err != nil {
		return "", err
	}
	signingKey := oauth2_provider.DefaultSigningKey
	token := jwt.NewWithClaims(signingKey.SigningMethod(), claims)
	signingKey.PreProcessToken(token)
	return token.SignedString(signingKey.SignKey())
}

func createOIDCClaims(ctx context.Context, task *actions_model.ActionTask, audience string, now time.Time) (*actionsOIDCClaims, error) {
	job, run := task.Job, task.Job.Run
	gitCtx := GenerateGiteaContext(ctx, run, nil, job)

	workflowRepo, err := loadOIDCWorkflowRepo(ctx, run.WorkflowRepoID)
	if err != nil {
		return nil, fmt.Errorf("load workflow source repository: %w", err)
	}
	rootJob, err := rootWorkflowJob(ctx, job)
	if err != nil {
		return nil, err
	}
	workflow := actions_module.WorkflowDisplayName(run.WorkflowID, rootJob.WorkflowPayload)
	workflowRef := buildOIDCWorkflowRef(workflowRepo, run.WorkflowPath, workflowSourceRef(run, gitCtx))
	jobWorkflowRepository, jobWorkflowRepositoryID, jobWorkflowRef, jobWorkflowSHA := "", "", "", ""
	if job.ParentJobID != 0 {
		jobWorkflowRepo, err := loadOIDCWorkflowRepo(ctx, job.WorkflowSourceRepoID)
		if err != nil {
			return nil, fmt.Errorf("load job workflow source repository: %w", err)
		}
		jobWorkflowRef, err = buildOIDCJobWorkflowRef(ctx, job, jobWorkflowRepo)
		if err != nil {
			return nil, err
		}
		jobWorkflowRepository = jobWorkflowRepo.FullName()
		jobWorkflowRepositoryID = strconv.FormatInt(jobWorkflowRepo.ID, 10)
		jobWorkflowSHA = job.WorkflowSourceCommitSHA
	}

	subject, err := buildOIDCSubject(run, contextString(gitCtx, "ref"))
	if err != nil {
		return nil, err
	}

	return &actionsOIDCClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    OIDCIssuer(),
			Subject:   subject,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(actionsOIDCTokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
		Actor:                   run.TriggerUser.Name,
		ActorID:                 strconv.FormatInt(run.TriggerUser.ID, 10),
		Repository:              run.Repo.FullName(),
		RepositoryID:            strconv.FormatInt(run.Repo.ID, 10),
		RepositoryOwner:         run.Repo.OwnerName,
		RepositoryOwnerID:       strconv.FormatInt(run.Repo.OwnerID, 10),
		RunID:                   strconv.FormatInt(run.ID, 10),
		RunNumber:               strconv.FormatInt(run.Index, 10),
		RunAttempt:              contextString(gitCtx, "run_attempt"),
		Workflow:                workflow,
		WorkflowRepository:      workflowRepo.FullName(),
		WorkflowRepositoryID:    strconv.FormatInt(workflowRepo.ID, 10),
		WorkflowRef:             workflowRef,
		WorkflowSHA:             run.WorkflowCommitSHA,
		JobWorkflowRepository:   jobWorkflowRepository,
		JobWorkflowRepositoryID: jobWorkflowRepositoryID,
		JobWorkflowRef:          jobWorkflowRef,
		JobWorkflowSHA:          jobWorkflowSHA,
		RepositoryVisibility:    repositoryVisibility(run.Repo),
		EventName:               run.TriggerEvent,
		Ref:                     contextString(gitCtx, "ref"),
		RefType:                 contextString(gitCtx, "ref_type"),
		SHA:                     contextString(gitCtx, "sha"),
		JobID:                   job.JobID,
		BaseRef:                 contextString(gitCtx, "base_ref"),
		HeadRef:                 contextString(gitCtx, "head_ref"),
		RunnerEnvironment:       "self-hosted",
	}, nil
}

func validateOIDCAudience(audience string) error {
	if len(audience) == 0 || len(audience) > actionsOIDCMaxAudienceLength || !utf8.ValidString(audience) || strings.TrimSpace(audience) != audience {
		return ErrOIDCInvalidAudience
	}
	for _, r := range audience {
		if unicode.IsControl(r) {
			return ErrOIDCInvalidAudience
		}
	}
	return nil
}

func defaultOIDCAudience(repo *repo_model.Repository) string {
	return strings.TrimSuffix(setting.AppURL, "/") + "/" + url.PathEscape(repo.OwnerName)
}

func buildOIDCSubject(run *actions_model.ActionRun, ref string) (string, error) {
	repositoryIdentity := fmt.Sprintf("%d/%d", run.Repo.OwnerID, run.Repo.ID)
	switch run.TriggerEvent {
	case actions_module.GithubEventPullRequest, actions_module.GithubEventPullRequestTarget:
		return "repo:" + repositoryIdentity + ":pull_request", nil
	}
	if ref == "" {
		return "", errors.New("OIDC subject requires an authoritative ref")
	}
	return "repo:" + repositoryIdentity + ":ref:" + escapeOIDCSubjectValue(ref), nil
}

func escapeOIDCSubjectValue(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	return strings.ReplaceAll(value, ":", "%3A")
}

func loadOIDCWorkflowRepo(ctx context.Context, repoID int64) (*repo_model.Repository, error) {
	if repoID == 0 {
		return nil, errors.New("workflow source repository is not recorded")
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}
	return repo, nil
}

func rootWorkflowJob(ctx context.Context, job *actions_model.ActionRunJob) (*actions_model.ActionRunJob, error) {
	visited := map[int64]struct{}{job.ID: {}}
	for job.ParentJobID != 0 {
		if _, ok := visited[job.ParentJobID]; ok {
			return nil, fmt.Errorf("reusable workflow job parent cycle at job %d", job.ParentJobID)
		}
		visited[job.ParentJobID] = struct{}{}
		parent, err := actions_model.GetRunJobByRunAndID(ctx, job.RunID, job.ParentJobID)
		if err != nil {
			return nil, err
		}
		job = parent
	}
	return job, nil
}

func workflowSourceRef(run *actions_model.ActionRun, gitCtx GiteaContext) string {
	if run.WorkflowRepoID == run.RepoID && !run.IsScopedRun {
		return contextString(gitCtx, "ref")
	}
	return run.WorkflowCommitSHA
}

func buildOIDCJobWorkflowRef(ctx context.Context, job *actions_model.ActionRunJob, sourceRepo *repo_model.Repository) (string, error) {
	parent, err := actions_model.GetRunJobByRunAndID(ctx, job.RunID, job.ParentJobID)
	if err != nil {
		return "", err
	}
	uses, err := ResolveUses(ctx, parent.CallUses)
	if err != nil {
		return "", nil
	}
	ref := job.WorkflowSourceCommitSHA
	if uses.Ref != "" {
		ref = uses.Ref
	}
	return buildOIDCWorkflowRef(sourceRepo, uses.Path, ref), nil
}

func buildOIDCWorkflowRef(repo *repo_model.Repository, workflowPath, ref string) string {
	if repo == nil || workflowPath == "" || ref == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s@%s", repo.FullName(), workflowPath, ref)
}

func contextString(ctx GiteaContext, key string) string {
	value, _ := ctx[key].(string)
	return value
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
	default:
		return "public"
	}
}
