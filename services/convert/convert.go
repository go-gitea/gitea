// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/gitdiff"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/nektos/act/pkg/model"
)

// ToEmail convert models.EmailAddress to api.Email
func ToEmail(email *user_model.EmailAddress) *api.Email {
	return &api.Email{
		Email:    email.Email,
		Verified: email.IsActivated,
		Primary:  email.IsPrimary,
	}
}

// ToEmail convert models.EmailAddress to api.Email
func ToEmailSearch(email *user_model.SearchEmailResult) *api.Email {
	return &api.Email{
		Email:    email.Email,
		Verified: email.IsActivated,
		Primary:  email.IsPrimary,
		UserID:   email.UID,
		UserName: email.Name,
	}
}

// ToBranch convert a git.Commit and git.Branch to an api.Branch
func ToBranch(ctx context.Context, repo *repo_model.Repository, branchName string, c *git.Commit, bp *git_model.ProtectedBranch, user *user_model.User, isRepoAdmin bool) (*api.Branch, error) {
	if bp == nil {
		var hasPerm bool
		var canPush bool
		var err error
		if user != nil {
			hasPerm, err = access_model.HasAccessUnit(ctx, user, repo, unit.TypeCode, perm.AccessModeWrite)
			if err != nil {
				return nil, err
			}

			perms, err := access_model.GetUserRepoPermission(ctx, repo, user)
			if err != nil {
				return nil, err
			}
			canPush = issues_model.CanMaintainerWriteToBranch(ctx, perms, branchName, user)
		}

		return &api.Branch{
			Name:                branchName,
			Commit:              ToPayloadCommit(ctx, repo, c),
			Protected:           false,
			RequiredApprovals:   0,
			EnableStatusCheck:   false,
			StatusCheckContexts: []string{},
			UserCanPush:         canPush,
			UserCanMerge:        hasPerm,
		}, nil
	}

	branch := &api.Branch{
		Name:                branchName,
		Commit:              ToPayloadCommit(ctx, repo, c),
		Protected:           true,
		RequiredApprovals:   bp.RequiredApprovals,
		EnableStatusCheck:   bp.EnableStatusCheck,
		StatusCheckContexts: bp.StatusCheckContexts,
	}

	if isRepoAdmin {
		branch.EffectiveBranchProtectionName = bp.RuleName
	}

	if user != nil {
		permission, err := access_model.GetUserRepoPermission(ctx, repo, user)
		if err != nil {
			return nil, err
		}
		bp.Repo = repo
		branch.UserCanPush = bp.CanUserPush(ctx, user)
		branch.UserCanMerge = git_model.IsUserMergeWhitelisted(ctx, bp, user.ID, permission)
	}

	return branch, nil
}

// getWhitelistEntities returns the names of the entities that are in the whitelist
func getWhitelistEntities[T *user_model.User | *organization.Team](entities []T, whitelistIDs []int64) []string {
	whitelistUserIDsSet := container.SetOf(whitelistIDs...)
	whitelistNames := make([]string, 0)
	for _, entity := range entities {
		switch v := any(entity).(type) {
		case *user_model.User:
			if whitelistUserIDsSet.Contains(v.ID) {
				whitelistNames = append(whitelistNames, v.Name)
			}
		case *organization.Team:
			if whitelistUserIDsSet.Contains(v.ID) {
				whitelistNames = append(whitelistNames, v.Name)
			}
		}
	}

	return whitelistNames
}

