// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
)

// PullRequestType defines pull request type
type PullRequestType int

// Enumerate all the pull request types
const (
	PullRequestGitea PullRequestType = iota
	PullRequestGit
)

// PullRequestStatus defines pull request status
type PullRequestStatus int

// Enumerate all the pull request status
const (
	PullRequestStatusConflict PullRequestStatus = iota
	PullRequestStatusChecking
	PullRequestStatusMergeable
	PullRequestStatusManuallyMerged
	PullRequestStatusError
)

// PullRequest represents relation between pull request and repositories.
type PullRequest struct {
	ID              int64 `xorm:"pk autoincr"`
	Type            PullRequestType
	Status          PullRequestStatus
	ConflictedFiles []string `xorm:"TEXT JSON"`

	IssueID int64  `xorm:"INDEX"`
	Issue   *Issue `xorm:"-"`
	Index   int64

	HeadRepoID      int64       `xorm:"INDEX"`
	HeadRepo        *Repository `xorm:"-"`
	BaseRepoID      int64       `xorm:"INDEX"`
	BaseRepo        *Repository `xorm:"-"`
	HeadBranch      string
	BaseBranch      string
	ProtectedBranch *ProtectedBranch `xorm:"-"`
	MergeBase       string           `xorm:"VARCHAR(40)"`

	HasMerged      bool               `xorm:"INDEX"`
	MergedCommitID string             `xorm:"VARCHAR(40)"`
	MergerID       int64              `xorm:"INDEX"`
	Merger         *User              `xorm:"-"`
	MergedUnix     timeutil.TimeStamp `xorm:"updated INDEX"`
}

// MustHeadUserName returns the HeadRepo's username if failed return blank
func (pr *PullRequest) MustHeadUserName() string {
	if err := pr.LoadHeadRepo(); err != nil {
		if !IsErrRepoNotExist(err) {
			log.Error("LoadHeadRepo: %v", err)
		} else {
			log.Warn("LoadHeadRepo %d but repository does not exist: %v", pr.HeadRepoID, err)
		}
		return ""
	}
	return pr.HeadRepo.MustOwnerName()
}

// Note: don't try to get Issue because will end up recursive querying.
func (pr *PullRequest) loadAttributes(e Engine) (err error) {
	if pr.HasMerged && pr.Merger == nil {
		pr.Merger, err = getUserByID(e, pr.MergerID)
		if IsErrUserNotExist(err) {
			pr.MergerID = -1
			pr.Merger = NewGhostUser()
		} else if err != nil {
			return fmt.Errorf("getUserByID [%d]: %v", pr.MergerID, err)
		}
	}

	return nil
}

// LoadAttributes loads pull request attributes from database
func (pr *PullRequest) LoadAttributes() error {
	return pr.loadAttributes(x)
}

// LoadBaseRepo loads pull request base repository from database
func (pr *PullRequest) LoadBaseRepo() error {
	if pr.BaseRepo == nil {
		if pr.HeadRepoID == pr.BaseRepoID && pr.HeadRepo != nil {
			pr.BaseRepo = pr.HeadRepo
			return nil
		}
		var repo Repository
		if has, err := x.ID(pr.BaseRepoID).Get(&repo); err != nil {
			return err
		} else if !has {
			return ErrRepoNotExist{ID: pr.BaseRepoID}
		}
		pr.BaseRepo = &repo
	}
	return nil
}

// LoadHeadRepo loads pull request head repository from database
func (pr *PullRequest) LoadHeadRepo() error {
	if pr.HeadRepo == nil {
		if pr.HeadRepoID == pr.BaseRepoID && pr.BaseRepo != nil {
			pr.HeadRepo = pr.BaseRepo
			return nil
		}
		var repo Repository
		if has, err := x.ID(pr.HeadRepoID).Get(&repo); err != nil {
			return err
		} else if !has {
			return ErrRepoNotExist{ID: pr.HeadRepoID}
		}
		pr.HeadRepo = &repo
	}
	return nil
}

// LoadIssue loads issue information from database
func (pr *PullRequest) LoadIssue() (err error) {
	return pr.loadIssue(x)
}

func (pr *PullRequest) loadIssue(e Engine) (err error) {
	if pr.Issue != nil {
		return nil
	}

	pr.Issue, err = getIssueByID(e, pr.IssueID)
	if err == nil {
		pr.Issue.PullRequest = pr
	}
	return err
}

