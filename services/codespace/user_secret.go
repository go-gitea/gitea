// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	repository_service "gitea.dev/services/repository"
	secret_service "gitea.dev/services/secrets"
)

const (
	userSecretCountLimit           = 100
	userSecretValueSizeLimit       = 48 * 1024
	repositorySecretTotalSizeLimit = 512 * 1024
	userSecretRepositorySearchSize = 20
)

var (
	ErrUserSecretNotFound     = errors.New("codespace user secret not found")
	ErrUserSecretNameInvalid  = errors.New("codespace user secret name is invalid")
	ErrUserSecretNameConflict = errors.New("codespace user secret name already exists")
	ErrUserSecretValueInvalid = errors.New("codespace user secret value is invalid")
	ErrUserSecretCountLimit   = errors.New("codespace user secret count limit exceeded")
	ErrUserSecretSizeLimit    = errors.New("codespace user secret size limit exceeded")
)

// UserSecretView is the value-free representation used by user settings.
type UserSecretView struct {
	ID              int64
	Name            string
	AllRepositories bool
	Repositories    []*repo_model.Repository
	CreatedUnix     int64
	UpdatedUnix     int64
}

// SearchWritableSecretRepositories returns repositories eligible for a personal Codespace secret.
func SearchWritableSecretRepositories(ctx context.Context, user *user_model.User, keyword string) ([]*repo_model.Repository, error) {
	if user == nil || user.ID <= 0 || user.Type != user_model.UserTypeIndividual {
		return nil, util.NewInvalidArgumentErrorf("individual user is required")
	}
	result := make([]*repo_model.Repository, 0, userSecretRepositorySearchSize)
	for page := 1; ; page++ {
		repositories, total, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{Page: page, PageSize: 100},
			Actor:       user, Keyword: strings.TrimSpace(keyword), Private: true, UnitType: unit.TypeCode,
		})
		if err != nil {
			return nil, err
		}
		for _, repo := range repositories {
			allowed, err := userCanUseSecretRepository(ctx, user, repo)
			if err != nil {
				return nil, err
			}
			if allowed {
				result = append(result, repo)
				if len(result) == userSecretRepositorySearchSize {
					return result, nil
				}
			}
		}
		if len(repositories) == 0 || int64(page*100) >= total {
			return result, nil
		}
	}
}

// RuntimeSecret is one environment variable made available to an authorized Codespace runtime.
type RuntimeSecret struct {
	Name  string
	Value string
}

// ListUserSecrets returns a user's secrets without decrypting their values.
func ListUserSecrets(ctx context.Context, userID int64) ([]UserSecretView, error) {
	var secrets []*codespace_model.UserSecret
	if err := db.GetEngine(ctx).Where("user_id = ?", userID).Asc("name").Find(&secrets); err != nil {
		return nil, err
	}
	if len(secrets) == 0 {
		return nil, nil
	}
	secretIDs := make([]int64, 0, len(secrets))
	for _, secret := range secrets {
		secretIDs = append(secretIDs, secret.ID)
	}
	var bindings []*codespace_model.UserSecretRepository
	if err := db.GetEngine(ctx).In("secret_id", secretIDs).Asc("secret_id", "repo_id").Find(&bindings); err != nil {
		return nil, err
	}
	repoIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		repoIDs = append(repoIDs, binding.RepoID)
	}
	repoMap, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return nil, err
	}
	repositoryList := repo_model.RepositoryListOfMap(repoMap)
	if err := repositoryList.LoadOwners(ctx); err != nil {
		return nil, err
	}
	bindingsBySecret := make(map[int64][]*repo_model.Repository, len(secrets))
	for _, binding := range bindings {
		if repo := repoMap[binding.RepoID]; repo != nil {
			bindingsBySecret[binding.SecretID] = append(bindingsBySecret[binding.SecretID], repo)
		}
	}
	views := make([]UserSecretView, 0, len(secrets))
	for _, secret := range secrets {
		selectedRepositories := bindingsBySecret[secret.ID]
		slices.SortFunc(selectedRepositories, func(a, b *repo_model.Repository) int {
			return strings.Compare(a.FullName(), b.FullName())
		})
		views = append(views, UserSecretView{
			ID: secret.ID, Name: secret.Name, AllRepositories: secret.AllRepositories, Repositories: selectedRepositories,
			CreatedUnix: secret.CreatedUnix, UpdatedUnix: secret.UpdatedUnix,
		})
	}
	return views, nil
}

