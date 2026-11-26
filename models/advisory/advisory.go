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
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type State int

const (
	StateTriage    State = 1
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

func parseName[T comparable](names map[T]string, name string) (res T) {
	for value, candidate := range names {
		if candidate == name {
			return value
		}
	}
	return res
}

func ParseState(name string) State {
	return parseName(stateNames, name)
}

var stateStyles = map[State]struct{ icon, color string }{
	StateTriage:    {"octicon-issue-draft", "grey"},
	StateDraft:     {"octicon-pencil", "grey"},
	StatePublished: {"octicon-shield", "green"},
	StateClosed:    {"octicon-shield-check", "purple"},
	StateWithdrawn: {"octicon-shield-x", "grey"},
}

func (s State) Icon() string {
	return stateStyles[s].icon
}

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

func (s Severity) Color() string {
	return severityColors[s]
}

func (s Severity) LocaleKey() string {
	name := cmp.Or(s.String(), "unknown")
	if s == SeverityMedium {
		name = "moderate" // Match GitHub's UI while keeping "medium" in the API.
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
	Identifier        string                 `xorm:"VARCHAR(19) UNIQUE(s) NOT NULL"`
	Index             int64                  `xorm:"INDEX NOT NULL DEFAULT 0"` // Discussions use the number while URLs use the identifier.
	CveID             string                 `xorm:"VARCHAR(32)"`
	Summary           string                 `xorm:"VARCHAR(1024) NOT NULL"`
	Description       string                 `xorm:"LONGTEXT"`
	ReportDescription string                 `xorm:"LONGTEXT"` // the description of a report when it was accepted, never public
	Severity          Severity               `xorm:"NOT NULL DEFAULT 0"`
	CvssVector        string                 `xorm:"VARCHAR(255)"`
	CvssScoreTenths   int                    `xorm:"NOT NULL DEFAULT 0"` // Stored in tenths for exact scores across databases.
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
	IsMetadataOnly    bool                   `xorm:"-"`

	Labels            []*issues_model.Label `xorm:"-"`
	Vulnerabilities   []*Vulnerability      `xorm:"-"`
	Credits           []*Credit             `xorm:"-"`
	CollaboratorUsers []*user_model.User    `xorm:"-"`
	CollaboratorTeams []*organization.Team  `xorm:"-"`
}

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

func (a *Advisory) IsPublic() bool {
	return a.State == StatePublished || a.State == StateWithdrawn
}

func (a *Advisory) CvssScore() float64 {
	return float64(a.CvssScoreTenths) / 10
}

func (a *Advisory) CvssVersion() cvss.Version {
	return cvss.DetectVersion(a.CvssVector)
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

func (a *Advisory) LoadAttributes(ctx context.Context) error {
	return List{a}.LoadAttributes(ctx)
}

func (a *Advisory) LoadCollaborators(ctx context.Context) error {
	return List{a}.LoadCollaborators(ctx)
}

func CreateAdvisory(ctx context.Context, advisory *Advisory) error {
	random := util.FastCryptoRandomString(12, "23456789cfghjmpqrvwx") // GitHub's GHSA alphabet, lowercase for case-insensitive collations
	advisory.Identifier = identifierPrefix + random[:4] + "-" + random[4:8] + "-" + random[8:]
	return db.WithTx(ctx, func(ctx context.Context) (err error) {
		if advisory.Index, err = db.GetNextResourceIndex(ctx, "security_advisory_index", advisory.RepoID); err != nil {
			return err
		}
		if err := db.Insert(ctx, advisory); err != nil {
			return err
		}
		return replaceDetails(ctx, advisory)
	})
}

var ErrAdvisoryChanged = util.NewInvalidArgumentErrorf("the advisory has been changed in the meantime")

// UpdateAdvisory requires loaded details because it replaces the child rows.
func UpdateAdvisory(ctx context.Context, advisory *Advisory) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		updated, err := db.GetEngine(ctx).Where("id = ? AND content_version = ?", advisory.ID, advisory.ContentVersion).
			Cols("summary", "description", "cve_id", "severity", "cvss_vector", "cvss_score_tenths", "cwe_ids").Incr("content_version").Update(advisory)
		if err != nil {
			return err
		} else if updated == 0 {
			return ErrAdvisoryChanged
		}
		advisory.ContentVersion++
		return replaceDetails(ctx, advisory)
	})
}