// ToBranchProtection convert a ProtectedBranch to api.BranchProtection
func ToBranchProtection(ctx context.Context, bp *git_model.ProtectedBranch, repo *repo_model.Repository) *api.BranchProtection {
	readers, err := access_model.GetRepoReaders(ctx, repo)
	if err != nil {
		log.Error("GetRepoReaders: %v", err)
	}

	pushWhitelistUsernames := getWhitelistEntities(readers, bp.WhitelistUserIDs)
	forcePushAllowlistUsernames := getWhitelistEntities(readers, bp.ForcePushAllowlistUserIDs)
	mergeWhitelistUsernames := getWhitelistEntities(readers, bp.MergeWhitelistUserIDs)
	approvalsWhitelistUsernames := getWhitelistEntities(readers, bp.ApprovalsWhitelistUserIDs)

	teamReaders, err := organization.OrgFromUser(repo.Owner).TeamsWithAccessToRepo(ctx, repo.ID, perm.AccessModeRead)
	if err != nil {
		log.Error("Repo.Owner.TeamsWithAccessToRepo: %v", err)
	}

	pushWhitelistTeams := getWhitelistEntities(teamReaders, bp.WhitelistTeamIDs)
	forcePushAllowlistTeams := getWhitelistEntities(teamReaders, bp.ForcePushAllowlistTeamIDs)
	mergeWhitelistTeams := getWhitelistEntities(teamReaders, bp.MergeWhitelistTeamIDs)
	approvalsWhitelistTeams := getWhitelistEntities(teamReaders, bp.ApprovalsWhitelistTeamIDs)

	branchName := ""
	if !git_model.IsRuleNameSpecial(bp.RuleName) {
		branchName = bp.RuleName
	}

	return &api.BranchProtection{
		BranchName:                    branchName,
		RuleName:                      bp.RuleName,
		Priority:                      bp.Priority,
		EnablePush:                    bp.CanPush,
		EnablePushWhitelist:           bp.EnableWhitelist,
		PushWhitelistUsernames:        pushWhitelistUsernames,
		PushWhitelistTeams:            pushWhitelistTeams,
		PushWhitelistDeployKeys:       bp.WhitelistDeployKeys,
		EnableForcePush:               bp.CanForcePush,
		EnableForcePushAllowlist:      bp.EnableForcePushAllowlist,
		ForcePushAllowlistUsernames:   forcePushAllowlistUsernames,
		ForcePushAllowlistTeams:       forcePushAllowlistTeams,
		ForcePushAllowlistDeployKeys:  bp.ForcePushAllowlistDeployKeys,
		EnableMergeWhitelist:          bp.EnableMergeWhitelist,
		MergeWhitelistUsernames:       mergeWhitelistUsernames,
		MergeWhitelistTeams:           mergeWhitelistTeams,
		EnableStatusCheck:             bp.EnableStatusCheck,
		StatusCheckContexts:           bp.StatusCheckContexts,
		RequiredApprovals:             bp.RequiredApprovals,
		EnableApprovalsWhitelist:      bp.EnableApprovalsWhitelist,
		ApprovalsWhitelistUsernames:   approvalsWhitelistUsernames,
		ApprovalsWhitelistTeams:       approvalsWhitelistTeams,
		BlockOnRejectedReviews:        bp.BlockOnRejectedReviews,
		BlockOnOfficialReviewRequests: bp.BlockOnOfficialReviewRequests,
		BlockOnOutdatedBranch:         bp.BlockOnOutdatedBranch,
		DismissStaleApprovals:         bp.DismissStaleApprovals,
		IgnoreStaleApprovals:          bp.IgnoreStaleApprovals,
		RequireSignedCommits:          bp.RequireSignedCommits,
		ProtectedFilePatterns:         bp.ProtectedFilePatterns,
		UnprotectedFilePatterns:       bp.UnprotectedFilePatterns,
		BlockAdminMergeOverride:       bp.BlockAdminMergeOverride,
		Created:                       bp.CreatedUnix.AsTime(),
		Updated:                       bp.UpdatedUnix.AsTime(),
	}
}