// CreateUserSecret creates a secret with an independently selected repository scope.
func CreateUserSecret(ctx context.Context, user *user_model.User, name, value string, allRepositories bool, repoIDs []int64) error {
	if user == nil || user.ID <= 0 {
		return util.NewInvalidArgumentErrorf("user is required")
	}
	repoIDs, err := normalizeUserSecretRepositoryIDs(repoIDs)
	if err != nil {
		return err
	}
	if allRepositories {
		repoIDs = nil
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(user.ID), func(ctx context.Context) error {
		return withUserSecretRepositoryLocks(ctx, repoIDs, func(ctx context.Context) error {
			return db.WithTx(ctx, func(ctx context.Context) error {
				currentUser, repositories, err := loadWritableSecretRepositories(ctx, user.ID, repoIDs)
				if err != nil {
					return err
				}
				return insertUserSecret(ctx, currentUser, name, value, allRepositories, repositories)
			})
		})
	})
}

// UpdateUserSecretValue replaces a secret value without changing its repository selection.
func UpdateUserSecretValue(ctx context.Context, userID, secretID int64, value string) error {
	if err := validateUserSecretValue(value); err != nil {
		return err
	}
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, value)
	if err != nil {
		return err
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(userID), func(ctx context.Context) error {
		_, err := getUserSecret(ctx, userID, secretID)
		if err != nil {
			return err
		}
		repoIDs, err := getUserSecretRepositoryIDs(ctx, secretID)
		if err != nil {
			return err
		}
		return withUserSecretRepositoryLocks(ctx, repoIDs, func(ctx context.Context) error {
			return db.WithTx(ctx, func(ctx context.Context) error {
				secret, err := getUserSecret(ctx, userID, secretID)
				if err != nil {
					return err
				}
				repoIDs, err := getUserSecretRepositoryIDs(ctx, secretID)
				if err != nil {
					return err
				}
				if err := checkUserSecretScopeSize(ctx, userID, secretID, int64(len(value)), secret.AllRepositories, repoIDs); err != nil {
					return err
				}
				secret.DataEncrypted = encrypted
				secret.DataSize = int64(len(value))
				secret.UpdatedUnix = time.Now().Unix()
				_, err = db.GetEngine(ctx).ID(secretID).Cols("data_encrypted", "data_size", "updated_unix").Update(secret)
				return err
			})
		})
	})
}

// UpdateUserSecretRepositoryAccess replaces the complete repository scope for one secret.
func UpdateUserSecretRepositoryAccess(ctx context.Context, user *user_model.User, secretID int64, allRepositories bool, repoIDs []int64) error {
	if user == nil || user.ID <= 0 {
		return util.NewInvalidArgumentErrorf("user is required")
	}
	repoIDs, err := normalizeUserSecretRepositoryIDs(repoIDs)
	if err != nil {
		return err
	}
	if allRepositories {
		repoIDs = nil
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(user.ID), func(ctx context.Context) error {
		_, err := getUserSecret(ctx, user.ID, secretID)
		if err != nil {
			return err
		}
		existingRepoIDs, err := getUserSecretRepositoryIDs(ctx, secretID)
		if err != nil {
			return err
		}
		lockedRepoIDs := slices.Clone(repoIDs)
		lockedRepoIDs = append(lockedRepoIDs, existingRepoIDs...)
		return withUserSecretRepositoryLocks(ctx, lockedRepoIDs, func(ctx context.Context) error {
			return db.WithTx(ctx, func(ctx context.Context) error {
				currentUser, repositories, err := loadWritableSecretRepositories(ctx, user.ID, repoIDs)
				if err != nil {
					return err
				}
				secret, err := getUserSecret(ctx, currentUser.ID, secretID)
				if err != nil {
					return err
				}
				if err := checkUserSecretScopeSize(ctx, currentUser.ID, secret.ID, secret.DataSize, allRepositories, repoIDs); err != nil {
					return err
				}
				if _, err := db.GetEngine(ctx).Where("secret_id = ?", secretID).Delete(new(codespace_model.UserSecretRepository)); err != nil {
					return err
				}
				if err := insertUserSecretRepositoryBindings(ctx, secretID, repositories); err != nil {
					return err
				}
				secret.AllRepositories = allRepositories
				secret.UpdatedUnix = time.Now().Unix()
				_, err = db.GetEngine(ctx).ID(secretID).Cols("all_repositories", "updated_unix").Update(secret)
				return err
			})
		})
	})
}

