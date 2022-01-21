// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
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
	PullRequestStatusEmpty
)

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

	IssueID int64  `xorm:"INDEX"`
	Issue   *Issue `xorm:"-"`
	Index   int64

	HeadRepoID      int64                  `xorm:"INDEX"`
	HeadRepo        *repo_model.Repository `xorm:"-"`
	BaseRepoID      int64                  `xorm:"INDEX"`
	BaseRepo        *repo_model.Repository `xorm:"-"`
	HeadBranch      string
	HeadCommitID    string `xorm:"-"`
	BaseBranch      string
	ProtectedBranch *ProtectedBranch `xorm:"-"`
	MergeBase       string           `xorm:"VARCHAR(40)"`

	HasMerged      bool               `xorm:"INDEX"`
	MergedCommitID string             `xorm:"VARCHAR(40)"`
	MergerID       int64              `xorm:"INDEX"`
	Merger         *user_model.User   `xorm:"-"`
	MergedUnix     timeutil.TimeStamp `xorm:"updated INDEX"`

	isHeadRepoLoaded bool `xorm:"-"`

	Flow PullRequestFlow `xorm:"NOT NULL DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(PullRequest))
}

// MustHeadUserName returns the HeadRepo's username if failed return blank
func (pr *PullRequest) MustHeadUserName() string {
	if err := pr.LoadHeadRepo(); err != nil {
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

// Note: don't try to get Issue because will end up recursive querying.
func (pr *PullRequest) loadAttributes(e db.Engine) (err error) {
	if pr.HasMerged && pr.Merger == nil {
		pr.Merger, err = user_model.GetUserByIDEngine(e, pr.MergerID)
		if user_model.IsErrUserNotExist(err) {
			pr.MergerID = -1
			pr.Merger = user_model.NewGhostUser()
		} else if err != nil {
			return fmt.Errorf("getUserByID [%d]: %v", pr.MergerID, err)
		}
	}

	return nil
}

// LoadAttributes loads pull request attributes from database
func (pr *PullRequest) LoadAttributes() error {
	return pr.loadAttributes(db.GetEngine(db.DefaultContext))
}

func (pr *PullRequest) loadHeadRepo(ctx context.Context) (err error) {
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

		pr.HeadRepo, err = repo_model.GetRepositoryByIDCtx(ctx, pr.HeadRepoID)
		if err != nil && !repo_model.IsErrRepoNotExist(err) { // Head repo maybe deleted, but it should still work
			return fmt.Errorf("getRepositoryByID(head): %v", err)
		}
		pr.isHeadRepoLoaded = true
	}
	return nil
}

// LoadHeadRepo loads the head repository
func (pr *PullRequest) LoadHeadRepo() error {
	return pr.loadHeadRepo(db.DefaultContext)
}

// LoadBaseRepo loads the target repository
func (pr *PullRequest) LoadBaseRepo() error {
	return pr.loadBaseRepo(db.DefaultContext)
}

func (pr *PullRequest) loadBaseRepo(ctx context.Context) (err error) {
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

	pr.BaseRepo, err = repo_model.GetRepositoryByIDCtx(ctx, pr.BaseRepoID)
	if err != nil {
		return fmt.Errorf("repo_model.GetRepositoryByID(base): %v", err)
	}
	return nil
}

// LoadIssue loads issue information from database
func (pr *PullRequest) LoadIssue() (err error) {
	return pr.loadIssue(db.GetEngine(db.DefaultContext))
}

func (pr *PullRequest) loadIssue(e db.Engine) (err error) {
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
	return pr.loadProtectedBranch(db.DefaultContext)
}

func (pr *PullRequest) loadProtectedBranch(ctx context.Context) (err error) {
	if pr.ProtectedBranch == nil {
		if pr.BaseRepo == nil {
			if pr.BaseRepoID == 0 {
				return nil
			}
			pr.BaseRepo, err = repo_model.GetRepositoryByIDCtx(ctx, pr.BaseRepoID)
			if err != nil {
				return
			}
		}
		pr.ProtectedBranch, err = getProtectedBranchBy(db.GetEngine(ctx), pr.BaseRepo.ID, pr.BaseBranch)
	}
	return
}

// GetDefaultMergeMessage returns default message used when merging pull request
func (pr *PullRequest) GetDefaultMergeMessage() string {
	if pr.HeadRepo == nil {
		var err error
		pr.HeadRepo, err = repo_model.GetRepositoryByID(pr.HeadRepoID)
		if err != nil {
			log.Error("GetRepositoryById[%d]: %v", pr.HeadRepoID, err)
			return ""
		}
	}
	if err := pr.LoadIssue(); err != nil {
		log.Error("Cannot load issue %d for PR id %d: Error: %v", pr.IssueID, pr.ID, err)
		return ""
	}
	if err := pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return ""
	}

	issueReference := "#"
	if pr.BaseRepo.UnitEnabled(unit.TypeExternalTracker) {
		issueReference = "!"
	}

	if pr.BaseRepoID == pr.HeadRepoID {
		return fmt.Sprintf("Merge pull request '%s' (%s%d) from %s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadBranch, pr.BaseBranch)
	}

	return fmt.Sprintf("Merge pull request '%s' (%s%d) from %s:%s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseBranch)
}

// ReviewCount represents a count of Reviews
type ReviewCount struct {
	IssueID int64
	Type    ReviewType
	Count   int64
}

// GetApprovalCounts returns the approval counts by type
// FIXME: Only returns official counts due to double counting of non-official counts
func (pr *PullRequest) GetApprovalCounts() ([]*ReviewCount, error) {
	return pr.getApprovalCounts(db.GetEngine(db.DefaultContext))
}

func (pr *PullRequest) getApprovalCounts(e db.Engine) ([]*ReviewCount, error) {
	rCounts := make([]*ReviewCount, 0, 6)
	sess := e.Where("issue_id = ?", pr.IssueID)
	return rCounts, sess.Select("issue_id, type, count(id) as `count`").Where("official = ? AND dismissed = ?", true, false).GroupBy("issue_id, type").Table("review").Find(&rCounts)
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

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

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

		if err := review.loadReviewer(sess); err != nil && !user_model.IsErrUserNotExist(err) {
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
	if pr.BaseRepo.UnitEnabled(unit.TypeExternalTracker) {
		return fmt.Sprintf("%s (!%d)", pr.Issue.Title, pr.Issue.Index)
	}
	return fmt.Sprintf("%s (#%d)", pr.Issue.Title, pr.Issue.Index)
}

// GetGitRefName returns git ref for hidden pull request branch
func (pr *PullRequest) GetGitRefName() string {
	return fmt.Sprintf("%s%d/head", git.PullPrefix, pr.Index)
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

// SetMerged sets a pull request to merged and closes the corresponding issue
func (pr *PullRequest) SetMerged() (bool, error) {
	if pr.HasMerged {
		return false, fmt.Errorf("PullRequest[%d] already merged", pr.Index)
	}
	if pr.MergedCommitID == "" || pr.MergedUnix == 0 || pr.Merger == nil {
		return false, fmt.Errorf("Unable to merge PullRequest[%d], some required fields are empty", pr.Index)
	}

	pr.HasMerged = true

	ctx, committer, err := db.TxContext()
	if err != nil {
		return false, err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

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

	if err := pr.Issue.loadRepo(ctx); err != nil {
		return false, err
	}

	if err := pr.Issue.Repo.GetOwner(ctx); err != nil {
		return false, err
	}

	if _, err := pr.Issue.changeStatus(ctx, pr.Merger, true, true); err != nil {
		return false, fmt.Errorf("Issue.changeStatus: %v", err)
	}

	// We need to save all of the data used to compute this merge as it may have already been changed by TestPatch. FIXME: need to set some state to prevent TestPatch from running whilst we are merging.
	if _, err := sess.Where("id = ?", pr.ID).Cols("has_merged, status, merge_base, merged_commit_id, merger_id, merged_unix").Update(pr); err != nil {
		return false, fmt.Errorf("Failed to update pr[%d]: %v", pr.ID, err)
	}

	if err := committer.Commit(); err != nil {
		return false, fmt.Errorf("Commit: %v", err)
	}
	return true, nil
}

// NewPullRequest creates new pull request with labels for repository.
func NewPullRequest(outerCtx context.Context, repo *repo_model.Repository, issue *Issue, labelIDs []int64, uuids []string, pr *PullRequest) (err error) {
	idx, err := db.GetNextResourceIndex("issue_index", repo.ID)
	if err != nil {
		return fmt.Errorf("generate pull request index failed: %v", err)
	}

	issue.Index = idx

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	ctx.WithContext(outerCtx)

	if err = newIssue(ctx, issue.Poster, NewIssueOptions{
		Repo:        repo,
		Issue:       issue,
		LabelIDs:    labelIDs,
		Attachments: uuids,
		IsPull:      true,
	}); err != nil {
		if IsErrUserDoesNotHaveAccessToRepo(err) || IsErrNewIssueInsert(err) {
			return err
		}
		return fmt.Errorf("newIssue: %v", err)
	}

	pr.Index = issue.Index
	pr.BaseRepo = repo
	pr.IssueID = issue.ID
	if err = db.Insert(ctx, pr); err != nil {
		return fmt.Errorf("insert pull repo: %v", err)
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	return nil
}

// GetUnmergedPullRequest returns a pull request that is open and has not been merged
// by given head/base and repo/branch.
func GetUnmergedPullRequest(headRepoID, baseRepoID int64, headBranch, baseBranch string, flow PullRequestFlow) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := db.GetEngine(db.DefaultContext).
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
func GetLatestPullRequestByHeadInfo(repoID int64, branch string) (*PullRequest, error) {
	pr := new(PullRequest)
	has, err := db.GetEngine(db.DefaultContext).
		Where("head_repo_id = ? AND head_branch = ? AND flow = ?", repoID, branch, PullRequestFlowGithub).
		OrderBy("id DESC").
		Get(pr)
	if !has {
		return nil, err
	}
	return pr, err
}

// GetPullRequestByIndex returns a pull request by the given index
func GetPullRequestByIndex(repoID, index int64) (*PullRequest, error) {
	if index < 1 {
		return nil, ErrPullRequestNotExist{}
	}
	pr := &PullRequest{
		BaseRepoID: repoID,
		Index:      index,
	}

	has, err := db.GetEngine(db.DefaultContext).Get(pr)
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

func getPullRequestByID(e db.Engine, id int64) (*PullRequest, error) {
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
	return getPullRequestByID(db.GetEngine(db.DefaultContext), id)
}

// GetPullRequestByIssueIDWithNoAttributes returns pull request with no attributes loaded by given issue ID.
func GetPullRequestByIssueIDWithNoAttributes(issueID int64) (*PullRequest, error) {
	var pr PullRequest
	has, err := db.GetEngine(db.DefaultContext).Where("issue_id = ?", issueID).Get(&pr)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPullRequestNotExist{0, issueID, 0, 0, "", ""}
	}
	return &pr, nil
}

func getPullRequestByIssueID(e db.Engine, issueID int64) (*PullRequest, error) {
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

// GetAllUnmergedAgitPullRequestByPoster get all unmerged agit flow pull request
// By poster id.
func GetAllUnmergedAgitPullRequestByPoster(uid int64) ([]*PullRequest, error) {
	pulls := make([]*PullRequest, 0, 10)

	err := db.GetEngine(db.DefaultContext).
		Where("has_merged=? AND flow = ? AND issue.is_closed=? AND issue.poster_id=?",
			false, PullRequestFlowAGit, false, uid).
		Join("INNER", "issue", "issue.id=pull_request.issue_id").
		Find(&pulls)

	return pulls, err
}

// GetPullRequestByIssueID returns pull request by given issue ID.
func GetPullRequestByIssueID(issueID int64) (*PullRequest, error) {
	return getPullRequestByIssueID(db.GetEngine(db.DefaultContext), issueID)
}

// Update updates all fields of pull request.
func (pr *PullRequest) Update() error {
	_, err := db.GetEngine(db.DefaultContext).ID(pr.ID).AllCols().Update(pr)
	return err
}

// UpdateCols updates specific fields of pull request.
func (pr *PullRequest) UpdateCols(cols ...string) error {
	_, err := db.GetEngine(db.DefaultContext).ID(pr.ID).Cols(cols...).Update(pr)
	return err
}

// UpdateColsIfNotMerged updates specific fields of a pull request if it has not been merged
func (pr *PullRequest) UpdateColsIfNotMerged(cols ...string) error {
	_, err := db.GetEngine(db.DefaultContext).Where("id = ? AND has_merged = ?", pr.ID, false).Cols(cols...).Update(pr)
	return err
}

// IsWorkInProgress determine if the Pull Request is a Work In Progress by its title
func (pr *PullRequest) IsWorkInProgress() bool {
	if err := pr.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return false
	}
	return HasWorkInProgressPrefix(pr.Issue.Title)
}

// HasWorkInProgressPrefix determines if the given PR title has a Work In Progress prefix
func HasWorkInProgressPrefix(title string) bool {
	for _, prefix := range setting.Repository.PullRequest.WorkInProgressPrefixes {
		if strings.HasPrefix(strings.ToUpper(title), prefix) {
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

// UpdateCommitDivergence update Divergence of a pull request
func (pr *PullRequest) UpdateCommitDivergence(ahead, behind int) error {
	return pr.updateCommitDivergence(db.GetEngine(db.DefaultContext), ahead, behind)
}

func (pr *PullRequest) updateCommitDivergence(e db.Engine, ahead, behind int) error {
	if pr.ID == 0 {
		return fmt.Errorf("pull ID is 0")
	}
	pr.CommitsAhead = ahead
	pr.CommitsBehind = behind
	_, err := e.ID(pr.ID).Cols("commits_ahead", "commits_behind").Update(pr)
	return err
}

// IsSameRepo returns true if base repo and head repo is the same
func (pr *PullRequest) IsSameRepo() bool {
	return pr.BaseRepoID == pr.HeadRepoID
}

// GetBaseBranchHTMLURL returns the HTML URL of the base branch
func (pr *PullRequest) GetBaseBranchHTMLURL() string {
	if err := pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return ""
	}
	if pr.BaseRepo == nil {
		return ""
	}
	return pr.BaseRepo.HTMLURL() + "/src/branch/" + util.PathEscapeSegments(pr.BaseBranch)
}

// GetHeadBranchHTMLURL returns the HTML URL of the head branch
func (pr *PullRequest) GetHeadBranchHTMLURL() string {
	if pr.Flow == PullRequestFlowAGit {
		return ""
	}

	if err := pr.LoadHeadRepo(); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return ""
	}
	if pr.HeadRepo == nil {
		return ""
	}
	return pr.HeadRepo.HTMLURL() + "/src/branch/" + util.PathEscapeSegments(pr.HeadBranch)
}