// LoadProtectedBranch loads the protected branch of the base branch
func (pr *PullRequest) LoadProtectedBranch() (err error) {
	return pr.loadProtectedBranch(x)
}

func (pr *PullRequest) loadProtectedBranch(e Engine) (err error) {
	if pr.BaseRepo == nil {
		if pr.BaseRepoID == 0 {
			return nil
		}
		pr.BaseRepo, err = getRepositoryByID(e, pr.BaseRepoID)
		if err != nil {
			return
		}
	}
	pr.ProtectedBranch, err = getProtectedBranchBy(e, pr.BaseRepo.ID, pr.BaseBranch)
	return
}

// GetDefaultMergeMessage returns default message used when merging pull request
func (pr *PullRequest) GetDefaultMergeMessage() string {
	if pr.HeadRepo == nil {
		var err error
		pr.HeadRepo, err = GetRepositoryByID(pr.HeadRepoID)
		if err != nil {
			log.Error("GetRepositoryById[%d]: %v", pr.HeadRepoID, err)
			return ""
		}
	}
	if err := pr.LoadIssue(); err != nil {
		log.Error("Cannot load issue %d for PR id %d: Error: %v", pr.IssueID, pr.ID, err)
		return ""
	}

	if pr.BaseRepoID == pr.HeadRepoID {
		return fmt.Sprintf("Merge pull request '%s' (#%d) from %s into %s", pr.Issue.Title, pr.Issue.Index, pr.HeadBranch, pr.BaseBranch)
	}

	return fmt.Sprintf("Merge pull request '%s' (#%d) from %s:%s into %s", pr.Issue.Title, pr.Issue.Index, pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseBranch)
}

// GetCommitMessages returns the commit messages between head and merge base (if there is one)
func (pr *PullRequest) GetCommitMessages() string {
	if err := pr.LoadIssue(); err != nil {
		log.Error("Cannot load issue %d for PR id %d: Error: %v", pr.IssueID, pr.ID, err)
		return ""
	}

	if err := pr.Issue.LoadPoster(); err != nil {
		log.Error("Cannot load poster %d for pr id %d, index %d Error: %v", pr.Issue.PosterID, pr.ID, pr.Index, err)
		return ""
	}

	if pr.HeadRepo == nil {
		var err error
		pr.HeadRepo, err = GetRepositoryByID(pr.HeadRepoID)
		if err != nil {
			log.Error("GetRepositoryById[%d]: %v", pr.HeadRepoID, err)
			return ""
		}
	}

	gitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		log.Error("Unable to open head repository: Error: %v", err)
		return ""
	}
	defer gitRepo.Close()

	headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
	if err != nil {
		log.Error("Unable to get head commit: %s Error: %v", pr.HeadBranch, err)
		return ""
	}

	mergeBase, err := gitRepo.GetCommit(pr.MergeBase)
	if err != nil {
		log.Error("Unable to get merge base commit: %s Error: %v", pr.MergeBase, err)
		return ""
	}

	limit := setting.Repository.PullRequest.DefaultMergeMessageCommitsLimit

	list, err := gitRepo.CommitsBetweenLimit(headCommit, mergeBase, limit, 0)
	if err != nil {
		log.Error("Unable to get commits between: %s %s Error: %v", pr.HeadBranch, pr.MergeBase, err)
		return ""
	}

	maxSize := setting.Repository.PullRequest.DefaultMergeMessageSize

	posterSig := pr.Issue.Poster.NewGitSig().String()

	authorsMap := map[string]bool{}
	authors := make([]string, 0, list.Len())
	stringBuilder := strings.Builder{}
	element := list.Front()
	for element != nil {
		commit := element.Value.(*git.Commit)

		if maxSize < 0 || stringBuilder.Len() < maxSize {
			toWrite := []byte(commit.CommitMessage)
			if len(toWrite) > maxSize-stringBuilder.Len() && maxSize > -1 {
				toWrite = append(toWrite[:maxSize-stringBuilder.Len()], "..."...)
			}
			if _, err := stringBuilder.Write(toWrite); err != nil {
				log.Error("Unable to write commit message Error: %v", err)
				return ""
			}

			if _, err := stringBuilder.WriteRune('\n'); err != nil {
				log.Error("Unable to write commit message Error: %v", err)
				return ""
			}
		}

		authorString := commit.Author.String()
		if !authorsMap[authorString] && authorString != posterSig {
			authors = append(authors, authorString)
			authorsMap[authorString] = true
		}
		element = element.Next()
	}

	// Consider collecting the remaining authors
	if limit >= 0 && setting.Repository.PullRequest.DefaultMergeMessageAllAuthors {
		skip := limit
		limit = 30
		for {
			list, err := gitRepo.CommitsBetweenLimit(headCommit, mergeBase, limit, skip)
			if err != nil {
				log.Error("Unable to get commits between: %s %s Error: %v", pr.HeadBranch, pr.MergeBase, err)
				return ""

			}
			if list.Len() == 0 {
				break
			}
			element := list.Front()
			for element != nil {
				commit := element.Value.(*git.Commit)

				authorString := commit.Author.String()
				if !authorsMap[authorString] && authorString != posterSig {
					authors = append(authors, authorString)
					authorsMap[authorString] = true
				}
				element = element.Next()
			}

		}
	}

	if len(authors) > 0 {
		if _, err := stringBuilder.WriteRune('\n'); err != nil {
			log.Error("Unable to write to string builder Error: %v", err)
			return ""
		}
	}

	for _, author := range authors {
		if _, err := stringBuilder.Write([]byte("Co-authored-by: ")); err != nil {
			log.Error("Unable to write to string builder Error: %v", err)
			return ""
		}
		if _, err := stringBuilder.Write([]byte(author)); err != nil {
			log.Error("Unable to write to string builder Error: %v", err)
			return ""
		}
		if _, err := stringBuilder.WriteRune('\n'); err != nil {
			log.Error("Unable to write to string builder Error: %v", err)
			return ""
		}
	}

	return stringBuilder.String()
}

