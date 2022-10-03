// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitref

import (
	"fmt"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
)

// GetReference gets the Reference object that a refName refers to
func GetReference(gitRepo *git.Repository, refName string) (*git.Reference, error) {
	refs, err := gitRepo.GetRefsFiltered(refName)
	if err != nil {
		return nil, err
	}
	var ref *git.Reference
	for _, ref = range refs {
		if ref.Name == refName {
			return ref, nil
		}
	}
	return nil, git.ErrRefNotFound{RefName: refName}
}

// UpdateReferenceWithChecks creates or updates a reference, checking for format, permissions and special cases
func UpdateReferenceWithChecks(ctx *context.APIContext, refName, commitID string) (*git.Reference, error) {
	err := CheckReferenceEditability(refName, commitID, ctx.Repo.Repository.ID, ctx.Doer.ID)
	if err != nil {
		return nil, err
	}

	if err := ctx.Repo.GitRepo.SetReference(refName, commitID); err != nil {
		message := err.Error()
		prefix := fmt.Sprintf("exit status 128 - fatal: update_ref failed for ref '%s': ", refName)
		if strings.HasPrefix(message, prefix) {
			return nil, fmt.Errorf(strings.TrimRight(strings.TrimPrefix(message, prefix), "\n"))
		}
		return nil, err
	}

	return ctx.Repo.GitRepo.GetReference(refName)
}

// RemoveReferenceWithChecks deletes a reference, checking for format, permission and special cases
func RemoveReferenceWithChecks(ctx *context.APIContext, refName string) error {
	err := CheckReferenceEditability(refName, "", ctx.Repo.Repository.ID, ctx.Doer.ID)
	if err != nil {
		return err
	}

	return ctx.Repo.GitRepo.RemoveReference(refName)
}

func CheckReferenceEditability(refName, commitID string, repoID, userID int64) error {
	refParts := strings.Split(refName, "/")

	// Must have at least 3 parts, e.g. refs/heads/new-branch
	if len(refParts) <= 2 {
		return git.ErrInvalidRefName{
			RefName: refName,
			Reason:  "reference name must contain at least three slash-separted components",
		}
	}

	refPrefix := refParts[0]
	refType := refParts[2]
	refRest := strings.Join(refParts[2:], "/")

	// Must start with 'refs/'
	if refPrefix != "refs" {
		return git.ErrInvalidRefName{
			RefName: refName,
			Reason:  "reference must start with 'refs/'",
		}
	}

	// 'refs/pull/*' is not allowed
	if refType == "pull" {
		return git.ErrInvalidRefName{
			RefName: refName,
			Reason:  "refs/pull/* is read-only",
		}
	}

	if refType == "tags" {
		// If the 2nd part is "tags" then we need ot make sure the user is allowed to
		//   modify this tag (not protected or is admin)
		if protectedTags, err := git_model.GetProtectedTags(repoID); err == nil {
			isAllowed, err := git_model.IsUserAllowedToControlTag(protectedTags, refRest, userID)
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
	}

	if refType == "heads" {
		// If the 2nd part is "heas" then we need to make sure the user is allowed to
		//   modify this branch (not protected or is admin)
		isProtected, err := git_model.IsProtectedBranch(repoID, refRest)
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