// ToTag convert a git.Tag to an api.Tag
func ToTag(repo *repo_model.Repository, t *git.Tag) *api.Tag {
	tarballURL := util.URLJoin(repo.HTMLURL(), "archive", t.Name+".tar.gz")
	zipballURL := util.URLJoin(repo.HTMLURL(), "archive", t.Name+".zip")

	// Archive URLs are "" if the download feature is disabled
	if setting.Repository.DisableDownloadSourceArchives {
		tarballURL = ""
		zipballURL = ""
	}

	return &api.Tag{
		Name:       t.Name,
		Message:    strings.TrimSpace(t.Message),
		ID:         t.ID.String(),
		Commit:     ToCommitMeta(repo, t),
		ZipballURL: zipballURL,
		TarballURL: tarballURL,
	}
}

// ToActionTask convert a actions_model.ActionTask to an api.ActionTask
func ToActionTask(ctx context.Context, t *actions_model.ActionTask) (*api.ActionTask, error) {
	if err := t.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	url := strings.TrimSuffix(setting.AppURL, "/") + t.GetRunLink()

	return &api.ActionTask{
		ID:           t.ID,
		Name:         t.Job.Name,
		HeadBranch:   t.Job.Run.PrettyRef(),
		HeadSHA:      t.Job.CommitSHA,
		RunNumber:    t.Job.Run.Index,
		Event:        t.Job.Run.TriggerEvent,
		DisplayTitle: t.Job.Run.Title,
		Status:       t.Status.String(),
		WorkflowID:   t.Job.Run.WorkflowID,
		URL:          url,
		CreatedAt:    t.Created.AsLocalTime(),
		UpdatedAt:    t.Updated.AsLocalTime(),
		RunStartedAt: t.Started.AsLocalTime(),
	}, nil
}

func ToActionWorkflowRun(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun) (*api.ActionWorkflowRun, error) {
	err := run.LoadAttributes(ctx)
	if err != nil {
		return nil, err
	}
	status, conclusion := ToActionsStatus(run.Status)
	return &api.ActionWorkflowRun{
		ID:           run.ID,
		URL:          fmt.Sprintf("%s/actions/runs/%d", repo.APIURL(), run.ID),
		HTMLURL:      run.HTMLURL(),
		RunNumber:    run.Index,
		StartedAt:    run.Started.AsLocalTime(),
		CompletedAt:  run.Stopped.AsLocalTime(),
		Event:        string(run.Event),
		DisplayTitle: run.Title,
		HeadBranch:   git.RefName(run.Ref).BranchName(),
		HeadSha:      run.CommitSHA,
		Status:       status,
		Conclusion:   conclusion,
		Path:         fmt.Sprintf("%s@%s", run.WorkflowID, run.Ref),
		Repository:   ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeNone}),
		TriggerActor: ToUser(ctx, run.TriggerUser, nil),
		// We do not have a way to get a different User for the actor than the trigger user
		Actor: ToUser(ctx, run.TriggerUser, nil),
	}, nil
}

func ToWorkflowRunAction(status actions_model.Status) string {
	var action string
	switch status {
	case actions_model.StatusWaiting, actions_model.StatusBlocked:
		action = "requested"
	case actions_model.StatusRunning:
		action = "in_progress"
	}
	if status.IsDone() {
		action = "completed"
	}
	return action
}

func ToActionsStatus(status actions_model.Status) (string, string) {
	var action string
	var conclusion string
	switch status {
	// This is a naming conflict of the webhook between Gitea and GitHub Actions
	case actions_model.StatusWaiting:
		action = "queued"
	case actions_model.StatusBlocked:
		action = "waiting"
	case actions_model.StatusRunning:
		action = "in_progress"
	}
	if status.IsDone() {
		action = "completed"
		switch status {
		case actions_model.StatusSuccess:
			conclusion = "success"
		case actions_model.StatusCancelled:
			conclusion = "cancelled"
		case actions_model.StatusFailure:
			conclusion = "failure"
		case actions_model.StatusSkipped:
			conclusion = "skipped"
		}
	}
	return action, conclusion
}

