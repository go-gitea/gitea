package models

import (
	"code.gitea.io/gitea/modules/util"
)

func FilterConfidentialIssue(doer *User, list IssueList) (IssueList, int, error) {
	var cutoff int = 0
	var err error
	var allowed bool
	result := make(IssueList, len(list))

	for i := range list {
		if list[i].Confidential {
			allowed, err = UserAllowedToLookAtConfidentialIssue(doer, list[i])
			if err != nil {
				return nil, 0, err
			}
			if allowed {
				result[i-cutoff] = list[i]
			} else {
				cutoff += 1
			}
		} else {
			result[i-cutoff] = list[i]
		}

	}
	return result[:len(list)-cutoff], cutoff, nil
}

func UserAllowedToLookAtConfidentialIssue(doer *User, issue *Issue) (bool, error) {
	if doer == nil {
		return false, nil
	}

	// Issue Creator is allowed
	if issue.PosterID == doer.ID {
		return true, nil
	}

	// Assignees are allowed
	assignees, err := GetAssigneeIDsByIssue(issue.ID)
	if err != nil {
		return false, err
	}
	if util.IsInt64InSlice(doer.ID, assignees) {
		return true, nil
	}

	// RepoAdmins are allowed
	if err := issue.LoadRepo(); err != nil {
		return false, err
	}
	return IsUserRepoAdmin(issue.Repo, doer)
}
