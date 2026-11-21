// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package disclosure

import (
	"context"
	"fmt"
	"html/template"
	"slices"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// Severity is the severity of a security disclosure, chosen manually by the maintainers
type Severity string

// Enumerate all severities, in increasing order of rank
const (
	SeverityInformative Severity = "informative" // not a security issue, but worth informing users about
	SeverityLow         Severity = "low"
	SeverityModerate    Severity = "moderate"
	SeverityHigh        Severity = "high"
	SeverityCritical    Severity = "critical"
)

// ParseSeverity parses a severity string, returning an error for unknown values
func ParseSeverity(s string) (Severity, error) {
	switch Severity(s) {
	case SeverityInformative, SeverityLow, SeverityModerate, SeverityHigh, SeverityCritical:
		return Severity(s), nil
	}
	return "", util.NewInvalidArgumentErrorf("invalid severity %q", s)
}

// Rank returns the numeric rank of the severity, for sorting
func (s Severity) Rank() int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityModerate:
		return 3
	case SeverityLow:
		return 2
	}
	return 1
}

// LabelClass returns the Fomantic label color class for the severity
func (s Severity) LabelClass() string {
	switch s {
	case SeverityCritical, SeverityHigh:
		return "red"
	case SeverityModerate:
		return "orange"
	case SeverityLow:
		return "yellow"
	}
	return ""
}

// Status is the publication status of a security disclosure
type Status string

// Enumerate all statuses
const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusWithdrawn Status = "withdrawn"
)

// ParseStatus parses a status string, returning an error for unknown values
func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusDraft, StatusPublished, StatusWithdrawn:
		return Status(s), nil
	}
	return "", util.NewInvalidArgumentErrorf("invalid status %q", s)
}

// StringList is a list of strings stored as JSON in a TEXT column
type StringList []string

// FromDB implements the convert.Conversion interface
func (l *StringList) FromDB(bs []byte) error {
	if len(bs) == 0 {
		*l = nil
		return nil
	}
	return json.Unmarshal(bs, l)
}

// ToDB implements the convert.Conversion interface
func (l StringList) ToDB() ([]byte, error) {
	return json.Marshal(l)
}

// cvssVectorPrefixes are the accepted CVSS vector prefixes, they encode the CVSS version
var cvssVectorPrefixes = []string{"CVSS:3.0/", "CVSS:3.1/", "CVSS:4.0/"}

// ValidateCVSSVector checks that a CVSS vector string carries a known CVSS version prefix
func ValidateCVSSVector(v string) error {
	if v == "" {
		return nil
	}
	if len(v) > 128 {
		return util.NewInvalidArgumentErrorf("CVSS vector too long")
	}
	for _, prefix := range cvssVectorPrefixes {
		if strings.HasPrefix(v, prefix) {
			return nil
		}
	}
	return util.NewInvalidArgumentErrorf("invalid CVSS vector, must start with one of %s", strings.Join(cvssVectorPrefixes, ", "))
}

// CVSSVersion returns the CVSS version ("3.0", "3.1" or "4.0") encoded in the vector prefix, or ""
func CVSSVersion(v string) string {
	for _, prefix := range cvssVectorPrefixes {
		if strings.HasPrefix(v, prefix) {
			return strings.TrimSuffix(strings.TrimPrefix(prefix, "CVSS:"), "/")
		}
	}
	return ""
}

