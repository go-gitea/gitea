// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type State int

const (
	StateTriage    State = 1 // reported privately, waiting for maintainers
	StateDraft     State = 2
	StatePublished State = 3
	StateClosed    State = 4
	StateWithdrawn State = 5
)

var stateNames = map[State]string{
	StateTriage:    "triage",
	StateDraft:     "draft",
	StatePublished: "published",
	StateClosed:    "closed",
	StateWithdrawn: "withdrawn",
}

func (s State) String() string {
	return stateNames[s]
}

// parseName returns the zero value for unknown names
func parseName[T comparable](names map[T]string, name string) (res T) {
	for v, n := range names {
		if n == name {
			return v
		}
	}
	return res
}

func ParseState(name string) State {
	return parseName(stateNames, name)
}

var stateStyles = map[State]struct{ icon, color string }{
	StateTriage:    {"octicon-report", "yellow"},
	StateDraft:     {"octicon-pencil", "grey"},
	StatePublished: {"octicon-shield-check", "green"},
	StateClosed:    {"octicon-x-circle", "red"},
	StateWithdrawn: {"octicon-shield-x", "grey"},
}

func (s State) Icon() string {
	return stateStyles[s].icon
}

// Color is the label color of the state in the web UI
func (s State) Color() string {
	return stateStyles[s].color
}

var stateTransitions = map[State][]State{
	StateTriage:    {StateDraft, StateClosed},
	StateDraft:     {StatePublished, StateClosed},
	StateClosed:    {StateDraft},
	StatePublished: {StateWithdrawn},
}

func (s State) CanTransitionTo(target State) bool {
	return slices.Contains(stateTransitions[s], target)
}

// Transitions returns the states the advisory can change to without a close reason
func (s State) Transitions() []State {
	return slices.DeleteFunc(slices.Clone(stateTransitions[s]), func(t State) bool { return t == StateClosed })
}

// CloseReason tells wrong reports and duplicates apart
type CloseReason int

const (
	CloseReasonNone      CloseReason = 0
	CloseReasonInvalid   CloseReason = 1
	CloseReasonDuplicate CloseReason = 2
)

var closeReasonNames = map[CloseReason]string{
	CloseReasonInvalid:   "invalid",
	CloseReasonDuplicate: "duplicate",
}

func (r CloseReason) String() string {
	return closeReasonNames[r]
}

func ParseCloseReason(name string) CloseReason {
	return parseName(closeReasonNames, name)
}

type Severity int

const (
	SeverityUnknown  Severity = 0
	SeverityLow      Severity = 1
	SeverityMedium   Severity = 2
	SeverityHigh     Severity = 3
	SeverityCritical Severity = 4
)

// Severities are the severities which can be chosen, in ascending order
var Severities = []Severity{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}

var severityNames = map[Severity]string{
	SeverityLow:      "low",
	SeverityMedium:   "medium",
	SeverityHigh:     "high",
	SeverityCritical: "critical",
}

func (s Severity) String() string {
	return severityNames[s]
}

func ParseSeverity(name string) Severity {
	return parseName(severityNames, name)
}

var severityColors = map[Severity]string{
	SeverityMedium:   "yellow",
	SeverityHigh:     "orange",
	SeverityCritical: "red",
}

// Color is the label color of the severity in the web UI
func (s Severity) Color() string {
	return severityColors[s]
}

// LocaleKey names medium "moderate" in the web UI like GitHub, the API keeps "medium"
func (s Severity) LocaleKey() string {
	name := cmp.Or(s.String(), "unknown")
	if s == SeverityMedium {
		name = "moderate"
	}
	return "repo.security_advisories.severity." + name
}

type ErrAdvisoryNotExist struct {
	RepoID     int64
	Identifier string
}

func (err ErrAdvisoryNotExist) Error() string {
	return fmt.Sprintf("security advisory does not exist [repo_id: %d, identifier: %s]", err.RepoID, err.Identifier)
}

func (err ErrAdvisoryNotExist) Unwrap() error {
	return util.ErrNotExist
}