// UpdateAdvisoryState invalidates pending edits because the transition can change permissions.
func UpdateAdvisoryState(ctx context.Context, advisory *Advisory, oldState State, cols ...string) error {
	updated, err := db.GetEngine(ctx).Where("id = ? AND state = ?", advisory.ID, oldState).Cols(append(cols, "state")...).Incr("content_version").Update(advisory)
	if err != nil {
		return err
	} else if updated == 0 {
		return ErrAdvisoryChanged
	}
	advisory.ContentVersion++
	return nil
}

func detailBeans() []any {
	return []any{new(Vulnerability), new(Credit), new(LabelLink)}
}

func replaceDetails(ctx context.Context, advisory *Advisory) error {
	for _, bean := range detailBeans() {
		if _, err := db.GetEngine(ctx).Where("advisory_id = ?", advisory.ID).Delete(bean); err != nil {
			return err
		}
	}
	var rows []any
	for _, vulnerability := range advisory.Vulnerabilities {
		vulnerability.ID, vulnerability.AdvisoryID = 0, advisory.ID
		rows = append(rows, vulnerability)
	}
	for _, credit := range advisory.Credits {
		credit.ID, credit.AdvisoryID = 0, advisory.ID
		rows = append(rows, credit)
	}
	for _, label := range advisory.Labels {
		rows = append(rows, &LabelLink{AdvisoryID: advisory.ID, LabelID: label.ID})
	}
	return db.Insert(ctx, rows...)
}

const identifierPrefix = "GTSA-"

// normalizeIdentifier keeps lookups consistent across database collations
func normalizeIdentifier(identifier string) string {
	lower := strings.ToLower(identifier)
	if rest, ok := strings.CutPrefix(lower, strings.ToLower(identifierPrefix)); ok {
		return identifierPrefix + rest
	}
	return lower
}

func GetAdvisoryByIdentifier(ctx context.Context, repoID int64, identifier string) (*Advisory, error) {
	identifier = normalizeIdentifier(identifier)
	advisory, found, err := db.Get[Advisory](ctx, builder.Eq{"repo_id": repoID, "identifier": identifier})
	if err != nil {
		return nil, err
	} else if !found {
		return nil, ErrAdvisoryNotExist{RepoID: repoID, Identifier: identifier}
	}
	return advisory, nil
}

type FindAdvisoriesOptions struct {
	db.ListOptions
	RepoID      int64
	OrgID       int64
	ExcludeID   int64
	States      []State
	Viewer      Viewer
	Keyword     string
	Severity    Severity
	LabelIDs    []int64
	Ecosystem   string
	CweID       string
	CloseReason CloseReason
	SortBy      string
	Ascending   bool
}

func (opts FindAdvisoriesOptions) ToConds() builder.Cond {
	cond := opts.Viewer.visibilityCond()
	if opts.OrgID == 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	} else {
		orgRepos := builder.Select("id").From("repository").Where(builder.Eq{"owner_id": opts.OrgID})
		cond = cond.And(builder.In("repo_id", builder.Select("repo_id").From("repo_unit").
			Where(builder.Eq{"type": unit.TypeSecurityAdvisories}.And(builder.In("repo_id", orgRepos)))))
	}
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
			builder.Eq{"identifier": normalizeIdentifier(keyword)},
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
		cond = cond.And(builder.Like{"cwe_ids", `"` + strings.ToUpper(opts.CweID) + `"`}) // Match a whole JSON array element.
	}
	if opts.CloseReason != CloseReasonNone {
		cond = cond.And(builder.Eq{"close_reason": opts.CloseReason})
	}
	return cond
}

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

