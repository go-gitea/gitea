// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"gitea.dev/actionslib/pkg/model"
	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
)

const OIDCSigningAlgorithm = "RS256"

var oidcSigningKey struct {
	sync.Mutex
	key oauth2_provider.JWTSigningKey
}

// OIDCSigningKey retries after errors, another instance sharing the data path may still be writing the key
func OIDCSigningKey() (oauth2_provider.JWTSigningKey, error) {
	oidcSigningKey.Lock()
	defer oidcSigningKey.Unlock()
	if oidcSigningKey.key == nil {
		key, err := oauth2_provider.LoadOrCreateAsymmetricKey(filepath.Join(setting.AppDataPath, "jwt", "actions_oidc.pem"), OIDCSigningAlgorithm)
		if err != nil {
			return nil, err
		}
		if oidcSigningKey.key, err = oauth2_provider.CreateJWTSigningKey(OIDCSigningAlgorithm, key); err != nil {
			return nil, err
		}
	}
	return oidcSigningKey.key, nil
}

type oidcClaims struct {
	jwt.RegisteredClaims
	Audience             string `json:"aud"` // a single string like GitHub, jwt.ClaimStrings encodes an array
	Actor                string `json:"actor"`
	ActorID              string `json:"actor_id"`
	Repository           string `json:"repository"`
	RepositoryID         string `json:"repository_id"`
	RepositoryOwner      string `json:"repository_owner"`
	RepositoryOwnerID    string `json:"repository_owner_id"`
	RepositoryVisibility string `json:"repository_visibility"`
	RunID                string `json:"run_id"`
	RunNumber            string `json:"run_number"`
	RunAttempt           string `json:"run_attempt"`
	EventName            string `json:"event_name"`
	Ref                  string `json:"ref"`
	RefType              string `json:"ref_type"`
	RefProtected         string `json:"ref_protected"`
	SHA                  string `json:"sha"`
	BaseRef              string `json:"base_ref"`
	HeadRef              string `json:"head_ref"`
	Workflow             string `json:"workflow"`
	WorkflowRef          string `json:"workflow_ref"`
	WorkflowSHA          string `json:"workflow_sha"`
	JobWorkflowRef       string `json:"job_workflow_ref"`
	JobWorkflowSHA       string `json:"job_workflow_sha"`
	RunnerEnvironment    string `json:"runner_environment"`
}

type workflowFile struct {
	repo, path, ref, sha string
}

func (w workflowFile) String() string {
	return w.repo + "/" + w.path + "@" + w.ref
}

func OIDCClaimsSupported() []string {
	claims := []string{"exp", "iat", "iss", "jti", "nbf", "sub"}
	for field := range reflect.TypeFor[oidcClaims]().Fields() {
		if name, _, _ := strings.Cut(field.Tag.Get("json"), ","); name != "" {
			claims = append(claims, name)
		}
	}
	return claims
}

func OIDCIssuer() string {
	return setting.AppURL + "api/actions/oidc"
}

func taskAllowsOIDCToken(ctx context.Context, task *actions_model.ActionTask) (bool, error) {
	if task.Job.TokenPermissions == nil || !task.Job.TokenPermissions.IDToken {
		return false, nil
	}
	task.Job.Repo = task.Job.Run.Repo
	perms, err := actions_model.ComputeTaskTokenPermissions(ctx, task, task.Job.Repo)
	return perms.IDToken, err
}

func CreateOIDCToken(ctx context.Context, requestToken, audience string) (string, error) {
	taskID, err := taskIDFromToken(requestToken, true)
	if err != nil {
		return "", util.NewPermissionDeniedErrorf("invalid ID token request token")
	}
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if errors.Is(err, util.ErrNotExist) {
		return "", util.NewPermissionDeniedErrorf("task %d does not exist", taskID)
	} else if err != nil {
		return "", err
	}
	if task.Status != actions_model.StatusRunning && task.Status != actions_model.StatusCancelling { // cleanup steps of cancelled jobs may still need one
		return "", util.NewPermissionDeniedErrorf("task %d is not active", taskID)
	}
	if err := task.LoadJob(ctx); err != nil {
		return "", err
	}
	if err := task.Job.LoadAttributes(ctx); err != nil {
		return "", err
	}
	if allowed, err := taskAllowsOIDCToken(ctx, task); err != nil {
		return "", err
	} else if !allowed {
		return "", util.NewPermissionDeniedErrorf("task %d lacks the id-token permission", taskID)
	}

	claims, err := createOIDCClaims(ctx, task.Job, audience, time.Now())
	if err != nil {
		return "", err
	}
	key, err := OIDCSigningKey()
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(key.SigningMethod(), claims)
	key.PreProcessToken(token)
	return token.SignedString(key.SignKey())
}