// GetApprovers returns the approvers of the pull request
func (pr *PullRequest) GetApprovers() string {

	stringBuilder := strings.Builder{}
	if err := pr.getReviewedByLines(&stringBuilder); err != nil {
		log.Error("Unable to getReviewedByLines: Error: %v", err)
		return ""
	}

	return stringBuilder.String()
}

func (pr *PullRequest) getReviewedByLines(writer io.Writer) error {
	maxReviewers := setting.Repository.PullRequest.DefaultMergeMessageMaxApprovers

	if maxReviewers == 0 {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	// Note: This doesn't page as we only expect a very limited number of reviews
	reviews, err := findReviews(sess, FindReviewOptions{
		Type:         ReviewTypeApprove,
		IssueID:      pr.IssueID,
		OfficialOnly: setting.Repository.PullRequest.DefaultMergeMessageOfficialApproversOnly,
	})
	if err != nil {
		log.Error("Unable to FindReviews for PR ID %d: %v", pr.ID, err)
		return err
	}

	reviewersWritten := 0

	for _, review := range reviews {
		if maxReviewers > 0 && reviewersWritten > maxReviewers {
			break
		}

		if err := review.loadReviewer(sess); err != nil && !IsErrUserNotExist(err) {
			log.Error("Unable to LoadReviewer[%d] for PR ID %d : %v", review.ReviewerID, pr.ID, err)
			return err
		} else if review.Reviewer == nil {
			continue
		}
		if _, err := writer.Write([]byte("Reviewed-by: ")); err != nil {
			return err
		}
		if _, err := writer.Write([]byte(review.Reviewer.NewGitSig().String())); err != nil {
			return err
		}
		if _, err := writer.Write([]byte{'\n'}); err != nil {
			return err
		}
		reviewersWritten++
	}
	return sess.Commit()
}

// GetDefaultSquashMessage returns default message used when squash and merging pull request
func (pr *PullRequest) GetDefaultSquashMessage() string {
	if err := pr.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return ""
	}
	if err := pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return ""
	}
	if pr.BaseRepo.UnitEnabled(UnitTypeExternalTracker) {
		return fmt.Sprintf("%s (!%d)", pr.Issue.Title, pr.Issue.Index)
	}
	return fmt.Sprintf("%s (#%d)", pr.Issue.Title, pr.Issue.Index)
}

// GetGitRefName returns git ref for hidden pull request branch
func (pr *PullRequest) GetGitRefName() string {
	return fmt.Sprintf("refs/pull/%d/head", pr.Index)
}

