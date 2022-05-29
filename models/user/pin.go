// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"encoding/json"
)

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

func PinRepo(ownerID, repoID int64) error {

	repos, err := GetPinnedRepositoryIDs(ownerID)

	if err != nil {
		return err
	}
	alreadyPresent := false
	for _, r := range repos {
		if r == repoID {
			alreadyPresent = true
			break
		}
	}

	if !alreadyPresent {
		repos = append(repos, repoID)
	}

	return setPinnedRepositories(ownerID, repos)
}

func UnpinRepo(ownerID, repoID int64) error {

	prevRepos, err := GetPinnedRepositoryIDs(ownerID)
	if err != nil {
		return err
	}
	var nextRepos []int64

	for _, r := range prevRepos {
		if r != repoID {
			nextRepos = append(nextRepos, r)
		}
	}

	return setPinnedRepositories(ownerID, nextRepos)
}
