// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/setting"
	repository_service "gitea.dev/services/repository"
)

var (
	// ErrCreatePermissionDenied is returned when the user cannot create a Codespace for the repository.
	ErrCreatePermissionDenied = errors.New("codespace create permission denied")
	// ErrCreateStateUnavailable is returned when Codespace is not accepting new creates.
	ErrCreateStateUnavailable = errors.New("codespace create state unavailable")
	// ErrCreateEnvironmentUnavailable is returned when the selected environment is not available to the user.
	ErrCreateEnvironmentUnavailable = errors.New("codespace create environment unavailable")
	// ErrCreateRequestChanged is returned when the reviewed repository configuration is no longer current.
	ErrCreateRequestChanged = errors.New("codespace create request changed")
)

// CreateCodespaceOptions contains a creator request from a repository page.
type CreateCodespaceOptions struct {
	User                     *user_model.User
	Repo                     *repo_model.Repository
	RefType                  string
	RefName                  string
	RequestHash              string
	DevContainerSelection    string
	EnvironmentTag           string
	PermissionGrants         map[string]string
	RecommendedSecretValues  map[string]string
	RecommendedSecretEnabled map[string]bool
}

// CreateCodespaceResult contains the new object identity and initial state.
type CreateCodespaceResult struct {
	CodespaceUUID  string
	Status         string
	EnvironmentTag string
}

// CreateCodespacePlan is a side-effect-free description presented for user confirmation.
type CreateCodespacePlan struct {
	RequestHash            string
	RefType                string
	RefName                string
	PullRequest            *CreatePullRequestSource
	DevContainerOptions    []CreateDevContainerOption
	Environments           []CreateEnvironmentOption
	Permissions            []CreatePermissionRequest
	RecommendedSecrets     []CreateRecommendedSecret
	AvailableSecrets       []CreateSecretSummary
	SecretInjectionAllowed bool
}

// CreateEnvironmentOption describes one visible Manager environment on the confirmation page.
type CreateEnvironmentOption struct {
	Tag         string
	Description string
	Site        bool
	Personal    bool
	Online      bool
	Selected    bool
}

// CreateRecommendedSecret describes one Dev Container recommended secret without exposing its value.
type CreateRecommendedSecret struct {
	Name        string
	Description string
	Configured  bool
	Available   bool
}

// CreateSecretSummary identifies one secret currently available to the source repository without exposing its value.
type CreateSecretSummary struct {
	Name        string
	Description string
}

// CreatePermissionRequest describes one repository permission used by the Codespace.
type CreatePermissionRequest struct {
	RepositoryID       int64
	RepositoryFullName string
	UnitType           unit_model.Type
	UnitName           string
	Mode               perm_model.AccessMode
	ModeName           string
	FormName           string
	Required           bool
}

// CreatePullRequestSource describes the pull request shown on the confirmation page.
type CreatePullRequestSource struct {
	Index            int64
	HeadRepoFullName string
	HeadBranch       string
	BaseBranch       string
	SnapshotOnly     bool
	IsFork           bool
}

type createSourceRef struct {
	Type                 string
	StoredName           string
	FormName             string
	CommitSHA            string
	PullRequest          *CreatePullRequestSource
	pullHeadRepositoryID int64
}

type preparedCodespace struct {
	sourceRef              *createSourceRef
	devContainer           *createDevContainerPlan
	devContainerOptions    []CreateDevContainerOption
	requestHash            string
	availableSecrets       []CreateSecretSummary
	secretInjectionAllowed bool
}

var codespacePermissionUnits = map[string]unit_model.Type{
	"code":     unit_model.TypeCode,
	"issues":   unit_model.TypeIssues,
	"pulls":    unit_model.TypePullRequests,
	"wiki":     unit_model.TypeWiki,
	"releases": unit_model.TypeReleases,
	"actions":  unit_model.TypeActions,
}

// PrepareCodespace validates a creation request without changing persistent state.
func PrepareCodespace(ctx context.Context, opts CreateCodespaceOptions) (*CreateCodespacePlan, error) {
	prepared, err := prepareCodespace(ctx, opts)
	if err != nil {
		return nil, err
	}
	environments, err := listVisibleCreateEnvironments(ctx, opts.User.ID, opts.EnvironmentTag)
	if err != nil {
		return nil, err
	}
	return &CreateCodespacePlan{
		RequestHash:            prepared.requestHash,
		RefType:                prepared.sourceRef.Type,
		RefName:                prepared.sourceRef.FormName,
		PullRequest:            prepared.sourceRef.PullRequest,
		DevContainerOptions:    prepared.devContainerOptions,
		Environments:           environments,
		Permissions:            prepared.devContainer.Permissions,
		RecommendedSecrets:     prepared.devContainer.RecommendedSecrets,
		AvailableSecrets:       prepared.availableSecrets,
		SecretInjectionAllowed: prepared.secretInjectionAllowed,
	}, nil
}

