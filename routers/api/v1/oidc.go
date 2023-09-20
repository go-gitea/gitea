// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// OIDC provider for Gitea Actions
package v1

import (
	"fmt"
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"

	"github.com/golang-jwt/jwt/v5"
)

type IDTokenResponse struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type IDTokenErrorResponse struct {
	ErrorDescription string `json:"error_description"`
}

type IDToken struct {
	jwt.RegisteredClaims

	Ref                  string `json:"ref,omitempty"`
	SHA                  string `json:"sha,omitempty"`
	Repository           string `json:"repository,omitempty"`
	RepositoryOwner      string `json:"repository_owner,omitempty"`
	RepositoryOwnerID    int    `json:"repository_owner_id,omitempty"`
	RunID                int    `json:"run_id,omitempty"`
	RunNumber            int    `json:"run_number,omitempty"`
	RunAttempt           int    `json:"run_attempt,omitempty"`
	RepositoryVisibility string `json:"repository_visibility,omitempty"`
	RepositoryID         int    `json:"repository_id,omitempty"`
	ActorID              int    `json:"actor_id,omitempty"`
	Actor                string `json:"actor,omitempty"`
	Workflow             string `json:"workflow,omitempty"`
	EventName            string `json:"event_name,omitempty"`
	RefType              string `json:"ref_type,omitempty"`
	HeadRef              string `json:"head_ref,omitempty"`
	BaseRef              string `json:"base_ref,omitempty"`

	// Github's OIDC tokens have all of these, but I wasn't sure how
	// to populate them. Leaving them here to make future work easier.

	/*
		WorkflowRef       string `json:"workflow_ref,omitempty"`
		WorkflowSHA       string `json:"workflow_sha,omitempty"`
		JobWorkflowRef    string `json:"job_workflow_ref,omitempty"`
		JobWorkflowSHA    string `json:"job_workflow_sha,omitempty"`
		RunnerEnvironment string `json:"runner_environment,omitempty"`
	*/
}

func generateOIDCToken(ctx *context.APIContext) {
	if ctx.Doer == nil || ctx.Data["AuthedMethod"] != (&auth_service.OAuth2{}).Name() || ctx.Data["IsActionsToken"] != true {
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	task := ctx.Data["ActionsTask"].(*actions_model.ActionTask)
	if err := task.LoadJob(ctx); err != nil {
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	if mayCreateToken := task.Job.MayCreateIDToken(); !mayCreateToken {
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	if err := task.Job.LoadAttributes(ctx); err != nil {
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	if err := task.Job.Run.LoadAttributes(ctx); err != nil {
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	if err := task.Job.Run.Repo.LoadAttributes(ctx); err != nil {
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	eventName := task.Job.Run.EventName()
	ref, sha, baseRef, headRef := task.Job.Run.RefShaBaseRefAndHeadRef()

	jwtAudience := jwt.ClaimStrings{task.Job.Run.Repo.Owner.HTMLURL()}
	requestedAudience := ctx.Req.URL.Query().Get("audience")
	if requestedAudience != "" {
		jwtAudience = append(jwtAudience, requestedAudience)
	}

	// generate OIDC token
	issueTime := timeutil.TimeStampNow()
	expirationTime := timeutil.TimeStampNow().Add(15 * 60)
	notBeforeTime := timeutil.TimeStampNow().Add(-15 * 60)
	idToken := &IDToken{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    setting.AppURL,
			Audience:  jwtAudience,
			ExpiresAt: jwt.NewNumericDate(expirationTime.AsTime()),
			NotBefore: jwt.NewNumericDate(notBeforeTime.AsTime()),
			IssuedAt:  jwt.NewNumericDate(issueTime.AsTime()),
			Subject:   fmt.Sprintf("repo:%s:ref:%s", task.Job.Run.Repo.FullName(), ref),
		},
		Ref:               ref,
		SHA:               sha,
		Repository:        task.Job.Run.Repo.FullName(),
		RepositoryOwner:   task.Job.Run.Repo.OwnerName,
		RepositoryOwnerID: int(task.Job.Run.Repo.OwnerID),
		RunID:             int(task.Job.RunID),
		RunNumber:         int(task.Job.Run.Index),
		RunAttempt:        int(task.Job.Attempt),
		RepositoryID:      int(task.Job.Run.RepoID),
		ActorID:           int(task.Job.Run.TriggerUserID),
		Actor:             task.Job.Run.TriggerUser.Name,
		Workflow:          task.Job.Run.WorkflowID,
		EventName:         eventName,
		RefType:           git.RefName(task.Job.Run.Ref).RefType(),
		BaseRef:           baseRef,
		HeadRef:           headRef,
	}

	if task.Job.Run.Repo.IsPrivate {
		idToken.RepositoryVisibility = "private"
	} else {
		idToken.RepositoryVisibility = "public"
	}

	signedIDToken, err := oauth2.SignToken(idToken, oauth2.DefaultSigningKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, &IDTokenErrorResponse{
			ErrorDescription: "unable to sign token",
		})
		return
	}

	ctx.JSON(http.StatusOK, IDTokenResponse{
		Value: signedIDToken,
		Count: len(signedIDToken),
	})
}
