// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrReviewNotExist represents a "ReviewNotExist" kind of error.
type ErrReviewNotExist struct {
	ID int64
}

// IsErrReviewNotExist checks if an error is a ErrReviewNotExist.
func IsErrReviewNotExist(err error) bool {
	_, ok := err.(ErrReviewNotExist)
	return ok
}

func (err ErrReviewNotExist) Error() string {
	return fmt.Sprintf("review does not exist [id: %d]", err.ID)
}

func (err ErrReviewNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrNotValidReviewRequest an not allowed review request modify
type ErrNotValidReviewRequest struct {
	Reason string
	UserID int64
	RepoID int64
}

// IsErrNotValidReviewRequest checks if an error is a ErrNotValidReviewRequest.
func IsErrNotValidReviewRequest(err error) bool {
	_, ok := err.(ErrNotValidReviewRequest)
	return ok
}

func (err ErrNotValidReviewRequest) Error() string {
	return fmt.Sprintf("%s [user_id: %d, repo_id: %d]",
		err.Reason,
		err.UserID,
		err.RepoID)
}

func (err ErrNotValidReviewRequest) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrReviewRequestOnClosedPR represents an error when an user tries to request a re-review on a closed or merged PR.
type ErrReviewRequestOnClosedPR struct{}

// IsErrReviewRequestOnClosedPR checks if an error is an ErrReviewRequestOnClosedPR.
func IsErrReviewRequestOnClosedPR(err error) bool {
	_, ok := err.(ErrReviewRequestOnClosedPR)
	return ok
}

func (err ErrReviewRequestOnClosedPR) Error() string {
	return "cannot request a re-review on a closed or merged PR"
}

func (err ErrReviewRequestOnClosedPR) Unwrap() error {
	return util.ErrPermissionDenied
}

// ReviewType defines the sort of feedback a review gives
type ReviewType int

// ReviewTypeUnknown unknown review type
const ReviewTypeUnknown ReviewType = -1

const (
	// ReviewTypePending is a review which is not published yet
	ReviewTypePending ReviewType = iota
	// ReviewTypeApprove approves changes
	ReviewTypeApprove
	// ReviewTypeComment gives general feedback
	ReviewTypeComment
	// ReviewTypeReject gives feedback blocking merge
	ReviewTypeReject
	// ReviewTypeRequest request review from others
	ReviewTypeRequest
)

// Icon returns the corresponding icon for the review type
func (rt ReviewType) Icon() string {
	switch rt {
	case ReviewTypeApprove:
		return "check"
	case ReviewTypeReject:
		return "diff"
	case ReviewTypeComment:
		return "comment"
	case ReviewTypeRequest:
		return "dot-fill"
	default:
		return "comment"
	}
}

// Review represents collection of code comments giving feedback for a PR
type Review struct {
	ID               int64 `xorm:"pk autoincr"`
	Type             ReviewType
	Reviewer         *user_model.User   `xorm:"-"`
	ReviewerID       int64              `xorm:"index"`
	ReviewerTeamID   int64              `xorm:"NOT NULL DEFAULT 0"`
	ReviewerTeam     *organization.Team `xorm:"-"`
	OriginalAuthor   string
	OriginalAuthorID int64
	Issue            *Issue `xorm:"-"`
	IssueID          int64  `xorm:"index"`
	Content          string `xorm:"TEXT"`
	// Official is a review made by an assigned approver (counts towards approval)
	Official  bool   `xorm:"NOT NULL DEFAULT false"`
	CommitID  string `xorm:"VARCHAR(64)"`
	Stale     bool   `xorm:"NOT NULL DEFAULT false"`
	Dismissed bool   `xorm:"NOT NULL DEFAULT false"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	// CodeComments are the initial code comments of the review
	CodeComments CodeComments `xorm:"-"`

	Comments []*Comment `xorm:"-"`
}

func init() {
	db.RegisterModel(new(Review))
}

// LoadCodeComments loads CodeComments
func (r *Review) LoadCodeComments(ctx context.Context) (err error) {
	if r.CodeComments != nil {
		return err
	}
	if err = r.LoadIssue(ctx); err != nil {
		return err
	}
	r.CodeComments, err = fetchCodeCommentsByReview(ctx, r.Issue, nil, r, false)
	return err
}

func (r *Review) LoadIssue(ctx context.Context) (err error) {
	if r.Issue != nil {
		return err
	}
	r.Issue, err = GetIssueByID(ctx, r.IssueID)
	return err
}

// LoadReviewer loads reviewer
func (r *Review) LoadReviewer(ctx context.Context) (err error) {
	if r.ReviewerID == 0 || r.Reviewer != nil {
		return err
	}
	r.Reviewer, err = user_model.GetPossibleUserByID(ctx, r.ReviewerID)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			return fmt.Errorf("GetPossibleUserByID [%d]: %w", r.ReviewerID, err)
		}
		r.ReviewerID = user_model.GhostUserID
		r.Reviewer = user_model.NewGhostUser()
		return nil
	}
	return err
}

// LoadReviewerTeam loads reviewer team
func (r *Review) LoadReviewerTeam(ctx context.Context) (err error) {
	if r.ReviewerTeamID == 0 || r.ReviewerTeam != nil {
		return nil
	}

	r.ReviewerTeam, err = organization.GetTeamByID(ctx, r.ReviewerTeamID)
	return err
}

// LoadAttributes loads all attributes except CodeComments
func (r *Review) LoadAttributes(ctx context.Context) (err error) {
	if err = r.LoadIssue(ctx); err != nil {
		return err
	}
	if err = r.LoadCodeComments(ctx); err != nil {
		return err
	}
	if err = r.LoadReviewer(ctx); err != nil {
		return err
	}
	if err = r.LoadReviewerTeam(ctx); err != nil {
		return err
	}
	return err
}

// HTMLTypeColorName returns the color used in the ui indicating the review
func (r *Review) HTMLTypeColorName() string {
	switch r.Type {
	case ReviewTypeApprove:
		if !r.Official {
			return "grey"
		}
		if r.Stale {
			return "yellow"
		}
		return "green"
	case ReviewTypeComment:
		return "grey"
	case ReviewTypeReject:
		return "red"
	case ReviewTypeRequest:
		return "yellow"
	}
	return "grey"
}

// TooltipContent returns the locale string describing the review type
func (r *Review) TooltipContent() string {
	switch r.Type {
	case ReviewTypeApprove:
		if r.Stale {
			return "repo.issues.review.stale"
		}
		if !r.Official {
			return "repo.issues.review.unofficial"
		}
		return "repo.issues.review.official"
	case ReviewTypeComment:
		return "repo.issues.review.commented"
	case ReviewTypeReject:
		return "repo.issues.review.rejected"
	case ReviewTypeRequest:
		return "repo.issues.review.requested"
	}
	return ""
}

// GetReviewByID returns the review by the given ID
func GetReviewByID(ctx context.Context, id int64) (*Review, error) {
	review := new(Review)
	if has, err := db.GetEngine(ctx).ID(id).Get(review); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReviewNotExist{ID: id}
	}
	return review, nil
}

// CreateReviewOptions represent the options to create a review. Type, Issue and Reviewer are required.
type CreateReviewOptions struct {
	Content      string
	Type         ReviewType
	Issue        *Issue
	Reviewer     *user_model.User
	ReviewerTeam *organization.Team
	Official     bool
	CommitID     string
	Stale        bool
}

// IsOfficialReviewer check if at least one of the provided reviewers can make official reviews in issue (counts towards required approvals)
func IsOfficialReviewer(ctx context.Context, issue *Issue, reviewer *user_model.User) (bool, error) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		return false, err
	}

	pr := issue.PullRequest
	rule, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return false, err
	}
	if rule == nil {
		// if no rule is found, then user with write access can make official reviews
		err := pr.LoadBaseRepo(ctx)
		if err != nil {
			return false, err
		}
		writeAccess, err := access_model.HasAccessUnit(ctx, reviewer, pr.BaseRepo, unit.TypeCode, perm.AccessModeWrite)
		if err != nil {
			return false, err
		}
		return writeAccess, nil
	}

	official, err := git_model.IsUserOfficialReviewer(ctx, rule, reviewer)
	if official || err != nil {
		return official, err
	}

	return false, nil
}

// IsOfficialReviewerTeam check if reviewer in this team can make official reviews in issue (counts towards required approvals)
func IsOfficialReviewerTeam(ctx context.Context, issue *Issue, team *organization.Team) (bool, error) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		return false, err
	}
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, issue.PullRequest.BaseRepoID, issue.PullRequest.BaseBranch)
	if err != nil {
		return false, err
	}
	if pb == nil {
		return false, nil
	}

	if !pb.EnableApprovalsWhitelist {
		return team.UnitAccessMode(ctx, unit.TypeCode) >= perm.AccessModeWrite, nil
	}

	return slices.Contains(pb.ApprovalsWhitelistTeamIDs, team.ID), nil
}

// CreateReview creates a new review based on opts
func CreateReview(ctx context.Context, opts CreateReviewOptions) (*Review, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	review := &Review{
		Issue:        opts.Issue,
		IssueID:      opts.Issue.ID,
		Reviewer:     opts.Reviewer,
		ReviewerTeam: opts.ReviewerTeam,
		Content:      opts.Content,
		Official:     opts.Official,
		CommitID:     opts.CommitID,
		Stale:        opts.Stale,
	}

	if opts.Reviewer != nil {
		review.Type = opts.Type
		review.ReviewerID = opts.Reviewer.ID

		reviewCond := builder.Eq{"reviewer_id": opts.Reviewer.ID, "issue_id": opts.Issue.ID}
		// make sure user review requests are cleared
		if opts.Type != ReviewTypePending {
			if _, err := sess.Where(reviewCond.And(builder.Eq{"type": ReviewTypeRequest})).Delete(new(Review)); err != nil {
				return nil, err
			}
		}
		// make sure if the created review gets dismissed no old review surface
		// other types can be ignored, as they don't affect branch protection
		if opts.Type == ReviewTypeApprove || opts.Type == ReviewTypeReject {
			if _, err := sess.Where(reviewCond.And(builder.In("type", ReviewTypeApprove, ReviewTypeReject))).
				Cols("dismissed").Update(&Review{Dismissed: true}); err != nil {
				return nil, err
			}
		}
	} else if opts.ReviewerTeam != nil {
		review.Type = ReviewTypeRequest
		review.ReviewerTeamID = opts.ReviewerTeam.ID
	} else {
		return nil, fmt.Errorf("provide either reviewer or reviewer team")
	}

	if _, err := sess.Insert(review); err != nil {
		return nil, err
	}
	return review, committer.Commit()
}

// GetCurrentReview returns the current pending review of reviewer for given issue
func GetCurrentReview(ctx context.Context, reviewer *user_model.User, issue *Issue) (*Review, error) {
	if reviewer == nil {
		return nil, nil
	}
	reviews, err := FindReviews(ctx, FindReviewOptions{
		Types:      []ReviewType{ReviewTypePending},
		IssueID:    issue.ID,
		ReviewerID: reviewer.ID,
	})
	if err != nil {
		return nil, err
	}
	if len(reviews) == 0 {
		return nil, ErrReviewNotExist{}
	}
	reviews[0].Reviewer = reviewer
	reviews[0].Issue = issue
	return reviews[0], nil
}

// ReviewExists returns whether a review exists for a particular line of code in the PR
func ReviewExists(ctx context.Context, issue *Issue, treePath string, line int64) (bool, error) {
	return db.GetEngine(ctx).Cols("id").Exist(&Comment{IssueID: issue.ID, TreePath: treePath, Line: line, Type: CommentTypeCode})
}

// ContentEmptyErr represents an content empty error
type ContentEmptyErr struct{}

func (ContentEmptyErr) Error() string {
	return "Review content is empty"
}

// IsContentEmptyErr returns true if err is a ContentEmptyErr
func IsContentEmptyErr(err error) bool {
	_, ok := err.(ContentEmptyErr)
	return ok
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(ctx context.Context, doer *user_model.User, issue *Issue, reviewType ReviewType, content, commitID string, stale bool, attachmentUUIDs []string) (*Review, *Comment, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	official := false

	review, err := GetCurrentReview(ctx, doer, issue)
	if err != nil {
		if !IsErrReviewNotExist(err) {
			return nil, nil, err
		}

		if reviewType != ReviewTypeApprove && len(strings.TrimSpace(content)) == 0 {
			return nil, nil, ContentEmptyErr{}
		}

		if reviewType == ReviewTypeApprove || reviewType == ReviewTypeReject {
			// Only reviewers latest review of type approve and reject shall count as "official", so existing reviews needs to be cleared
			if _, err := db.Exec(ctx, "UPDATE `review` SET official=? WHERE issue_id=? AND reviewer_id=?", false, issue.ID, doer.ID); err != nil {
				return nil, nil, err
			}
			if official, err = IsOfficialReviewer(ctx, issue, doer); err != nil {
				return nil, nil, err
			}
		}

		// No current review. Create a new one!
		if review, err = CreateReview(ctx, CreateReviewOptions{
			Type:     reviewType,
			Issue:    issue,
			Reviewer: doer,
			Content:  content,
			Official: official,
			CommitID: commitID,
			Stale:    stale,
		}); err != nil {
			return nil, nil, err
		}
	} else {
		if err := review.LoadCodeComments(ctx); err != nil {
			return nil, nil, err
		}
		if reviewType != ReviewTypeApprove && len(review.CodeComments) == 0 && len(strings.TrimSpace(content)) == 0 {
			return nil, nil, ContentEmptyErr{}
		}

		if reviewType == ReviewTypeApprove || reviewType == ReviewTypeReject {
			// Only reviewers latest review of type approve and reject shall count as "official", so existing reviews needs to be cleared
			if _, err := db.Exec(ctx, "UPDATE `review` SET official=? WHERE issue_id=? AND reviewer_id=?", false, issue.ID, doer.ID); err != nil {
				return nil, nil, err
			}
			if official, err = IsOfficialReviewer(ctx, issue, doer); err != nil {
				return nil, nil, err
			}
		}

		review.Official = official
		review.Issue = issue
		review.Content = content
		review.Type = reviewType
		review.CommitID = commitID
		review.Stale = stale

		if _, err := sess.ID(review.ID).Cols("content, type, official, commit_id, stale").Update(review); err != nil {
			return nil, nil, err
		}
	}

	comm, err := CreateComment(ctx, &CreateCommentOptions{
		Type:        CommentTypeReview,
		Doer:        doer,
		Content:     review.Content,
		Issue:       issue,
		Repo:        issue.Repo,
		ReviewID:    review.ID,
		Attachments: attachmentUUIDs,
	})
	if err != nil || comm == nil {
		return nil, nil, err
	}

	// try to remove team review request if need
	if issue.Repo.Owner.IsOrganization() && (reviewType == ReviewTypeApprove || reviewType == ReviewTypeReject) {
		teamReviewRequests := make([]*Review, 0, 10)
		if err := sess.SQL("SELECT * FROM review WHERE issue_id = ? AND reviewer_team_id > 0 AND type = ?", issue.ID, ReviewTypeRequest).Find(&teamReviewRequests); err != nil {
			return nil, nil, err
		}

		for _, teamReviewRequest := range teamReviewRequests {
			ok, err := organization.IsTeamMember(ctx, issue.Repo.OwnerID, teamReviewRequest.ReviewerTeamID, doer.ID)
			if err != nil {
				return nil, nil, err
			} else if !ok {
				continue
			}

			if _, err := db.DeleteByID[Review](ctx, teamReviewRequest.ID); err != nil {
				return nil, nil, err
			}
		}
	}

	comm.Review = review
	return review, comm, committer.Commit()
}

// GetReviewByIssueIDAndUserID get the latest review of reviewer for a pull request
func GetReviewByIssueIDAndUserID(ctx context.Context, issueID, userID int64) (*Review, error) {
	review := new(Review)

	has, err := db.GetEngine(ctx).Where(
		builder.In("type", ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest).
			And(builder.Eq{"issue_id": issueID, "reviewer_id": userID, "original_author_id": 0})).
		Desc("id").
		Get(review)
	if err != nil {
		return nil, err
	}

	if !has {
		return nil, ErrReviewNotExist{}
	}

	return review, nil
}

// GetTeamReviewerByIssueIDAndTeamID get the latest review request of reviewer team for a pull request
func GetTeamReviewerByIssueIDAndTeamID(ctx context.Context, issueID, teamID int64) (*Review, error) {
	review := new(Review)

	has, err := db.GetEngine(ctx).Where(builder.Eq{"issue_id": issueID, "reviewer_team_id": teamID}).
		Desc("id").
		Get(review)
	if err != nil {
		return nil, err
	}

	if !has {
		return nil, ErrReviewNotExist{0}
	}

	return review, err
}

// MarkReviewsAsStale marks existing reviews as stale
func MarkReviewsAsStale(ctx context.Context, issueID int64) (err error) {
	_, err = db.GetEngine(ctx).Exec("UPDATE `review` SET stale=? WHERE issue_id=?", true, issueID)

	return err
}

// MarkReviewsAsNotStale marks existing reviews as not stale for a giving commit SHA
func MarkReviewsAsNotStale(ctx context.Context, issueID int64, commitID string) (err error) {
	_, err = db.GetEngine(ctx).Exec("UPDATE `review` SET stale=? WHERE issue_id=? AND commit_id=?", false, issueID, commitID)

	return err
}

// DismissReview change the dismiss status of a review
func DismissReview(ctx context.Context, review *Review, isDismiss bool) (err error) {
	if review.Dismissed == isDismiss || (review.Type != ReviewTypeApprove && review.Type != ReviewTypeReject) {
		return nil
	}

	review.Dismissed = isDismiss

	if review.ID == 0 {
		return ErrReviewNotExist{}
	}

	_, err = db.GetEngine(ctx).ID(review.ID).Cols("dismissed").Update(review)

	return err
}

// InsertReviews inserts review and review comments
func InsertReviews(ctx context.Context, reviews []*Review) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	for _, review := range reviews {
		if _, err := sess.NoAutoTime().Insert(review); err != nil {
			return err
		}

		if _, err := sess.NoAutoTime().Insert(&Comment{
			Type:             CommentTypeReview,
			Content:          review.Content,
			PosterID:         review.ReviewerID,
			OriginalAuthor:   review.OriginalAuthor,
			OriginalAuthorID: review.OriginalAuthorID,
			IssueID:          review.IssueID,
			ReviewID:         review.ID,
			CreatedUnix:      review.CreatedUnix,
			UpdatedUnix:      review.UpdatedUnix,
		}); err != nil {
			return err
		}

		for _, c := range review.Comments {
			c.ReviewID = review.ID
		}

		if len(review.Comments) > 0 {
			if _, err := sess.NoAutoTime().Insert(review.Comments); err != nil {
				return err
			}
		}

		if err := UpdateIssueNumComments(ctx, review.IssueID); err != nil {
			return err
		}
	}

	return committer.Commit()
}

// AddReviewRequest add a review request from one reviewer
func AddReviewRequest(ctx context.Context, issue *Issue, reviewer, doer *user_model.User) (*Comment, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	review, err := GetReviewByIssueIDAndUserID(ctx, issue.ID, reviewer.ID)
	if err != nil && !IsErrReviewNotExist(err) {
		return nil, err
	}

	if review != nil {
		// skip it when reviewer hase been request to review
		if review.Type == ReviewTypeRequest {
			return nil, committer.Commit() // still commit the transaction, or committer.Close() will rollback it, even if it's a reused transaction.
		}

		if issue.IsClosed {
			return nil, ErrReviewRequestOnClosedPR{}
		}

		if issue.IsPull {
			if err := issue.LoadPullRequest(ctx); err != nil {
				return nil, err
			}
			if issue.PullRequest.HasMerged {
				return nil, ErrReviewRequestOnClosedPR{}
			}
		}
	}

	// if the reviewer is an official reviewer,
	// remove the official flag in the all previous reviews
	official, err := IsOfficialReviewer(ctx, issue, reviewer)
	if err != nil {
		return nil, err
	} else if official {
		if _, err := sess.Exec("UPDATE `review` SET official=? WHERE issue_id=? AND reviewer_id=?", false, issue.ID, reviewer.ID); err != nil {
			return nil, err
		}
	}

	review, err = CreateReview(ctx, CreateReviewOptions{
		Type:     ReviewTypeRequest,
		Issue:    issue,
		Reviewer: reviewer,
		Official: official,
		Stale:    false,
	})
	if err != nil {
		return nil, err
	}

	comment, err := CreateComment(ctx, &CreateCommentOptions{
		Type:            CommentTypeReviewRequest,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: false,       // Use RemovedAssignee as !isRequest
		AssigneeID:      reviewer.ID, // Use AssigneeID as reviewer ID
		ReviewID:        review.ID,
	})
	if err != nil {
		return nil, err
	}

	// func caller use the created comment to retrieve created review too.
	comment.Review = review

	return comment, committer.Commit()
}

// RemoveReviewRequest remove a review request from one reviewer
func RemoveReviewRequest(ctx context.Context, issue *Issue, reviewer, doer *user_model.User) (*Comment, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	review, err := GetReviewByIssueIDAndUserID(ctx, issue.ID, reviewer.ID)
	if err != nil && !IsErrReviewNotExist(err) {
		return nil, err
	}

	if review == nil || review.Type != ReviewTypeRequest {
		return nil, nil
	}

	if _, err = db.DeleteByBean(ctx, review); err != nil {
		return nil, err
	}

	official, err := IsOfficialReviewer(ctx, issue, reviewer)
	if err != nil {
		return nil, err
	} else if official {
		if err := restoreLatestOfficialReview(ctx, issue.ID, reviewer.ID); err != nil {
			return nil, err
		}
	}

	comment, err := CreateComment(ctx, &CreateCommentOptions{
		Type:            CommentTypeReviewRequest,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: true,        // Use RemovedAssignee as !isRequest
		AssigneeID:      reviewer.ID, // Use AssigneeID as reviewer ID
	})
	if err != nil {
		return nil, err
	}

	return comment, committer.Commit()
}

// Recalculate the latest official review for reviewer
func restoreLatestOfficialReview(ctx context.Context, issueID, reviewerID int64) error {
	review, err := GetReviewByIssueIDAndUserID(ctx, issueID, reviewerID)
	if err != nil && !IsErrReviewNotExist(err) {
		return err
	}

	if review != nil {
		if _, err := db.Exec(ctx, "UPDATE `review` SET official=? WHERE id=?", true, review.ID); err != nil {
			return err
		}
	}

	return nil
}

// AddTeamReviewRequest add a review request from one team
func AddTeamReviewRequest(ctx context.Context, issue *Issue, reviewer *organization.Team, doer *user_model.User) (*Comment, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	review, err := GetTeamReviewerByIssueIDAndTeamID(ctx, issue.ID, reviewer.ID)
	if err != nil && !IsErrReviewNotExist(err) {
		return nil, err
	}

	// This team already has been requested to review - therefore skip this.
	if review != nil {
		return nil, nil
	}

	official, err := IsOfficialReviewerTeam(ctx, issue, reviewer)
	if err != nil {
		return nil, fmt.Errorf("isOfficialReviewerTeam(): %w", err)
	} else if !official {
		if official, err = IsOfficialReviewer(ctx, issue, doer); err != nil {
			return nil, fmt.Errorf("isOfficialReviewer(): %w", err)
		}
	}

	if review, err = CreateReview(ctx, CreateReviewOptions{
		Type:         ReviewTypeRequest,
		Issue:        issue,
		ReviewerTeam: reviewer,
		Official:     official,
		Stale:        false,
	}); err != nil {
		return nil, err
	}

	if official {
		if _, err := db.Exec(ctx, "UPDATE `review` SET official=? WHERE issue_id=? AND reviewer_team_id=?", false, issue.ID, reviewer.ID); err != nil {
			return nil, err
		}
	}

	comment, err := CreateComment(ctx, &CreateCommentOptions{
		Type:            CommentTypeReviewRequest,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: false,       // Use RemovedAssignee as !isRequest
		AssigneeTeamID:  reviewer.ID, // Use AssigneeTeamID as reviewer team ID
		ReviewID:        review.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateComment(): %w", err)
	}

	return comment, committer.Commit()
}

// RemoveTeamReviewRequest remove a review request from one team
func RemoveTeamReviewRequest(ctx context.Context, issue *Issue, reviewer *organization.Team, doer *user_model.User) (*Comment, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	review, err := GetTeamReviewerByIssueIDAndTeamID(ctx, issue.ID, reviewer.ID)
	if err != nil && !IsErrReviewNotExist(err) {
		return nil, err
	}

	if review == nil {
		return nil, nil
	}

	if _, err = db.DeleteByBean(ctx, review); err != nil {
		return nil, err
	}

	official, err := IsOfficialReviewerTeam(ctx, issue, reviewer)
	if err != nil {
		return nil, fmt.Errorf("isOfficialReviewerTeam(): %w", err)
	}

	if official {
		// recalculate which is the latest official review from that team
		review, err := GetReviewByIssueIDAndUserID(ctx, issue.ID, -reviewer.ID)
		if err != nil && !IsErrReviewNotExist(err) {
			return nil, err
		}

		if review != nil {
			if _, err := db.Exec(ctx, "UPDATE `review` SET official=? WHERE id=?", true, review.ID); err != nil {
				return nil, err
			}
		}
	}

	if doer == nil {
		return nil, committer.Commit()
	}

	comment, err := CreateComment(ctx, &CreateCommentOptions{
		Type:            CommentTypeReviewRequest,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: true,        // Use RemovedAssignee as !isRequest
		AssigneeTeamID:  reviewer.ID, // Use AssigneeTeamID as reviewer team ID
	})
	if err != nil {
		return nil, fmt.Errorf("CreateComment(): %w", err)
	}

	return comment, committer.Commit()
}

// MarkConversation Add or remove Conversation mark for a code comment
func MarkConversation(ctx context.Context, comment *Comment, doer *user_model.User, isResolve bool) (err error) {
	if comment.Type != CommentTypeCode {
		return nil
	}

	if isResolve {
		if comment.ResolveDoerID != 0 {
			return nil
		}

		if _, err = db.GetEngine(ctx).Exec("UPDATE `comment` SET resolve_doer_id=? WHERE id=?", doer.ID, comment.ID); err != nil {
			return err
		}
	} else {
		if comment.ResolveDoerID == 0 {
			return nil
		}

		if _, err = db.GetEngine(ctx).Exec("UPDATE `comment` SET resolve_doer_id=? WHERE id=?", 0, comment.ID); err != nil {
			return err
		}
	}

	return nil
}

// CanMarkConversation  Add or remove Conversation mark for a code comment permission check
// the PR writer , offfcial reviewer and poster can do it
func CanMarkConversation(ctx context.Context, issue *Issue, doer *user_model.User) (permResult bool, err error) {
	if doer == nil || issue == nil {
		return false, fmt.Errorf("issue or doer is nil")
	}

	if doer.ID != issue.PosterID {
		if err = issue.LoadRepo(ctx); err != nil {
			return false, err
		}

		p, err := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
		if err != nil {
			return false, err
		}

		permResult = p.CanAccess(perm.AccessModeWrite, unit.TypePullRequests)
		if !permResult {
			if permResult, err = IsOfficialReviewer(ctx, issue, doer); err != nil {
				return false, err
			}
		}

		if !permResult {
			return false, nil
		}
	}

	return true, nil
}

// DeleteReview delete a review and it's code comments
func DeleteReview(ctx context.Context, r *Review) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if r.ID == 0 {
		return fmt.Errorf("review is not allowed to be 0")
	}

	if r.Type == ReviewTypeRequest {
		return fmt.Errorf("review request can not be deleted using this method")
	}

	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}

	if _, err := db.Delete[Comment](ctx, opts); err != nil {
		return err
	}

	opts = FindCommentsOptions{
		Type:     CommentTypeReview,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}

	if _, err := db.Delete[Comment](ctx, opts); err != nil {
		return err
	}

	opts = FindCommentsOptions{
		Type:     CommentTypeDismissReview,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}

	if _, err := db.Delete[Comment](ctx, opts); err != nil {
		return err
	}

	if _, err := db.DeleteByID[Review](ctx, r.ID); err != nil {
		return err
	}

	if r.Official {
		if err := restoreLatestOfficialReview(ctx, r.IssueID, r.ReviewerID); err != nil {
			return err
		}
	}

	return committer.Commit()
}

// GetCodeCommentsCount return count of CodeComments a Review has
func (r *Review) GetCodeCommentsCount(ctx context.Context) int {
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}
	conds := opts.ToConds()
	if r.ID == 0 {
		conds = conds.And(builder.Eq{"invalidated": false})
	}

	count, err := db.GetEngine(ctx).Where(conds).Count(new(Comment))
	if err != nil {
		return 0
	}
	return int(count)
}

// HTMLURL formats a URL-string to the related review issue-comment
func (r *Review) HTMLURL(ctx context.Context) string {
	opts := FindCommentsOptions{
		Type:     CommentTypeReview,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}
	comment := new(Comment)
	has, err := db.GetEngine(ctx).Where(opts.ToConds()).Get(comment)
	if err != nil || !has {
		return ""
	}
	return comment.HTMLURL(ctx)
}

// RemapExternalUser ExternalUserRemappable interface
func (r *Review) RemapExternalUser(externalName string, externalID, userID int64) error {
	r.OriginalAuthor = externalName
	r.OriginalAuthorID = externalID
	r.ReviewerID = userID
	return nil
}

// GetUserID ExternalUserRemappable interface
func (r *Review) GetUserID() int64 { return r.ReviewerID }

// GetExternalName ExternalUserRemappable interface
func (r *Review) GetExternalName() string { return r.OriginalAuthor }

// GetExternalID ExternalUserRemappable interface
func (r *Review) GetExternalID() int64 { return r.OriginalAuthorID }

// UpdateReviewsMigrationsByType updates reviews' migrations information via given git service type and original id and poster id
func UpdateReviewsMigrationsByType(ctx context.Context, tp structs.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := db.GetEngine(ctx).Table("review").
		Where("original_author_id = ?", originalAuthorID).
		And(migratedIssueCond(tp)).
		Update(map[string]any{
			"reviewer_id":        posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}
