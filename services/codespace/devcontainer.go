// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/util"

	"github.com/tailscale/hujson"
)

const (
	devContainerTemplateSelectionPrefix   = "template:"
	devContainerPrimaryPath               = ".devcontainer/devcontainer.json"
	devContainerRootPath                  = ".devcontainer.json"
	maxDevContainerConfigurations         = 32
	maxCodespacePermissionRepositories    = 32
	maxCodespacePermissionRepositoryRules = 128
	maxRecommendedSecrets                 = 100
)

// CreateDevContainerOption describes one configuration available at the selected commit.
type CreateDevContainerOption struct {
	Selection string
	Name      string
	Scope     string
	Selected  bool
}

// ErrCreateConfigurationInvalid allows the confirmation page to offer another configuration.
var ErrCreateConfigurationInvalid = errors.New("selected Dev Container configuration is invalid")

// createDevContainerPlan contains the immutable runtime choice and confirmation data.
type createDevContainerPlan struct {
	Source                 string
	Selection              string
	Path                   string
	Content                string
	PermissionRepositories map[string]map[string]string
	Permissions            []CreatePermissionRequest
	RecommendedSecrets     []CreateRecommendedSecret
}

type devContainerDocument struct {
	Name           string                        `json:"name"`
	Image          string                        `json:"image"`
	Build          any                           `json:"build"`
	DockerFile     string                        `json:"dockerFile"`
	DockerCompose  any                           `json:"dockerComposeFile"`
	Features       map[string]any                `json:"features"`
	Mounts         []devContainerMount           `json:"mounts"`
	Secrets        map[string]devContainerSecret `json:"secrets"`
	Customizations devContainerCustomizations    `json:"customizations"`
}

type devContainerMount struct {
	Type   string `json:"type"`
	Source string `json:"source"`
}

type devContainerSecret struct {
	Description string `json:"description"`
}

type devContainerCustomizations struct {
	Gitea devContainerGiteaCustomization `json:"gitea"`
}

type devContainerGiteaCustomization struct {
	Repositories map[string]devContainerRepositoryPermission `json:"repositories"`
}

type devContainerRepositoryPermission struct {
	Permissions map[string]string `json:"permissions"`
}

