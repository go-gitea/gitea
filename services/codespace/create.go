// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	repository_service "gitea.dev/services/repository"

	"go.yaml.in/yaml/v4"
)

const (
	codespaceRepoConfigPath = ".gitea/codespace.yaml"
)

var (
	// ErrCreatePermissionDenied is returned when the user cannot create a Codespace for the repository.
	ErrCreatePermissionDenied = errors.New("codespace create permission denied")
	// ErrCreateStateUnavailable is returned when Codespace is not accepting new creates.
	ErrCreateStateUnavailable = errors.New("codespace create state unavailable")
)

// CreateCodespaceOptions contains a creator request from a repository page.
type CreateCodespaceOptions struct {
	User    *user_model.User
	Repo    *repo_model.Repository
	RefType string
	RefName string
}

// CreateCodespaceResult contains the new object identity and initial state.
type CreateCodespaceResult struct {
	CodespaceUUID string
	Status        string
	RepoTag       string
}

type createRef struct {
	Type         string
	Name         string
	CommitSHA    string
	ConfigBranch string
}

// CreateCodespace validates repository input and creates the initial Codespace row.
func CreateCodespace(ctx context.Context, opts CreateCodespaceOptions) (*CreateCodespaceResult, error) {
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

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, opts.Repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	source, err := resolveCreateRef(ctx, opts.User, opts.Repo, gitRepo, opts.RefType, opts.RefName)
	if err != nil {
		return nil, err
	}
	repoTag, err := loadCreateRepoTag(ctx, opts.Repo, gitRepo, source)
	if err != nil {
		return nil, err
	}
	gitProtocol, err := createGitProtocol()
	if err != nil {
		return nil, err
	}
	if _, err := resolveGitTransportCapabilities(gitProtocol); err != nil {
		return nil, err
	}

	ownerIDs := sortedOwnerRelationIDs(opts.User.ID, opts.Repo.OwnerID)
	var result *CreateCodespaceResult
	err = withOwnerRelationLocks(ctx, ownerIDs, func(ctx context.Context) error {
		return globallock.LockAndDo(ctx, repository_service.WorkingLockKey(opts.Repo.ID), func(ctx context.Context) error {
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
				if repo.OwnerID != opts.Repo.OwnerID {
					return errors.New("repository owner changed while creating codespace")
				}
				canRead, err := access_model.HasAccessUnit(ctx, user, repo, unit_model.TypeCode, perm_model.AccessModeRead)
				if err != nil {
					return err
				}
				if !canRead {
					return ErrCreatePermissionDenied
				}
				hasManager, err := matchingCreateManagerExists(ctx, repo.OwnerID, repoTag)
				if err != nil {
					return err
				}
				codespace := newCreateCodespaceRow(user.ID, repo.ID, source, repoTag, gitProtocol, hasManager)
				if _, err := db.GetEngine(ctx).Insert(codespace); err != nil {
					return err
				}
				result = &CreateCodespaceResult{
					CodespaceUUID: codespace.UUID,
					Status:        codespace.Status,
					RepoTag:       codespace.RepoTag,
				}
				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}
	if result.Status == codespace_model.StatusFailed {
		appendCreateNoManagerLog(ctx, result.CodespaceUUID, result.RepoTag)
	}
	return result, nil
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

func resolveCreateRef(ctx context.Context, user *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, refType, rawRef string) (*createRef, error) {
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
		return resolveCreateBranch(ctx, gitRepo, ref)
	case "tag":
		if tag, ok := strings.CutPrefix(ref, "refs/tags/"); ok {
			ref = tag
		}
		return resolveCreateTag(ctx, gitRepo, ref)
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
		return &createRef{Type: "commit", Name: commit.ID.String(), CommitSHA: commit.ID.String()}, nil
	case "pull":
		return resolveCreatePull(ctx, user, repo, gitRepo, ref)
	default:
		return nil, fmt.Errorf("unsupported ref type: %s", refType)
	}
}

func resolveCreateBranch(ctx context.Context, gitRepo *git.Repository, branch string) (*createRef, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, errors.New("branch is required")
	}
	commit, err := gitRepo.GetBranchCommit(ctx, branch)
	if err != nil {
		return nil, err
	}
	return &createRef{Type: "branch", Name: branch, CommitSHA: commit.ID.String()}, nil
}

func resolveCreateTag(ctx context.Context, gitRepo *git.Repository, tag string) (*createRef, error) {
	if strings.TrimSpace(tag) == "" {
		return nil, errors.New("tag is required")
	}
	commit, err := gitRepo.GetTagCommit(ctx, tag)
	if err != nil {
		return nil, err
	}
	return &createRef{Type: "tag", Name: tag, CommitSHA: commit.ID.String()}, nil
}

func resolveCreatePull(ctx context.Context, user *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, rawIndex string) (*createRef, error) {
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
	if pr.HeadRepoID != pr.BaseRepoID {
		if err := pr.LoadHeadRepo(ctx); err != nil {
			return nil, err
		}
		if pr.HeadRepo == nil {
			return nil, errors.New("pull request head repository not found")
		}
		if err := validateCreateRepository(pr.HeadRepo); err != nil {
			return nil, err
		}
		canRead, err := access_model.HasAccessUnit(ctx, user, pr.HeadRepo, unit_model.TypeCode, perm_model.AccessModeRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, ErrCreatePermissionDenied
		}
	}
	refName := pr.GetGitHeadRefName()
	commitSHA, err := gitRepo.GetRefCommitID(ctx, refName)
	if err != nil {
		return nil, err
	}
	return &createRef{
		Type:         "pull",
		Name:         refName,
		CommitSHA:    commitSHA,
		ConfigBranch: pr.BaseBranch,
	}, nil
}

func loadCreateRepoTag(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, source *createRef) (string, error) {
	configBranch := repo.DefaultBranch
	if source.ConfigBranch != "" {
		configBranch = source.ConfigBranch
	} else if source.Type == "branch" {
		configBranch = source.Name
	}
	commit, err := gitRepo.GetBranchCommit(ctx, configBranch)
	if err != nil {
		return "", fmt.Errorf("read codespace config branch %q: %w", configBranch, err)
	}
	blob, err := commit.GetBlobByPath(ctx, gitRepo, codespaceRepoConfigPath)
	if err != nil {
		if git.IsErrNotExist(err) || errors.Is(err, util.ErrNotExist) {
			return "default", nil
		}
		return "", err
	}
	if blob.Size(ctx) > setting.Codespace.CodespaceRepoConfigMaxSize {
		return "", fmt.Errorf("codespace config exceeds %d bytes", setting.Codespace.CodespaceRepoConfigMaxSize)
	}
	content, err := blob.GetBlobBytes(ctx, setting.Codespace.CodespaceRepoConfigMaxSize+1)
	if err != nil {
		return "", err
	}
	if int64(len(content)) > setting.Codespace.CodespaceRepoConfigMaxSize {
		return "", fmt.Errorf("codespace config exceeds %d bytes", setting.Codespace.CodespaceRepoConfigMaxSize)
	}
	return parseCodespaceRepoConfig(content)
}

func parseCodespaceRepoConfig(content []byte) (string, error) {
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&doc); err != nil {
		return "", fmt.Errorf("parse codespace config: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", errors.New("codespace config must contain one document")
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return "", errors.New("codespace config must be a mapping")
	}
	mapping := doc.Content[0]
	seen := map[string]struct{}{}
	tag := "default"
	for i := 0; i < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valueNode := mapping.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			return "", errors.New("codespace config key must be a string")
		}
		key := keyNode.Value
		if _, ok := seen[key]; ok {
			return "", fmt.Errorf("duplicate codespace config field %q", key)
		}
		seen[key] = struct{}{}
		if key != "tag" {
			return "", fmt.Errorf("unknown codespace config field %q", key)
		}
		if valueNode.Kind != yaml.ScalarNode {
			return "", errors.New("codespace config tag must be a string")
		}
		tag = strings.ToLower(strings.TrimSpace(valueNode.Value))
		if tag == "" {
			tag = "default"
		}
	}
	if !tagPattern.MatchString(tag) {
		return "", fmt.Errorf("invalid codespace config tag %q", tag)
	}
	return tag, nil
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

func matchingCreateManagerExists(ctx context.Context, ownerID int64, repoTag string) (bool, error) {
	var managers []*codespace_model.Manager
	if err := db.GetEngine(ctx).In("owner_id", []int64{0, ownerID}).Find(&managers); err != nil {
		return false, err
	}
	for _, manager := range managers {
		tags, err := decodeManagerTags(manager)
		if err != nil {
			return false, err
		}
		if slices.Contains(tags, repoTag) {
			return true, nil
		}
	}
	return false, nil
}

func newCreateCodespaceRow(userID, repoID int64, source *createRef, repoTag, gitProtocol string, hasManager bool) *codespace_model.Codespace {
	now := time.Now().Unix()
	codespaceUUID := codespace_model.NewUUID()
	codespace := &codespace_model.Codespace{
		UUID:         codespaceUUID,
		UserID:       userID,
		RepoID:       repoID,
		RefType:      source.Type,
		RefName:      source.Name,
		RepoTag:      repoTag,
		GitProtocol:  gitProtocol,
		CommitSHA:    source.CommitSHA,
		Status:       codespace_model.StatusCreating,
		AutoStopMode: codespace_model.AutoStopModeDefault,
		CreatedUnix:  now,
		UpdatedUnix:  now,
		LogFilename:  codespaceUUID + ".log",
	}
	if hasManager {
		codespace.OperationRVersion = 1
		codespace.OperationType = codespace_model.OperationCreate
		codespace.OperationStatus = codespace_model.OperationStatusQueued
		codespace.OperationTrigger = codespace_model.OperationTriggerUser
		codespace.OperationCreatedUnix = now
	} else {
		codespace.Status = codespace_model.StatusFailed
	}
	return codespace
}

func sortedOwnerRelationIDs(ids ...int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	slices.Sort(result)
	return result
}

func withOwnerRelationLocks(ctx context.Context, ownerIDs []int64, fn func(context.Context) error) error {
	var releases []func()
	for _, ownerID := range ownerIDs {
		releaser, err := globallock.Lock(ctx, codespaceOwnerRelationLockKey(ownerID))
		if err != nil {
			for _, v := range slices.Backward(releases) {
				v()
			}
			return err
		}
		releases = append(releases, releaser)
	}
	defer func() {
		for _, v := range slices.Backward(releases) {
			v()
		}
	}()
	return fn(ctx)
}

func codespaceOwnerRelationLockKey(ownerID int64) string {
	return fmt.Sprintf("codespace_owner_%d", ownerID)
}

func appendCreateNoManagerLog(ctx context.Context, codespaceUUID, repoTag string) {
	err := globallock.LockAndDo(ctx, updateLogLockKey(codespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
			if err != nil || !has || codespace.Status != codespace_model.StatusFailed || hasActiveOperation(codespace) {
				return err
			}
			line := fmt.Sprintf("No registered Codespace Manager matches repository tag %q.", repoTag)
			encoded, err := encodeLogLines([]LogLine{{
				TimestampUnixNano: time.Now().UnixNano(),
				Message:           line,
			}})
			if err != nil {
				return err
			}
			return appendEncodedLogLines(ctx, codespace, encoded, 1)
		})
	})
	if err != nil {
		log.Warn("failed to write no-manager codespace log for %s: %v", codespaceUUID, err)
	}
}
