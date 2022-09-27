// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"context"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrRepoPriorityNotExist represents a "RepoPriorityNotExist" kind of error.
type ErrRepoPriorityNotExist struct {
	PriorityID int64
	RepoID     int64
}

// IsErrRepoPriorityNotExist checks if an error is a RepoErrPriorityNotExist.
func IsErrRepoPriorityNotExist(err error) bool {
	_, ok := err.(ErrRepoPriorityNotExist)
	return ok
}

func (err ErrRepoPriorityNotExist) Error() string {
	return fmt.Sprintf("priority does not exist [priority_id: %d, repo_id: %d]", err.PriorityID, err.RepoID)
}

// ErrOrgPriorityNotExist represents a "OrgPriorityNotExist" kind of error.
type ErrOrgPriorityNotExist struct {
	PriorityID int64
	OrgID      int64
}

// IsErrOrgPriorityNotExist checks if an error is a OrgErrPriorityNotExist.
func IsErrOrgPriorityNotExist(err error) bool {
	_, ok := err.(ErrOrgPriorityNotExist)
	return ok
}

func (err ErrOrgPriorityNotExist) Error() string {
	return fmt.Sprintf("priority does not exist [priority_id: %d, org_id: %d]", err.PriorityID, err.OrgID)
}

// ErrPriorityNotExist represents a "PriorityNotExist" kind of error.
type ErrPriorityNotExist struct {
	PriorityID int64
}

// IsErrPriorityNotExist checks if an error is a ErrPriorityNotExist.
func IsErrPriorityNotExist(err error) bool {
	_, ok := err.(ErrPriorityNotExist)
	return ok
}

func (err ErrPriorityNotExist) Error() string {
	return fmt.Sprintf("priority does not exist [priority_id: %d]", err.PriorityID)
}

// PriorityColorPattern is a regexp witch can validate PriorityColor
var PriorityColorPattern = regexp.MustCompile("^#?(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})$")