func createOIDCClaims(ctx context.Context, job *actions_model.ActionRunJob, audience string, now time.Time) (*oidcClaims, error) {
	run := job.Run
	gitCtx := GenerateGiteaContext(ctx, run, nil, job)
	contextString := func(key string) string {
		value, _ := gitCtx[key].(string)
		return value
	}
	ref := contextString("ref")

	rootWorkflow, workflowName, err := runWorkflowFile(ctx, run, ref)
	if err != nil {
		return nil, err
	}
	jobWorkflow, err := jobWorkflowFile(ctx, job, rootWorkflow)
	if err != nil {
		return nil, err
	}

	// GitHub's immutable format, the IDs keep a recreated owner or repo name from matching
	subject := fmt.Sprintf("repo:%s@%d/%s@%d", run.Repo.OwnerName, run.Repo.OwnerID, run.Repo.Name, run.RepoID)
	if run.TriggerEvent == actions_module.GithubEventPullRequest {
		subject += ":pull_request"
	} else {
		subject += ":ref:" + ref
	}
	if audience == "" {
		audience = setting.AppURL + url.PathEscape(run.Repo.OwnerName)
	}
	visibility := "internal"
	if run.Repo.IsPrivate {
		visibility = "private"
	} else if run.Repo.Owner.Visibility.IsPublic() {
		visibility = "public"
	}

	return &oidcClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    OIDCIssuer(),
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Minute)), // backdated like GitHub for relying parties with skewed clocks
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		Audience:             audience,
		Actor:                contextString("actor"),
		ActorID:              contextString("actor_id"),
		Repository:           contextString("repository"),
		RepositoryID:         contextString("repository_id"),
		RepositoryOwner:      contextString("repository_owner"),
		RepositoryOwnerID:    contextString("repository_owner_id"),
		RepositoryVisibility: visibility,
		RunID:                contextString("run_id"),
		RunNumber:            contextString("run_number"),
		RunAttempt:           contextString("run_attempt"),
		EventName:            contextString("event_name"),
		Ref:                  ref,
		RefType:              contextString("ref_type"),
		RefProtected:         strconv.FormatBool(gitCtx["ref_protected"] == true),
		SHA:                  contextString("sha"),
		BaseRef:              contextString("base_ref"),
		HeadRef:              contextString("head_ref"),
		Workflow:             workflowName,
		WorkflowRef:          rootWorkflow.String(),
		WorkflowSHA:          rootWorkflow.sha,
		JobWorkflowRef:       jobWorkflow.String(),
		JobWorkflowSHA:       jobWorkflow.sha,
		RunnerEnvironment:    "self-hosted",
	}, nil
}

// runWorkflowFile repeats the directory choice workflow detection made at the recorded commit, and names the workflow like GitHub
func runWorkflowFile(ctx context.Context, run *actions_model.ActionRun, ref string) (file workflowFile, name string, err error) {
	repo, err := repo_model.GetRepositoryByID(ctx, run.WorkflowRepoID)
	if err != nil {
		return file, "", err
	}
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return file, "", err
	}
	defer gitRepo.Close()
	commit, err := gitRepo.GetCommit(ctx, run.WorkflowCommitSHA)
	if err != nil {
		return file, "", err
	}

	listWorkflows := actions_module.ListWorkflows
	if run.IsScopedRun {
		listWorkflows = actions_module.ListScopedWorkflows
		ref = run.WorkflowCommitSHA // the source repo's branch is not recorded
	}
	dir, _, err := listWorkflows(ctx, gitRepo, commit)
	if err != nil {
		return file, "", err
	}
	file = workflowFile{repo: repo.FullName(), path: path.Join(dir, run.WorkflowID), ref: ref, sha: run.WorkflowCommitSHA}
	content, err := commit.GetFileContent(ctx, gitRepo, file.path, 1024*1024)
	if err != nil {
		return file, "", err
	}
	if workflow, err := model.ReadWorkflow(strings.NewReader(content)); err == nil && strings.TrimSpace(workflow.Name) != "" {
		return file, workflow.Name, nil
	}
	return file, file.path, nil
}

// jobWorkflowFile lets local reusable workflow calls inherit their caller's ref, like on GitHub
func jobWorkflowFile(ctx context.Context, job *actions_model.ActionRunJob, root workflowFile) (workflowFile, error) {
	if job.ParentJobID == 0 {
		return root, nil
	}
	caller, err := actions_model.GetRunJobByRunAndID(ctx, job.RunID, job.ParentJobID)
	if err != nil {
		return workflowFile{}, err
	}
	uses, err := ResolveUses(ctx, caller.CallUses)
	if err != nil {
		return workflowFile{}, err
	}
	if uses.IsLocal() {
		file, err := jobWorkflowFile(ctx, caller, root)
		file.path, file.sha = uses.Path, job.WorkflowSourceCommitSHA
		return file, err
	}
	repo, err := repo_model.GetRepositoryByID(ctx, job.WorkflowSourceRepoID)
	if err != nil {
		return workflowFile{}, err
	}
	ref := cmp.Or(caller.ReusableWorkflowRef, job.WorkflowSourceCommitSHA) // callers expanded before the ref was recorded
	return workflowFile{repo: repo.FullName(), path: uses.Path, ref: ref, sha: job.WorkflowSourceCommitSHA}, nil
}
