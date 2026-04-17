// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "code.gitea.io/gitea/modules/json"

type contributorUser User

type contributorPayload struct {
	*contributorUser
	CompatUserName string `json:"username"`
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	Contributions  int64  `json:"contributions"`
	Additions      int64  `json:"additions"`
	Deletions      int64  `json:"deletions"`
	Commits        int64  `json:"commits"`
	FilesChanged   int64  `json:"fileschanged"`
}

// Contributor represents a repository contributor.
// swagger:model
type Contributor struct {
	*User `json:",inline"`
	// Name of the contributor, used for anonymous contributors
	Name string `json:"name,omitempty"`
	// Email of the contributor, used for anonymous contributors
	Email string `json:"email,omitempty"`
	// Contributions is the number of commits made by the contributor for Github API compatibility
	Contributions int64 `json:"contributions"`
	// Additions is the number of lines added by the contributor
	Additions int64 `json:"additions"`
	// Deletions is the number of lines deleted by the contributor
	Deletions int64 `json:"deletions"`
	// Commits is the number of commits made by the contributor
	Commits int64 `json:"commits"`
	// FilesChanged is the number of files changed by the contributor
	FilesChanged int64 `json:"fileschanged"`
}

// MarshalJSON implements the json.Marshaler interface for Contributor.
func (c Contributor) MarshalJSON() ([]byte, error) {
	var user *contributorUser
	username := ""
	if c.User != nil {
		tmp := contributorUser(*c.User)
		user = &tmp
		username = c.User.UserName
	}

	return json.Marshal(contributorPayload{
		contributorUser: user,
		CompatUserName:  username,
		Name:            c.Name,
		Email:           c.Email,
		Contributions:   c.Contributions,
		Additions:       c.Additions,
		Deletions:       c.Deletions,
		Commits:         c.Commits,
		FilesChanged:    c.FilesChanged,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface for Contributor.
func (c *Contributor) UnmarshalJSON(data []byte) error {
	var parsed contributorPayload

	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	if parsed.contributorUser != nil {
		c.User = parseContributorUser(*parsed.contributorUser, parsed.CompatUserName)
	} else {
		c.User = parseContributorUser(contributorUser{}, parsed.CompatUserName)
	}
	c.Name = parsed.Name
	c.Email = parsed.Email
	c.Contributions = parsed.Contributions
	c.Additions = parsed.Additions
	c.Deletions = parsed.Deletions
	c.Commits = parsed.Commits
	c.FilesChanged = parsed.FilesChanged

	return nil
}

func parseContributorUser(user contributorUser, compatUserName string) *User {
	if user == (contributorUser{}) && compatUserName == "" {
		return nil
	}
	if user.UserName == "" {
		user.UserName = compatUserName
	}
	res := User(user)
	return &res
}