// ToActionWorkflowJob convert a actions_model.ActionRunJob to an api.ActionWorkflowJob
// task is optional and can be nil
func ToActionWorkflowJob(ctx context.Context, repo *repo_model.Repository, task *actions_model.ActionTask, job *actions_model.ActionRunJob) (*api.ActionWorkflowJob, error) {
	err := job.LoadAttributes(ctx)
	if err != nil {
		return nil, err
	}

	jobIndex := 0
	jobs, err := actions_model.GetRunJobsByRunID(ctx, job.RunID)
	if err != nil {
		return nil, err
	}
	for i, j := range jobs {
		if j.ID == job.ID {
			jobIndex = i
			break
		}
	}

	status, conclusion := ToActionsStatus(job.Status)
	var runnerID int64
	var runnerName string
	var steps []*api.ActionWorkflowStep

	if job.TaskID != 0 {
		if task == nil {
			task, _, err = db.GetByID[actions_model.ActionTask](ctx, job.TaskID)
			if err != nil {
				return nil, err
			}
		}

		runnerID = task.RunnerID
		if runner, ok, _ := db.GetByID[actions_model.ActionRunner](ctx, runnerID); ok {
			runnerName = runner.Name
		}
		for i, step := range task.Steps {
			stepStatus, stepConclusion := ToActionsStatus(job.Status)
			steps = append(steps, &api.ActionWorkflowStep{
				Name:        step.Name,
				Number:      int64(i),
				Status:      stepStatus,
				Conclusion:  stepConclusion,
				StartedAt:   step.Started.AsTime().UTC(),
				CompletedAt: step.Stopped.AsTime().UTC(),
			})
		}
	}

	return &api.ActionWorkflowJob{
		ID: job.ID,
		// missing api endpoint for this location
		URL:     fmt.Sprintf("%s/actions/jobs/%d", repo.APIURL(), job.ID),
		HTMLURL: fmt.Sprintf("%s/jobs/%d", job.Run.HTMLURL(), jobIndex),
		RunID:   job.RunID,
		// Missing api endpoint for this location, artifacts are available under a nested url
		RunURL:      fmt.Sprintf("%s/actions/runs/%d", repo.APIURL(), job.RunID),
		Name:        job.Name,
		Labels:      job.RunsOn,
		RunAttempt:  job.Attempt,
		HeadSha:     job.Run.CommitSHA,
		HeadBranch:  git.RefName(job.Run.Ref).BranchName(),
		Status:      status,
		Conclusion:  conclusion,
		RunnerID:    runnerID,
		RunnerName:  runnerName,
		Steps:       steps,
		CreatedAt:   job.Created.AsTime().UTC(),
		StartedAt:   job.Started.AsTime().UTC(),
		CompletedAt: job.Stopped.AsTime().UTC(),
	}, nil
}

func getActionWorkflowEntry(ctx context.Context, repo *repo_model.Repository, commit *git.Commit, folder string, entry *git.TreeEntry) *api.ActionWorkflow {
	cfgUnit := repo.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	defaultBranch, _ := commit.GetBranchName()

	workflowURL := fmt.Sprintf("%s/actions/workflows/%s", repo.APIURL(), util.PathEscapeSegments(entry.Name()))
	workflowRepoURL := fmt.Sprintf("%s/src/branch/%s/%s/%s", repo.HTMLURL(ctx), util.PathEscapeSegments(defaultBranch), util.PathEscapeSegments(folder), util.PathEscapeSegments(entry.Name()))
	badgeURL := fmt.Sprintf("%s/actions/workflows/%s/badge.svg?branch=%s", repo.HTMLURL(ctx), util.PathEscapeSegments(entry.Name()), url.QueryEscape(repo.DefaultBranch))

	// See https://docs.github.com/en/rest/actions/workflows?apiVersion=2022-11-28#get-a-workflow
	// State types:
	// - active
	// - deleted
	// - disabled_fork
	// - disabled_inactivity
	// - disabled_manually
	state := "active"
	if cfg.IsWorkflowDisabled(entry.Name()) {
		state = "disabled_manually"
	}

	// The CreatedAt and UpdatedAt fields currently reflect the timestamp of the latest commit, which can later be refined
	// by retrieving the first and last commits for the file history. The first commit would indicate the creation date,
	// while the last commit would represent the modification date. The DeletedAt could be determined by identifying
	// the last commit where the file existed. However, this implementation has not been done here yet, as it would likely
	// cause a significant performance degradation.
	createdAt := commit.Author.When
	updatedAt := commit.Author.When

	content, err := actions.GetContentFromEntry(entry)
	name := entry.Name()
	if err == nil {
		workflow, err := model.ReadWorkflow(bytes.NewReader(content))
		if err == nil {
			// Only use the name when specified in the workflow file
			if workflow.Name != "" {
				name = workflow.Name
			}
		} else {
			log.Error("getActionWorkflowEntry: Failed to parse workflow: %v", err)
		}
	} else {
		log.Error("getActionWorkflowEntry: Failed to get content from entry: %v", err)
	}

	return &api.ActionWorkflow{
		ID:        entry.Name(),
		Name:      name,
		Path:      path.Join(folder, entry.Name()),
		State:     state,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		URL:       workflowURL,
		HTMLURL:   workflowRepoURL,
		BadgeURL:  badgeURL,
	}
}

