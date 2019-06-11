// Copyright 2018 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"fmt"
	"net/http"
)

// CommitMeta contains meta information of a commit in terms of API.
type CommitMeta struct {
	URL string `json:"url"`
	SHA string `json:"sha"`
}

// CommitUser contains information of a user in the context of a commit.
type CommitUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Date  string `json:"date"`
}

// RepoCommit contains information of a commit in the context of a repository.
type RepoCommit struct {
	URL       string      `json:"url"`
	Author    *CommitUser `json:"author"`
	Committer *CommitUser `json:"committer"`
	Message   string      `json:"message"`
	Tree      *CommitMeta `json:"tree"`
}

// Commit contains information generated from a Git commit.
type Commit struct {
	*CommitMeta
	HTMLURL    string        `json:"html_url"`
	RepoCommit *RepoCommit   `json:"commit"`
	Author     *User         `json:"author"`
	Committer  *User         `json:"committer"`
	Parents    []*CommitMeta `json:"parents"`
}

func (c *Client) GetSingleCommit(user, repo, commitID string) (*Commit, error) {
	commit := new(Commit)
	return commit, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/commits/%s", user, repo, commitID), nil, nil, &commit)
}

func (c *Client) GetReferenceSHA(user, repo, ref string) (string, error) {
	data, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/commits/%s", user, repo, ref),
		http.Header{"Accept": []string{MediaApplicationSHA}}, nil)
	return string(data), err
}
