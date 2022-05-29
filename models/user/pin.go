// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"encoding/json"
	"fmt"
)

const maxPinnedRepos = 3

func GetPinnedRepositoryIDs(userID int64) ([]int64, error) {
	pinnedstring, err := GetUserSetting(userID, PinnedRepositories)

	if err != nil {
		return nil, err
	}

	if len(pinnedstring) == 0 {
		return nil, nil
	}
	var parsedValues []int64

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

func PinRepos(ownerID int64, repoIDs ...int64) error {

	repos, err := GetPinnedRepositoryIDs(ownerID)

	if err != nil {
		return err
	}
	newrepos := make([]int64, 0, len(repoIDs)+len(repos))

	allrepos := append(repos, repoIDs...)

	for _, toadd := range allrepos {
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
