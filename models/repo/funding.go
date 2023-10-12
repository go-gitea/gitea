// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"gopkg.in/yaml.v3"
)

var FundingCandidates = []string{
	".gitea/FUNDING",
	".github/FUNDING",
}

func getFundingEntry(provider *api.FundingProvider, text string) *api.RepoFundingEntry {
	entry := new(api.RepoFundingEntry)
	entry.Text = fmt.Sprintf(provider.Text, text)
	entry.URL = fmt.Sprintf(provider.URL, text)

	if provider.Icon != "" {
		entry.Icon = setting.AppSubURL + "/assets/" + provider.Icon
	}

	return entry
}

// GetFundingFromPath the given funding file.
// It never returns a nil config.
func GetFundingFromPath(r *Repository, path string, commit *git.Commit) ([]*api.RepoFundingEntry, error) {
	var err error

	treeEntry, err := commit.GetTreeEntryByPath(path)
	if err != nil {
		return nil, err
	}

	reader, err := treeEntry.Blob().DataAsync()
	if err != nil {
		log.Debug("DataAsync: %v", err)
		return nil, nil
	}

	defer reader.Close()

	configContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	fundingMap := make(map[string]any)
	if err := yaml.Unmarshal(configContent, &fundingMap); err != nil {
		return nil, err
	}

	entryList := make([]*api.RepoFundingEntry, 0)
	for providerName, fundingData := range fundingMap {
		provider := setting.GetFundingProviderByName(providerName)
		if provider == nil {
			return nil, fmt.Errorf("Funding Provider %s not found", providerName)
		}

		dataType := reflect.TypeOf(fundingData)
		switch dataType.Kind() {
		case reflect.String:
			entryList = append(entryList, getFundingEntry(provider, fundingData.(string)))
		case reflect.Slice:
			stringSlice := reflect.ValueOf(fundingData)
			for i := 0; i < stringSlice.Len(); i++ {
				str, ok := stringSlice.Index(i).Interface().(string)
				if !ok {
					return nil, fmt.Errorf("%s has a invalid type. Expected string or string array.", providerName)
				}
				entryList = append(entryList, getFundingEntry(provider, str))
			}
		default:
			return nil, fmt.Errorf("%s has a invalid type. Expected string or string array.", providerName)
		}
	}

	return entryList, nil
}

func GetFundingFromCommit(r *Repository, commit *git.Commit) ([]*api.RepoFundingEntry, error) {
	for _, configName := range FundingCandidates {
		if _, err := commit.GetTreeEntryByPath(configName + ".yaml"); err == nil {
			return GetFundingFromPath(r, configName+".yaml", commit)
		}

		if _, err := commit.GetTreeEntryByPath(configName + ".yml"); err == nil {
			return GetFundingFromPath(r, configName+".yml", commit)
		}
	}

	return nil, nil
}

// GetFundingFromDefaultBranch returns the funding for this repo.
// It never returns a nil config.
func GetFundingFromDefaultBranch(ctx context.Context, r *Repository) ([]*api.RepoFundingEntry, error) {
	if r.IsEmpty {
		return nil, nil
	}

	gitRepo, err := git.OpenRepository(ctx, r.RepoPath())
	if err != nil {
		return nil, err
	}

	commit, err := gitRepo.GetBranchCommit(r.DefaultBranch)
	if err != nil {
		return nil, err
	}

	return GetFundingFromCommit(r, commit)
}

// IsFundingConfig returns if the given path is a funding config.
func IsFundingConfig(path string) bool {
	for _, name := range FundingCandidates {
		if path == name+".yaml" || path == name+".yml" {
			return true
		}
	}
	return false
}