func prepareCreateDevContainer(ctx context.Context, user *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, sourceRef *createSourceRef, selection string) (*createDevContainerPlan, []CreateDevContainerOption, error) {
	commit, err := gitRepo.GetCommit(ctx, sourceRef.CommitSHA)
	if err != nil {
		return nil, nil, fmt.Errorf("load Dev Container commit: %w", err)
	}
	paths, err := discoverDevContainerPaths(ctx, gitRepo, commit)
	if err != nil {
		return nil, nil, err
	}

	templates, err := listVisibleDevContainerTemplates(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	selection = strings.TrimSpace(selection)
	if selection == "" {
		if slices.Contains(paths, devContainerPrimaryPath) {
			selection = devContainerPrimaryPath
		} else if slices.Contains(paths, devContainerRootPath) {
			selection = devContainerRootPath
		} else if len(paths) > 0 {
			selection = paths[0]
		} else if len(templates) > 0 {
			selection = devContainerTemplateSelectionPrefix + strconv.FormatInt(templates[0].ID, 10)
		}
	}

	options := make([]CreateDevContainerOption, 0, len(paths)+len(templates))
	for _, configPath := range paths {
		options = append(options, CreateDevContainerOption{
			Selection: configPath,
			Name:      configPath,
			Scope:     "repository",
			Selected:  selection == configPath,
		})
	}
	for _, template := range templates {
		key := devContainerTemplateSelectionPrefix + strconv.FormatInt(template.ID, 10)
		scope := "personal"
		if template.UserID == 0 {
			scope = "site"
		}
		options = append(options, CreateDevContainerOption{
			Selection: key,
			Name:      template.Name,
			Scope:     scope,
			Selected:  selection == key,
		})
	}

	var selected *createDevContainerPlan
	if slices.Contains(paths, selection) {
		selected, err = loadRepositoryDevContainer(ctx, gitRepo, commit, selection)
	} else if index := slices.IndexFunc(templates, func(template *codespace_model.DevContainerTemplate) bool {
		return devContainerTemplateSelectionPrefix+strconv.FormatInt(template.ID, 10) == selection
	}); index >= 0 {
		selected, err = loadTemplateDevContainer(templates[index])
		if err != nil {
			err = errors.Join(ErrCreateConfigurationInvalid, err)
		}
	} else {
		err = ErrCreateConfigurationInvalid
	}
	if err != nil {
		return nil, options, err
	}
	permissions, err := resolveCreatePermissions(ctx, user, repo, selected.PermissionRepositories)
	if err != nil {
		return nil, options, err
	}
	selected.Permissions = permissions
	return selected, options, nil
}

func discoverDevContainerPaths(ctx context.Context, gitRepo *git.Repository, commit *git.Commit) ([]string, error) {
	paths := make([]string, 0, 4)
	for _, configPath := range []string{devContainerPrimaryPath, devContainerRootPath} {
		_, err := commit.GetTreeEntryByPath(ctx, gitRepo, configPath)
		if err == nil {
			paths = append(paths, configPath)
			continue
		}
		if !git.IsErrNotExist(err) && !errors.Is(err, util.ErrNotExist) {
			return nil, err
		}
	}

	root, err := commit.Tree().SubTree(ctx, gitRepo, ".devcontainer")
	if err != nil {
		if git.IsErrNotExist(err) || errors.Is(err, util.ErrNotExist) {
			return paths, nil
		}
		return nil, err
	}
	entries, err := root.ListEntries(ctx, gitRepo)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b *git.TreeEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		configPath := path.Join(".devcontainer", entry.Name(), "devcontainer.json")
		_, err := commit.GetTreeEntryByPath(ctx, gitRepo, configPath)
		if err != nil {
			if git.IsErrNotExist(err) || errors.Is(err, util.ErrNotExist) {
				continue
			}
			return nil, err
		}
		paths = append(paths, configPath)
		if len(paths) > maxDevContainerConfigurations {
			return nil, fmt.Errorf("repository has more than %d Dev Container configurations", maxDevContainerConfigurations)
		}
	}
	return paths, nil
}

func loadRepositoryDevContainer(ctx context.Context, gitRepo *git.Repository, commit *git.Commit, configPath string) (*createDevContainerPlan, error) {
	entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, configPath)
	if err != nil {
		return nil, err
	}
	if !entry.IsRegular() {
		return nil, fmt.Errorf("%w: configuration %q must be a regular file", ErrCreateConfigurationInvalid, configPath)
	}
	blob, err := commit.GetBlobByPath(ctx, gitRepo, configPath)
	if err != nil {
		return nil, err
	}
	if blob.Size(ctx) > devContainerConfigMaxSize {
		return nil, fmt.Errorf("%w: configuration %q exceeds %d bytes", ErrCreateConfigurationInvalid, configPath, devContainerConfigMaxSize)
	}
	content, err := blob.GetBlobBytes(ctx, devContainerConfigMaxSize+1)
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > devContainerConfigMaxSize {
		return nil, fmt.Errorf("%w: configuration %q exceeds %d bytes", ErrCreateConfigurationInvalid, configPath, devContainerConfigMaxSize)
	}
	document, err := parseDevContainerDocument(content, configPath)
	if err != nil {
		return nil, errors.Join(ErrCreateConfigurationInvalid, err)
	}
	repositories, err := devContainerPermissionRepositories(document.Customizations)
	if err != nil {
		return nil, fmt.Errorf("%w: parse Dev Container configuration %q: %w", ErrCreateConfigurationInvalid, configPath, err)
	}
	recommendedSecrets, err := parseRecommendedSecrets(document.Secrets)
	if err != nil {
		return nil, fmt.Errorf("%w: parse Dev Container configuration %q: %w", ErrCreateConfigurationInvalid, configPath, err)
	}
	return &createDevContainerPlan{
		Source:                 codespace_model.DevContainerSourceRepository,
		Selection:              configPath,
		Path:                   configPath,
		PermissionRepositories: repositories,
		RecommendedSecrets:     recommendedSecrets,
	}, nil
}