func ListActionWorkflows(ctx context.Context, gitrepo *git.Repository, repo *repo_model.Repository) ([]*api.ActionWorkflow, error) {
	defaultBranchCommit, err := gitrepo.GetBranchCommit(repo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	folder, entries, err := actions.ListWorkflows(defaultBranchCommit)
	if err != nil {
		return nil, err
	}

	workflows := make([]*api.ActionWorkflow, len(entries))
	for i, entry := range entries {
		workflows[i] = getActionWorkflowEntry(ctx, repo, defaultBranchCommit, folder, entry)
	}

	return workflows, nil
}

func GetActionWorkflow(ctx context.Context, gitrepo *git.Repository, repo *repo_model.Repository, workflowID string) (*api.ActionWorkflow, error) {
	entries, err := ListActionWorkflows(ctx, gitrepo, repo)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.ID == workflowID {
			return entry, nil
		}
	}

	return nil, util.NewNotExistErrorf("workflow %q not found", workflowID)
}

// ToActionArtifact convert a actions_model.ActionArtifact to an api.ActionArtifact
func ToActionArtifact(repo *repo_model.Repository, art *actions_model.ActionArtifact) (*api.ActionArtifact, error) {
	url := fmt.Sprintf("%s/actions/artifacts/%d", repo.APIURL(), art.ID)

	return &api.ActionArtifact{
		ID:                 art.ID,
		Name:               art.ArtifactName,
		SizeInBytes:        art.FileSize,
		Expired:            art.Status == actions_model.ArtifactStatusExpired,
		URL:                url,
		ArchiveDownloadURL: url + "/zip",
		CreatedAt:          art.CreatedUnix.AsLocalTime(),
		UpdatedAt:          art.UpdatedUnix.AsLocalTime(),
		ExpiresAt:          art.ExpiredUnix.AsLocalTime(),
		WorkflowRun: &api.ActionWorkflowRun{
			ID:           art.RunID,
			RepositoryID: art.RepoID,
			HeadSha:      art.CommitSHA,
		},
	}, nil
}

func ToActionRunner(ctx context.Context, runner *actions_model.ActionRunner) *api.ActionRunner {
	status := runner.Status()
	apiStatus := "offline"
	if runner.IsOnline() {
		apiStatus = "online"
	}
	labels := make([]*api.ActionRunnerLabel, len(runner.AgentLabels))
	for i, label := range runner.AgentLabels {
		labels[i] = &api.ActionRunnerLabel{
			ID:   int64(i),
			Name: label,
			Type: "custom",
		}
	}
	return &api.ActionRunner{
		ID:        runner.ID,
		Name:      runner.Name,
		Status:    apiStatus,
		Busy:      status == runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE,
		Ephemeral: runner.Ephemeral,
		Labels:    labels,
	}
}