// FindDuplicateCandidates caps the close dialog choices while the API accepts older advisories by identifier.
func FindDuplicateCandidates(ctx context.Context, advisory *Advisory, viewer Viewer) ([]*Advisory, error) {
	opts := FindAdvisoriesOptions{RepoID: advisory.RepoID, ExcludeID: advisory.ID, Viewer: viewer}
	var candidates []*Advisory
	return candidates, db.GetEngine(ctx).Where(opts.ToConds().And(builder.Eq{"duplicate_of_id": 0})).OrderBy(opts.ToOrders()).Limit(100).Cols("identifier", "summary").Find(&candidates)
}

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
	for _, row := range rows {
		counts[row.State] = row.Count
	}
	return counts, nil
}

func DeleteAdvisory(ctx context.Context, advisory *Advisory) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if has, err := db.Exist[Advisory](ctx, builder.Eq{"duplicate_of_id": advisory.ID}); err != nil {
			return err
		} else if has {
			return util.NewInvalidArgumentErrorf("advisories with duplicates cannot be deleted")
		}
		deleted, err := deleteAdvisories(ctx, builder.Eq{"id": advisory.ID}.And(builder.NotIn("state", StatePublished, StateWithdrawn)))
		if err != nil {
			return err
		} else if deleted == 0 {
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

func deleteAdvisories(ctx context.Context, cond builder.Cond) (int64, error) {
	engine := db.GetEngine(ctx)
	ids := builder.Select("id").From("security_advisory").Where(cond) // Avoid parameter limits and MySQL's ban on subqueries against the delete target.
	for _, bean := range append(detailBeans(), new(Collaborator), new(Comment)) {
		if _, err := engine.Where(builder.In("advisory_id", ids)).Delete(bean); err != nil {
			return 0, err
		}
	}
	return engine.Where(cond).Delete(new(Advisory))
}

type List []*Advisory

func (list List) byID() map[int64]*Advisory {
	res := make(map[int64]*Advisory, len(list))
	for _, advisory := range list {
		res[advisory.ID] = advisory
	}
	return res
}

func (list List) ids() []int64 {
	return container.FilterSlice(list, func(advisory *Advisory) (int64, bool) { return advisory.ID, true })
}

func (list List) loadRepos(ctx context.Context) error {
	for _, advisory := range list {
		if err := advisory.LoadRepo(ctx); err != nil {
			return err
		}
		if err := advisory.Repo.LoadOwner(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (list List) LoadLabels(ctx context.Context) error {
	var links []*LabelLink
	if err := db.GetEngine(ctx).In("advisory_id", list.ids()).Find(&links); err != nil {
		return err
	}
	labels, err := issues_model.GetLabelsByIDs(ctx, container.FilterSlice(links, func(link *LabelLink) (int64, bool) { return link.LabelID, true }))
	if err != nil {
		return err
	}
	labelsByID := make(map[int64]*issues_model.Label, len(labels))
	for _, label := range labels {
		labelsByID[label.ID] = label
	}
	byID := list.byID()
	for _, advisory := range list {
		advisory.Labels = []*issues_model.Label{}
	}
	for _, link := range links {
		if label := labelsByID[link.LabelID]; label != nil {
			advisory := byID[link.AdvisoryID]
			advisory.Labels = append(advisory.Labels, label)
		}
	}
	return nil
}

func (list List) LoadAttributes(ctx context.Context) error {
	if len(list) == 0 {
		return nil
	}
	for _, load := range []func(context.Context) error{list.loadRepos, list.LoadLabels, list.loadDetails, list.LoadUsers, list.loadDuplicateOf} {
		if err := load(ctx); err != nil {
			return fmt.Errorf("LoadAttributes: %w", err)
		}
	}
	return nil
}

func (list List) loadDetails(ctx context.Context) error {
	ids := list.ids()
	engine := db.GetEngine(ctx)
	var vulnerabilities []*Vulnerability
	if err := engine.In("advisory_id", ids).OrderBy("id").Find(&vulnerabilities); err != nil {
		return err
	}
	var credits []*Credit
	if err := engine.In("advisory_id", ids).OrderBy("id").Find(&credits); err != nil {
		return err
	}
	byID := list.byID()
	for _, advisory := range list {
		advisory.Vulnerabilities = []*Vulnerability{}
		advisory.Credits = []*Credit{}
	}
	for _, vulnerability := range vulnerabilities {
		advisory := byID[vulnerability.AdvisoryID]
		advisory.Vulnerabilities = append(advisory.Vulnerabilities, vulnerability)
	}
	for _, credit := range credits {
		advisory := byID[credit.AdvisoryID]
		advisory.Credits = append(advisory.Credits, credit)
	}
	return nil
}

// LoadUsers loads the reporters and publishers, and the credited users if the credits are loaded
func (list List) LoadUsers(ctx context.Context) error {
	var userIDs []int64
	for _, advisory := range list {
		userIDs = append(userIDs, advisory.ReporterID, advisory.PublisherID)
		for _, credit := range advisory.Credits {
			userIDs = append(userIDs, credit.UserID)
		}
	}
	users, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return err
	}
	for _, advisory := range list {
		advisory.Reporter = user_model.GetPossibleUserFromMap(advisory.ReporterID, users)
		advisory.Publisher = user_model.GetPossibleUserFromMap(advisory.PublisherID, users)
		for _, credit := range advisory.Credits {
			credit.User = user_model.GetPossibleUserFromMap(credit.UserID, users)
		}
	}
	return nil
}

func (list List) loadDuplicateOf(ctx context.Context) error {
	duplicateOf := make(map[int64]*Advisory)
	if ids := container.FilterSlice(list, func(advisory *Advisory) (int64, bool) { return advisory.DuplicateOfID, advisory.DuplicateOfID != 0 }); len(ids) > 0 {
		if err := db.GetEngine(ctx).In("id", ids).Find(&duplicateOf); err != nil {
			return err
		}
	}
	for _, advisory := range list {
		if advisory.DuplicateOf = duplicateOf[advisory.DuplicateOfID]; advisory.DuplicateOf != nil {
			advisory.DuplicateOf.Repo = advisory.Repo
		}
	}
	return nil
}

func (list List) LoadCollaborators(ctx context.Context) error {
	if len(list) == 0 {
		return nil
	}
	var rows []*Collaborator
	if err := db.GetEngine(ctx).In("advisory_id", list.ids()).OrderBy("id").Find(&rows); err != nil {
		return err
	}
	users, err := user_model.GetUsersMapByIDs(ctx, container.FilterSlice(rows, func(row *Collaborator) (int64, bool) { return row.UserID, row.UserID != 0 }))
	if err != nil {
		return err
	}
	teams, err := organization.GetTeamsByIDs(ctx, container.FilterSlice(rows, func(row *Collaborator) (int64, bool) { return row.TeamID, row.TeamID != 0 }))
	if err != nil {
		return err
	}
	byID := list.byID()
	for _, advisory := range list {
		advisory.CollaboratorUsers, advisory.CollaboratorTeams = []*user_model.User{}, []*organization.Team{}
	}
	for _, row := range rows {
		advisory := byID[row.AdvisoryID]
		if user := users[row.UserID]; user != nil {
			advisory.CollaboratorUsers = append(advisory.CollaboratorUsers, user)
		} else if team := teams[row.TeamID]; team != nil {
			advisory.CollaboratorTeams = append(advisory.CollaboratorTeams, team)
		}
	}
	return nil
}