// APIFormat assumes following fields have been assigned with valid values:
// Required - Issue
// Optional - Merger
func (pr *PullRequest) APIFormat() *api.PullRequest {
	return pr.apiFormat(x)
}

func (pr *PullRequest) apiFormat(e Engine) *api.PullRequest {
	var (
		baseBranch *git.Branch
		headBranch *git.Branch
		baseCommit *git.Commit
		headCommit *git.Commit
		err        error
	)
	if err = pr.Issue.loadRepo(e); err != nil {
		log.Error("pr.Issue.loadRepo[%d]: %v", pr.ID, err)
		return nil
	}
	apiIssue := pr.Issue.apiFormat(e)
	if pr.BaseRepo == nil {
		pr.BaseRepo, err = getRepositoryByID(e, pr.BaseRepoID)
		if err != nil {
			log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
			return nil
		}
	}
	if pr.HeadRepoID != 0 && pr.HeadRepo == nil {
		pr.HeadRepo, err = getRepositoryByID(e, pr.HeadRepoID)
		if err != nil && !IsErrRepoNotExist(err) {
			log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
			return nil
		}
	}

	apiPullRequest := &api.PullRequest{
		ID:        pr.ID,
		URL:       pr.Issue.HTMLURL(),
		Index:     pr.Index,
		Poster:    apiIssue.Poster,
		Title:     apiIssue.Title,
		Body:      apiIssue.Body,
		Labels:    apiIssue.Labels,
		Milestone: apiIssue.Milestone,
		Assignee:  apiIssue.Assignee,
		Assignees: apiIssue.Assignees,
		State:     apiIssue.State,
		Comments:  apiIssue.Comments,
		HTMLURL:   pr.Issue.HTMLURL(),
		DiffURL:   pr.Issue.DiffURL(),
		PatchURL:  pr.Issue.PatchURL(),
		HasMerged: pr.HasMerged,
		MergeBase: pr.MergeBase,
		Deadline:  apiIssue.Deadline,
		Created:   pr.Issue.CreatedUnix.AsTimePtr(),
		Updated:   pr.Issue.UpdatedUnix.AsTimePtr(),
	}
	baseBranch, err = pr.BaseRepo.GetBranch(pr.BaseBranch)
	if err != nil {
		if git.IsErrBranchNotExist(err) {
			apiPullRequest.Base = nil
		} else {
			log.Error("GetBranch[%s]: %v", pr.BaseBranch, err)
			return nil
		}
	} else {
		apiBaseBranchInfo := &api.PRBranchInfo{
			Name:       pr.BaseBranch,
			Ref:        pr.BaseBranch,
			RepoID:     pr.BaseRepoID,
			Repository: pr.BaseRepo.innerAPIFormat(e, AccessModeNone, false),
		}
		baseCommit, err = baseBranch.GetCommit()
		if err != nil {
			if git.IsErrNotExist(err) {
				apiBaseBranchInfo.Sha = ""
			} else {
				log.Error("GetCommit[%s]: %v", baseBranch.Name, err)
				return nil
			}
		} else {
			apiBaseBranchInfo.Sha = baseCommit.ID.String()
		}
		apiPullRequest.Base = apiBaseBranchInfo
	}

	if pr.HeadRepo != nil {
		headBranch, err = pr.HeadRepo.GetBranch(pr.HeadBranch)
		if err != nil {
			if git.IsErrBranchNotExist(err) {
				apiPullRequest.Head = nil
			} else {
				log.Error("GetBranch[%s]: %v", pr.HeadBranch, err)
				return nil
			}
		} else {
			apiHeadBranchInfo := &api.PRBranchInfo{
				Name:       pr.HeadBranch,
				Ref:        pr.HeadBranch,
				RepoID:     pr.HeadRepoID,
				Repository: pr.HeadRepo.innerAPIFormat(e, AccessModeNone, false),
			}
			headCommit, err = headBranch.GetCommit()
			if err != nil {
				if git.IsErrNotExist(err) {
					apiHeadBranchInfo.Sha = ""
				} else {
					log.Error("GetCommit[%s]: %v", headBranch.Name, err)
					return nil
				}
			} else {
				apiHeadBranchInfo.Sha = headCommit.ID.String()
			}
			apiPullRequest.Head = apiHeadBranchInfo
		}
	} else {
		apiPullRequest.Head = &api.PRBranchInfo{
			Name:   pr.HeadBranch,
			Ref:    fmt.Sprintf("refs/pull/%d/head", pr.Index),
			RepoID: -1,
		}
	}

	if pr.Status != PullRequestStatusChecking {
		mergeable := !(pr.Status == PullRequestStatusConflict || pr.Status == PullRequestStatusError) && !pr.IsWorkInProgress()
		apiPullRequest.Mergeable = mergeable
	}
	if pr.HasMerged {
		apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
		apiPullRequest.MergedCommitID = &pr.MergedCommitID
		apiPullRequest.MergedBy = pr.Merger.APIFormat()
	}

	return apiPullRequest
}