// ToVerification convert a git.Commit.Signature to an api.PayloadCommitVerification
func ToVerification(ctx context.Context, c *git.Commit) *api.PayloadCommitVerification {
	verif := asymkey_service.ParseCommitWithSignature(ctx, c)
	commitVerification := &api.PayloadCommitVerification{
		Verified: verif.Verified,
		Reason:   verif.Reason,
	}
	if c.Signature != nil {
		commitVerification.Signature = c.Signature.Signature
		commitVerification.Payload = c.Signature.Payload
	}
	if verif.SigningUser != nil {
		commitVerification.Signer = &api.PayloadUser{
			Name:  verif.SigningUser.Name,
			Email: verif.SigningUser.Email,
		}
	}
	return commitVerification
}

// ToPublicKey convert asymkey_model.PublicKey to api.PublicKey
func ToPublicKey(apiLink string, key *asymkey_model.PublicKey) *api.PublicKey {
	return &api.PublicKey{
		ID:          key.ID,
		Key:         key.Content,
		URL:         fmt.Sprintf("%s%d", apiLink, key.ID),
		Title:       key.Name,
		Fingerprint: key.Fingerprint,
		Created:     key.CreatedUnix.AsTime(),
		Updated:     key.UpdatedUnix.AsTime(),
	}
}

// ToGPGKey converts models.GPGKey to api.GPGKey
func ToGPGKey(key *asymkey_model.GPGKey) *api.GPGKey {
	subkeys := make([]*api.GPGKey, len(key.SubsKey))
	for id, k := range key.SubsKey {
		subkeys[id] = &api.GPGKey{
			ID:                k.ID,
			PrimaryKeyID:      k.PrimaryKeyID,
			KeyID:             k.KeyID,
			PublicKey:         k.Content,
			Created:           k.CreatedUnix.AsTime(),
			Expires:           k.ExpiredUnix.AsTime(),
			CanSign:           k.CanSign,
			CanEncryptComms:   k.CanEncryptComms,
			CanEncryptStorage: k.CanEncryptStorage,
			CanCertify:        k.CanSign,
			Verified:          k.Verified,
		}
	}
	emails := make([]*api.GPGKeyEmail, len(key.Emails))
	for i, e := range key.Emails {
		emails[i] = ToGPGKeyEmail(e)
	}
	return &api.GPGKey{
		ID:                key.ID,
		PrimaryKeyID:      key.PrimaryKeyID,
		KeyID:             key.KeyID,
		PublicKey:         key.Content,
		Created:           key.CreatedUnix.AsTime(),
		Expires:           key.ExpiredUnix.AsTime(),
		Emails:            emails,
		SubsKey:           subkeys,
		CanSign:           key.CanSign,
		CanEncryptComms:   key.CanEncryptComms,
		CanEncryptStorage: key.CanEncryptStorage,
		CanCertify:        key.CanSign,
		Verified:          key.Verified,
	}
}

// ToGPGKeyEmail convert models.EmailAddress to api.GPGKeyEmail
func ToGPGKeyEmail(email *user_model.EmailAddress) *api.GPGKeyEmail {
	return &api.GPGKeyEmail{
		Email:    email.Email,
		Verified: email.IsActivated,
	}
}

// ToGitHook convert git.Hook to api.GitHook
func ToGitHook(h *git.Hook) *api.GitHook {
	return &api.GitHook{
		Name:     h.Name(),
		IsActive: h.IsActive,
		Content:  h.Content,
	}
}

// ToDeployKey convert asymkey_model.DeployKey to api.DeployKey
func ToDeployKey(apiLink string, key *asymkey_model.DeployKey) *api.DeployKey {
	return &api.DeployKey{
		ID:          key.ID,
		KeyID:       key.KeyID,
		Key:         key.Content,
		Fingerprint: key.Fingerprint,
		URL:         fmt.Sprintf("%s%d", apiLink, key.ID),
		Title:       key.Name,
		Created:     key.CreatedUnix.AsTime(),
		ReadOnly:    key.Mode == perm.AccessModeRead, // All deploy keys are read-only.
	}
}