type Advisory struct {
	ID                int64                  `xorm:"pk autoincr"`
	RepoID            int64                  `xorm:"UNIQUE(s) NOT NULL"`
	Repo              *repo_model.Repository `xorm:"-"`
	Identifier        string                 `xorm:"VARCHAR(14) UNIQUE(s) NOT NULL"`
	Index             int64                  `xorm:"INDEX NOT NULL DEFAULT 0"` // short number for discussions, URLs keep the identifier
	CveID             string                 `xorm:"VARCHAR(32)"`
	Summary           string                 `xorm:"VARCHAR(1024) NOT NULL"`
	Description       string                 `xorm:"LONGTEXT"`
	ReportDescription string                 `xorm:"LONGTEXT"` // the description of a report when it was accepted, never public
	Severity          Severity               `xorm:"NOT NULL DEFAULT 0"`
	CvssV3Vector      string                 `xorm:"VARCHAR(255)"`
	CvssV3ScoreTenths int                    `xorm:"NOT NULL DEFAULT 0"` // stored in tenths to be exact on all databases
	CvssV4Vector      string                 `xorm:"VARCHAR(255)"`
	CvssV4ScoreTenths int                    `xorm:"NOT NULL DEFAULT 0"`
	ContentVersion    int                    `xorm:"NOT NULL DEFAULT 0"`
	CweIDs            []string               `xorm:"'cwe_ids' JSON TEXT"`
	State             State                  `xorm:"INDEX NOT NULL"`
	IsReport          bool                   `xorm:"NOT NULL DEFAULT false"`
	ReporterID        int64                  `xorm:"INDEX NOT NULL"`
	Reporter          *user_model.User       `xorm:"-"`
	PublisherID       int64                  `xorm:"NOT NULL DEFAULT 0"`
	Publisher         *user_model.User       `xorm:"-"`
	CreatedUnix       timeutil.TimeStamp     `xorm:"INDEX created"`
	UpdatedUnix       timeutil.TimeStamp     `xorm:"INDEX updated"`
	PublishedUnix     timeutil.TimeStamp     `xorm:"INDEX NOT NULL DEFAULT 0"`
	ClosedUnix        timeutil.TimeStamp     `xorm:"NOT NULL DEFAULT 0"`
	WithdrawnUnix     timeutil.TimeStamp     `xorm:"NOT NULL DEFAULT 0"`
	CloseReason       CloseReason            `xorm:"NOT NULL DEFAULT 0"`
	DuplicateOfID     int64                  `xorm:"NOT NULL DEFAULT 0"`
	DuplicateOf       *Advisory              `xorm:"-"`

	Labels            []*issues_model.Label `xorm:"-"`
	Vulnerabilities   []*Vulnerability      `xorm:"-"`
	Credits           []*Credit             `xorm:"-"`
	CollaboratorUsers []*user_model.User    `xorm:"-"`
	CollaboratorTeams []*organization.Team  `xorm:"-"`
}

// Index numbers the advisories of a repository
type Index db.ResourceIndex

func init() {
	db.RegisterModel(new(Advisory))
	db.RegisterModel(new(Index))
}

func (Advisory) TableName() string {
	return "security_advisory"
}

func (Index) TableName() string {
	return "security_advisory_index"
}

// IsPublic reports whether everyone who can read the advisories unit can see the advisory
func (a *Advisory) IsPublic() bool {
	return a.State == StatePublished || a.State == StateWithdrawn
}

func (a *Advisory) CvssV3Score() float64 {
	return float64(a.CvssV3ScoreTenths) / 10
}

func (a *Advisory) CvssV4Score() float64 {
	return float64(a.CvssV4ScoreTenths) / 10
}

func (a *Advisory) HasCVSS() bool {
	return a.CvssV3Vector != "" || a.CvssV4Vector != ""
}

func (a *Advisory) LoadRepo(ctx context.Context) (err error) {
	if a.Repo == nil {
		a.Repo, err = repo_model.GetRepositoryByID(ctx, a.RepoID)
	}
	return err
}

func (a *Advisory) Link() string {
	return a.Repo.Link() + "/security/advisories/" + a.Identifier
}