func (pr *PullRequest) getHeadRepo(e Engine) (err error) {
	pr.HeadRepo, err = getRepositoryByID(e, pr.HeadRepoID)
	if err != nil && !IsErrRepoNotExist(err) {
		return fmt.Errorf("getRepositoryByID(head): %v", err)
	}
	return nil
}

// GetHeadRepo loads the head repository
func (pr *PullRequest) GetHeadRepo() error {
	return pr.getHeadRepo(x)
}

// GetBaseRepo loads the target repository
func (pr *PullRequest) GetBaseRepo() (err error) {
	if pr.BaseRepo != nil {
		return nil
	}

	pr.BaseRepo, err = GetRepositoryByID(pr.BaseRepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID(base): %v", err)
	}
	return nil
}

// IsChecking returns true if this pull request is still checking conflict.
func (pr *PullRequest) IsChecking() bool {
	return pr.Status == PullRequestStatusChecking
}

// CanAutoMerge returns true if this pull request can be merged automatically.
func (pr *PullRequest) CanAutoMerge() bool {
	return pr.Status == PullRequestStatusMergeable
}

// GetLastCommitStatus returns the last commit status for this pull request.
func (pr *PullRequest) GetLastCommitStatus() (status *CommitStatus, err error) {
	if err = pr.GetHeadRepo(); err != nil {
		return nil, err
	}

	if pr.HeadRepo == nil {
		return nil, ErrPullRequestHeadRepoMissing{pr.ID, pr.HeadRepoID}
	}

	headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer headGitRepo.Close()

	lastCommitID, err := headGitRepo.GetBranchCommitID(pr.HeadBranch)
	if err != nil {
		return nil, err
	}

	err = pr.LoadBaseRepo()
	if err != nil {
		return nil, err
	}

	statusList, err := GetLatestCommitStatus(pr.BaseRepo, lastCommitID, 0)
	if err != nil {
		return nil, err
	}
	return CalcCommitStatus(statusList), nil
}

// MergeStyle represents the approach to merge commits into base branch.
type MergeStyle string

const (
	// MergeStyleMerge create merge commit
	MergeStyleMerge MergeStyle = "merge"
	// MergeStyleRebase rebase before merging
	MergeStyleRebase MergeStyle = "rebase"
	// MergeStyleRebaseMerge rebase before merging with merge commit (--no-ff)
	MergeStyleRebaseMerge MergeStyle = "rebase-merge"
	// MergeStyleSquash squash commits into single commit before merging
	MergeStyleSquash MergeStyle = "squash"
)

// CheckUserAllowedToMerge checks whether the user is allowed to merge
func (pr *PullRequest) CheckUserAllowedToMerge(doer *User) (err error) {
	if doer == nil {
		return ErrNotAllowedToMerge{
			"Not signed in",
		}
	}

	if pr.BaseRepo == nil {
		if err = pr.GetBaseRepo(); err != nil {
			return fmt.Errorf("GetBaseRepo: %v", err)
		}
	}

	if protected, err := pr.BaseRepo.IsProtectedBranchForMerging(pr, pr.BaseBranch, doer); err != nil {
		return fmt.Errorf("IsProtectedBranch: %v", err)
	} else if protected {
		return ErrNotAllowedToMerge{
			"The branch is protected",
		}
	}

	return nil
}