func loadTemplateDevContainer(template *codespace_model.DevContainerTemplate) (*createDevContainerPlan, error) {
	content := strings.TrimSpace(template.Content)
	document, err := parseDevContainerDocument([]byte(content), template.Name)
	if err != nil {
		return nil, err
	}
	if err := validateTemplateDevContainer(document); err != nil {
		return nil, fmt.Errorf("parse Dev Container template %q: %w", template.Name, err)
	}
	repositories, err := devContainerPermissionRepositories(document.Customizations)
	if err != nil {
		return nil, fmt.Errorf("parse Dev Container template %q: %w", template.Name, err)
	}
	recommendedSecrets, err := parseRecommendedSecrets(document.Secrets)
	if err != nil {
		return nil, fmt.Errorf("parse Dev Container template %q: %w", template.Name, err)
	}
	return &createDevContainerPlan{
		Source:                 codespace_model.DevContainerSourceTemplate,
		Selection:              devContainerTemplateSelectionPrefix + strconv.FormatInt(template.ID, 10),
		Content:                content,
		PermissionRepositories: repositories,
		RecommendedSecrets:     recommendedSecrets,
	}, nil
}

func parseDevContainerDocument(content []byte, name string) (*devContainerDocument, error) {
	if int64(len(content)) > devContainerConfigMaxSize {
		return nil, fmt.Errorf("Dev Container configuration %q exceeds %d bytes", name, devContainerConfigMaxSize)
	}
	standard, err := hujson.Standardize(content)
	if err != nil {
		return nil, fmt.Errorf("parse Dev Container configuration %q: %w", name, err)
	}
	var document devContainerDocument
	if err := json.Unmarshal(standard, &document); err != nil {
		return nil, fmt.Errorf("parse Dev Container configuration %q: %w", name, err)
	}
	return &document, nil
}

func validateTemplateDevContainer(document *devContainerDocument) error {
	if strings.TrimSpace(document.Image) == "" || document.Build != nil || strings.TrimSpace(document.DockerFile) != "" || document.DockerCompose != nil {
		return errors.New("template must select an image")
	}
	for reference := range document.Features {
		if strings.HasPrefix(reference, "./") || strings.HasPrefix(reference, "../") {
			return errors.New("template cannot use relative Features")
		}
	}
	for _, mount := range document.Mounts {
		source := strings.TrimSpace(mount.Source)
		if source == "" || strings.Contains(source, "${") {
			continue
		}
		if mount.Type == "bind" && !path.IsAbs(source) {
			return errors.New("template cannot use relative bind mounts")
		}
	}
	return nil
}

func parseRecommendedSecrets(configured map[string]devContainerSecret) ([]CreateRecommendedSecret, error) {
	if len(configured) > maxRecommendedSecrets {
		return nil, fmt.Errorf("Dev Container recommends more than %d secrets", maxRecommendedSecrets)
	}
	secrets := make([]CreateRecommendedSecret, 0, len(configured))
	seen := make(map[string]struct{}, len(configured))
	for rawName, configuredSecret := range configured {
		name := strings.ToUpper(strings.TrimSpace(rawName))
		if err := validateUserSecretName(name); err != nil {
			return nil, fmt.Errorf("invalid recommended secret %q: %w", rawName, err)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate recommended secret %q", name)
		}
		seen[name] = struct{}{}
		secrets = append(secrets, CreateRecommendedSecret{Name: name, Description: strings.TrimSpace(configuredSecret.Description)})
	}
	sort.Slice(secrets, func(i, j int) bool { return secrets[i].Name < secrets[j].Name })
	return secrets, nil
}