// Priority represents a priority of repository for issues.
type Priority struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	OrgID           int64 `xorm:"INDEX"`
	Name            string
	Description     string
	Color           string `xorm:"VARCHAR(7)"`
	Weight          int
	NumIssues       int
	NumClosedIssues int
	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`

	NumOpenIssues     int    `xorm:"-"`
	NumOpenRepoIssues int64  `xorm:"-"`
	IsChecked         bool   `xorm:"-"`
	QueryString       string `xorm:"-"`
	IsSelected        bool   `xorm:"-"`
	IsExcluded        bool   `xorm:"-"`
}

func init() {
	db.RegisterModel(new(Priority))
}

// CalOpenIssues sets the number of open issues of a label based on the already stored number of closed issues.
func (priority *Priority) CalOpenIssues() {
	priority.NumOpenIssues = priority.NumIssues - priority.NumClosedIssues
}

// CalOpenOrgIssues calculates the open issues of a label for a specific repo
func (priority *Priority) CalOpenOrgIssues(repoID, priorityID int64) {
	counts, _ := CountIssuesByRepo(&IssuesOptions{
		RepoID:      repoID,
		PriorityIDs: []int64{priorityID},
		IsClosed:    util.OptionalBoolFalse,
	})

	for _, count := range counts {
		priority.NumOpenRepoIssues += count
	}
}

// LoadSelectedLabelsAfterClick calculates the set of selected labels when a label is clicked
func (priority *Priority) LoadSelectedPrioritiesAfterClick(currentSelectedPriorities []int64) {
	var priorityQuerySlice []string
	labelSelected := false
	labelID := strconv.FormatInt(priority.ID, 10)
	for _, s := range currentSelectedPriorities {
		if s == priority.ID {
			labelSelected = true
		} else if -s == priority.ID {
			labelSelected = true
			priority.IsExcluded = true
		} else if s != 0 {
			priorityQuerySlice = append(priorityQuerySlice, strconv.FormatInt(s, 10))
		}
	}
	if !labelSelected {
		priorityQuerySlice = append(priorityQuerySlice, labelID)
	}
	priority.IsSelected = labelSelected
	priority.QueryString = strings.Join(priorityQuerySlice, ",")
}

// BelongsToOrg returns true if label is an organization label
func (priority *Priority) BelongsToOrg() bool {
	return priority.OrgID > 0
}

// BelongsToRepo returns true if label is a repository label
func (priority *Priority) BelongsToRepo() bool {
	return priority.RepoID > 0
}

// ForegroundColor calculates the text color for labels based
// on their background color.
func (priority *Priority) ForegroundColor() template.CSS {
	if strings.HasPrefix(priority.Color, "#") {
		if color, err := strconv.ParseUint(priority.Color[1:], 16, 64); err == nil {
			// NOTE: see web_src/js/components/ContextPopup.vue for similar implementation
			luminance := Luminance(uint32(color))

			// prefer white or black based upon contrast
			if luminance < LuminanceThreshold {
				return template.CSS("#fff")
			}
			return template.CSS("#000")
		}
	}

	// default to black
	return template.CSS("#000")
}

// NewPriority creates a new Priority
func NewPriority(ctx context.Context, priority *Priority) error {
	if !LabelColorPattern.MatchString(priority.Color) {
		return fmt.Errorf("bad color code: %s", priority.Color)
	}

	// normalize case
	priority.Color = strings.ToLower(priority.Color)

	// add leading hash
	if priority.Color[0] != '#' {
		priority.Color = "#" + priority.Color
	}

	// convert 3-character shorthand into 6-character version
	if len(priority.Color) == 4 {
		r := priority.Color[1]
		g := priority.Color[2]
		b := priority.Color[3]
		priority.Color = fmt.Sprintf("#%c%c%c%c%c%c", r, r, g, g, b, b)
	}

	return db.Insert(ctx, priority)
}

// NewPriorities creates new labels
func NewPriorities(priorities ...*Priority) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, priority := range priorities {
		if !LabelColorPattern.MatchString(priority.Color) {
			return fmt.Errorf("bad color code: %s", priority.Color)
		}
		if err := db.Insert(ctx, priority); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// UpdateLabel updates label information.
func UpdatePriority(priority *Priority) error {
	if !PriorityColorPattern.MatchString(priority.Color) {
		return fmt.Errorf("bad color code: %s", priority.Color)
	}
	return updatePriorityCols(db.DefaultContext, priority, "name", "description", "color", "weight")
}

// DeletePriority delete a label
func DeletePriority(id, priorityID int64) error {
	priority, err := GetPriorityByID(db.DefaultContext, priorityID)
	if err != nil {
		if IsErrPriorityNotExist(err) {
			return nil
		}
		return err
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	if priority.BelongsToOrg() && priority.OrgID != id {
		return nil
	}
	if priority.BelongsToRepo() && priority.RepoID != id {
		return nil
	}

	if _, err = sess.ID(priorityID).Delete(new(Priority)); err != nil {
		return err
	} else if _, err = sess.
		Where("label_id = ?", priorityID).
		Delete(new(IssueLabel)); err != nil {
		return err
	}

	// delete comments about now deleted label_id
	if _, err = sess.Where("priority_id = ?", priorityID).Cols("priority_id").Delete(&Comment{}); err != nil {
		return err
	}

	return committer.Commit()
}

// GetLabelByID returns a label by given ID.
func GetPriorityByID(ctx context.Context, priorityID int64) (*Priority, error) {
	if priorityID <= 0 {
		return nil, ErrLabelNotExist{priorityID}
	}

	priority := &Priority{}
	has, err := db.GetEngine(ctx).ID(priorityID).Get(priority)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPriorityNotExist{priority.ID}
	}
	return priority, nil
}

// GetLabelsByIDs returns a list of labels by IDs
func GetPrioritiesByIDs(prioritiesIDs []int64) ([]*Priority, error) {
	priorities := make([]*Priority, 0, len(prioritiesIDs))
	return priorities, db.GetEngine(db.DefaultContext).Table("priorities").
		In("id", prioritiesIDs).
		Asc("name").
		Cols("id", "repo_id", "org_id").
		Find(&priorities)
}

// __________                           .__  __
// \______   \ ____ ______   ____  _____|__|/  |_  ___________ ___.__.
//  |       _// __ \\____ \ /  _ \/  ___/  \   __\/  _ \_  __ <   |  |
//  |    |   \  ___/|  |_> >  <_> )___ \|  ||  | (  <_> )  | \/\___  |
//  |____|_  /\___  >   __/ \____/____  >__||__|  \____/|__|   / ____|
//         \/     \/|__|              \/                       \/

// GetLabelInRepoByName returns a label by name in given repository.
func GetPriorityInRepoByName(ctx context.Context, repoID int64, labelName string) (*Priority, error) {
	if len(labelName) == 0 || repoID <= 0 {
		return nil, ErrRepoPriorityNotExist{0, repoID}
	}

	priority := &Priority{
		Name:   labelName,
		RepoID: repoID,
	}
	has, err := db.GetByBean(ctx, priority)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoPriorityNotExist{0, priority.RepoID}
	}
	return priority, nil
}

// GetLabelInRepoByID returns a label by ID in given repository.
func GetPriorityInRepoByID(ctx context.Context, repoID, priorityID int64) (*Priority, error) {
	if priorityID <= 0 || repoID <= 0 {
		return nil, ErrRepoPriorityNotExist{priorityID, repoID}
	}

	priority := &Priority{
		ID:     priorityID,
		RepoID: repoID,
	}
	has, err := db.GetByBean(ctx, priority)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoPriorityNotExist{priority.ID, priority.RepoID}
	}
	return priority, nil
}

// GetLabelIDsInRepoByNames returns a list of labelIDs by names in a given
// repository.
// it silently ignores label names that do not belong to the repository.
func GetPriorityIDsInRepoByNames(repoID int64, priorityNames []string) ([]int64, error) {
	priorityIDs := make([]int64, 0, len(priorityNames))
	return priorityIDs, db.GetEngine(db.DefaultContext).Table("priority").
		Where("repo_id = ?", repoID).
		In("name", priorityNames).
		Asc("name").
		Cols("id").
		Find(&priorityIDs)
}

// BuildLabelNamesIssueIDsCondition returns a builder where get issue ids match label names
func BuildPriorityNamesIssueIDsCondition(priorityNames []string) *builder.Builder {
	return builder.Select("issue_label.issue_id").
		From("issue").
		InnerJoin("priority", "priority.id = issue.priority_id").
		Where(
			builder.In("priority.name", priorityNames),
		).
		GroupBy("issue.id")
}

// GetLabelsInRepoByIDs returns a list of labels by IDs in given repository,
// it silently ignores label IDs that do not belong to the repository.
func GetPriorityInRepoByIDs(repoID int64, priorityIDs []int64) ([]*Priority, error) {
	priorities := make([]*Priority, 0, len(priorityIDs))
	return priorities, db.GetEngine(db.DefaultContext).
		Where("repo_id = ?", repoID).
		In("id", priorities).
		Asc("name").
		Find(&priorities)
}

// GetLabelsByRepoID returns all labels that belong to given repository by ID.
func GetPrioritiesByRepoID(ctx context.Context, repoID int64, sortType string, listOptions db.ListOptions) ([]*Priority, error) {
	if repoID <= 0 {
		return nil, ErrRepoPriorityNotExist{0, repoID}
	}
	priorities := make([]*Priority, 0, 10)
	sess := db.GetEngine(ctx).Where("repo_id = ?", repoID)

	switch sortType {
	case "reverseweight":
		sess.Desc("weight")
	case "weight":
		sess.Asc("weight")
	case "reversealphabetically":
		sess.Desc("name")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("name")
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return priorities, sess.Find(&priorities)
}

// CountLabelsByRepoID count number of all labels that belong to given repository by ID.
func CountPrioritiesByRepoID(repoID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("repo_id = ?", repoID).Count(&Priority{})
}

// ________
// \_____  \_______  ____
//  /   |   \_  __ \/ ___\
// /    |    \  | \/ /_/  >
// \_______  /__|  \___  /
//         \/     /_____/

// GetLabelInOrgByName returns a label by name in given organization.
func GetPriorityInOrgByName(ctx context.Context, orgID int64, priorityName string) (*Priority, error) {
	if len(priorityName) == 0 || orgID <= 0 {
		return nil, ErrOrgPriorityNotExist{0, orgID}
	}

	priority := &Priority{
		Name:  priorityName,
		OrgID: orgID,
	}
	has, err := db.GetByBean(ctx, priority)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrOrgPriorityNotExist{0, priority.OrgID}
	}
	return priority, nil
}

// GetLabelInOrgByID returns a label by ID in given organization.
func GetPriorityInOrgByID(ctx context.Context, orgID, priorityID int64) (*Priority, error) {
	if priorityID <= 0 || orgID <= 0 {
		return nil, ErrOrgPriorityNotExist{priorityID, orgID}
	}

	priority := &Priority{
		ID:    priorityID,
		OrgID: orgID,
	}
	has, err := db.GetByBean(ctx, priority)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrOrgPriorityNotExist{priority.ID, priority.OrgID}
	}
	return priority, nil
}

// GetLabelIDsInOrgByNames returns a list of labelIDs by names in a given
// organization.
func GetPriorityIDsInOrgByNames(orgID int64, priorityNames []string) ([]int64, error) {
	if orgID <= 0 {
		return nil, ErrOrgPriorityNotExist{0, orgID}
	}
	priorityIDs := make([]int64, 0, len(priorityNames))

	return priorityIDs, db.GetEngine(db.DefaultContext).Table("priority").
		Where("org_id = ?", orgID).
		In("name", priorityNames).
		Asc("name").
		Cols("id").
		Find(&priorityIDs)
}

// GetLabelsInOrgByIDs returns a list of labels by IDs in given organization,
// it silently ignores label IDs that do not belong to the organization.
func GetPriorityInOrgByIDs(orgID int64, priorityIDs []int64) ([]*Priority, error) {
	priorities := make([]*Priority, 0, len(priorityIDs))
	return priorities, db.GetEngine(db.DefaultContext).
		Where("org_id = ?", orgID).
		In("id", priorityIDs).
		Asc("name").
		Find(&priorities)
}

// GetLabelsByOrgID returns all labels that belong to given organization by ID.
func GetPrioritiesByOrgID(ctx context.Context, orgID int64, sortType string, listOptions db.ListOptions) ([]*Priority, error) {
	if orgID <= 0 {
		return nil, ErrOrgPriorityNotExist{0, orgID}
	}
	priorities := make([]*Priority, 0, 10)
	sess := db.GetEngine(ctx).Where("org_id = ?", orgID)

	switch sortType {
	case "reverseweight":
		sess.Desc("weight")
	case "weight":
		sess.Asc("weight")
	case "reversealphabetically":
		sess.Desc("name")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("name")
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return priorities, sess.Find(&priorities)
}

// CountLabelsByOrgID count all labels that belong to given organization by ID.
func CountPrioritiesByOrgID(orgID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("org_id = ?", orgID).Count(&Priority{})
}

// .___
// |   | ______ ________ __   ____
// |   |/  ___//  ___/  |  \_/ __ \
// |   |\___ \ \___ \|  |  /\  ___/
// |___/____  >____  >____/  \___ |
//          \/     \/            \/

// GetLabelsByIssueID returns all labels that belong to given issue by ID.
func GetPrioritiesByIssueID(ctx context.Context, issueID int64) ([]*Priority, error) {
	var priorities []*Priority
	return priorities, db.GetEngine(ctx).Where("issue.id = ?", issueID).
		Join("LEFT", "issue", "issue.priority_id = label.id").
		Asc("priority.name").
		Find(&priorities)
}

// GetLabelsByIssueID returns all labels that belong to given issue by ID.
func GetPriorityByIssueID(ctx context.Context, issueID int64) (*Priority, error) {
	priorities, err := GetPrioritiesByIssueID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if len(priorities) == 0 {
		return nil, nil
	}
	return priorities[0], nil
}

func updatePriorityCols(ctx context.Context, priority *Priority, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(priority.ID).
		SetExpr("num_issues",
			builder.Select("count(*)").From("priority").
				Where(builder.Eq{"id": priority.ID}),
		).
		SetExpr("num_closed_issues",
			builder.Select("count(*)").From("priority").
				InnerJoin("issue", "priority.id = issue.priority_id").
				Where(builder.Eq{
					"priority.id":     priority.ID,
					"issue.is_closed": true,
				}),
		).
		Cols(cols...).Update(priority)
	return err
}

// HasIssueLabel returns true if issue has been labeled.
func HasIssuePriority(ctx context.Context, issueID, priority int64) bool {
	has, _ := db.GetEngine(ctx).Where("id = ? AND priority = ?", issueID, priority).Get(new(Issue))
	return has
}

// newIssuePriority this function creates a new label it does not check if the label is valid for the issue
// YOU MUST CHECK THIS BEFORE THIS FUNCTION
func newIssuePriority(ctx context.Context, issue *Issue, priority *Priority, doer *user_model.User) (err error) {
	issue.PriorityID = priority.ID

	if err = UpdateIssueCols(ctx, issue, "priority_id"); err != nil {
		return fmt.Errorf("updateIssueCols: %v", err)
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return
	}

	opts := &CreateCommentOptions{
		Type:     CommentTypePriority,
		Doer:     doer,
		Repo:     issue.Repo,
		Issue:    issue,
		Priority: priority,
		Content:  "1",
	}
	if _, err = CreateCommentCtx(ctx, opts); err != nil {
		return err
	}

	return updatePriorityCols(ctx, priority, "num_issues", "num_closed_issue")
}

// NewIssueLabel creates a new issue-label relation.
func NewIssuePriority(issue *Issue, priority *Priority, doer *user_model.User) (err error) {
	if HasIssueLabel(db.DefaultContext, issue.ID, priority.ID) {
		return nil
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	// Do NOT add invalid labels
	if issue.RepoID != priority.RepoID && issue.Repo.OwnerID != priority.OrgID {
		return nil
	}

	if err = newIssuePriority(ctx, issue, priority, doer); err != nil {
		return err
	}

	issue.Priority = nil
	if err = issue.LoadPriorities(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

// newIssueLabels add labels to an issue. It will check if the labels are valid for the issue
func newIssuePriorities(ctx context.Context, issue *Issue, priorities []*Priority, doer *user_model.User) (err error) {
	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}
	for _, priority := range priorities {
		// Don't add already present labels and invalid labels
		if HasIssueLabel(ctx, issue.ID, priority.ID) ||
			(priority.RepoID != issue.RepoID && priority.OrgID != issue.Repo.OwnerID) {
			continue
		}

		if err = newIssuePriority(ctx, issue, priority, doer); err != nil {
			return fmt.Errorf("newIssuePriority: %v", err)
		}
	}

	return nil
}

// NewIssueLabels creates a list of issue-label relations.
func NewIssuePriorities(issue *Issue, priorities []*Priority, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = newIssuePriorities(ctx, issue, priorities, doer); err != nil {
		return err
	}

	if err = issue.LoadPriorities(ctx); err != nil {
		issue.Priority = nil
		return err
	}

	return committer.Commit()
}

func deleteIssuePriority(ctx context.Context, issue *Issue, priority *Priority, doer *user_model.User) (err error) {
	issue.PriorityID = 0

	if err = UpdateIssueCols(ctx, issue, "priority_id"); err != nil {
		return fmt.Errorf("updateIssueCols: %v", err)
	}
	if err = issue.LoadRepo(ctx); err != nil {
		return
	}

	opts := &CreateCommentOptions{
		Type:     CommentTypePriority,
		Doer:     doer,
		Repo:     issue.Repo,
		Issue:    issue,
		Priority: priority,
	}
	if _, err = CreateCommentCtx(ctx, opts); err != nil {
		return err
	}

	return updatePriorityCols(ctx, priority, "num_issues", "num_closed_issue")
}

// DeleteIssueLabel deletes issue-label relation.
func DeleteIssuePriority(ctx context.Context, issue *Issue, priority *Priority, doer *user_model.User) error {
	if err := deleteIssuePriority(ctx, issue, priority, doer); err != nil {
		return err
	}

	issue.Priority = nil
	return issue.LoadPriorities(ctx)
}

// DeleteLabelsByRepoID  deletes labels of some repository
func DeletePrioritiesByRepoID(ctx context.Context, repoID int64) error {
	_, err := db.DeleteByBean(ctx, &Priority{RepoID: repoID})
	return err
}

// CountOrphanedLabels return count of labels witch are broken and not accessible via ui anymore
func CountOrphanedPriorities() (int64, error) {
	noref, err := db.GetEngine(db.DefaultContext).Table("priority").Where("repo_id=? AND org_id=?", 0, 0).Count()
	if err != nil {
		return 0, err
	}

	norepo, err := db.GetEngine(db.DefaultContext).Table("priority").
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("repository")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	noorg, err := db.GetEngine(db.DefaultContext).Table("priority").
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("user")),
		)).
		Count()
	if err != nil {
		return 0, err
	}

	return noref + norepo + noorg, nil
}

// DeleteOrphanedLabels delete labels witch are broken and not accessible via ui anymore
func DeleteOrphanedPriorities() error {
	// delete labels with no reference
	if _, err := db.GetEngine(db.DefaultContext).Table("priority").Where("repo_id=? AND org_id=?", 0, 0).Delete(new(Priority)); err != nil {
		return err
	}

	// delete priorities with none existing repos
	if _, err := db.GetEngine(db.DefaultContext).
		Where(builder.And(
			builder.Gt{"repo_id": 0},
			builder.NotIn("repo_id", builder.Select("id").From("repository")),
		)).
		Delete(Priority{}); err != nil {
		return err
	}

	// delete priorities with none existing orgs
	if _, err := db.GetEngine(db.DefaultContext).
		Where(builder.And(
			builder.Gt{"org_id": 0},
			builder.NotIn("org_id", builder.Select("id").From("user")),
		)).
		Delete(Priority{}); err != nil {
		return err
	}

	return nil
}
