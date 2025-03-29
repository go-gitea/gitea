// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/label"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrRepoLabelNotExist represents a "RepoLabelNotExist" kind of error.
type ErrRepoLabelNotExist struct {
	LabelID int64
	RepoID  int64
}

// IsErrRepoLabelNotExist checks if an error is a RepoErrLabelNotExist.
func IsErrRepoLabelNotExist(err error) bool {
	_, ok := err.(ErrRepoLabelNotExist)
	return ok
}

func (err ErrRepoLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, repo_id: %d]", err.LabelID, err.RepoID)
}

func (err ErrRepoLabelNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrOrgLabelNotExist represents a "OrgLabelNotExist" kind of error.
type ErrOrgLabelNotExist struct {
	LabelID int64
	OrgID   int64
}

// IsErrOrgLabelNotExist checks if an error is a OrgErrLabelNotExist.
func IsErrOrgLabelNotExist(err error) bool {
	_, ok := err.(ErrOrgLabelNotExist)
	return ok
}

func (err ErrOrgLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, org_id: %d]", err.LabelID, err.OrgID)
}

func (err ErrOrgLabelNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrLabelNotExist represents a "LabelNotExist" kind of error.
type ErrLabelNotExist struct {
	LabelID int64
}

// IsErrLabelNotExist checks if an error is a ErrLabelNotExist.
func IsErrLabelNotExist(err error) bool {
	_, ok := err.(ErrLabelNotExist)
	return ok
}

func (err ErrLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d]", err.LabelID)
}

func (err ErrLabelNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Label represents a label of repository for issues.
type Label struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	OrgID           int64 `xorm:"INDEX"`
	Name            string
	Exclusive       bool
	Description     string
	Color           string `xorm:"VARCHAR(7)"`
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

	ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT NULL"`
}

func init() {
	db.RegisterModel(new(Label))
	db.RegisterModel(new(IssueLabel))
}

// CalOpenIssues sets the number of open issues of a label based on the already stored number of closed issues.
func (l *Label) CalOpenIssues() {
	l.NumOpenIssues = l.NumIssues - l.NumClosedIssues
}

// SetArchived set the label as archived
func (l *Label) SetArchived(isArchived bool) {
	if !isArchived {
		l.ArchivedUnix = timeutil.TimeStamp(0)
	} else if isArchived && !l.IsArchived() {
		// Only change the date when it is newly archived.
		l.ArchivedUnix = timeutil.TimeStampNow()
	}
}

// IsArchived returns true if label is an archived
func (l *Label) IsArchived() bool {
	return !l.ArchivedUnix.IsZero()
}

// CalOpenOrgIssues calculates the open issues of a label for a specific repo
func (l *Label) CalOpenOrgIssues(ctx context.Context, repoID, labelID int64) {
	counts, _ := CountIssuesByRepo(ctx, &IssuesOptions{
		RepoIDs:  []int64{repoID},
		LabelIDs: []int64{labelID},
		IsClosed: optional.Some(false),
	})

	for _, count := range counts {
		l.NumOpenRepoIssues += count
	}
}

// LoadSelectedLabelsAfterClick calculates the set of selected labels when a label is clicked
func (l *Label) LoadSelectedLabelsAfterClick(currentSelectedLabels []int64, currentSelectedExclusiveScopes []string) {
	labelQueryParams := container.Set[string]{}
	labelSelected := false
	exclusiveScope := l.ExclusiveScope()
	for i, curSel := range currentSelectedLabels {
		if curSel == l.ID {
			labelSelected = true
		} else if -curSel == l.ID {
			labelSelected = true
			l.IsExcluded = true
		} else if curSel != 0 {
			// Exclude other labels in the same scope from selection
			if curSel < 0 || exclusiveScope == "" || exclusiveScope != currentSelectedExclusiveScopes[i] {
				labelQueryParams.Add(strconv.FormatInt(curSel, 10))
			}
		}
	}

	if !labelSelected {
		labelQueryParams.Add(strconv.FormatInt(l.ID, 10))
	}
	l.IsSelected = labelSelected

	// Sort and deduplicate the ids to avoid the crawlers asking for the
	// same thing with simply a different order of parameters
	labelQuerySliceStrings := labelQueryParams.Values()
	slices.Sort(labelQuerySliceStrings) // the sort is still needed because the underlying map of Set doesn't guarantee order
	l.QueryString = strings.Join(labelQuerySliceStrings, ",")
}

// BelongsToOrg returns true if label is an organization label
func (l *Label) BelongsToOrg() bool {
	return l.OrgID > 0
}