// CreateCodespace validates repository input and creates the initial Codespace row.
func CreateCodespace(ctx context.Context, opts CreateCodespaceOptions) (*CreateCodespaceResult, error) {
	if strings.TrimSpace(opts.RequestHash) == "" {
		return nil, errors.New("codespace create confirmation is required")
	}
	if opts.User == nil || opts.User.ID <= 0 {
		return nil, errors.New("user is required")
	}
	if opts.Repo == nil || opts.Repo.ID <= 0 {
		return nil, errors.New("repository is required")
	}

	var result *CreateCodespaceResult
	err := globallock.LockAndDo(ctx, codespaceUserRelationLockKey(opts.User.ID), func(ctx context.Context) error {
		return globallock.LockAndDo(ctx, repository_service.WorkingLockKey(opts.Repo.ID), func(ctx context.Context) error {
			prepared, err := prepareCodespace(ctx, opts)
			if err != nil {
				return err
			}
			if subtle.ConstantTimeCompare([]byte(prepared.requestHash), []byte(opts.RequestHash)) != 1 {
				return ErrCreateRequestChanged
			}
			grantedModes, err := normalizePermissionGrants(prepared.devContainer.Permissions, opts.PermissionGrants)
			if err != nil {
				return err
			}
			return db.WithTx(ctx, func(ctx context.Context) error {
				user, err := user_model.GetUserByID(ctx, opts.User.ID)
				if err != nil {
					return err
				}
				if err := validateCreateUser(user); err != nil {
					return err
				}
				repo, err := repo_model.GetRepositoryByID(ctx, opts.Repo.ID)
				if err != nil {
					return err
				}
				if err := validateCreateRepository(repo); err != nil {
					return err
				}
				canRead, err := access_model.HasAccessUnit(ctx, user, repo, unit_model.TypeCode, perm_model.AccessModeRead)
				if err != nil {
					return err
				}
				if !canRead {
					return ErrCreatePermissionDenied
				}
				environmentTag := strings.ToLower(strings.TrimSpace(opts.EnvironmentTag))
				if !tagPattern.MatchString(environmentTag) {
					return ErrCreateEnvironmentUnavailable
				}
				environments, err := listVisibleCreateEnvironments(ctx, user.ID, environmentTag)
				if err != nil {
					return err
				}
				available := false
				for _, environment := range environments {
					if environment.Tag == environmentTag {
						available = true
						break
					}
				}
				if !available {
					return ErrCreateEnvironmentUnavailable
				}
				authorizationID, err := ensurePermissionAuthorization(ctx, user.ID, repo.ID, permissionRequestHash(repo.ID, prepared.devContainer.Permissions), prepared.devContainer.Permissions, grantedModes)
				if err != nil {
					return err
				}
				if prepared.secretInjectionAllowed {
					if err := configureRecommendedSecrets(ctx, user, repo, prepared.devContainer.RecommendedSecrets, opts.RecommendedSecretValues, opts.RecommendedSecretEnabled); err != nil {
						return err
					}
				}
				codespace := newCreateCodespaceRow(user.ID, repo.ID, environmentTag, prepared.sourceRef, prepared.devContainer, authorizationID)
				if _, err := db.GetEngine(ctx).Insert(codespace); err != nil {
					return err
				}
				result = &CreateCodespaceResult{
					CodespaceUUID:  codespace.UUID,
					Status:         codespace.Status,
					EnvironmentTag: codespace.EnvironmentTag,
				}
				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func prepareCodespace(ctx context.Context, opts CreateCodespaceOptions) (*preparedCodespace, error) {
	if !setting.Codespace.Enabled {
		return nil, ErrCreateStateUnavailable
	}
	if opts.User == nil || opts.User.ID <= 0 {
		return nil, errors.New("user is required")
	}
	if opts.Repo == nil || opts.Repo.ID <= 0 {
		return nil, errors.New("repository is required")
	}
	if err := validateCreateUser(opts.User); err != nil {
		return nil, err
	}
	if err := validateCreateRepository(opts.Repo); err != nil {
		return nil, err
	}
	canRead, err := access_model.HasAccessUnit(ctx, opts.User, opts.Repo, unit_model.TypeCode, perm_model.AccessModeRead)
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, ErrCreatePermissionDenied
	}
	gitProtocol, err := createGitProtocol()
	if err != nil {
		return nil, err
	}
	if _, err := resolveGitTransportCapabilities(gitProtocol); err != nil {
		return nil, err
	}
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, opts.Repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	sourceRef, err := resolveCreateSourceRef(ctx, opts.User, opts.Repo, gitRepo, opts.RefType, opts.RefName)
	if err != nil {
		return nil, err
	}
	devContainer, options, err := prepareCreateDevContainer(ctx, opts.User, opts.Repo, gitRepo, sourceRef, opts.DevContainerSelection)
	if err != nil {
		return nil, err
	}
	secretInjectionAllowed := sourceRef.PullRequest == nil || !sourceRef.PullRequest.IsFork
	if secretInjectionAllowed {
		secretInjectionAllowed, err = userCanUseSecretRepository(ctx, opts.User, opts.Repo)
		if err != nil {
			return nil, err
		}
	}
	var availableSecrets []CreateSecretSummary
	if secretInjectionAllowed {
		devContainer.RecommendedSecrets, availableSecrets, err = resolveCreateSecrets(ctx, opts.User.ID, opts.Repo.ID, devContainer.RecommendedSecrets)
		if err != nil {
			return nil, err
		}
	}
	if pull := sourceRef.PullRequest; pull != nil && !pull.SnapshotOnly && pull.HeadRepoFullName != opts.Repo.FullName() {
		required := CreatePermissionRequest{
			RepositoryID:       sourceRef.pullHeadRepositoryID,
			RepositoryFullName: pull.HeadRepoFullName,
			UnitType:           unit_model.TypeCode,
			UnitName:           "code",
			Mode:               perm_model.AccessModeWrite,
			ModeName:           "write",
			FormName:           fmt.Sprintf("permission_%d_%d", sourceRef.pullHeadRepositoryID, unit_model.TypeCode),
			Required:           true,
		}
		replaced := false
		for i, permission := range devContainer.Permissions {
			if permission.RepositoryID == required.RepositoryID && permission.UnitType == required.UnitType {
				devContainer.Permissions[i] = required
				replaced = true
				break
			}
		}
		if !replaced {
			devContainer.Permissions = append(devContainer.Permissions, required)
		}
		sort.Slice(devContainer.Permissions, func(i, j int) bool {
			if devContainer.Permissions[i].RepositoryID != devContainer.Permissions[j].RepositoryID {
				return devContainer.Permissions[i].RepositoryID < devContainer.Permissions[j].RepositoryID
			}
			return devContainer.Permissions[i].UnitType < devContainer.Permissions[j].UnitType
		})
	}
	return &preparedCodespace{
		sourceRef:              sourceRef,
		devContainer:           devContainer,
		devContainerOptions:    options,
		requestHash:            createPlanHash(opts.Repo.ID, sourceRef, devContainer),
		availableSecrets:       availableSecrets,
		secretInjectionAllowed: secretInjectionAllowed,
	}, nil
}

func validateCreateUser(user *user_model.User) error {
	if user == nil || user.ID <= 0 {
		return errors.New("user is required")
	}
	if !user.IsActive || user.ProhibitLogin || user.MustChangePassword {
		return ErrCreatePermissionDenied
	}
	return nil
}

func validateCreateRepository(repo *repo_model.Repository) error {
	if repo == nil || repo.ID <= 0 {
		return errors.New("repository is required")
	}
	if repo.IsEmpty {
		return errors.New("repository is empty")
	}
	if repo.IsArchived || repo.IsBeingCreated() || repo.IsBroken() {
		return errors.New("repository state does not allow codespace creation")
	}
	return nil
}

func resolveCreateSourceRef(ctx context.Context, user *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, refType, rawRef string) (*createSourceRef, error) {
	refType = strings.ToLower(strings.TrimSpace(refType))
	ref := strings.TrimSpace(rawRef)
	if refType == "" {
		refType = "branch"
	}
	switch refType {
	case "branch":
		if ref == "" {
			ref = repo.DefaultBranch
		}
		if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			ref = branch
		}
		if ref == "" {
			return nil, errors.New("branch is required")
		}
		commit, err := gitRepo.GetBranchCommit(ctx, ref)
		if err != nil {
			return nil, err
		}
		return &createSourceRef{Type: "branch", StoredName: ref, FormName: ref, CommitSHA: commit.ID.String()}, nil
	case "tag":
		if tag, ok := strings.CutPrefix(ref, "refs/tags/"); ok {
			ref = tag
		}
		if ref == "" {
			return nil, errors.New("tag is required")
		}
		commit, err := gitRepo.GetTagCommit(ctx, ref)
		if err != nil {
			return nil, err
		}
		return &createSourceRef{Type: "tag", StoredName: ref, FormName: ref, CommitSHA: commit.ID.String()}, nil
	case "commit":
		if ref == "" {
			return nil, errors.New("commit is required")
		}
		objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
		if len(ref) != objectFormat.FullLength() || !git.IsStringLikelyCommitID(objectFormat, ref) {
			return nil, fmt.Errorf("invalid commit: %s", ref)
		}
		commit, err := gitRepo.GetCommit(ctx, ref)
		if err != nil {
			return nil, err
		}
		return &createSourceRef{Type: "commit", StoredName: commit.ID.String(), FormName: commit.ID.String(), CommitSHA: commit.ID.String()}, nil
	case "pull":
		return resolveCreatePull(ctx, user, repo, gitRepo, ref)
	default:
		return nil, fmt.Errorf("unsupported ref type: %s", refType)
	}
}

func resolveCreatePull(ctx context.Context, user *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, rawIndex string) (*createSourceRef, error) {
	index, err := strconv.ParseInt(strings.TrimSpace(rawIndex), 10, 64)
	if err != nil || index <= 0 {
		return nil, errors.New("pull request index is required")
	}
	pr, err := issues_model.GetPullRequestByIndex(ctx, repo.ID, index)
	if err != nil {
		return nil, err
	}
	if pr.BaseRepoID != repo.ID {
		return nil, errors.New("pull request base repository mismatch")
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return nil, err
	}
	if pr.Issue.IsClosed {
		return nil, errors.New("pull request is closed")
	}
	pullSource := &CreatePullRequestSource{
		Index:        index,
		HeadBranch:   pr.HeadBranch,
		BaseBranch:   pr.BaseBranch,
		SnapshotOnly: pr.IsAgitFlow(),
		IsFork:       pr.HeadRepoID != pr.BaseRepoID,
	}
	if !pullSource.SnapshotOnly {
		if err := pr.LoadHeadRepo(ctx); err != nil {
			return nil, err
		}
		if pr.HeadRepo == nil {
			return nil, errors.New("pull request head repository not found")
		}
		if err := validateCreateRepository(pr.HeadRepo); err != nil {
			return nil, err
		}
		exists, err := git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, errors.New("pull request head branch not found")
		}
		canRead, err := access_model.HasAccessUnit(ctx, user, pr.HeadRepo, unit_model.TypeCode, perm_model.AccessModeRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, ErrCreatePermissionDenied
		}
		pullSource.HeadRepoFullName = pr.HeadRepo.FullName()
	} else {
		pullSource.HeadRepoFullName = repo.FullName()
	}
	refName := pr.GetGitHeadRefName()
	commitSHA, err := gitRepo.GetRefCommitID(ctx, refName)
	if err != nil {
		return nil, err
	}
	return &createSourceRef{
		Type:                 "pull",
		StoredName:           refName,
		FormName:             strconv.FormatInt(index, 10),
		CommitSHA:            commitSHA,
		PullRequest:          pullSource,
		pullHeadRepositoryID: pr.HeadRepoID,
	}, nil
}

func resolveCreatePermissions(ctx context.Context, user *user_model.User, sourceRepo *repo_model.Repository, requested map[string]map[string]string) ([]CreatePermissionRequest, error) {
	permissions := make([]CreatePermissionRequest, 0)
	for fullName, units := range requested {
		ownerName, repoName, _ := strings.Cut(fullName, "/")
		target, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
		if err != nil {
			return nil, fmt.Errorf("resolve codespace permission repository %q: %w", fullName, err)
		}
		if target.ID == sourceRepo.ID {
			return nil, fmt.Errorf("codespace permission repository %q is the source repository", fullName)
		}
		for unitName, modeName := range units {
			unitType := codespacePermissionUnits[unitName]
			if !target.UnitEnabled(ctx, unitType) {
				return nil, fmt.Errorf("codespace permission unit %q is not enabled for %q", unitName, fullName)
			}
			mode := perm_model.ParseAccessMode(modeName, perm_model.AccessModeRead, perm_model.AccessModeWrite)
			allowed, err := access_model.HasAccessUnit(ctx, user, target, unitType, mode)
			if err != nil {
				return nil, err
			}
			if !allowed {
				return nil, fmt.Errorf("user cannot grant %s access to %s for %q", modeName, unitName, fullName)
			}
			permissions = append(permissions, CreatePermissionRequest{
				RepositoryID: target.ID, RepositoryFullName: target.FullName(), UnitType: unitType,
				UnitName: unitName, Mode: mode, ModeName: modeName,
				FormName: fmt.Sprintf("permission_%d_%d", target.ID, unitType),
			})
		}
	}
	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].RepositoryID != permissions[j].RepositoryID {
			return permissions[i].RepositoryID < permissions[j].RepositoryID
		}
		return permissions[i].UnitType < permissions[j].UnitType
	})
	return permissions, nil
}

func createPlanHash(repoID int64, sourceRef *createSourceRef, devContainer *createDevContainerPlan) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%d\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", repoID, sourceRef.Type, sourceRef.StoredName, sourceRef.CommitSHA, devContainer.Path, devContainer.ContentSHA256, devContainer.DefaultImage)
	if pull := sourceRef.PullRequest; pull != nil {
		fmt.Fprintf(hash, "\x00%d\x00%s\x00%s\x00%s\x00%t", pull.Index, pull.HeadRepoFullName, pull.HeadBranch, pull.BaseBranch, pull.SnapshotOnly)
	}
	for _, permission := range devContainer.Permissions {
		fmt.Fprintf(hash, "\x00%d\x00%d\x00%d\x00%t", permission.RepositoryID, permission.UnitType, permission.Mode, permission.Required)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func permissionRequestHash(repoID int64, permissions []CreatePermissionRequest) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%d", repoID)
	for _, permission := range permissions {
		fmt.Fprintf(hash, "\x00%d\x00%d\x00%d", permission.RepositoryID, permission.UnitType, permission.Mode)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func createGitProtocol() (string, error) {
	protocol := strings.ToLower(strings.TrimSpace(setting.Codespace.GitProtocol))
	switch protocol {
	case "":
		return codespace_model.GitProtocolHTTP, nil
	case codespace_model.GitProtocolHTTP, codespace_model.GitProtocolSSH:
		return protocol, nil
	default:
		return "", fmt.Errorf("invalid codespace git protocol %q", setting.Codespace.GitProtocol)
	}
}

func listVisibleCreateEnvironments(ctx context.Context, userID int64, selectedTag string) ([]CreateEnvironmentOption, error) {
	var managers []*codespace_model.Manager
	if err := db.GetEngine(ctx).
		In("user_id", []int64{0, userID}).
		Where("last_online_unix > 0").
		Asc("user_id", "id").
		Find(&managers); err != nil {
		return nil, err
	}
	type aggregate struct {
		option       CreateEnvironmentOption
		descriptions map[string]struct{}
	}
	byTag := make(map[string]*aggregate)
	for _, manager := range managers {
		environments, err := decodeManagerEnvironments(manager)
		if err != nil {
			return nil, err
		}
		for _, environment := range environments {
			item := byTag[environment.Tag]
			if item == nil {
				item = &aggregate{
					option:       CreateEnvironmentOption{Tag: environment.Tag},
					descriptions: make(map[string]struct{}),
				}
				byTag[environment.Tag] = item
			}
			if manager.UserID == 0 {
				item.option.Site = true
			} else {
				item.option.Personal = true
			}
			if managerAllowsOnlineOrRecovering(manager) {
				item.option.Online = true
			}
			if environment.Description != "" {
				item.descriptions[environment.Description] = struct{}{}
			}
		}
	}
	selectedTag = strings.ToLower(strings.TrimSpace(selectedTag))
	result := make([]CreateEnvironmentOption, 0, len(byTag))
	for _, item := range byTag {
		if len(item.descriptions) == 1 {
			for description := range item.descriptions {
				item.option.Description = description
			}
		}
		item.option.Selected = item.option.Tag == selectedTag
		result = append(result, item.option)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Tag < result[j].Tag
	})
	return result, nil
}

func normalizePermissionGrants(permissions []CreatePermissionRequest, submitted map[string]string) ([]perm_model.AccessMode, error) {
	if len(permissions) == 0 {
		return nil, nil
	}
	grants := make([]perm_model.AccessMode, len(permissions))
	for i, permission := range permissions {
		if permission.Required {
			grants[i] = permission.Mode
			continue
		}
		var mode perm_model.AccessMode
		switch submitted[permission.FormName] {
		case "none":
			mode = perm_model.AccessModeNone
		case "read":
			mode = perm_model.AccessModeRead
		case "write":
			mode = perm_model.AccessModeWrite
		default:
			return nil, fmt.Errorf("codespace permission confirmation is missing for %s %s", permission.RepositoryFullName, permission.UnitName)
		}
		if mode > permission.Mode {
			return nil, fmt.Errorf("codespace permission confirmation exceeds the requested %s access for %s %s", permission.ModeName, permission.RepositoryFullName, permission.UnitName)
		}
		grants[i] = mode
	}
	return grants, nil
}

func ensurePermissionAuthorization(ctx context.Context, userID, sourceRepoID int64, requestHash string, permissions []CreatePermissionRequest, grantedModes []perm_model.AccessMode) (int64, error) {
	if len(permissions) == 0 {
		return 0, nil
	}
	var authorizations []*codespace_model.PermissionAuthorization
	err := db.GetEngine(ctx).
		Where("user_id = ? AND source_repo_id = ? AND request_hash = ? AND revoked_unix = 0", userID, sourceRepoID, requestHash).
		Find(&authorizations)
	if err != nil {
		return 0, err
	}
	for _, authorization := range authorizations {
		var rules []*codespace_model.PermissionRepository
		if err := db.GetEngine(ctx).Where("authorization_id = ?", authorization.ID).Asc("target_repo_id", "unit_type").Find(&rules); err != nil {
			return 0, err
		}
		if len(rules) != len(permissions) {
			continue
		}
		matches := true
		for i, rule := range rules {
			permission := permissions[i]
			if rule.TargetRepoID != permission.RepositoryID || rule.UnitType != permission.UnitType || rule.RequestedMode != permission.Mode || rule.GrantedMode != grantedModes[i] {
				matches = false
				break
			}
		}
		if matches {
			return authorization.ID, nil
		}
	}
	now := time.Now().Unix()
	authorization := &codespace_model.PermissionAuthorization{
		UserID: userID, SourceRepoID: sourceRepoID, RequestHash: requestHash,
		CreatedUnix: now, UpdatedUnix: now,
	}
	if _, err := db.GetEngine(ctx).Insert(authorization); err != nil {
		return 0, err
	}
	rules := make([]*codespace_model.PermissionRepository, 0, len(permissions))
	for i, permission := range permissions {
		rules = append(rules, &codespace_model.PermissionRepository{
			AuthorizationID: authorization.ID,
			TargetRepoID:    permission.RepositoryID,
			UnitType:        permission.UnitType,
			RequestedMode:   permission.Mode,
			GrantedMode:     grantedModes[i],
		})
	}
	if _, err := db.GetEngine(ctx).Insert(rules); err != nil {
		return 0, err
	}
	return authorization.ID, nil
}

func newCreateCodespaceRow(userID, repoID int64, environmentTag string, sourceRef *createSourceRef, devContainer *createDevContainerPlan, authorizationID int64) *codespace_model.Codespace {
	now := time.Now().Unix()
	codespaceUUID := codespace_model.NewUUID()
	codespace := &codespace_model.Codespace{
		UUID:                      codespaceUUID,
		UserID:                    userID,
		RepoID:                    repoID,
		RefType:                   sourceRef.Type,
		RefName:                   sourceRef.StoredName,
		EnvironmentTag:            environmentTag,
		CommitSHA:                 sourceRef.CommitSHA,
		DevContainerPath:          devContainer.Path,
		DevContainerContentSHA256: devContainer.ContentSHA256,
		DevContainerDefaultImage:  devContainer.DefaultImage,
		PermissionAuthorizationID: authorizationID,
		Status:                    codespace_model.StatusCreating,
		AutoStopMode:              codespace_model.AutoStopModeDefault,
		CreatedUnix:               now,
		UpdatedUnix:               now,
	}
	codespace.OperationRVersion = 1
	codespace.OperationType = codespace_model.OperationCreate
	codespace.OperationStatus = codespace_model.OperationStatusQueued
	codespace.OperationTrigger = codespace_model.OperationTriggerUser
	codespace.OperationCreatedUnix = now
	return codespace
}

func codespaceUserRelationLockKey(userID int64) string {
	return fmt.Sprintf("codespace_user_%d", userID)
}