func (a *Advisory) HTMLURL() string {
	return a.Repo.HTMLURL() + "/security/advisories/" + a.Identifier
}

// LoadAttributes loads everything but the collaborators
func (a *Advisory) LoadAttributes(ctx context.Context) error {
	return List{a}.LoadAttributes(ctx)
}

func (a *Advisory) LoadCollaborators(ctx context.Context) error {
	return List{a}.LoadCollaborators(ctx)
}

// generateIdentifier is lower case only because some database collations are case-insensitive,
// a collision of 36^12 values per repository is left to the unique index
func generateIdentifier() string {
	r := util.FastCryptoRandomString(12, "abcdefghijklmnopqrstuvwxyz0123456789")
	return r[:4] + "-" + r[4:8] + "-" + r[8:]
}

func CreateAdvisory(ctx context.Context, a *Advisory) error {
	a.Identifier = generateIdentifier()
	return db.WithTx(ctx, func(ctx context.Context) (err error) {
		if a.Index, err = db.GetNextResourceIndex(ctx, "security_advisory_index", a.RepoID); err != nil {
			return err
		}
		if err := db.Insert(ctx, a); err != nil {
			return err
		}
		return replaceDetails(ctx, a)
	})
}

var ErrAdvisoryChanged = util.NewInvalidArgumentErrorf("the advisory has been changed in the meantime")

// UpdateAdvisory updates the columns and replaces the details if the content version is still a.ContentVersion
func UpdateAdvisory(ctx context.Context, a *Advisory, cols ...string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		n, err := db.GetEngine(ctx).Where("id = ? AND content_version = ?", a.ID, a.ContentVersion).Cols(cols...).Incr("content_version").Update(a)
		if err != nil {
			return err
		} else if n == 0 {
			return ErrAdvisoryChanged
		}
		a.ContentVersion++
		return replaceDetails(ctx, a)
	})
}

// UpdateAdvisoryState only updates the advisory if it is still in the old state, to not apply a transition twice.
// The content version changes too because the edit permissions depend on the state.
func UpdateAdvisoryState(ctx context.Context, a *Advisory, oldState State, cols ...string) error {
	n, err := db.GetEngine(ctx).Where("id = ? AND state = ?", a.ID, oldState).Cols(append(cols, "state")...).Incr("content_version").Update(a)
	if err != nil {
		return err
	} else if n == 0 {
		return ErrAdvisoryChanged
	}
	a.ContentVersion++
	return nil
}

// detailBeans are the rows which are replaced together with the content of an advisory
func detailBeans() []any {
	return []any{new(Vulnerability), new(Credit), new(LabelLink)}
}

// childBeans are all rows which belong to an advisory, also the ones changed independently of its content
func childBeans() []any {
	return append(detailBeans(), new(Collaborator), new(Comment))
}

// replaceDetails inserts the vulnerabilities and credits of the advisory as new rows, they get the new IDs
func replaceDetails(ctx context.Context, a *Advisory) error {
	for _, bean := range detailBeans() {
		if _, err := db.GetEngine(ctx).Where("advisory_id = ?", a.ID).Delete(bean); err != nil {
			return err
		}
	}
	var rows []any
	for _, v := range a.Vulnerabilities {
		v.ID, v.AdvisoryID = 0, a.ID
		rows = append(rows, v)
	}
	for _, c := range a.Credits {
		c.ID, c.AdvisoryID = 0, a.ID
		rows = append(rows, c)
	}
	for _, l := range a.Labels {
		rows = append(rows, &LabelLink{AdvisoryID: a.ID, LabelID: l.ID})
	}
	return db.Insert(ctx, rows...)
}

// GetAdvisoryByIdentifier ignores the case like case-insensitive database collations do
func GetAdvisoryByIdentifier(ctx context.Context, repoID int64, identifier string) (*Advisory, error) {
	identifier = strings.ToLower(identifier)
	a, has, err := db.Get[Advisory](ctx, builder.Eq{"repo_id": repoID, "identifier": identifier})
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrAdvisoryNotExist{RepoID: repoID, Identifier: identifier}
	}
	return a, nil
}