// BelongsToRepo returns true if label is a repository label
func (l *Label) BelongsToRepo() bool {
	return l.RepoID > 0
}

// ExclusiveScope returns scope substring of label name, or empty string if none exists
func (l *Label) ExclusiveScope() string {
	if !l.Exclusive {
		return ""
	}
	lastIndex := strings.LastIndex(l.Name, "/")
	if lastIndex == -1 || lastIndex == 0 || lastIndex == len(l.Name)-1 {
		return ""
	}
	return l.Name[:lastIndex]
}

// NewLabel creates a new label
func NewLabel(ctx context.Context, l *Label) error {
	color, err := label.NormalizeColor(l.Color)
	if err != nil {
		return err
	}
	l.Color = color

	return db.Insert(ctx, l)
}

// NewLabels creates new labels
func NewLabels(ctx context.Context, labels ...*Label) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, l := range labels {
		color, err := label.NormalizeColor(l.Color)
		if err != nil {
			return err
		}
		l.Color = color

		if err := db.Insert(ctx, l); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// UpdateLabel updates label information.
func UpdateLabel(ctx context.Context, l *Label) error {
	color, err := label.NormalizeColor(l.Color)
	if err != nil {
		return err
	}
	l.Color = color

	return updateLabelCols(ctx, l, "name", "description", "color", "exclusive", "archived_unix")
}

// DeleteLabel delete a label
func DeleteLabel(ctx context.Context, id, labelID int64) error {
	l, err := GetLabelByID(ctx, labelID)
	if err != nil {
		if IsErrLabelNotExist(err) {
			return nil
		}
		return err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	if l.BelongsToOrg() && l.OrgID != id {
		return nil
	}
	if l.BelongsToRepo() && l.RepoID != id {
		return nil
	}

	if _, err = db.DeleteByID[Label](ctx, labelID); err != nil {
		return err
	} else if _, err = sess.
		Where("label_id = ?", labelID).
		Delete(new(IssueLabel)); err != nil {
		return err
	}

	// delete comments about now deleted label_id
	if _, err = sess.Where("label_id = ?", labelID).Cols("label_id").Delete(&Comment{}); err != nil {
		return err
	}

	return committer.Commit()
}

// GetLabelByID returns a label by given ID.
func GetLabelByID(ctx context.Context, labelID int64) (*Label, error) {
	if labelID <= 0 {
		return nil, ErrLabelNotExist{labelID}
	}

	l := &Label{}
	has, err := db.GetEngine(ctx).ID(labelID).Get(l)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLabelNotExist{l.ID}
	}
	return l, nil
}

// GetLabelsByIDs returns a list of labels by IDs
func GetLabelsByIDs(ctx context.Context, labelIDs []int64, cols ...string) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	if len(labelIDs) == 0 {
		return labels, nil
	}
	return labels, db.GetEngine(ctx).Table("label").
		In("id", labelIDs).
		Asc("name").
		Cols(cols...).
		Find(&labels)
}

// GetLabelInRepoByName returns a label by name in given repository.
func GetLabelInRepoByName(ctx context.Context, repoID int64, labelName string) (*Label, error) {
	if len(labelName) == 0 || repoID <= 0 {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"name": labelName, "repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}
	return l, nil
}

// GetLabelInRepoByID returns a label by ID in given repository.
func GetLabelInRepoByID(ctx context.Context, repoID, labelID int64) (*Label, error) {
	if labelID <= 0 || repoID <= 0 {
		return nil, ErrRepoLabelNotExist{labelID, repoID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"id": labelID, "repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrRepoLabelNotExist{labelID, repoID}
	}
	return l, nil
}

// GetLabelIDsInRepoByNames returns a list of labelIDs by names in a given
// repository.
// it silently ignores label names that do not belong to the repository.
func GetLabelIDsInRepoByNames(ctx context.Context, repoID int64, labelNames []string) ([]int64, error) {
	labelIDs := make([]int64, 0, len(labelNames))
	return labelIDs, db.GetEngine(ctx).Table("label").
		Where("repo_id = ?", repoID).
		In("name", labelNames).
		Asc("name").
		Cols("id").
		Find(&labelIDs)
}

// GetLabelIDsInOrgByNames returns a list of labelIDs by names in a given org.
func GetLabelIDsInOrgByNames(ctx context.Context, orgID int64, labelNames []string) ([]int64, error) {
	labelIDs := make([]int64, 0, len(labelNames))
	return labelIDs, db.GetEngine(ctx).Table("label").
		Where("org_id = ?", orgID).
		In("name", labelNames).
		Asc("name").
		Cols("id").
		Find(&labelIDs)
}

// BuildLabelNamesIssueIDsCondition returns a builder where get issue ids match label names
func BuildLabelNamesIssueIDsCondition(labelNames []string) *builder.Builder {
	return builder.Select("issue_label.issue_id").
		From("issue_label").
		InnerJoin("label", "label.id = issue_label.label_id").
		Where(
			builder.In("label.name", labelNames),
		).
		GroupBy("issue_label.issue_id")
}

// GetLabelsInRepoByIDs returns a list of labels by IDs in given repository,
// it silently ignores label IDs that do not belong to the repository.
func GetLabelsInRepoByIDs(ctx context.Context, repoID int64, labelIDs []int64) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	if len(labelIDs) == 0 {
		return labels, nil
	}
	return labels, db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		In("id", labelIDs).
		Asc("name").
		Find(&labels)
}

// GetLabelsByRepoID returns all labels that belong to given repository by ID.
func GetLabelsByRepoID(ctx context.Context, repoID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	if repoID <= 0 {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}
	labels := make([]*Label, 0, 10)
	sess := db.GetEngine(ctx).Where("repo_id = ?", repoID)

	switch sortType {
	case "reversealphabetically":
		sess.Desc("name")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("name")
	}

	if listOptions.Page > 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return labels, sess.Find(&labels)
}

// CountLabelsByRepoID count number of all labels that belong to given repository by ID.
func CountLabelsByRepoID(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Where("repo_id = ?", repoID).Count(&Label{})
}

// GetLabelInOrgByName returns a label by name in given organization.
func GetLabelInOrgByName(ctx context.Context, orgID int64, labelName string) (*Label, error) {
	if len(labelName) == 0 || orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"name": labelName, "org_id": orgID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}
	return l, nil
}

