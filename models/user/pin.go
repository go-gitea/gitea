// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"

	"code.gitea.io/gitea/modules/json"
)

const maxPinnedRepos = 3

// GetPinnedRepositoryIDs returns all the repository IDs pinned by the given user or org. 
// If they've never pinned a repository, an empty array is returned.
func GetPinnedRepositoryIDs(ownerID int64) ([]int64, error) {
	pinnedstring, err := GetUserSetting(ownerID, PinnedRepositories)
	if err != nil {
		return nil, err
	}

	var parsedValues []int64
	if pinnedstring == "" {
		return parsedValues, nil
	}

	err = json.Unmarshal([]byte(pinnedstring), &parsedValues)

	if err != nil {
		return nil, err
	}

	return parsedValues, nil
}

func setPinnedRepositories(ownerID int64, repos []int64) error {
	stringed, err := json.Marshal(repos)
	if err != nil {
		return err
	}

	return SetUserSetting(ownerID, PinnedRepositories, string(stringed))
}

type TooManyPinnedReposError struct {
	count int
}

func (e *TooManyPinnedReposError) Error() string {
	return fmt.Sprintf("can pin at most %d repositories, %d pinned repositories is too much", maxPinnedRepos, e.count)
}

// PinRepos pin the specified repos for the given user or organization.
// The caller must ensure all repos belong to the owner.
func PinRepos(ownerID int64, repoIDs ...int64) error {
	repos, err := GetPinnedRepositoryIDs(ownerID)
	if err != nil {
		return err
	}
	newrepos := make([]int64, 0, len(repoIDs)+len(repos))

	repos = append(repos, repoIDs...)

	for _, toadd := range repos {
		alreadypresent := false
		for _, present := range newrepos {
			if toadd == present {
				alreadypresent = true
				break
			}
		}
		if !alreadypresent {
			newrepos = append(newrepos, toadd)
		}
	}
	if len(newrepos) > maxPinnedRepos {
		return &TooManyPinnedReposError{count: len(newrepos)}
	}
	return setPinnedRepositories(ownerID, newrepos)
}

// UnpinRepos unpin the given repositories for the given user or organization
func UnpinRepos(ownerID int64, repoIDs ...int64) error {
	prevRepos, err := GetPinnedRepositoryIDs(ownerID)
	if err != nil {
		return err
	}
	var nextRepos []int64

	for _, r := range prevRepos {
		keep := true
		for _, unp := range repoIDs {
			if r == unp {
				keep = false
				break
			}
		}
		if keep {
			nextRepos = append(nextRepos, r)
		}
	}

	return setPinnedRepositories(ownerID, nextRepos)
}