// DeleteUserSecret removes a secret and every repository binding.
func DeleteUserSecret(ctx context.Context, userID, secretID int64) error {
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(userID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			if _, err := getUserSecret(ctx, userID, secretID); err != nil {
				return err
			}
			if _, err := db.GetEngine(ctx).Where("secret_id = ?", secretID).Delete(new(codespace_model.UserSecretRepository)); err != nil {
				return err
			}
			_, err := db.GetEngine(ctx).ID(secretID).Delete(new(codespace_model.UserSecret))
			return err
		})
	})
}

func resolveCodespaceRuntimeSecrets(ctx context.Context, user *user_model.User, codespace *codespace_model.Codespace) ([]RuntimeSecret, error) {
	if codespace.RepoID <= 0 {
		return nil, nil
	}
	repo, err := repo_model.GetRepositoryByID(ctx, codespace.RepoID)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	allowed, err := userCanUseSecretRepository(ctx, user, repo)
	if err != nil || !allowed {
		return nil, err
	}
	if codespace.RefType == "pull" {
		pullName, hasPrefix := strings.CutPrefix(codespace.RefName, "refs/pull/")
		pullName, hasSuffix := strings.CutSuffix(pullName, "/head")
		pullIndex, err := strconv.ParseInt(pullName, 10, 64)
		if !hasPrefix || !hasSuffix || err != nil || pullIndex <= 0 {
			return nil, fmt.Errorf("invalid codespace pull ref %q", codespace.RefName)
		}
		pull, err := issues_model.GetPullRequestByIndex(ctx, repo.ID, pullIndex)
		if err != nil {
			if issues_model.IsErrPullRequestNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		if pull.HeadRepoID != pull.BaseRepoID {
			return nil, nil
		}
	}
	return resolveUserSecretsForRepository(ctx, user.ID, repo.ID)
}

func resolveUserSecretsForRepository(ctx context.Context, userID, repoID int64) ([]RuntimeSecret, error) {
	var secrets []*codespace_model.UserSecret
	if err := db.GetEngine(ctx).Table("codespace_user_secret").
		Where("codespace_user_secret.user_id = ? AND (codespace_user_secret.all_repositories = ? OR EXISTS (SELECT 1 FROM codespace_user_secret_repository WHERE codespace_user_secret_repository.secret_id = codespace_user_secret.id AND codespace_user_secret_repository.repo_id = ?))", userID, true, repoID).
		Asc("codespace_user_secret.name").Find(&secrets); err != nil {
		return nil, err
	}
	result := make([]RuntimeSecret, 0, len(secrets))
	var totalSize int64
	for _, secret := range secrets {
		value, err := secret_module.DecryptSecret(setting.SecretKey, secret.DataEncrypted)
		if err != nil {
			return nil, err
		}
		if err := validateUserSecret(secret.Name, value); err != nil {
			return nil, err
		}
		totalSize += int64(len(value))
		if totalSize > repositorySecretTotalSizeLimit {
			return nil, ErrUserSecretSizeLimit
		}
		result = append(result, RuntimeSecret{Name: secret.Name, Value: value})
	}
	return result, nil
}

func configureRecommendedSecrets(ctx context.Context, user *user_model.User, repo *repo_model.Repository, recommendations []CreateRecommendedSecret, values map[string]string, enabled map[string]bool) error {
	needsWrite := false
	for _, recommendation := range recommendations {
		if !recommendation.Available && (recommendation.Configured && enabled[recommendation.Name] || !recommendation.Configured && values[recommendation.Name] != "") {
			needsWrite = true
			break
		}
	}
	if !needsWrite {
		return nil
	}
	allowed, err := userCanUseSecretRepository(ctx, user, repo)
	if err != nil {
		return err
	}
	if !allowed {
		return util.NewPermissionDeniedErrorf("code write access is required")
	}
	for _, recommendation := range recommendations {
		if recommendation.Available {
			continue
		}
		if recommendation.Configured {
			if !enabled[recommendation.Name] {
				continue
			}
			secret := new(codespace_model.UserSecret)
			has, err := db.GetEngine(ctx).Where("user_id = ? AND name = ?", user.ID, recommendation.Name).Get(secret)
			if err != nil {
				return err
			}
			if !has {
				return ErrUserSecretNotFound
			}
			if secret.AllRepositories {
				continue
			}
			if err := bindUserSecretToRepository(ctx, repo, secret); err != nil {
				return err
			}
			continue
		}
		if value := values[recommendation.Name]; value != "" {
			if err := insertUserSecret(ctx, user, recommendation.Name, value, false, []*repo_model.Repository{repo}); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateUserSecret(name, value string) error {
	if err := validateUserSecretName(name); err != nil {
		return err
	}
	return validateUserSecretValue(value)
}

func validateUserSecretValue(value string) error {
	if value == "" || len(value) > userSecretValueSizeLimit || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%w: %w", ErrUserSecretValueInvalid, util.ErrInvalidArgument)
	}
	return nil
}

func validateUserSecretName(name string) error {
	if err := secret_service.ValidateName(name); err != nil {
		return fmt.Errorf("%w: %v", ErrUserSecretNameInvalid, err)
	}
	if strings.HasPrefix(name, "CODESPACE_") || name == "HOME" || name == "USER" || name == "LOGNAME" || name == "SHELL" || name == "PATH" || name == "TERM" || name == "COLORTERM" {
		return fmt.Errorf("%w: reserved name", ErrUserSecretNameInvalid)
	}
	return nil
}

func checkUserSecretScopeSize(ctx context.Context, userID, excludedSecretID, newSize int64, allRepositories bool, repoIDs []int64) error {
	var secrets []*codespace_model.UserSecret
	query := db.GetEngine(ctx).Where("user_id = ?", userID)
	if excludedSecretID > 0 {
		query = query.And("id <> ?", excludedSecretID)
	}
	if err := query.Find(&secrets); err != nil {
		return err
	}
	secretSizes := make(map[int64]int64, len(secrets))
	selectedSecretIDs := make([]int64, 0, len(secrets))
	var allRepositoriesTotal int64
	for _, secret := range secrets {
		secretSizes[secret.ID] = secret.DataSize
		if secret.AllRepositories {
			allRepositoriesTotal += secret.DataSize
		} else {
			selectedSecretIDs = append(selectedSecretIDs, secret.ID)
		}
	}
	if allRepositories {
		allRepositoriesTotal += newSize
	}
	if allRepositoriesTotal > repositorySecretTotalSizeLimit {
		return ErrUserSecretSizeLimit
	}
	totalsByRepository := make(map[int64]int64)
	if len(selectedSecretIDs) > 0 {
		var bindings []*codespace_model.UserSecretRepository
		if err := db.GetEngine(ctx).In("secret_id", selectedSecretIDs).Find(&bindings); err != nil {
			return err
		}
		for _, binding := range bindings {
			totalsByRepository[binding.RepoID] += secretSizes[binding.SecretID]
		}
	}
	if !allRepositories {
		for _, repoID := range repoIDs {
			totalsByRepository[repoID] += newSize
		}
	}
	for _, selectedTotal := range totalsByRepository {
		if allRepositoriesTotal+selectedTotal > repositorySecretTotalSizeLimit {
			return ErrUserSecretSizeLimit
		}
	}
	return nil
}

func getUserSecret(ctx context.Context, userID, secretID int64) (*codespace_model.UserSecret, error) {
	secret := new(codespace_model.UserSecret)
	has, err := db.GetEngine(ctx).ID(secretID).Where("user_id = ?", userID).Get(secret)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrUserSecretNotFound
	}
	return secret, nil
}

func loadWritableSecretRepositories(ctx context.Context, userID int64, repoIDs []int64) (*user_model.User, []*repo_model.Repository, error) {
	user, err := user_model.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if user.Type != user_model.UserTypeIndividual {
		return nil, nil, util.NewInvalidArgumentErrorf("individual user is required")
	}
	if len(repoIDs) == 0 {
		return user, nil, nil
	}
	repoMap, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return nil, nil, err
	}
	if len(repoMap) != len(repoIDs) {
		return nil, nil, repo_model.ErrRepoNotExist{}
	}
	repositories := make([]*repo_model.Repository, 0, len(repoIDs))
	for _, repoID := range repoIDs {
		repo := repoMap[repoID]
		allowed, err := userCanUseSecretRepository(ctx, user, repo)
		if err != nil {
			return nil, nil, err
		}
		if !allowed {
			return nil, nil, util.NewPermissionDeniedErrorf("code write access is required")
		}
		repositories = append(repositories, repo)
	}
	return user, repositories, nil
}

func insertUserSecret(ctx context.Context, user *user_model.User, name, value string, allRepositories bool, repositories []*repo_model.Repository) error {
	name = strings.ToUpper(strings.TrimSpace(name))
	if err := validateUserSecret(name, value); err != nil {
		return err
	}
	has, err := db.GetEngine(ctx).Where("user_id = ? AND name = ?", user.ID, name).Exist(new(codespace_model.UserSecret))
	if err != nil {
		return err
	}
	if has {
		return ErrUserSecretNameConflict
	}
	count, err := db.GetEngine(ctx).Where("user_id = ?", user.ID).Count(new(codespace_model.UserSecret))
	if err != nil {
		return err
	}
	if count >= userSecretCountLimit {
		return ErrUserSecretCountLimit
	}
	repoIDs := make([]int64, 0, len(repositories))
	for _, repo := range repositories {
		repoIDs = append(repoIDs, repo.ID)
	}
	if err := checkUserSecretScopeSize(ctx, user.ID, 0, int64(len(value)), allRepositories, repoIDs); err != nil {
		return err
	}
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, value)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	secret := &codespace_model.UserSecret{
		UserID: user.ID, Name: name, DataEncrypted: encrypted, DataSize: int64(len(value)),
		AllRepositories: allRepositories, CreatedUnix: now, UpdatedUnix: now,
	}
	if err := db.Insert(ctx, secret); err != nil {
		return err
	}
	return insertUserSecretRepositoryBindings(ctx, secret.ID, repositories)
}