// Disclosure represents a security disclosure of a repository.
// Only maintainers can create or edit disclosures; drafts are only visible to them,
// published (and withdrawn) disclosures are visible to anyone who can read the unit.
type Disclosure struct {
	ID             int64              `xorm:"pk autoincr"`
	RepoID         int64              `xorm:"INDEX UNIQUE(repo_num)"`
	Num            int64              `xorm:"UNIQUE(repo_num)"` // per-repo sequential number, shown as the short identifier
	AuthorID       int64              `xorm:"INDEX"`
	Title          string             `xorm:"VARCHAR(255) NOT NULL"`
	Summary        string             `xorm:"TEXT"`
	Content        string             `xorm:"LONGTEXT"` // markdown
	Severity       Severity           `xorm:"VARCHAR(16) INDEX NOT NULL DEFAULT 'none'"`
	CVSSVector     string             `xorm:"VARCHAR(128)"`
	CweIds         StringList         `xorm:"TEXT"`
	CveID          string             `xorm:"VARCHAR(64)"` // optional reference to an existing CVE, never requested
	References     StringList         `xorm:"TEXT"`
	Status         Status             `xorm:"VARCHAR(16) INDEX NOT NULL DEFAULT 'draft'"`
	WithdrawReason string             `xorm:"TEXT"` // required when withdrawing
	PublishedAt    timeutil.TimeStamp `xorm:"INDEX"`
	PublishedByID  int64              `xorm:"INDEX"`
	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"updated"`

	Repo      *repo_model.Repository `xorm:"-"`
	Author    *user_model.User       `xorm:"-"`
	Publisher *user_model.User       `xorm:"-"`
	Versions  []*Version             `xorm:"-"`

	RenderedContent template.HTML `xorm:"-"` // rendered markdown of Content, set by the routers
}

func init() {
	db.RegisterModel(new(Disclosure))
}

// ErrDisclosureNotExist represents a "DisclosureNotExist" kind of error.
type ErrDisclosureNotExist struct {
	ID     int64
	RepoID int64
}

// IsErrDisclosureNotExist checks if an error is a ErrDisclosureNotExist.
func IsErrDisclosureNotExist(err error) bool {
	_, ok := err.(ErrDisclosureNotExist)
	return ok
}

func (err ErrDisclosureNotExist) Error() string {
	return fmt.Sprintf("disclosure does not exist [id: %d, repo_id: %d]", err.ID, err.RepoID)
}

func (err ErrDisclosureNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Identifier returns the short human-readable identifier of the disclosure, e.g. "SD-0007"
func (d *Disclosure) Identifier() string {
	return fmt.Sprintf("SD-%04d", d.Num)
}

// IsDraft returns whether the disclosure is still a draft
func (d *Disclosure) IsDraft() bool {
	return d.Status == StatusDraft
}

// GetDisclosureByID returns the disclosure with the given ID
func GetDisclosureByID(ctx context.Context, id int64) (*Disclosure, error) {
	d := &Disclosure{ID: id}
	has, err := db.GetEngine(ctx).Get(d)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDisclosureNotExist{ID: id}
	}
	return d, nil
}

// GetDisclosureByRepoAndID returns the disclosure with the given ID in the given repo
func GetDisclosureByRepoAndID(ctx context.Context, repoID, id int64) (*Disclosure, error) {
	d := &Disclosure{}
	has, err := db.GetEngine(ctx).Where(builder.Eq{"repo_id": repoID, "id": id}).Get(d)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDisclosureNotExist{ID: id, RepoID: repoID}
	}
	return d, nil
}

// CreateDisclosure inserts a disclosure and its versions in a transaction,
// assigning the per-repo sequential number
func CreateDisclosure(ctx context.Context, d *Disclosure, versions []*Version) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		// guard against concurrent creates with the UNIQUE(repo_id, num) constraint; the
		// disclosure list is written by maintainers only, so a collision is practically impossible
		var latest Disclosure
		has, err := db.GetEngine(ctx).Where(builder.Eq{"repo_id": d.RepoID}).OrderBy("num DESC").Limit(1).Get(&latest)
		if err != nil {
			return err
		}
		if has {
			d.Num = latest.Num + 1
		} else {
			d.Num = 1
		}
		if err := db.Insert(ctx, d); err != nil {
			return err
		}
		for _, v := range versions {
			v.ID = 0
			v.DisclosureID = d.ID
			if err := db.Insert(ctx, v); err != nil {
				return err
			}
		}
		return nil
	})
}

// disclosureUpdatableColumns are the columns written by UpdateDisclosure
var disclosureUpdatableColumns = []string{
	"title", "summary", "content", "severity", "cvss_vector", "cwe_ids", "cve_id",
	"references", "status", "withdraw_reason", "published_at", "published_by_id",
}

// UpdateDisclosure updates the given columns of the disclosure
func UpdateDisclosure(ctx context.Context, d *Disclosure) error {
	affected, err := db.GetEngine(ctx).ID(d.ID).Cols(disclosureUpdatableColumns...).Update(d)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrDisclosureNotExist{ID: d.ID, RepoID: d.RepoID}
	}
	return nil
}

// DeleteDisclosureByID deletes the disclosure and its versions
func DeleteDisclosureByID(ctx context.Context, repoID, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		affected, err := db.GetEngine(ctx).Where(builder.Eq{"id": id, "repo_id": repoID}).Delete(new(Disclosure))
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrDisclosureNotExist{ID: id, RepoID: repoID}
		}
		_, err = db.GetEngine(ctx).Where(builder.Eq{"disclosure_id": id}).Delete(new(Version))
		return err
	})
}

// LoadAttributes loads the relations of the disclosure
func (d *Disclosure) LoadAttributes(ctx context.Context) error {
	if d.Repo == nil {
		repo, err := repo_model.GetRepositoryByID(ctx, d.RepoID)
		if err != nil {
			return err
		}
		d.Repo = repo
	}
	if d.Author == nil && d.AuthorID > 0 {
		author, err := user_model.GetUserByID(ctx, d.AuthorID)
		if err != nil {
			return err
		}
		d.Author = author
	}
	if d.Publisher == nil && d.PublishedByID > 0 {
		publisher, err := user_model.GetUserByID(ctx, d.PublishedByID)
		if err != nil {
			return err
		}
		d.Publisher = publisher
	}
	if d.Versions == nil {
		versions, err := GetVersionsByDisclosureID(ctx, d.ID)
		if err != nil {
			return err
		}
		d.Versions = versions
	}
	return nil
}

// loadAttributes batch-loads the relations of a list of disclosures
func loadAttributes(ctx context.Context, list []*Disclosure) error {
	if len(list) == 0 {
		return nil
	}
	repoIDs := make([]int64, 0, len(list))
	userIDs := make([]int64, 0, len(list)*2)
	disclosureIDs := make([]int64, 0, len(list))
	for _, d := range list {
		repoIDs = append(repoIDs, d.RepoID)
		disclosureIDs = append(disclosureIDs, d.ID)
		if d.AuthorID > 0 {
			userIDs = append(userIDs, d.AuthorID)
		}
		if d.PublishedByID > 0 {
			userIDs = append(userIDs, d.PublishedByID)
		}
	}
	repoMap, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return err
	}
	userMap, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return err
	}
	versionsMap, err := GetVersionsByDisclosureIDs(ctx, disclosureIDs)
	if err != nil {
		return err
	}
	for _, d := range list {
		d.Repo = repoMap[d.RepoID]
		d.Author = userMap[d.AuthorID]
		d.Publisher = userMap[d.PublishedByID]
		d.Versions = versionsMap[d.ID]
	}
	return nil
}

// ListDisclosuresOptions are the options for listing disclosures
type ListDisclosuresOptions struct {
	db.ListOptions
	RepoID        int64
	ID            int64
	Keyword       string
	Severities    []Severity
	Status        Status
	Version       string // if set, only disclosures whose affected versions match this version (semver range or exact)
	IncludeDrafts bool
	OrderBy       string // "newest" (default), "oldest", "severity", "updated"

	affectedIDs []int64 // internal: result of the Version filter
}

// ToConds implements the db.FindOptions interface
func (opts ListDisclosuresOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.ID != 0 {
		cond = cond.And(builder.Eq{"id": opts.ID})
	}
	if opts.Keyword != "" {
		cond = cond.And(builder.Or(
			db.BuildCaseInsensitiveLike("title", opts.Keyword),
			db.BuildCaseInsensitiveLike("summary", opts.Keyword),
			db.BuildCaseInsensitiveLike("content", opts.Keyword),
		))
	}
	if len(opts.Severities) > 0 {
		severities := make([]string, 0, len(opts.Severities))
		for _, s := range opts.Severities {
			severities = append(severities, string(s))
		}
		cond = cond.And(builder.In("severity", severities))
	}
	if opts.Status != "" {
		cond = cond.And(builder.Eq{"status": opts.Status})
	}
	if !opts.IncludeDrafts {
		cond = cond.And(builder.Neq{"status": StatusDraft})
	}
	if opts.affectedIDs != nil {
		cond = cond.And(builder.In("id", opts.affectedIDs))
	}
	return cond
}

// ToOrders implements the db.FindOptions interface
func (opts ListDisclosuresOptions) ToOrders() string {
	switch opts.OrderBy {
	case "oldest":
		return "created_unix ASC, id ASC"
	case "severity":
		return "CASE severity WHEN 'critical' THEN 5 WHEN 'high' THEN 4 WHEN 'moderate' THEN 3 WHEN 'low' THEN 2 ELSE 1 END DESC, id DESC"
	case "updated":
		return "updated_unix DESC, id DESC"
	default:
		return "created_unix DESC, id DESC"
	}
}

// applyVersionFilter resolves the Version option into a list of matching disclosure IDs
func (opts *ListDisclosuresOptions) applyVersionFilter(ctx context.Context) error {
	if opts.Version == "" {
		return nil
	}
	ids, err := GetAffectedDisclosureIDsByVersion(ctx, opts.RepoID, opts.Version)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		ids = []int64{db.NoConditionID} // match nothing
	}
	opts.affectedIDs = ids
	return nil
}

// ListDisclosures returns the disclosures matching the options, with attributes loaded
func ListDisclosures(ctx context.Context, opts ListDisclosuresOptions) ([]*Disclosure, error) {
	if err := opts.applyVersionFilter(ctx); err != nil {
		return nil, err
	}
	list, err := db.Find[Disclosure](ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := loadAttributes(ctx, list); err != nil {
		return nil, err
	}
	return list, nil
}

// CountDisclosures counts the disclosures matching the options
func CountDisclosures(ctx context.Context, opts ListDisclosuresOptions) (int64, error) {
	if err := opts.applyVersionFilter(ctx); err != nil {
		return 0, err
	}
	return db.Count[Disclosure](ctx, opts)
}

// GetAffectedDisclosureIDsByVersion returns the IDs of the repo's disclosures whose affected
// version entries match the given version. Entries that parse as semver constraints are
// evaluated as ranges, anything else (tags, SHAs, dates) matches by exact string equality.
func GetAffectedDisclosureIDsByVersion(ctx context.Context, repoID int64, v string) ([]int64, error) {
	entries := make([]*Version, 0, 8)
	err := db.GetEngine(ctx).
		Where(builder.Eq{"kind": VersionKindAffected}).
		And(builder.In("disclosure_id", builder.Select("id").From("disclosure").Where(builder.Eq{"repo_id": repoID}))).
		Find(&entries)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		if AffectedVersionMatches(e.Version, v) && !slices.Contains(ids, e.DisclosureID) {
			ids = append(ids, e.DisclosureID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}