// FindAdvisoriesOptions lists the advisories of a repository the viewer can see, the viewer's permission is for that repository
type FindAdvisoriesOptions struct {
	db.ListOptions
	RepoID      int64
	ExcludeID   int64
	States      []State
	Viewer      Viewer
	Keyword     string // matches the summary, description, identifier, CVE ID and package names
	Severity    Severity
	LabelIDs    []int64 // all labels must be set
	Ecosystem   string
	CweID       string
	CloseReason CloseReason
	SortBy      string // created, updated or published
	Ascending   bool
}

func (opts FindAdvisoriesOptions) ToConds() builder.Cond {
	cond := builder.Eq{"repo_id": opts.RepoID}.And(opts.Viewer.visibilityCond())
	if opts.ExcludeID != 0 {
		cond = cond.And(builder.Neq{"id": opts.ExcludeID})
	}
	if len(opts.States) > 0 {
		cond = cond.And(builder.In("state", opts.States))
	}
	if keyword := strings.TrimSpace(opts.Keyword); keyword != "" {
		cond = cond.And(builder.Or(
			db.BuildCaseInsensitiveLike("summary", keyword),
			db.BuildCaseInsensitiveLike("description", keyword),
			builder.Eq{"identifier": strings.ToLower(keyword)},
			builder.Eq{"cve_id": strings.ToUpper(keyword)},
			builder.In("id", advisoryIDsFrom("security_advisory_vulnerability", db.BuildCaseInsensitiveLike("package_name", keyword))),
		))
	}
	if opts.Severity != SeverityUnknown {
		cond = cond.And(builder.Eq{"severity": opts.Severity})
	}
	if len(opts.LabelIDs) > 0 {
		cond = cond.And(labelsCond(opts.LabelIDs))
	}
	if opts.Ecosystem != "" {
		cond = cond.And(builder.In("id", advisoryIDsFrom("security_advisory_vulnerability", builder.Eq{"ecosystem": opts.Ecosystem})))
	}
	if opts.CweID != "" {
		cond = cond.And(cweIDCond(opts.CweID))
	}
	if opts.CloseReason != CloseReasonNone {
		cond = cond.And(builder.Eq{"close_reason": opts.CloseReason})
	}
	return cond
}

// advisoryIDsFrom selects the advisory IDs of the matching rows of a child table
func advisoryIDsFrom(table string, cond builder.Cond) *builder.Builder {
	return builder.Select("advisory_id").From(table).Where(cond)
}

// labelsCond works like the issue filter: a negative ID excludes the label, 0 means no label at all
func labelsCond(labelIDs []int64) builder.Cond {
	cond := builder.NewCond()
	for _, labelID := range labelIDs {
		switch {
		case labelID > 0:
			cond = cond.And(builder.In("id", advisoryIDsFrom("security_advisory_label", builder.Eq{"label_id": labelID})))
		case labelID < 0:
			cond = cond.And(builder.NotIn("id", advisoryIDsFrom("security_advisory_label", builder.Eq{"label_id": -labelID})))
		default:
			cond = cond.And(builder.NotIn("id", builder.Select("advisory_id").From("security_advisory_label")))
		}
	}
	return cond
}

// cweIDCond matches the quoted JSON array element, so that CWE-79 doesn't match CWE-790
func cweIDCond(cweID string) builder.Cond {
	return builder.Like{"cwe_ids", `"` + strings.ToUpper(cweID) + `"`}
}

func (opts FindAdvisoriesOptions) ToOrders() string {
	column := "created_unix"
	switch opts.SortBy {
	case "updated":
		column = "updated_unix"
	case "published":
		column = "published_unix"
	}
	if opts.Ascending {
		return column + " ASC, id ASC"
	}
	return column + " DESC, id DESC"
}