func bindUserSecretToRepository(ctx context.Context, repo *repo_model.Repository, secret *codespace_model.UserSecret) error {
	if secret.AllRepositories {
		return nil
	}
	has, err := db.GetEngine(ctx).Where("secret_id = ? AND repo_id = ?", secret.ID, repo.ID).Exist(new(codespace_model.UserSecretRepository))
	if err != nil || has {
		return err
	}
	var bindings []*codespace_model.UserSecretRepository
	if err := db.GetEngine(ctx).Where("secret_id = ?", secret.ID).Find(&bindings); err != nil {
		return err
	}
	repoIDs := make([]int64, 0, len(bindings)+1)
	for _, binding := range bindings {
		repoIDs = append(repoIDs, binding.RepoID)
	}
	repoIDs = append(repoIDs, repo.ID)
	if err := checkUserSecretScopeSize(ctx, secret.UserID, secret.ID, secret.DataSize, false, repoIDs); err != nil {
		return err
	}
	return db.Insert(ctx, &codespace_model.UserSecretRepository{SecretID: secret.ID, RepoID: repo.ID})
}

func getUserSecretRepositoryIDs(ctx context.Context, secretID int64) ([]int64, error) {
	var bindings []*codespace_model.UserSecretRepository
	if err := db.GetEngine(ctx).Where("secret_id = ?", secretID).Find(&bindings); err != nil {
		return nil, err
	}
	repoIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		repoIDs = append(repoIDs, binding.RepoID)
	}
	return repoIDs, nil
}