// ToOrganization convert user_model.User to api.Organization
func ToOrganization(ctx context.Context, org *organization.Organization) *api.Organization {
	return &api.Organization{
		ID:                        org.ID,
		AvatarURL:                 org.AsUser().AvatarLink(ctx),
		Name:                      org.Name,
		UserName:                  org.Name,
		FullName:                  org.FullName,
		Email:                     org.Email,
		Description:               org.Description,
		Website:                   org.Website,
		Location:                  org.Location,
		Visibility:                org.Visibility.String(),
		RepoAdminChangeTeamAccess: org.RepoAdminChangeTeamAccess,
	}
}

// ToTeam convert models.Team to api.Team
func ToTeam(ctx context.Context, team *organization.Team, loadOrg ...bool) (*api.Team, error) {
	teams, err := ToTeams(ctx, []*organization.Team{team}, len(loadOrg) != 0 && loadOrg[0])
	if err != nil || len(teams) == 0 {
		return nil, err
	}
	return teams[0], nil
}

// ToTeams convert models.Team list to api.Team list
func ToTeams(ctx context.Context, teams []*organization.Team, loadOrgs bool) ([]*api.Team, error) {
	cache := make(map[int64]*api.Organization)
	apiTeams := make([]*api.Team, 0, len(teams))
	for _, t := range teams {
		if err := t.LoadUnits(ctx); err != nil {
			return nil, err
		}

		apiTeam := &api.Team{
			ID:                      t.ID,
			Name:                    t.Name,
			Description:             t.Description,
			IncludesAllRepositories: t.IncludesAllRepositories,
			CanCreateOrgRepo:        t.CanCreateOrgRepo,
			Permission:              t.AccessMode.ToString(),
			Units:                   t.GetUnitNames(),
			UnitsMap:                t.GetUnitsMap(),
		}

		if loadOrgs {
			apiOrg, ok := cache[t.OrgID]
			if !ok {
				org, err := organization.GetOrgByID(ctx, t.OrgID)
				if err != nil {
					return nil, err
				}
				apiOrg = ToOrganization(ctx, org)
				cache[t.OrgID] = apiOrg
			}
			apiTeam.Organization = apiOrg
		}

		apiTeams = append(apiTeams, apiTeam)
	}
	return apiTeams, nil
}

// ToAnnotatedTag convert git.Tag to api.AnnotatedTag
func ToAnnotatedTag(ctx context.Context, repo *repo_model.Repository, t *git.Tag, c *git.Commit) *api.AnnotatedTag {
	return &api.AnnotatedTag{
		Tag:          t.Name,
		SHA:          t.ID.String(),
		Object:       ToAnnotatedTagObject(repo, c),
		Message:      t.Message,
		URL:          util.URLJoin(repo.APIURL(), "git/tags", t.ID.String()),
		Tagger:       ToCommitUser(t.Tagger),
		Verification: ToVerification(ctx, c),
	}
}

// ToAnnotatedTagObject convert a git.Commit to an api.AnnotatedTagObject
func ToAnnotatedTagObject(repo *repo_model.Repository, commit *git.Commit) *api.AnnotatedTagObject {
	return &api.AnnotatedTagObject{
		SHA:  commit.ID.String(),
		Type: string(git.ObjectCommit),
		URL:  util.URLJoin(repo.APIURL(), "git/commits", commit.ID.String()),
	}
}

