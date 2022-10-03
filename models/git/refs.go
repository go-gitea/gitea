// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"strings"

	"code.gitea.io/gitea/modules/git"
)

// CheckReferenceEditability checks if the reference can be modified by the user or any user
func CheckReferenceEditability(refName, commitID string, repoID, userID int64) error {
	refParts := strings.Split(refName, "/")

	// Must have at least 3 parts, e.g. refs/heads/new-branch
	if len(refParts) <= 2 {
		return git.ErrInvalidRefName{
			RefName: refName,
			Reason:  "reference name must contain at least three slash-separted components",
		}
	}

	// Must start with 'refs/'
	if refParts[0] != "refs/" {
		return git.ErrInvalidRefName{
			RefName: refName,
			Reason:  "reference must start with 'refs/'",
		}
	}

	// 'refs/pull/*' is not allowed
	if refParts[1] == "pull" {
		return git.ErrInvalidRefName{
			RefName: refName,
			Reason:  "refs/pull/* is read-only",
		}
	}

	if refParts[1] == "tags" {
		// If the 2nd part is "tags" then we need ot make sure the user is allowed to
		//   modify this tag (not protected or is admin)
		if protectedTags, err := GetProtectedTags(repoID); err == nil {
			isAllowed, err := IsUserAllowedToControlTag(protectedTags, refName, userID)
			if err != nil {
				return err
			}
			if !isAllowed {
				return git.ErrProtectedRefName{
					RefName: refName,
					Message: "you're not authorized to change a protected tag",
				}
			}
		}
	} else if refParts[1] == "heads" {
		// If the 2nd part is "heas" then we need to make sure the user is allowed to
		//   modify this branch (not protected or is admin)
		isProtected, err := IsProtectedBranch(repoID, refName)
		if err != nil {
			return err
		}
		if !isProtected {
			return git.ErrProtectedRefName{
				RefName: refName,
				Message: "changes must be made through a pull request",
			}
		}
	}

	return nil
}