// GetLabelInOrgByID returns a label by ID in given organization.
func GetLabelInOrgByID(ctx context.Context, orgID, labelID int64) (*Label, error) {
	if labelID <= 0 || orgID <= 0 {
		return nil, ErrOrgLabelNotExist{labelID, orgID}
	}

	l, exist, err := db.Get[Label](ctx, builder.Eq{"id": labelID, "org_id": orgID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrOrgLabelNotExist{labelID, orgID}
	}
	return l, nil
}

// GetLabelsInOrgByIDs returns a list of labels by IDs in given organization,
// it silently ignores label IDs that do not belong to the organization.
func GetLabelsInOrgByIDs(ctx context.Context, orgID int64, labelIDs []int64) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	if len(labelIDs) == 0 {
		return labels, nil
	}
	return labels, db.GetEngine(ctx).
		Where("org_id = ?", orgID).
		In("id", labelIDs).
		Asc("name").
		Find(&labels)
}

// GetLabelsByOrgID returns all labels that belong to given organization by ID.
func GetLabelsByOrgID(ctx context.Context, orgID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	if orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}
	labels := make([]*Label, 0, 10)
	sess := db.GetEngine(ctx).Where("org_id = ?", orgID)

	switch sortType {
	case "reversealphabetically":
		sess.Desc("name")
	case "leastissues":
		sess.Asc("num_issues")
	case "mostissues":
		sess.Desc("num_issues")
	default:
		sess.Asc("name")
	}

	if listOptions.Page > 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return labels, sess.Find(&labels)
}

// GetLabelIDsByNames returns a list of labelIDs by names.
// It doesn't filter them by repo or org, so it could return labels belonging to different repos/orgs.
// It's used for filtering issues via indexer, otherwise it would be useless.
// Since it could return labels with the same name, so the length of returned ids could be more than the length of names.
func GetLabelIDsByNames(ctx context.Context, labelNames []string) ([]int64, error) {
	labelIDs := make([]int64, 0, len(labelNames))
	return labelIDs, db.GetEngine(ctx).Table("label").
		In("name", labelNames).
		Cols("id").
		Find(&labelIDs)
}

// CountLabelsByOrgID count all labels that belong to given organization by ID.
func CountLabelsByOrgID(ctx context.Context, orgID int64) (int64, error) {
	return db.GetEngine(ctx).Where("org_id = ?", orgID).Count(&Label{})
}

func updateLabelCols(ctx context.Context, l *Label, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(l.ID).
		SetExpr("num_issues",
			builder.Select("count(*)").From("issue_label").
				Where(builder.Eq{"label_id": l.ID}),
		).
		SetExpr("num_closed_issues",
			builder.Select("count(*)").From("issue_label").
				InnerJoin("issue", "issue_label.issue_id = issue.id").
				Where(builder.Eq{
					"issue_label.label_id": l.ID,
					"issue.is_closed":      true,
				}),
		).
		Cols(cols...).Update(l)
	return err
}
