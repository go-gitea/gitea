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
	Path      string
	Selected  bool
}

// createDevContainerPlan contains the immutable runtime choice and confirmation data.
type createDevContainerPlan struct {
	Source                 string
	Selection              string
	Path                   string
	Name                   string
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

	configs := make([]*createDevContainerPlan, 0, len(paths))
	for _, configPath := range paths {
		config, err := loadRepositoryDevContainer(ctx, gitRepo, commit, configPath)
		if err != nil {
			return nil, nil, err
		}
		configs = append(configs, config)
	}
	templates, err := listVisibleDevContainerTemplates(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	templateConfigs := make([]*createDevContainerPlan, 0, len(templates))
	for _, template := range templates {
		config, err := loadTemplateDevContainer(template)
		if err != nil {
			return nil, nil, err
		}
		templateConfigs = append(templateConfigs, config)
	}

	selection = strings.TrimSpace(selection)
	if selection == "" {
		if slices.Contains(paths, devContainerPrimaryPath) {
			selection = devContainerPrimaryPath
		} else if slices.Contains(paths, devContainerRootPath) {
			selection = devContainerRootPath
		} else if len(configs) > 0 {
			selection = configs[0].Path
		} else if len(templateConfigs) > 0 {
			selection = templateConfigs[0].Selection
		}
	}

	var selected *createDevContainerPlan
	if index := slices.IndexFunc(configs, func(config *createDevContainerPlan) bool {
		return config.Path == selection
	}); index >= 0 {
		selected = configs[index]
	} else if index := slices.IndexFunc(templateConfigs, func(config *createDevContainerPlan) bool {
		return config.Selection == selection
	}); index >= 0 {
		selected = templateConfigs[index]
	} else {
		return nil, nil, fmt.Errorf("Dev Container configuration %q is not available", selection)
	}
	permissions, err := resolveCreatePermissions(ctx, user, repo, selected.PermissionRepositories)
	if err != nil {
		return nil, nil, err
	}
	selected.Permissions = permissions

	options := make([]CreateDevContainerOption, 0, len(configs)+len(templateConfigs))
	for _, config := range configs {
		options = append(options, CreateDevContainerOption{
			Selection: config.Path,
			Name:      config.Name,
			Path:      config.Path,
			Selected:  selection == config.Path,
		})
	}
	for _, config := range templateConfigs {
		options = append(options, CreateDevContainerOption{
			Selection: config.Selection,
			Name:      config.Name,
			Selected:  selection == config.Selection,
		})
	}
	return selected, options, nil
}

func discoverDevContainerPaths(ctx context.Context, gitRepo *git.Repository, commit *git.Commit) ([]string, error) {
	paths := make([]string, 0, 4)
	for _, configPath := range []string{devContainerPrimaryPath, devContainerRootPath} {
		entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, configPath)
		if err == nil {
			if !entry.IsRegular() {
				return nil, fmt.Errorf("Dev Container configuration %q must be a regular file", configPath)
			}
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
		configEntry, err := commit.GetTreeEntryByPath(ctx, gitRepo, configPath)
		if err != nil {
			if git.IsErrNotExist(err) || errors.Is(err, util.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if !configEntry.IsRegular() {
			return nil, fmt.Errorf("Dev Container configuration %q must be a regular file", configPath)
		}
		paths = append(paths, configPath)
		if len(paths) > maxDevContainerConfigurations {
			return nil, fmt.Errorf("repository has more than %d Dev Container configurations", maxDevContainerConfigurations)
		}
	}
	return paths, nil
}

func loadRepositoryDevContainer(ctx context.Context, gitRepo *git.Repository, commit *git.Commit, configPath string) (*createDevContainerPlan, error) {
	blob, err := commit.GetBlobByPath(ctx, gitRepo, configPath)
	if err != nil {
		return nil, err
	}
	if blob.Size(ctx) > devContainerConfigMaxSize {
		return nil, fmt.Errorf("Dev Container configuration %q exceeds %d bytes", configPath, devContainerConfigMaxSize)
	}
	content, err := blob.GetBlobBytes(ctx, devContainerConfigMaxSize+1)
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > devContainerConfigMaxSize {
		return nil, fmt.Errorf("Dev Container configuration %q exceeds %d bytes", configPath, devContainerConfigMaxSize)
	}
	document, err := parseDevContainerDocument(content, configPath)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(document.Name)
	if name == "" {
		name = configPath
	}
	repositories, err := devContainerPermissionRepositories(document.Customizations)
	if err != nil {
		return nil, fmt.Errorf("parse Dev Container configuration %q: %w", configPath, err)
	}
	recommendedSecrets, err := parseRecommendedSecrets(document.Secrets)
	if err != nil {
		return nil, fmt.Errorf("parse Dev Container configuration %q: %w", configPath, err)
	}
	return &createDevContainerPlan{
		Source:                 codespace_model.DevContainerSourceRepository,
		Selection:              configPath,
		Path:                   configPath,
		Name:                   name,
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
		Name:                   template.Name,
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