func resolveCreateSecrets(ctx context.Context, userID, repoID int64, recommendations []CreateRecommendedSecret) ([]CreateRecommendedSecret, []CreateSecretSummary, error) {
	var secrets []*codespace_model.UserSecret
	if err := db.GetEngine(ctx).Cols("id", "name", "all_repositories").Where("user_id = ?", userID).Asc("name").Find(&secrets); err != nil {
		return nil, nil, err
	}
	byName := make(map[string]*codespace_model.UserSecret, len(secrets))
	secretIDs := make([]int64, 0, len(secrets))
	for _, secret := range secrets {
		byName[secret.Name] = secret
		secretIDs = append(secretIDs, secret.ID)
	}
	availableSecretIDs := make(map[int64]struct{}, len(secrets))
	if len(secretIDs) > 0 {
		var bindings []*codespace_model.UserSecretRepository
		if err := db.GetEngine(ctx).In("secret_id", secretIDs).Where("repo_id = ?", repoID).Find(&bindings); err != nil {
			return nil, nil, err
		}
		for _, binding := range bindings {
			availableSecretIDs[binding.SecretID] = struct{}{}
		}
	}
	for i := range recommendations {
		secret := byName[recommendations[i].Name]
		if secret == nil {
			continue
		}
		recommendations[i].Configured = true
		_, selected := availableSecretIDs[secret.ID]
		recommendations[i].Available = secret.AllRepositories || selected
	}
	descriptions := make(map[string]string, len(recommendations))
	for _, recommendation := range recommendations {
		descriptions[recommendation.Name] = recommendation.Description
	}
	available := make([]CreateSecretSummary, 0, len(secrets))
	for _, secret := range secrets {
		_, selected := availableSecretIDs[secret.ID]
		if secret.AllRepositories || selected {
			available = append(available, CreateSecretSummary{Name: secret.Name, Description: descriptions[secret.Name]})
		}
	}
	return recommendations, available, nil
}

func devContainerPermissionRepositories(customizations devContainerCustomizations) (map[string]map[string]string, error) {
	if len(customizations.Gitea.Repositories) > maxCodespacePermissionRepositories {
		return nil, fmt.Errorf("Gitea customization requests more than %d repositories", maxCodespacePermissionRepositories)
	}
	repositories := make(map[string]map[string]string, len(customizations.Gitea.Repositories))
	ruleCount := 0
	for name, repository := range customizations.Gitea.Repositories {
		name = strings.TrimSpace(name)
		ownerName, repoName, ok := strings.Cut(name, "/")
		if !ok || ownerName == "" || repoName == "" || strings.Contains(repoName, "/") {
			return nil, fmt.Errorf("invalid Gitea permission repository %q", name)
		}
		if _, exists := repositories[name]; exists {
			return nil, fmt.Errorf("duplicate Gitea permission repository %q", name)
		}
		if len(repository.Permissions) == 0 {
			return nil, fmt.Errorf("Gitea permission repository %q must contain permissions", name)
		}
		ruleCount += len(repository.Permissions)
		if ruleCount > maxCodespacePermissionRepositoryRules {
			return nil, fmt.Errorf("Gitea customization requests more than %d repository permissions", maxCodespacePermissionRepositoryRules)
		}
		permissions := make(map[string]string, len(repository.Permissions))
		for unitName, modeName := range repository.Permissions {
			unitName = strings.ToLower(strings.TrimSpace(unitName))
			modeName = strings.ToLower(strings.TrimSpace(modeName))
			if _, exists := permissions[unitName]; exists {
				return nil, fmt.Errorf("duplicate Gitea permission unit %q for %q", unitName, name)
			}
			if _, ok := codespacePermissionUnits[unitName]; !ok {
				return nil, fmt.Errorf("unsupported Gitea permission unit %q", unitName)
			}
			if modeName != "read" && modeName != "write" {
				return nil, fmt.Errorf("Gitea permission %q for %q must be read or write", unitName, name)
			}
			permissions[unitName] = modeName
		}
		repositories[name] = permissions
	}
	return repositories, nil
}