// ToTagProtection convert a git.ProtectedTag to an api.TagProtection
func ToTagProtection(ctx context.Context, pt *git_model.ProtectedTag, repo *repo_model.Repository) *api.TagProtection {
	readers, err := access_model.GetRepoReaders(ctx, repo)
	if err != nil {
		log.Error("GetRepoReaders: %v", err)
	}

	whitelistUsernames := getWhitelistEntities(readers, pt.AllowlistUserIDs)

	teamReaders, err := organization.OrgFromUser(repo.Owner).TeamsWithAccessToRepo(ctx, repo.ID, perm.AccessModeRead)
	if err != nil {
		log.Error("Repo.Owner.TeamsWithAccessToRepo: %v", err)
	}

	whitelistTeams := getWhitelistEntities(teamReaders, pt.AllowlistTeamIDs)

	return &api.TagProtection{
		ID:                 pt.ID,
		NamePattern:        pt.NamePattern,
		WhitelistUsernames: whitelistUsernames,
		WhitelistTeams:     whitelistTeams,
		Created:            pt.CreatedUnix.AsTime(),
		Updated:            pt.UpdatedUnix.AsTime(),
	}
}

// ToTopicResponse convert from models.Topic to api.TopicResponse
func ToTopicResponse(topic *repo_model.Topic) *api.TopicResponse {
	return &api.TopicResponse{
		ID:        topic.ID,
		Name:      topic.Name,
		RepoCount: topic.RepoCount,
		Created:   topic.CreatedUnix.AsTime(),
		Updated:   topic.UpdatedUnix.AsTime(),
	}
}

// ToOAuth2Application convert from auth.OAuth2Application to api.OAuth2Application
func ToOAuth2Application(app *auth.OAuth2Application) *api.OAuth2Application {
	return &api.OAuth2Application{
		ID:                         app.ID,
		Name:                       app.Name,
		ClientID:                   app.ClientID,
		ClientSecret:               app.ClientSecret,
		ConfidentialClient:         app.ConfidentialClient,
		SkipSecondaryAuthorization: app.SkipSecondaryAuthorization,
		RedirectURIs:               app.RedirectURIs,
		Created:                    app.CreatedUnix.AsTime(),
	}
}

// ToLFSLock convert a LFSLock to api.LFSLock
func ToLFSLock(ctx context.Context, l *git_model.LFSLock) *api.LFSLock {
	u, err := user_model.GetUserByID(ctx, l.OwnerID)
	if err != nil {
		return nil
	}
	return &api.LFSLock{
		ID:       strconv.FormatInt(l.ID, 10),
		Path:     l.Path,
		LockedAt: l.Created.Round(time.Second),
		Owner: &api.LFSLockOwner{
			Name: u.Name,
		},
	}
}

// ToChangedFile convert a gitdiff.DiffFile to api.ChangedFile
func ToChangedFile(f *gitdiff.DiffFile, repo *repo_model.Repository, commit string) *api.ChangedFile {
	status := "changed"
	previousFilename := ""
	if f.IsDeleted {
		status = "deleted"
	} else if f.IsCreated {
		status = "added"
	} else if f.IsRenamed && f.Type == gitdiff.DiffFileCopy {
		status = "copied"
	} else if f.IsRenamed && f.Type == gitdiff.DiffFileRename {
		status = "renamed"
		previousFilename = f.OldName
	} else if f.Addition == 0 && f.Deletion == 0 {
		status = "unchanged"
	}

	file := &api.ChangedFile{
		Filename:         f.GetDiffFileName(),
		Status:           status,
		Additions:        f.Addition,
		Deletions:        f.Deletion,
		Changes:          f.Addition + f.Deletion,
		PreviousFilename: previousFilename,
		HTMLURL:          fmt.Sprint(repo.HTMLURL(), "/src/commit/", commit, "/", util.PathEscapeSegments(f.GetDiffFileName())),
		ContentsURL:      fmt.Sprint(repo.APIURL(), "/contents/", util.PathEscapeSegments(f.GetDiffFileName()), "?ref=", commit),
		RawURL:           fmt.Sprint(repo.HTMLURL(), "/raw/commit/", commit, "/", util.PathEscapeSegments(f.GetDiffFileName())),
	}

	return file
}