func insertUserSecretRepositoryBindings(ctx context.Context, secretID int64, repositories []*repo_model.Repository) error {
	for _, repo := range repositories {
		if err := db.Insert(ctx, &codespace_model.UserSecretRepository{SecretID: secretID, RepoID: repo.ID}); err != nil {
			return err
		}
	}
	return nil
}

func normalizeUserSecretRepositoryIDs(repoIDs []int64) ([]int64, error) {
	repoIDs = slices.Clone(repoIDs)
	for _, repoID := range repoIDs {
		if repoID <= 0 {
			return nil, util.NewInvalidArgumentErrorf("invalid repository")
		}
	}
	slices.Sort(repoIDs)
	return slices.Compact(repoIDs), nil
}

func userCanUseSecretRepository(ctx context.Context, user *user_model.User, repo *repo_model.Repository) (bool, error) {
	if user == nil || user.ID <= 0 || user.Type != user_model.UserTypeIndividual || repo == nil || repo.ID <= 0 {
		return false, nil
	}
	return access_model.HasAccessUnit(ctx, user, repo, unit.TypeCode, perm.AccessModeWrite)
}

func withUserSecretRepositoryLocks(ctx context.Context, repoIDs []int64, fn func(context.Context) error) error {
	// Acquire repository locks in ID order so concurrent secret updates cannot deadlock.
	repoIDs = append([]int64(nil), repoIDs...)
	slices.Sort(repoIDs)
	repoIDs = slices.Compact(repoIDs)
	var lockNext func(context.Context, int) error
	lockNext = func(ctx context.Context, index int) error {
		if index == len(repoIDs) {
			return fn(ctx)
		}
		return globallock.LockAndDo(ctx, repository_service.WorkingLockKey(repoIDs[index]), func(ctx context.Context) error {
			return lockNext(ctx, index+1)
		})
	}
	return lockNext(ctx, 0)
}
