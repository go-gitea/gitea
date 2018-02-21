// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "fmt"

// ActionList defines a list of actions
type ActionList []*Action

func (actions ActionList) getUserIDs() []int64 {
	userIDs := make(map[int64]struct{}, len(actions))
	for _, action := range actions {
		if _, ok := userIDs[action.ActUserID]; !ok {
			userIDs[action.ActUserID] = struct{}{}
		}
	}
	return keysInt64(userIDs)
}

func (actions ActionList) loadUsers(e Engine) ([]*User, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	userIDs := actions.getUserIDs()
	userMaps := make(map[int64]*User, len(userIDs))
	err := e.
		In("id", userIDs).
		Find(&userMaps)
	if err != nil {
		return nil, fmt.Errorf("find user: %v", err)
	}

	for _, action := range actions {
		action.ActUser = userMaps[action.ActUserID]
	}
	return valuesUser(userMaps), nil
}

// LoadUsers loads actions' all users
func (actions ActionList) LoadUsers() ([]*User, error) {
	return actions.loadUsers(x)
}

func (actions ActionList) getRepoIDs() []int64 {
	repoIDs := make(map[int64]struct{}, len(actions))
	for _, action := range actions {
		if _, ok := repoIDs[action.RepoID]; !ok {
			repoIDs[action.RepoID] = struct{}{}
		}
	}
	return keysInt64(repoIDs)
}

func (actions ActionList) loadRepositories(e Engine) ([]*Repository, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	repoIDs := actions.getRepoIDs()
	repoMaps := make(map[int64]*Repository, len(repoIDs))
	err := e.
		In("id", repoIDs).
		Find(&repoMaps)
	if err != nil {
		return nil, fmt.Errorf("find repository: %v", err)
	}

	for _, action := range actions {
		action.Repo = repoMaps[action.RepoID]
	}
	return valuesRepository(repoMaps), nil
}

// LoadRepositories loads actions' all repositories
func (actions ActionList) LoadRepositories() ([]*Repository, error) {
	return actions.loadRepositories(x)
}

// loadAttributes loads all attributes
func (actions ActionList) loadAttributes(e Engine) (err error) {
	if _, err = actions.loadUsers(e); err != nil {
		return
	}

	if _, err = actions.loadRepositories(e); err != nil {
		return
	}

	return nil
}

// LoadAttributes loads attributes of the actions
func (actions ActionList) LoadAttributes() error {
	return actions.loadAttributes(x)
}