// CountAdvisoriesByState counts with all filters but the state
func CountAdvisoriesByState(ctx context.Context, opts FindAdvisoriesOptions) (map[State]int64, error) {
	opts.States = nil
	var rows []struct {
		State State
		Count int64
	}
	if err := db.GetEngine(ctx).Table("security_advisory").Select("state, COUNT(*) AS count").Where(opts.ToConds()).GroupBy("state").Find(&rows); err != nil {
		return nil, err
	}
	counts := make(map[State]int64, len(rows))
	for _, r := range rows {
		counts[r.State] = r.Count
	}
	return counts, nil
}

// DeleteAdvisory deletes the advisory unless it is public, also if it has been published in the meantime
func DeleteAdvisory(ctx context.Context, a *Advisory) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if has, err := db.Exist[Advisory](ctx, builder.Eq{"duplicate_of_id": a.ID}); err != nil {
			return err
		} else if has {
			return util.NewInvalidArgumentErrorf("advisories with duplicates cannot be deleted")
		}
		n, err := deleteAdvisories(ctx, builder.Eq{"id": a.ID}.And(builder.NotIn("state", StatePublished, StateWithdrawn)))
		if err != nil {
			return err
		} else if n == 0 {
			return util.NewInvalidArgumentErrorf("published advisories cannot be deleted, withdraw them instead")
		}
		return nil
	})
}

func DeleteAdvisoriesByRepoID(ctx context.Context, repoID int64) error {
	if _, err := deleteAdvisories(ctx, builder.Eq{"repo_id": repoID}); err != nil {
		return err
	}
	return db.DeleteResourceIndex(ctx, "security_advisory_index", repoID)
}

// deleteAdvisories selects the details by a subquery because a list of IDs can exceed the parameter limit of some databases,
// MySQL doesn't allow such a subquery on the table it deletes from
func deleteAdvisories(ctx context.Context, cond builder.Cond) (int64, error) {
	e := db.GetEngine(ctx)
	ids := builder.Select("id").From("security_advisory").Where(cond)
	for _, bean := range childBeans() {
		if _, err := e.Where(builder.In("advisory_id", ids)).Delete(bean); err != nil {
			return 0, err
		}
	}
	return e.Where(cond).Delete(new(Advisory))
}

type List []*Advisory

func (list List) byID() map[int64]*Advisory {
	res := make(map[int64]*Advisory, len(list))
	for _, a := range list {
		res[a.ID] = a
	}
	return res
}

func (list List) ids() []int64 {
	return container.FilterSlice(list, func(a *Advisory) (int64, bool) { return a.ID, true })
}

func (list List) loadRepos(ctx context.Context) error {
	for _, a := range list {
		if err := a.LoadRepo(ctx); err != nil {
			return err
		}
		if err := a.Repo.LoadOwner(ctx); err != nil {
			return err
		}
	}
	return nil
}

// LoadLabels loads the labels, also done by LoadAttributes
func (list List) LoadLabels(ctx context.Context) error {
	var links []*LabelLink
	if err := db.GetEngine(ctx).In("advisory_id", list.ids()).Find(&links); err != nil {
		return err
	}
	labels, err := issues_model.GetLabelsByIDs(ctx, container.FilterSlice(links, func(l *LabelLink) (int64, bool) { return l.LabelID, true }))
	if err != nil {
		return err
	}
	labelsByID := make(map[int64]*issues_model.Label, len(labels))
	for _, l := range labels {
		labelsByID[l.ID] = l
	}
	byID := list.byID()
	for _, a := range list {
		a.Labels = []*issues_model.Label{}
	}
	for _, link := range links {
		if l := labelsByID[link.LabelID]; l != nil {
			byID[link.AdvisoryID].Labels = append(byID[link.AdvisoryID].Labels, l)
		}
	}
	return nil
}

func (list List) LoadAttributes(ctx context.Context) error {
	if len(list) == 0 {
		return nil
	}
	if err := list.loadRepos(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: loadRepos: %w", err)
	}
	if err := list.LoadLabels(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: LoadLabels: %w", err)
	}
	if err := list.loadVulnerabilities(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: loadVulnerabilities: %w", err)
	}
	if err := list.loadCredits(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: loadCredits: %w", err)
	}
	if err := list.loadUsers(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: loadUsers: %w", err)
	}
	if err := list.loadDuplicateOf(ctx); err != nil {
		return fmt.Errorf("LoadAttributes: loadDuplicateOf: %w", err)
	}
	return nil
}

