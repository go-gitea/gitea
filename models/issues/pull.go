// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	org_model "code.gitea.io/gitea/models/organization"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrPullRequestNotExist represents a "PullRequestNotExist" kind of error.
type ErrPullRequestNotExist struct {
	ID         int64
	IssueID    int64
	HeadRepoID int64
	BaseRepoID int64
	HeadBranch string
	BaseBranch string
}

// IsErrPullRequestNotExist checks if an error is a ErrPullRequestNotExist.
func IsErrPullRequestNotExist(err error) bool {
	_, ok := err.(ErrPullRequestNotExist)
	return ok
}

func (err ErrPullRequestNotExist) Error() string {
	return fmt.Sprintf("pull request does not exist [id: %d, issue_id: %d, head_repo_id: %d, base_repo_id: %d, head_branch: %s, base_branch: %s]",
		err.ID, err.IssueID, err.HeadRepoID, err.BaseRepoID, err.HeadBranch, err.BaseBranch)
}

func (err ErrPullRequestNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrPullRequestAlreadyExists represents a "PullRequestAlreadyExists"-error
type ErrPullRequestAlreadyExists struct {
	ID         int64
	IssueID    int64
	HeadRepoID int64
	BaseRepoID int64
	HeadBranch string
	BaseBranch string
}

// IsErrPullRequestAlreadyExists checks if an error is a ErrPullRequestAlreadyExists.
func IsErrPullRequestAlreadyExists(err error) bool {
	_, ok := err.(ErrPullRequestAlreadyExists)
	return ok
}

// Error does pretty-printing :D
func (err ErrPullRequestAlreadyExists) Error() string {
	return fmt.Sprintf("pull request already exists for these targets [id: %d, issue_id: %d, head_repo_id: %d, base_repo_id: %d, head_branch: %s, base_branch: %s]",
		err.ID, err.IssueID, err.HeadRepoID, err.BaseRepoID, err.HeadBranch, err.BaseBranch)
}

func (err ErrPullRequestAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrPullWasClosed is used close a closed pull request
type ErrPullWasClosed struct {
	ID    int64
	Index int64
}

// IsErrPullWasClosed checks if an error is a ErrErrPullWasClosed.
func IsErrPullWasClosed(err error) bool {
	_, ok := err.(ErrPullWasClosed)
	return ok
}

func (err ErrPullWasClosed) Error() string {
	return fmt.Sprintf("Pull request [%d] %d was already closed", err.ID, err.Index)
}

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
	PullRequestStatusEmpty
	PullRequestStatusAncestor
)

func (status PullRequestStatus) String() string {
	switch status {
	case PullRequestStatusConflict:
		return "CONFLICT"
	case PullRequestStatusChecking:
		return "CHECKING"
	case PullRequestStatusMergeable:
		return "MERGEABLE"
	case PullRequestStatusManuallyMerged:
		return "MANUALLY_MERGED"
	case PullRequestStatusError:
		return "ERROR"
	case PullRequestStatusEmpty:
		return "EMPTY"
	case PullRequestStatusAncestor:
		return "ANCESTOR"
	default:
		return strconv.Itoa(int(status))
	}
}

// PullRequestFlow the flow of pull request
type PullRequestFlow int

const (
	// PullRequestFlowGithub github flow from head branch to base branch
	PullRequestFlowGithub PullRequestFlow = iota
	// PullRequestFlowAGit Agit flow pull request, head branch is not exist
	PullRequestFlowAGit
)

// PullRequest represents relation between pull request and repositories.
type PullRequest struct {
	ID              int64 `xorm:"pk autoincr"`
	Type            PullRequestType
	Status          PullRequestStatus
	ConflictedFiles []string `xorm:"TEXT JSON"`
	CommitsAhead    int
	CommitsBehind   int

	ChangedProtectedFiles []string `xorm:"TEXT JSON"`

	IssueID            int64  `xorm:"INDEX"`
	Issue              *Issue `xorm:"-"`
	Index              int64
	RequestedReviewers []*user_model.User `xorm:"-"`

	HeadRepoID          int64                  `xorm:"INDEX"`
	HeadRepo            *repo_model.Repository `xorm:"-"`
	BaseRepoID          int64                  `xorm:"INDEX"`
	BaseRepo            *repo_model.Repository `xorm:"-"`
	HeadBranch          string
	HeadCommitID        string `xorm:"-"`
	BaseBranch          string
	MergeBase           string `xorm:"VARCHAR(64)"`
	AllowMaintainerEdit bool   `xorm:"NOT NULL DEFAULT false"`

	HasMerged      bool               `xorm:"INDEX"`
	MergedCommitID string             `xorm:"VARCHAR(64)"`
	MergerID       int64              `xorm:"INDEX"`
	Merger         *user_model.User   `xorm:"-"`
	MergedUnix     timeutil.TimeStamp `xorm:"updated INDEX"`

	isHeadRepoLoaded bool `xorm:"-"`

	Flow PullRequestFlow `xorm:"NOT NULL DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(PullRequest))
}

// DeletePullsByBaseRepoID deletes all pull requests by the base repository ID
func DeletePullsByBaseRepoID(ctx context.Context, repoID int64) error {
	deleteCond := builder.Select("id").From("pull_request").Where(builder.Eq{"pull_request.base_repo_id": repoID})

	// Delete scheduled auto merges
	if _, err := db.GetEngine(ctx).In("pull_id", deleteCond).
		Delete(&pull_model.AutoMerge{}); err != nil {
		return err
	}

	// Delete review states
	if _, err := db.GetEngine(ctx).In("pull_id", deleteCond).
		Delete(&pull_model.ReviewState{}); err != nil {
		return err
	}

	_, err := db.DeleteByBean(ctx, &PullRequest{BaseRepoID: repoID})
	return err
}

func (pr *PullRequest) String() string {
	if pr == nil {
		return "<PullRequest nil>"
	}

	s := new(strings.Builder)
	fmt.Fprintf(s, "<PullRequest [%d]", pr.ID)
	if pr.BaseRepo != nil {
		fmt.Fprintf(s, "%s#%d[%s...", pr.BaseRepo.FullName(), pr.Index, pr.BaseBranch)
	} else {
		fmt.Fprintf(s, "Repo[%d]#%d[%s...", pr.BaseRepoID, pr.Index, pr.BaseBranch)
	}
	if pr.HeadRepoID == pr.BaseRepoID {
		fmt.Fprintf(s, "%s]", pr.HeadBranch)
	} else if pr.HeadRepo != nil {
		fmt.Fprintf(s, "%s:%s]", pr.HeadRepo.FullName(), pr.HeadBranch)
	} else {
		fmt.Fprintf(s, "Repo[%d]:%s]", pr.HeadRepoID, pr.HeadBranch)
	}
	s.WriteByte('>')
	return s.String()
}

// MustHeadUserName returns the HeadRepo's username if failed return blank
func (pr *PullRequest) MustHeadUserName(ctx context.Context) string {
	if err := pr.LoadHeadRepo(ctx); err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			log.Error("LoadHeadRepo: %v", err)
		} else {
			log.Warn("LoadHeadRepo %d but repository does not exist: %v", pr.HeadRepoID, err)
		}
		return ""
	}
	if pr.HeadRepo == nil {
		return ""
	}
	return pr.HeadRepo.OwnerName
}

// LoadAttributes loads pull request attributes from database
// Note: don't try to get Issue because will end up recursive querying.
func (pr *PullRequest) LoadAttributes(ctx context.Context) (err error) {
	if pr.HasMerged && pr.Merger == nil {
		pr.Merger, err = user_model.GetUserByID(ctx, pr.MergerID)
		if user_model.IsErrUserNotExist(err) {
			pr.MergerID = user_model.GhostUserID
			pr.Merger = user_model.NewGhostUser()
		} else if err != nil {
			return fmt.Errorf("getUserByID [%d]: %w", pr.MergerID, err)
		}
	}

	return nil
}

// LoadHeadRepo loads the head repository, pr.HeadRepo will remain nil if it does not exist
// and thus ErrRepoNotExist will never be returned
func (pr *PullRequest) LoadHeadRepo(ctx context.Context) (err error) {
	if !pr.isHeadRepoLoaded && pr.HeadRepo == nil && pr.HeadRepoID > 0 {
		if pr.HeadRepoID == pr.BaseRepoID {
			if pr.BaseRepo != nil {
				pr.HeadRepo = pr.BaseRepo
				return nil
			} else if pr.Issue != nil && pr.Issue.Repo != nil {
				pr.HeadRepo = pr.Issue.Repo
				return nil
			}
		}

		pr.HeadRepo, err = repo_model.GetRepositoryByID(ctx, pr.HeadRepoID)
		if err != nil && !repo_model.IsErrRepoNotExist(err) { // Head repo maybe deleted, but it should still work
			return fmt.Errorf("pr[%d].LoadHeadRepo[%d]: %w", pr.ID, pr.HeadRepoID, err)
		}
		pr.isHeadRepoLoaded = true
	}
	return nil
}

// LoadRequestedReviewers loads the requested reviewers.
func (pr *PullRequest) LoadRequestedReviewers(ctx context.Context) error {
	if len(pr.RequestedReviewers) > 0 {
		return nil
	}

	reviews, err := GetReviewsByIssueID(ctx, pr.Issue.ID)
	if err != nil {
		return err
	}

	if err = reviews.LoadReviewers(ctx); err != nil {
		return err
	}
	for _, review := range reviews {
		pr.RequestedReviewers = append(pr.RequestedReviewers, review.Reviewer)
	}

	return nil
}

// LoadBaseRepo loads the target repository. ErrRepoNotExist may be returned.
func (pr *PullRequest) LoadBaseRepo(ctx context.Context) (err error) {
	if pr.BaseRepo != nil {
		return nil
	}

	if pr.HeadRepoID == pr.BaseRepoID && pr.HeadRepo != nil {
		pr.BaseRepo = pr.HeadRepo
		return nil
	}

	if pr.Issue != nil && pr.Issue.Repo != nil {
		pr.BaseRepo = pr.Issue.Repo
		return nil
	}

	pr.BaseRepo, err = repo_model.GetRepositoryByID(ctx, pr.BaseRepoID)
	if err != nil {
		return fmt.Errorf("pr[%d].LoadBaseRepo[%d]: %w", pr.ID, pr.BaseRepoID, err)
	}
	return nil
}

// LoadIssue loads issue information from database
func (pr *PullRequest) LoadIssue(ctx context.Context) (err error) {
	if pr.Issue != nil {
		return nil
	}

	pr.Issue, err = GetIssueByID(ctx, pr.IssueID)
	if err == nil {
		pr.Issue.PullRequest = pr
	}
	return err
}

// ReviewCount represents a count of Reviews
type ReviewCount struct {
	IssueID int64
	Type    ReviewType
	Count   int64
}

// GetApprovalCounts returns the approval counts by type
// FIXME: Only returns official counts due to double counting of non-official counts
func (pr *PullRequest) GetApprovalCounts(ctx context.Context) ([]*ReviewCount, error) {
	rCounts := make([]*ReviewCount, 0, 6)
	sess := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID)
	return rCounts, sess.Select("issue_id, type, count(id) as `count`").Where("official = ? AND dismissed = ?", true, false).GroupBy("issue_id, type").Table("review").Find(&rCounts)
}

// GetApprovers returns the approvers of the pull request
func (pr *PullRequest) GetApprovers(ctx context.Context) string {
	stringBuilder := strings.Builder{}
	if err := pr.getReviewedByLines(ctx, &stringBuilder); err != nil {
		log.Error("Unable to getReviewedByLines: Error: %v", err)
		return ""
	}

	return stringBuilder.String()
}

func (pr *PullRequest) getReviewedByLines(ctx context.Context, writer io.Writer) error {
	maxReviewers := setting.Repository.PullRequest.DefaultMergeMessageMaxApprovers

	if maxReviewers == 0 {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Note: This doesn't page as we only expect a very limited number of reviews
	reviews, err := FindLatestReviews(ctx, FindReviewOptions{
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

		if err := review.LoadReviewer(ctx); err != nil && !user_model.IsErrUserNotExist(err) {
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
	return committer.Commit()
}

// GetGitRefName returns git ref for hidden pull request branch
func (pr *PullRequest) GetGitRefName() string {
	return fmt.Sprintf("%s%d/head", git.PullPrefix, pr.Index)
}

func (pr *PullRequest) GetGitHeadBranchRefName() string {
	return fmt.Sprintf("%s%s", git.BranchPrefix, pr.HeadBranch)
}

// IsChecking returns true if this pull request is still checking conflict.
func (pr *PullRequest) IsChecking() bool {
	return pr.Status == PullRequestStatusChecking
}

// CanAutoMerge returns true if this pull request can be merged automatically.
func (pr *PullRequest) CanAutoMerge() bool {
	return pr.Status == PullRequestStatusMergeable
}

// IsEmpty returns true if this pull request is empty.
func (pr *PullRequest) IsEmpty() bool {
	return pr.Status == PullRequestStatusEmpty
}

// IsAncestor returns true if the Head Commit of this PR is an ancestor of the Base Commit
func (pr *PullRequest) IsAncestor() bool {
	return pr.Status == PullRequestStatusAncestor
}

// IsFromFork return true if this PR is from a fork.
func (pr *PullRequest) IsFromFork() bool {
	return pr.HeadRepoID != pr.BaseRepoID
}

// SetMerged sets a pull request to merged and closes the corresponding issue
func (pr *PullRequest) SetMerged(ctx context.Context) (bool, error) {
	if pr.HasMerged {
		return false, fmt.Errorf("PullRequest[%d] already merged", pr.Index)
	}
	if pr.MergedCommitID == "" || pr.MergedUnix == 0 || pr.Merger == nil {
		return false, fmt.Errorf("Unable to merge PullRequest[%d], some required fields are empty", pr.Index)
	}

	pr.HasMerged = true
	sess := db.GetEngine(ctx)

	if _, err := sess.Exec("UPDATE `issue` SET `repo_id` = `repo_id` WHERE `id` = ?", pr.IssueID); err != nil {
		return false, err
	}

	if _, err := sess.Exec("UPDATE `pull_request` SET `issue_id` = `issue_id` WHERE `id` = ?", pr.ID); err != nil {
		return false, err
	}

	pr.Issue = nil
	if err := pr.LoadIssue(ctx); err != nil {
		return false, err
	}

	if tmpPr, err := GetPullRequestByID(ctx, pr.ID); err != nil {
		return false, err
	} else if tmpPr.HasMerged {
		if pr.Issue.IsClosed {
			return false, nil
		}
		return false, fmt.Errorf("PullRequest[%d] already merged but it's associated issue [%d] is not closed", pr.Index, pr.IssueID)
	} else if pr.Issue.IsClosed {
		return false, fmt.Errorf("PullRequest[%d] already closed", pr.Index)
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		return false, err
	}

	if err := pr.Issue.Repo.LoadOwner(ctx); err != nil {
		return false, err
	}

	if _, err := changeIssueStatus(ctx, pr.Issue, pr.Merger, true, true); err != nil {
		return false, fmt.Errorf("Issue.changeStatus: %w", err)
	}

	// reset the conflicted files as there cannot be any if we're merged
	pr.ConflictedFiles = []string{}

	// We need to save all of the data used to compute this merge as it may have already been changed by TestPatch. FIXME: need to set some state to prevent TestPatch from running whilst we are merging.
	if _, err := sess.Where("id = ?", pr.ID).Cols("has_merged, status, merge_base, merged_commit_id, merger_id, merged_unix, conflicted_files").Update(pr); err != nil {
		return false, fmt.Errorf("Failed to update pr[%d]: %w", pr.ID, err)
	}

	return true, nil
}

// NewPullRequest creates new pull request with labels for repository.
func NewPullRequest(ctx context.Context, repo *repo_model.Repository, issue *Issue, labelIDs []int64, uuids []string, pr *PullRequest) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	idx, err := db.GetNextResourceIndex(ctx, "issue_index", repo.ID)
	if err != nil {
		return fmt.Errorf("generate pull request index failed: %w", err)
	}

	issue.Index = idx

	if err = NewIssueWithIndex(ctx, issue.Poster, NewIssueOptions{
		Repo:        repo,
		Issue:       issue,
		LabelIDs:    labelIDs,
		Attachments: uuids,
		IsPull:      true,
	}); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) || IsErrNewIssueInsert(err) {
			return err
		}
		return fmt.Errorf("newIssue: %w", err)
	}

	pr.Index = issue.Index
	pr.BaseRepo = repo
	pr.IssueID = issue.ID
	if err = db.Insert(ctx, pr); err != nil {
		return fmt.Errorf("insert pull repo: %w", err)
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	return nil
}

// GetUnmergedPullRequest returns a pull request that is open and has not been merged
// by given head/base and repo/branch.
func GetUnmergedPullRequest(ctx context.Context, headRepoID, baseRepoID int64, headBranch, baseBranch string, flow PullRequestFlow) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := db.GetEngine(ctx).
		Where("head_repo_id=? AND head_branch=? AND base_repo_id=? AND base_branch=? AND has_merged=? AND flow = ? AND issue.is_closed=?",
			headRepoID, headBranch, baseRepoID, baseBranch, false, flow, false).
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
func GetLatestPullRequestByHeadInfo(ctx context.Context, repoID int64, branch string) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := db.GetEngine(ctx).
		Where("head_repo_id = ? AND head_branch = ? AND flow = ?", repoID, branch, PullRequestFlowGithub).
		OrderBy("id DESC").
		Get(pr)
	if !has {
		return nil, err
	}
	return pr, err
}

// GetPullRequestByIndex returns a pull request by the given index
func GetPullRequestByIndex(ctx context.Context, repoID, index int64) (*PullRequest, error) {
	if index < 1 {
		return nil, ErrPullRequestNotExist{}
	}
	pr := &PullRequest{
		BaseRepoID: repoID,
		Index:      index,
	}

	has, err := db.GetEngine(ctx).Get(pr)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPullRequestNotExist{0, 0, 0, repoID, "", ""}
	}

	if err = pr.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	if err = pr.LoadIssue(ctx); err != nil {
		return nil, err
	}

	return pr, nil
}

// GetPullRequestByID returns a pull request by given ID.
func GetPullRequestByID(ctx context.Context, id int64) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := db.GetEngine(ctx).ID(id).Get(pr)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPullRequestNotExist{id, 0, 0, 0, "", ""}
	}
	return pr, pr.LoadAttributes(ctx)
}

// GetPullRequestByIssueIDWithNoAttributes returns pull request with no attributes loaded by given issue ID.
func GetPullRequestByIssueIDWithNoAttributes(ctx context.Context, issueID int64) (*PullRequest, error) {
	var pr PullRequest
	has, err := db.GetEngine(ctx).Where("issue_id = ?", issueID).Get(&pr)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPullRequestNotExist{0, issueID, 0, 0, "", ""}
	}
	return &pr, nil
}

// GetPullRequestByIssueID returns pull request by given issue ID.
func GetPullRequestByIssueID(ctx context.Context, issueID int64) (*PullRequest, error) {
	pr, exist, err := db.Get[PullRequest](ctx, builder.Eq{"issue_id": issueID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrPullRequestNotExist{0, issueID, 0, 0, "", ""}
	}
	return pr, pr.LoadAttributes(ctx)
}

// GetPullRequestsByBaseHeadInfo returns the pull request by given base and head
func GetPullRequestByBaseHeadInfo(ctx context.Context, baseID, headID int64, base, head string) (*PullRequest, error) {
	pr := &PullRequest{}
	sess := db.GetEngine(ctx).
		Join("INNER", "issue", "issue.id = pull_request.issue_id").
		Where("base_repo_id = ? AND base_branch = ? AND head_repo_id = ? AND head_branch = ?", baseID, base, headID, head)
	has, err := sess.Get(pr)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPullRequestNotExist{
			HeadRepoID: headID,
			BaseRepoID: baseID,
			HeadBranch: head,
			BaseBranch: base,
		}
	}

	if err = pr.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	if err = pr.LoadIssue(ctx); err != nil {
		return nil, err
	}

	return pr, nil
}

// GetAllUnmergedAgitPullRequestByPoster get all unmerged agit flow pull request
// By poster id.
func GetAllUnmergedAgitPullRequestByPoster(ctx context.Context, uid int64) ([]*PullRequest, error) {
	pulls := make([]*PullRequest, 0, 10)

	err := db.GetEngine(ctx).
		Where("has_merged=? AND flow = ? AND issue.is_closed=? AND issue.poster_id=?",
			false, PullRequestFlowAGit, false, uid).
		Join("INNER", "issue", "issue.id=pull_request.issue_id").
		Find(&pulls)

	return pulls, err
}

// Update updates all fields of pull request.
func (pr *PullRequest) Update(ctx context.Context) error {
	_, err := db.GetEngine(ctx).ID(pr.ID).AllCols().Update(pr)
	return err
}

// UpdateCols updates specific fields of pull request.
func (pr *PullRequest) UpdateCols(ctx context.Context, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(pr.ID).Cols(cols...).Update(pr)
	return err
}

// UpdateColsIfNotMerged updates specific fields of a pull request if it has not been merged
func (pr *PullRequest) UpdateColsIfNotMerged(ctx context.Context, cols ...string) error {
	_, err := db.GetEngine(ctx).Where("id = ? AND has_merged = ?", pr.ID, false).Cols(cols...).Update(pr)
	return err
}

// IsWorkInProgress determine if the Pull Request is a Work In Progress by its title
// Issue must be set before this method can be called.
func (pr *PullRequest) IsWorkInProgress(ctx context.Context) bool {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return false
	}
	return HasWorkInProgressPrefix(pr.Issue.Title)
}

// HasWorkInProgressPrefix determines if the given PR title has a Work In Progress prefix
func HasWorkInProgressPrefix(title string) bool {
	for _, prefix := range setting.Repository.PullRequest.WorkInProgressPrefixes {
		if strings.HasPrefix(strings.ToUpper(title), strings.ToUpper(prefix)) {
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
func (pr *PullRequest) GetWorkInProgressPrefix(ctx context.Context) string {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return ""
	}

	for _, prefix := range setting.Repository.PullRequest.WorkInProgressPrefixes {
		if strings.HasPrefix(strings.ToUpper(pr.Issue.Title), strings.ToUpper(prefix)) {
			return pr.Issue.Title[0:len(prefix)]
		}
	}
	return ""
}

// UpdateCommitDivergence update Divergence of a pull request
func (pr *PullRequest) UpdateCommitDivergence(ctx context.Context, ahead, behind int) error {
	if pr.ID == 0 {
		return fmt.Errorf("pull ID is 0")
	}
	pr.CommitsAhead = ahead
	pr.CommitsBehind = behind
	_, err := db.GetEngine(ctx).ID(pr.ID).Cols("commits_ahead", "commits_behind").Update(pr)
	return err
}

// IsSameRepo returns true if base repo and head repo is the same
func (pr *PullRequest) IsSameRepo() bool {
	return pr.BaseRepoID == pr.HeadRepoID
}

// GetBaseBranchLink returns the relative URL of the base branch
func (pr *PullRequest) GetBaseBranchLink(ctx context.Context) string {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return ""
	}
	if pr.BaseRepo == nil {
		return ""
	}
	return pr.BaseRepo.Link() + "/src/branch/" + util.PathEscapeSegments(pr.BaseBranch)
}

// GetHeadBranchLink returns the relative URL of the head branch
func (pr *PullRequest) GetHeadBranchLink(ctx context.Context) string {
	if pr.Flow == PullRequestFlowAGit {
		return ""
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return ""
	}
	if pr.HeadRepo == nil {
		return ""
	}
	return pr.HeadRepo.Link() + "/src/branch/" + util.PathEscapeSegments(pr.HeadBranch)
}

// UpdateAllowEdits update if PR can be edited from maintainers
func UpdateAllowEdits(ctx context.Context, pr *PullRequest) error {
	if _, err := db.GetEngine(ctx).ID(pr.ID).Cols("allow_maintainer_edit").Update(pr); err != nil {
		return err
	}
	return nil
}

// Mergeable returns if the pullrequest is mergeable.
func (pr *PullRequest) Mergeable(ctx context.Context) bool {
	// If a pull request isn't mergeable if it's:
	// - Being conflict checked.
	// - Has a conflict.
	// - Received a error while being conflict checked.
	// - Is a work-in-progress pull request.
	return pr.Status != PullRequestStatusChecking && pr.Status != PullRequestStatusConflict &&
		pr.Status != PullRequestStatusError && !pr.IsWorkInProgress(ctx)
}

// HasEnoughApprovals returns true if pr has enough granted approvals.
func HasEnoughApprovals(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	if protectBranch.RequiredApprovals == 0 {
		return true
	}
	return GetGrantedApprovalsCount(ctx, protectBranch, pr) >= protectBranch.RequiredApprovals
}

// GetGrantedApprovalsCount returns the number of granted approvals for pr. A granted approval must be authored by a user in an approval whitelist.
func GetGrantedApprovalsCount(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) int64 {
	sess := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeApprove).
		And("official = ?", true).
		And("dismissed = ?", false)
	if protectBranch.IgnoreStaleApprovals {
		sess = sess.And("stale = ?", false)
	}
	approvals, err := sess.Count(new(Review))
	if err != nil {
		log.Error("GetGrantedApprovalsCount: %v", err)
		return 0
	}

	return approvals
}

// MergeBlockedByRejectedReview returns true if merge is blocked by rejected reviews
func MergeBlockedByRejectedReview(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	if !protectBranch.BlockOnRejectedReviews {
		return false
	}
	rejectExist, err := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeReject).
		And("official = ?", true).
		And("dismissed = ?", false).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByRejectedReview: %v", err)
		return true
	}

	return rejectExist
}

// MergeBlockedByOfficialReviewRequests block merge because of some review request to official reviewer
// of from official review
func MergeBlockedByOfficialReviewRequests(ctx context.Context, protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	if !protectBranch.BlockOnOfficialReviewRequests {
		return false
	}
	has, err := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeRequest).
		And("official = ?", true).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByOfficialReviewRequests: %v", err)
		return true
	}

	return has
}

// MergeBlockedByOutdatedBranch returns true if merge is blocked by an outdated head branch
func MergeBlockedByOutdatedBranch(protectBranch *git_model.ProtectedBranch, pr *PullRequest) bool {
	return protectBranch.BlockOnOutdatedBranch && pr.CommitsBehind > 0
}

// GetCodeOwnersFromContent returns the code owners configuration
// Return empty slice if files missing
// Return warning messages on parsing errors
// We're trying to do the best we can when parsing a file.
// Invalid lines are skipped. Non-existent users and teams too.
func GetCodeOwnersFromContent(ctx context.Context, data string) ([]*CodeOwnerRule, []string) {
	if len(data) == 0 {
		return nil, nil
	}

	rules := make([]*CodeOwnerRule, 0)
	lines := strings.Split(data, "\n")
	warnings := make([]string, 0)

	for i, line := range lines {
		tokens := TokenizeCodeOwnersLine(line)
		if len(tokens) == 0 {
			continue
		} else if len(tokens) < 2 {
			warnings = append(warnings, fmt.Sprintf("Line: %d: incorrect format", i+1))
			continue
		}
		rule, wr := ParseCodeOwnersLine(ctx, tokens)
		for _, w := range wr {
			warnings = append(warnings, fmt.Sprintf("Line: %d: %s", i+1, w))
		}
		if rule == nil {
			continue
		}

		rules = append(rules, rule)
	}

	return rules, warnings
}

type CodeOwnerRule struct {
	Rule     *regexp.Regexp
	Negative bool
	Users    []*user_model.User
	Teams    []*org_model.Team
}

func ParseCodeOwnersLine(ctx context.Context, tokens []string) (*CodeOwnerRule, []string) {
	var err error
	rule := &CodeOwnerRule{
		Users:    make([]*user_model.User, 0),
		Teams:    make([]*org_model.Team, 0),
		Negative: strings.HasPrefix(tokens[0], "!"),
	}

	warnings := make([]string, 0)

	rule.Rule, err = regexp.Compile(fmt.Sprintf("^%s$", strings.TrimPrefix(tokens[0], "!")))
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("incorrect codeowner regexp: %s", err))
		return nil, warnings
	}

	for _, user := range tokens[1:] {
		user = strings.TrimPrefix(user, "@")

		// Only @org/team can contain slashes
		if strings.Contains(user, "/") {
			s := strings.Split(user, "/")
			if len(s) != 2 {
				warnings = append(warnings, fmt.Sprintf("incorrect codeowner group: %s", user))
				continue
			}
			orgName := s[0]
			teamName := s[1]

			org, err := org_model.GetOrgByName(ctx, orgName)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("incorrect codeowner organization: %s", user))
				continue
			}
			teams, err := org.LoadTeams(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("incorrect codeowner team: %s", user))
				continue
			}

			for _, team := range teams {
				if team.Name == teamName {
					rule.Teams = append(rule.Teams, team)
				}
			}
		} else {
			u, err := user_model.GetUserByName(ctx, user)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("incorrect codeowner user: %s", user))
				continue
			}
			rule.Users = append(rule.Users, u)
		}
	}

	if (len(rule.Users) == 0) && (len(rule.Teams) == 0) {
		warnings = append(warnings, "no users/groups matched")
		return nil, warnings
	}

	return rule, warnings
}

func TokenizeCodeOwnersLine(line string) []string {
	if len(line) == 0 {
		return nil
	}

	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, "\t", " ")

	tokens := make([]string, 0)

	escape := false
	token := ""
	for _, char := range line {
		if escape {
			token += string(char)
			escape = false
		} else if string(char) == "\\" {
			escape = true
		} else if string(char) == "#" {
			break
		} else if string(char) == " " {
			if len(token) > 0 {
				tokens = append(tokens, token)
				token = ""
			}
		} else {
			token += string(char)
		}
	}

	if len(token) > 0 {
		tokens = append(tokens, token)
	}

	return tokens
}

// InsertPullRequests inserted pull requests
func InsertPullRequests(ctx context.Context, prs ...*PullRequest) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)
	for _, pr := range prs {
		if err := insertIssue(ctx, pr.Issue); err != nil {
			return err
		}
		pr.IssueID = pr.Issue.ID
		if _, err := sess.NoAutoTime().Insert(pr); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// GetPullRequestByMergedCommit returns a merged pull request by the given commit
func GetPullRequestByMergedCommit(ctx context.Context, repoID int64, sha string) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := db.GetEngine(ctx).Where("base_repo_id = ? AND merged_commit_id = ?", repoID, sha).Get(pr)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPullRequestNotExist{0, 0, 0, repoID, "", ""}
	}

	if err = pr.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	if err = pr.LoadIssue(ctx); err != nil {
		return nil, err
	}

	return pr, nil
}
