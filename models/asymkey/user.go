// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package asymkey

import (
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
)

// User represents a commit User with a database ID
type User struct {
	ID int64
	git.Signature
}

// UserCommit represents a commit with validation of user.
type UserCommit struct { //revive:disable-line:exported
	User *User
	*git.Commit
}

// ValidateCommitWithEmail check if author's e-mail of commit is corresponding to a user.
func ValidateCommitWithEmail(c *git.Commit) *User {
	if c.Author == nil {
		return nil
	}
	u, err := user_model.GetUserByEmail(c.Author.Email)
	if err != nil {
		return nil
	}
	return &User{
		ID:        u.ID,
		Signature: *c.Author,
	}
}

// ValidateCommitsWithEmails checks if authors' e-mails of commits are corresponding to users.
func ValidateCommitsWithEmails(oldCommits []*git.Commit) ([]*UserCommit, error) {
	var (
		emails     = make(map[string]*User)
		newCommits = make([]*UserCommit, 0, len(oldCommits))
	)
	for _, c := range oldCommits {
		var u *User
		if c.Author != nil {
			if v, ok := emails[c.Author.Email]; !ok {
				user, err := user_model.GetUserByEmail(c.Author.Email)
				if err != nil {
					return nil, err
				}
				emails[c.Author.Email] = &User{
					ID:        user.ID,
					Signature: *c.Author,
				}
			} else {
				u = v
			}
		}

		newCommits = append(newCommits, &UserCommit{
			User:   u,
			Commit: c,
		})
	}
	return newCommits, nil
}