func (list List) loadVulnerabilities(ctx context.Context) error {
	var vulns []*Vulnerability
	if err := db.GetEngine(ctx).In("advisory_id", list.ids()).OrderBy("id").Find(&vulns); err != nil {
		return err
	}
	byID := list.byID()
	for _, a := range list {
		a.Vulnerabilities = []*Vulnerability{}
	}
	for _, v := range vulns {
		byID[v.AdvisoryID].Vulnerabilities = append(byID[v.AdvisoryID].Vulnerabilities, v)
	}
	return nil
}

func (list List) loadCredits(ctx context.Context) error {
	var credits []*Credit
	if err := db.GetEngine(ctx).In("advisory_id", list.ids()).OrderBy("id").Find(&credits); err != nil {
		return err
	}
	byID := list.byID()
	for _, a := range list {
		a.Credits = []*Credit{}
	}
	for _, c := range credits {
		byID[c.AdvisoryID].Credits = append(byID[c.AdvisoryID].Credits, c)
	}
	return nil
}

// loadUsers loads the reporters, publishers and credited users, the credits must be loaded
func (list List) loadUsers(ctx context.Context) error {
	var userIDs []int64
	for _, a := range list {
		userIDs = append(userIDs, a.ReporterID, a.PublisherID)
		for _, c := range a.Credits {
			userIDs = append(userIDs, c.UserID)
		}
	}
	users, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return err
	}
	for _, a := range list {
		a.Reporter = user_model.GetPossibleUserFromMap(a.ReporterID, users)
		a.Publisher = user_model.GetPossibleUserFromMap(a.PublisherID, users)
		for _, c := range a.Credits {
			c.User = user_model.GetPossibleUserFromMap(c.UserID, users)
		}
	}
	return nil
}

func (list List) loadDuplicateOf(ctx context.Context) error {
	duplicateOf := make(map[int64]*Advisory)
	if ids := container.FilterSlice(list, func(a *Advisory) (int64, bool) { return a.DuplicateOfID, a.DuplicateOfID != 0 }); len(ids) > 0 {
		if err := db.GetEngine(ctx).In("id", ids).Find(&duplicateOf); err != nil {
			return err
		}
	}
	for _, a := range list {
		if a.DuplicateOf = duplicateOf[a.DuplicateOfID]; a.DuplicateOf != nil {
			a.DuplicateOf.Repo = a.Repo // the original belongs to the same repository
		}
	}
	return nil
}

// LoadCollaborators always reloads the collaborators because they are changed independently of the advisory
func (list List) LoadCollaborators(ctx context.Context) error {
	if len(list) == 0 {
		return nil
	}
	var rows []*Collaborator
	if err := db.GetEngine(ctx).In("advisory_id", list.ids()).OrderBy("id").Find(&rows); err != nil {
		return err
	}
	users, err := user_model.GetUsersMapByIDs(ctx, container.FilterSlice(rows, func(c *Collaborator) (int64, bool) { return c.UserID, c.UserID != 0 }))
	if err != nil {
		return err
	}
	teams, err := organization.GetTeamsByIDs(ctx, container.FilterSlice(rows, func(c *Collaborator) (int64, bool) { return c.TeamID, c.TeamID != 0 }))
	if err != nil {
		return err
	}
	byID := list.byID()
	for _, a := range list {
		a.CollaboratorUsers, a.CollaboratorTeams = []*user_model.User{}, []*organization.Team{}
	}
	for _, c := range rows {
		if u := users[c.UserID]; u != nil {
			byID[c.AdvisoryID].CollaboratorUsers = append(byID[c.AdvisoryID].CollaboratorUsers, u)
		} else if t := teams[c.TeamID]; t != nil {
			byID[c.AdvisoryID].CollaboratorTeams = append(byID[c.AdvisoryID].CollaboratorTeams, t)
		}
	}
	return nil
}
