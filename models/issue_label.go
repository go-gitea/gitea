// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// LabelColorPattern is a regexp witch can validate LabelColor
var LabelColorPattern = regexp.MustCompile("^#?(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})$")

// Label represents a label of repository for issues.
type Label struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	OrgID           int64 `xorm:"INDEX"`
	Name            string
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
}

func init() {
	db.RegisterModel(new(Label))
	db.RegisterModel(new(IssueLabel))
}

// GetLabelTemplateFile loads the label template file by given name,
// then parses and returns a list of name-color pairs and optionally description.
func GetLabelTemplateFile(name string) ([][3]string, error) {
	data, err := GetRepoInitFile("label", name)
	if err != nil {
		return nil, ErrIssueLabelTemplateLoad{name, fmt.Errorf("GetRepoInitFile: %v", err)}
	}

	lines := strings.Split(string(data), "\n")
	list := make([][3]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, ";", 2)

		fields := strings.SplitN(parts[0], " ", 2)
		if len(fields) != 2 {
			return nil, ErrIssueLabelTemplateLoad{name, fmt.Errorf("line is malformed: %s", line)}
		}

		color := strings.Trim(fields[0], " ")
		if len(color) == 6 {
			color = "#" + color
		}
		if !LabelColorPattern.MatchString(color) {
			return nil, ErrIssueLabelTemplateLoad{name, fmt.Errorf("bad HTML color code in line: %s", line)}
		}

		var description string

		if len(parts) > 1 {
			description = strings.TrimSpace(parts[1])
		}

		fields[1] = strings.TrimSpace(fields[1])
		list = append(list, [3]string{fields[1], color, description})
	}

	return list, nil
}

// CalOpenIssues sets the number of open issues of a label based on the already stored number of closed issues.
func (label *Label) CalOpenIssues() {
	label.NumOpenIssues = label.NumIssues - label.NumClosedIssues
}

// CalOpenOrgIssues calculates the open issues of a label for a specific repo
func (label *Label) CalOpenOrgIssues(repoID, labelID int64) {
	repoIDs := []int64{repoID}
	labelIDs := []int64{labelID}

	counts, _ := CountIssuesByRepo(&IssuesOptions{
		RepoIDs:  repoIDs,
		LabelIDs: labelIDs,
	})

	for _, count := range counts {
		label.NumOpenRepoIssues += count
	}
}

// LoadSelectedLabelsAfterClick calculates the set of selected labels when a label is clicked
func (label *Label) LoadSelectedLabelsAfterClick(currentSelectedLabels []int64) {
	var labelQuerySlice []string
	labelSelected := false
	labelID := strconv.FormatInt(label.ID, 10)
	for _, s := range currentSelectedLabels {
		if s == label.ID {
			labelSelected = true
		} else if -s == label.ID {
			labelSelected = true
			label.IsExcluded = true
		} else if s != 0 {
			labelQuerySlice = append(labelQuerySlice, strconv.FormatInt(s, 10))
		}
	}
	if !labelSelected {
		labelQuerySlice = append(labelQuerySlice, labelID)
	}
	label.IsSelected = labelSelected
	label.QueryString = strings.Join(labelQuerySlice, ",")
}

// BelongsToOrg returns true if label is an organization label
func (label *Label) BelongsToOrg() bool {
	return label.OrgID > 0
}

// BelongsToRepo returns true if label is a repository label
func (label *Label) BelongsToRepo() bool {
	return label.RepoID > 0
}

// SrgbToLinear converts a component of an sRGB color to its linear intensity
// See: https://en.wikipedia.org/wiki/SRGB#The_reverse_transformation_(sRGB_to_CIE_XYZ)
func SrgbToLinear(color uint8) float64 {
	flt := float64(color) / 255
	if flt <= 0.04045 {
		return flt / 12.92
	}
	return math.Pow((flt+0.055)/1.055, 2.4)
}