// SetMerged sets a pull request to merged and closes the corresponding issue
func (pr *PullRequest) SetMerged() (bool, error) {
	if pr.HasMerged {
		return false, fmt.Errorf("PullRequest[%d] already merged", pr.Index)
	}
	if pr.MergedCommitID == "" || pr.MergedUnix == 0 || pr.Merger == nil {
		return false, fmt.Errorf("Unable to merge PullRequest[%d], some required fields are empty", pr.Index)
	}

	pr.HasMerged = true

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return false, err
	}

	if _, err := sess.Exec("UPDATE `issue` SET `repo_id` = `repo_id` WHERE `id` = ?", pr.IssueID); err != nil {
		return false, err
	}

	if _, err := sess.Exec("UPDATE `pull_request` SET `issue_id` = `issue_id` WHERE `id` = ?", pr.ID); err != nil {
		return false, err
	}

	pr.Issue = nil
	if err := pr.loadIssue(sess); err != nil {
		return false, err
	}

	if tmpPr, err := getPullRequestByID(sess, pr.ID); err != nil {
		return false, err
	} else if tmpPr.HasMerged {
		if pr.Issue.IsClosed {
			return false, nil
		}
		return false, fmt.Errorf("PullRequest[%d] already merged but it's associated issue [%d] is not closed", pr.Index, pr.IssueID)
	} else if pr.Issue.IsClosed {
		return false, fmt.Errorf("PullRequest[%d] already closed", pr.Index)
	}

	if err := pr.Issue.loadRepo(sess); err != nil {
		return false, err
	}

	if err := pr.Issue.Repo.getOwner(sess); err != nil {
		return false, err
	}

	if _, err := pr.Issue.changeStatus(sess, pr.Merger, true); err != nil {
		return false, fmt.Errorf("Issue.changeStatus: %v", err)
	}

	if _, err := sess.Where("id = ?", pr.ID).Cols("has_merged, status, merged_commit_id, merger_id, merged_unix").Update(pr); err != nil {
		return false, fmt.Errorf("Failed to update pr[%d]: %v", pr.ID, err)
	}

	if err := sess.Commit(); err != nil {
		return false, fmt.Errorf("Commit: %v", err)
	}
	return true, nil
}

// NewPullRequest creates new pull request with labels for repository.
func NewPullRequest(repo *Repository, pull *Issue, labelIDs []int64, uuids []string, pr *PullRequest) (err error) {
	// Retry several times in case INSERT fails due to duplicate key for (repo_id, index); see #7887
	i := 0
	for {
		if err = newPullRequestAttempt(repo, pull, labelIDs, uuids, pr); err == nil {
			return nil
		}
		if !IsErrNewIssueInsert(err) {
			return err
		}
		if i++; i == issueMaxDupIndexAttempts {
			break
		}
		log.Error("NewPullRequest: error attempting to insert the new issue; will retry. Original error: %v", err)
	}
	return fmt.Errorf("NewPullRequest: too many errors attempting to insert the new issue. Last error was: %v", err)
}

func newPullRequestAttempt(repo *Repository, pull *Issue, labelIDs []int64, uuids []string, pr *PullRequest) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = newIssue(sess, pull.Poster, NewIssueOptions{
		Repo:        repo,
		Issue:       pull,
		LabelIDs:    labelIDs,
		Attachments: uuids,
		IsPull:      true,
	}); err != nil {
		if IsErrUserDoesNotHaveAccessToRepo(err) || IsErrNewIssueInsert(err) {
			return err
		}
		return fmt.Errorf("newIssue: %v", err)
	}

	pr.Index = pull.Index
	pr.BaseRepo = repo

	pr.IssueID = pull.ID
	if _, err = sess.Insert(pr); err != nil {
		return fmt.Errorf("insert pull repo: %v", err)
	}

	if err = sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	return nil
}

// GetUnmergedPullRequest returns a pull request that is open and has not been merged
// by given head/base and repo/branch.
func GetUnmergedPullRequest(headRepoID, baseRepoID int64, headBranch, baseBranch string) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := x.
		Where("head_repo_id=? AND head_branch=? AND base_repo_id=? AND base_branch=? AND has_merged=? AND issue.is_closed=?",
			headRepoID, headBranch, baseRepoID, baseBranch, false, false).
		Join("INNER", "issue", "issue.id=pull_request.issue_id").
		Get(pr)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPullRequestNotExist{0, 0, headRepoID, baseRepoID, headBranch, baseBranch}
	}

	return pr, nil
}

