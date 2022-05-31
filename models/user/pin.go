// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"

	"code.gitea.io/gitea/modules/json"
)

const maxPinnedRepos = 3

// Get all the repositories pinned by a user. If they've never
// set pinned repositories, an empty array is returned.
func GetPinnedRepositoryIDs(userID int64) ([]int64, error) {
	pinnedstring, err := GetUserSetting(userID, PinnedRepositories)
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

func setPinnedRepositories(userID int64, repos []int64) error {
	stringed, err := json.Marshal(repos)
	if err != nil {
		return err
	}

	return SetUserSetting(userID, PinnedRepositories, string(stringed))
}

type TooManyPinnedReposError struct {
	count int
}

func (e *TooManyPinnedReposError) Error() string {
	return fmt.Sprintf("can pin at most %d repositories, %d pinned repositories is too much", maxPinnedRepos, e.count)
}

// Add some repos to a user's pinned repositories.
// The caller must ensure all repos belong to the
// owner.
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

// Remove some repos from a user's pinned repositories.
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