// Luminance returns the luminance of an sRGB color
func Luminance(color uint32) float64 {
	r := SrgbToLinear(uint8(0xFF & (color >> 16)))
	g := SrgbToLinear(uint8(0xFF & (color >> 8)))
	b := SrgbToLinear(uint8(0xFF & color))

	// luminance ratios for sRGB
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// LuminanceThreshold is the luminance at which white and black appear to have the same contrast
// i.e. x such that 1.05 / (x + 0.05) = (x + 0.05) / 0.05
// i.e. math.Sqrt(1.05*0.05) - 0.05
const LuminanceThreshold float64 = 0.179

// ForegroundColor calculates the text color for labels based
// on their background color.
func (label *Label) ForegroundColor() template.CSS {
	if strings.HasPrefix(label.Color, "#") {
		if color, err := strconv.ParseUint(label.Color[1:], 16, 64); err == nil {
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

// .____          ___.          .__
// |    |   _____ \_ |__   ____ |  |
// |    |   \__  \ | __ \_/ __ \|  |
// |    |___ / __ \| \_\ \  ___/|  |__
// >_______ (____  /___  /\___  >____/

func loadLabels(labelTemplate string) ([]string, error) {
	list, err := GetLabelTemplateFile(labelTemplate)
	if err != nil {
		return nil, err
	}

	labels := make([]string, len(list))
	for i := 0; i < len(list); i++ {
		labels[i] = list[i][0]
	}
	return labels, nil
}

// LoadLabelsFormatted loads the labels' list of a template file as a string separated by comma
func LoadLabelsFormatted(labelTemplate string) (string, error) {
	labels, err := loadLabels(labelTemplate)
	return strings.Join(labels, ", "), err
}

func initializeLabels(e db.Engine, id int64, labelTemplate string, isOrg bool) error {
	list, err := GetLabelTemplateFile(labelTemplate)
	if err != nil {
		return err
	}

	labels := make([]*Label, len(list))
	for i := 0; i < len(list); i++ {
		labels[i] = &Label{
			Name:        list[i][0],
			Description: list[i][2],
			Color:       list[i][1],
		}
		if isOrg {
			labels[i].OrgID = id
		} else {
			labels[i].RepoID = id
		}
	}
	for _, label := range labels {
		if err = newLabel(e, label); err != nil {
			return err
		}
	}
	return nil
}

// InitializeLabels adds a label set to a repository using a template
func InitializeLabels(ctx context.Context, repoID int64, labelTemplate string, isOrg bool) error {
	return initializeLabels(db.GetEngine(ctx), repoID, labelTemplate, isOrg)
}

func newLabel(e db.Engine, label *Label) error {
	_, err := e.Insert(label)
	return err
}

// NewLabel creates a new label
func NewLabel(label *Label) error {
	if !LabelColorPattern.MatchString(label.Color) {
		return fmt.Errorf("bad color code: %s", label.Color)
	}

	// normalize case
	label.Color = strings.ToLower(label.Color)

	// add leading hash
	if label.Color[0] != '#' {
		label.Color = "#" + label.Color
	}

	// convert 3-character shorthand into 6-character version
	if len(label.Color) == 4 {
		r := label.Color[1]
		g := label.Color[2]
		b := label.Color[3]
		label.Color = fmt.Sprintf("#%c%c%c%c%c%c", r, r, g, g, b, b)
	}

	return newLabel(db.GetEngine(db.DefaultContext), label)
}

// NewLabels creates new labels
func NewLabels(labels ...*Label) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, label := range labels {
		if !LabelColorPattern.MatchString(label.Color) {
			return fmt.Errorf("bad color code: %s", label.Color)
		}
		if err := newLabel(db.GetEngine(ctx), label); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// UpdateLabel updates label information.
func UpdateLabel(l *Label) error {
	if !LabelColorPattern.MatchString(l.Color) {
		return fmt.Errorf("bad color code: %s", l.Color)
	}
	return updateLabelCols(db.GetEngine(db.DefaultContext), l, "name", "description", "color")
}

// DeleteLabel delete a label
func DeleteLabel(id, labelID int64) error {
	label, err := GetLabelByID(labelID)
	if err != nil {
		if IsErrLabelNotExist(err) {
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

	if label.BelongsToOrg() && label.OrgID != id {
		return nil
	}
	if label.BelongsToRepo() && label.RepoID != id {
		return nil
	}

	if _, err = sess.ID(labelID).Delete(new(Label)); err != nil {
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

// getLabelByID returns a label by label id
func getLabelByID(e db.Engine, labelID int64) (*Label, error) {
	if labelID <= 0 {
		return nil, ErrLabelNotExist{labelID}
	}

	l := &Label{}
	has, err := e.ID(labelID).Get(l)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLabelNotExist{l.ID}
	}
	return l, nil
}

// GetLabelByID returns a label by given ID.
func GetLabelByID(id int64) (*Label, error) {
	return getLabelByID(db.GetEngine(db.DefaultContext), id)
}

// GetLabelsByIDs returns a list of labels by IDs
func GetLabelsByIDs(labelIDs []int64) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	return labels, db.GetEngine(db.DefaultContext).Table("label").
		In("id", labelIDs).
		Asc("name").
		Cols("id", "repo_id", "org_id").
		Find(&labels)
}

// __________                           .__  __
// \______   \ ____ ______   ____  _____|__|/  |_  ___________ ___.__.
//  |       _// __ \\____ \ /  _ \/  ___/  \   __\/  _ \_  __ <   |  |
//  |    |   \  ___/|  |_> >  <_> )___ \|  ||  | (  <_> )  | \/\___  |
//  |____|_  /\___  >   __/ \____/____  >__||__|  \____/|__|   / ____|
//         \/     \/|__|              \/                       \/

// getLabelInRepoByName returns a label by Name in given repository.
func getLabelInRepoByName(e db.Engine, repoID int64, labelName string) (*Label, error) {
	if len(labelName) == 0 || repoID <= 0 {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}

	l := &Label{
		Name:   labelName,
		RepoID: repoID,
	}
	has, err := e.Get(l)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoLabelNotExist{0, l.RepoID}
	}
	return l, nil
}

// getLabelInRepoByID returns a label by ID in given repository.
func getLabelInRepoByID(e db.Engine, repoID, labelID int64) (*Label, error) {
	if labelID <= 0 || repoID <= 0 {
		return nil, ErrRepoLabelNotExist{labelID, repoID}
	}

	l := &Label{
		ID:     labelID,
		RepoID: repoID,
	}
	has, err := e.Get(l)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoLabelNotExist{l.ID, l.RepoID}
	}
	return l, nil
}

// GetLabelInRepoByName returns a label by name in given repository.
func GetLabelInRepoByName(repoID int64, labelName string) (*Label, error) {
	return getLabelInRepoByName(db.GetEngine(db.DefaultContext), repoID, labelName)
}

// GetLabelIDsInRepoByNames returns a list of labelIDs by names in a given
// repository.
// it silently ignores label names that do not belong to the repository.
func GetLabelIDsInRepoByNames(repoID int64, labelNames []string) ([]int64, error) {
	labelIDs := make([]int64, 0, len(labelNames))
	return labelIDs, db.GetEngine(db.DefaultContext).Table("label").
		Where("repo_id = ?", repoID).
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

// GetLabelInRepoByID returns a label by ID in given repository.
func GetLabelInRepoByID(repoID, labelID int64) (*Label, error) {
	return getLabelInRepoByID(db.GetEngine(db.DefaultContext), repoID, labelID)
}

// GetLabelsInRepoByIDs returns a list of labels by IDs in given repository,
// it silently ignores label IDs that do not belong to the repository.
func GetLabelsInRepoByIDs(repoID int64, labelIDs []int64) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	return labels, db.GetEngine(db.DefaultContext).
		Where("repo_id = ?", repoID).
		In("id", labelIDs).
		Asc("name").
		Find(&labels)
}

func getLabelsByRepoID(e db.Engine, repoID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	if repoID <= 0 {
		return nil, ErrRepoLabelNotExist{0, repoID}
	}
	labels := make([]*Label, 0, 10)
	sess := e.Where("repo_id = ?", repoID)

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

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return labels, sess.Find(&labels)
}

// GetLabelsByRepoID returns all labels that belong to given repository by ID.
func GetLabelsByRepoID(repoID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	return getLabelsByRepoID(db.GetEngine(db.DefaultContext), repoID, sortType, listOptions)
}

// CountLabelsByRepoID count number of all labels that belong to given repository by ID.
func CountLabelsByRepoID(repoID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("repo_id = ?", repoID).Count(&Label{})
}

// ________
// \_____  \_______  ____
//  /   |   \_  __ \/ ___\
// /    |    \  | \/ /_/  >
// \_______  /__|  \___  /
//         \/     /_____/

// getLabelInOrgByName returns a label by Name in given organization
func getLabelInOrgByName(e db.Engine, orgID int64, labelName string) (*Label, error) {
	if len(labelName) == 0 || orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}

	l := &Label{
		Name:  labelName,
		OrgID: orgID,
	}
	has, err := e.Get(l)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrOrgLabelNotExist{0, l.OrgID}
	}
	return l, nil
}

// getLabelInOrgByID returns a label by ID in given organization.
func getLabelInOrgByID(e db.Engine, orgID, labelID int64) (*Label, error) {
	if labelID <= 0 || orgID <= 0 {
		return nil, ErrOrgLabelNotExist{labelID, orgID}
	}

	l := &Label{
		ID:    labelID,
		OrgID: orgID,
	}
	has, err := e.Get(l)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrOrgLabelNotExist{l.ID, l.OrgID}
	}
	return l, nil
}

// GetLabelInOrgByName returns a label by name in given organization.
func GetLabelInOrgByName(orgID int64, labelName string) (*Label, error) {
	return getLabelInOrgByName(db.GetEngine(db.DefaultContext), orgID, labelName)
}

// GetLabelIDsInOrgByNames returns a list of labelIDs by names in a given
// organization.
func GetLabelIDsInOrgByNames(orgID int64, labelNames []string) ([]int64, error) {
	if orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}
	labelIDs := make([]int64, 0, len(labelNames))

	return labelIDs, db.GetEngine(db.DefaultContext).Table("label").
		Where("org_id = ?", orgID).
		In("name", labelNames).
		Asc("name").
		Cols("id").
		Find(&labelIDs)
}

// GetLabelInOrgByID returns a label by ID in given organization.
func GetLabelInOrgByID(orgID, labelID int64) (*Label, error) {
	return getLabelInOrgByID(db.GetEngine(db.DefaultContext), orgID, labelID)
}

// GetLabelsInOrgByIDs returns a list of labels by IDs in given organization,
// it silently ignores label IDs that do not belong to the organization.
func GetLabelsInOrgByIDs(orgID int64, labelIDs []int64) ([]*Label, error) {
	labels := make([]*Label, 0, len(labelIDs))
	return labels, db.GetEngine(db.DefaultContext).
		Where("org_id = ?", orgID).
		In("id", labelIDs).
		Asc("name").
		Find(&labels)
}

func getLabelsByOrgID(e db.Engine, orgID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	if orgID <= 0 {
		return nil, ErrOrgLabelNotExist{0, orgID}
	}
	labels := make([]*Label, 0, 10)
	sess := e.Where("org_id = ?", orgID)

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

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	return labels, sess.Find(&labels)
}

// GetLabelsByOrgID returns all labels that belong to given organization by ID.
func GetLabelsByOrgID(orgID int64, sortType string, listOptions db.ListOptions) ([]*Label, error) {
	return getLabelsByOrgID(db.GetEngine(db.DefaultContext), orgID, sortType, listOptions)
}

// CountLabelsByOrgID count all labels that belong to given organization by ID.
func CountLabelsByOrgID(orgID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("org_id = ?", orgID).Count(&Label{})
}

// .___
// |   | ______ ________ __   ____
// |   |/  ___//  ___/  |  \_/ __ \
// |   |\___ \ \___ \|  |  /\  ___/
// |___/____  >____  >____/  \___ |
//          \/     \/            \/

func getLabelsByIssueID(e db.Engine, issueID int64) ([]*Label, error) {
	var labels []*Label
	return labels, e.Where("issue_label.issue_id = ?", issueID).
		Join("LEFT", "issue_label", "issue_label.label_id = label.id").
		Asc("label.name").
		Find(&labels)
}

// GetLabelsByIssueID returns all labels that belong to given issue by ID.
func GetLabelsByIssueID(issueID int64) ([]*Label, error) {
	return getLabelsByIssueID(db.GetEngine(db.DefaultContext), issueID)
}

func updateLabelCols(e db.Engine, l *Label, cols ...string) error {
	_, err := e.ID(l.ID).
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

// .___                            .____          ___.          .__
// |   | ______ ________ __   ____ |    |   _____ \_ |__   ____ |  |
// |   |/  ___//  ___/  |  \_/ __ \|    |   \__  \ | __ \_/ __ \|  |
// |   |\___ \ \___ \|  |  /\  ___/|    |___ / __ \| \_\ \  ___/|  |__
// |___/____  >____  >____/  \___  >_______ (____  /___  /\___  >____/
//          \/     \/            \/        \/    \/    \/     \/

// IssueLabel represents an issue-label relation.
type IssueLabel struct {
	ID      int64 `xorm:"pk autoincr"`
	IssueID int64 `xorm:"UNIQUE(s)"`
	LabelID int64 `xorm:"UNIQUE(s)"`
}

func hasIssueLabel(e db.Engine, issueID, labelID int64) bool {
	has, _ := e.Where("issue_id = ? AND label_id = ?", issueID, labelID).Get(new(IssueLabel))
	return has
}

// HasIssueLabel returns true if issue has been labeled.
func HasIssueLabel(issueID, labelID int64) bool {
	return hasIssueLabel(db.GetEngine(db.DefaultContext), issueID, labelID)
}

// newIssueLabel this function creates a new label it does not check if the label is valid for the issue
// YOU MUST CHECK THIS BEFORE THIS FUNCTION
func newIssueLabel(ctx context.Context, issue *Issue, label *Label, doer *user_model.User) (err error) {
	e := db.GetEngine(ctx)
	if _, err = e.Insert(&IssueLabel{
		IssueID: issue.ID,
		LabelID: label.ID,
	}); err != nil {
		return err
	}

	if err = issue.loadRepo(ctx); err != nil {
		return
	}

	opts := &CreateCommentOptions{
		Type:    CommentTypeLabel,
		Doer:    doer,
		Repo:    issue.Repo,
		Issue:   issue,
		Label:   label,
		Content: "1",
	}
	if _, err = createComment(ctx, opts); err != nil {
		return err
	}

	return updateLabelCols(e, label, "num_issues", "num_closed_issue")
}

// NewIssueLabel creates a new issue-label relation.
func NewIssueLabel(issue *Issue, label *Label, doer *user_model.User) (err error) {
	if HasIssueLabel(issue.ID, label.ID) {
		return nil
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	if err = issue.loadRepo(ctx); err != nil {
		return err
	}

	// Do NOT add invalid labels
	if issue.RepoID != label.RepoID && issue.Repo.OwnerID != label.OrgID {
		return nil
	}

	if err = newIssueLabel(ctx, issue, label, doer); err != nil {
		return err
	}

	issue.Labels = nil
	if err = issue.loadLabels(sess); err != nil {
		return err
	}

	return committer.Commit()
}

// newIssueLabels add labels to an issue. It will check if the labels are valid for the issue
func newIssueLabels(ctx context.Context, issue *Issue, labels []*Label, doer *user_model.User) (err error) {
	e := db.GetEngine(ctx)
	if err = issue.loadRepo(ctx); err != nil {
		return err
	}
	for _, label := range labels {
		// Don't add already present labels and invalid labels
		if hasIssueLabel(e, issue.ID, label.ID) ||
			(label.RepoID != issue.RepoID && label.OrgID != issue.Repo.OwnerID) {
			continue
		}

		if err = newIssueLabel(ctx, issue, label, doer); err != nil {
			return fmt.Errorf("newIssueLabel: %v", err)
		}
	}

	return nil
}

// NewIssueLabels creates a list of issue-label relations.
func NewIssueLabels(issue *Issue, labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = newIssueLabels(ctx, issue, labels, doer); err != nil {
		return err
	}

	issue.Labels = nil
	if err = issue.loadLabels(db.GetEngine(ctx)); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteIssueLabel(ctx context.Context, issue *Issue, label *Label, doer *user_model.User) (err error) {
	e := db.GetEngine(ctx)
	if count, err := e.Delete(&IssueLabel{
		IssueID: issue.ID,
		LabelID: label.ID,
	}); err != nil {
		return err
	} else if count == 0 {
		return nil
	}

	if err = issue.loadRepo(ctx); err != nil {
		return
	}

	opts := &CreateCommentOptions{
		Type:  CommentTypeLabel,
		Doer:  doer,
		Repo:  issue.Repo,
		Issue: issue,
		Label: label,
	}
	if _, err = createComment(ctx, opts); err != nil {
		return err
	}

	return updateLabelCols(e, label, "num_issues", "num_closed_issue")
}

// DeleteIssueLabel deletes issue-label relation.
func DeleteIssueLabel(issue *Issue, label *Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = deleteIssueLabel(ctx, issue, label, doer); err != nil {
		return err
	}

	issue.Labels = nil
	if err = issue.loadLabels(db.GetEngine(ctx)); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteLabelsByRepoID(sess db.Engine, repoID int64) error {
	deleteCond := builder.Select("id").From("label").Where(builder.Eq{"label.repo_id": repoID})

	if _, err := sess.In("label_id", deleteCond).
		Delete(&IssueLabel{}); err != nil {
		return err
	}

	_, err := sess.Delete(&Label{RepoID: repoID})
	return err
}