// GetLatestPullRequestByHeadInfo returns the latest pull request (regardless of its status)
// by given head information (repo and branch).
func GetLatestPullRequestByHeadInfo(repoID int64, branch string) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := x.
		Where("head_repo_id = ? AND head_branch = ?", repoID, branch).
		OrderBy("id DESC").
		Get(pr)
	if !has {
		return nil, err
	}
	return pr, err
}

// GetPullRequestByIndex returns a pull request by the given index
func GetPullRequestByIndex(repoID int64, index int64) (*PullRequest, error) {
	pr := &PullRequest{
		BaseRepoID: repoID,
		Index:      index,
	}

	has, err := x.Get(pr)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPullRequestNotExist{0, 0, 0, repoID, "", ""}
	}

	if err = pr.LoadAttributes(); err != nil {
		return nil, err
	}
	if err = pr.LoadIssue(); err != nil {
		return nil, err
	}

	return pr, nil
}

func getPullRequestByID(e Engine, id int64) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := e.ID(id).Get(pr)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPullRequestNotExist{id, 0, 0, 0, "", ""}
	}
	return pr, pr.loadAttributes(e)
}

// GetPullRequestByID returns a pull request by given ID.
func GetPullRequestByID(id int64) (*PullRequest, error) {
	return getPullRequestByID(x, id)
}

func getPullRequestByIssueID(e Engine, issueID int64) (*PullRequest, error) {
	pr := &PullRequest{
		IssueID: issueID,
	}
	has, err := e.Get(pr)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPullRequestNotExist{0, issueID, 0, 0, "", ""}
	}
	return pr, pr.loadAttributes(e)
}

// GetPullRequestByIssueID returns pull request by given issue ID.
func GetPullRequestByIssueID(issueID int64) (*PullRequest, error) {
	return getPullRequestByIssueID(x, issueID)
}

// Update updates all fields of pull request.
func (pr *PullRequest) Update() error {
	_, err := x.ID(pr.ID).AllCols().Update(pr)
	return err
}

// UpdateCols updates specific fields of pull request.
func (pr *PullRequest) UpdateCols(cols ...string) error {
	_, err := x.ID(pr.ID).Cols(cols...).Update(pr)
	return err
}

// UpdateColsIfNotMerged updates specific fields of a pull request if it has not been merged
func (pr *PullRequest) UpdateColsIfNotMerged(cols ...string) error {
	_, err := x.Where("id = ? AND has_merged = ?", pr.ID, false).Cols(cols...).Update(pr)
	return err
}

// IsWorkInProgress determine if the Pull Request is a Work In Progress by its title
func (pr *PullRequest) IsWorkInProgress() bool {
	if err := pr.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return false
	}

	for _, prefix := range setting.Repository.PullRequest.WorkInProgressPrefixes {
		if strings.HasPrefix(strings.ToUpper(pr.Issue.Title), prefix) {
			return true
		}
	}
	return false
}

// IsFilesConflicted determines if the  Pull Request has changes conflicting with the target branch.
func (pr *PullRequest) IsFilesConflicted() bool {
	return len(pr.ConflictedFiles) > 0
}

// GetWorkInProgressPrefix returns the prefix used to mark the pull request as a work in progress.
// It returns an empty string when none were found
func (pr *PullRequest) GetWorkInProgressPrefix() string {
	if err := pr.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return ""
	}

	for _, prefix := range setting.Repository.PullRequest.WorkInProgressPrefixes {
		if strings.HasPrefix(strings.ToUpper(pr.Issue.Title), prefix) {
			return pr.Issue.Title[0:len(prefix)]
		}
	}
	return ""
}

// IsHeadEqualWithBranch returns if the commits of branchName are available in pull request head
func (pr *PullRequest) IsHeadEqualWithBranch(branchName string) (bool, error) {
	var err error
	if err = pr.GetBaseRepo(); err != nil {
		return false, err
	}
	baseGitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return false, err
	}
	baseCommit, err := baseGitRepo.GetBranchCommit(branchName)
	if err != nil {
		return false, err
	}

	if err = pr.GetHeadRepo(); err != nil {
		return false, err
	}
	headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		return false, err
	}
	headCommit, err := headGitRepo.GetBranchCommit(pr.HeadBranch)
	if err != nil {
		return false, err
	}
	return baseCommit.HasPreviousCommit(headCommit.ID)
}
