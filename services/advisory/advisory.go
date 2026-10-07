// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"errors"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	advisory_model "gitea.dev/models/advisory"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	notify_service "gitea.dev/services/notify"
)

const (
	maxSummaryLength = 1024
	maxFieldLength   = 255
	maxListLength    = 50
)

var (
	cvePattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,19}$`)
	cwePattern = regexp.MustCompile(`^CWE-[1-9][0-9]{0,5}$`)
)

// ContentOptions are the editable details of an advisory
type ContentOptions struct {
	Summary         string
	Description     string
	CveID           string
	Severity        string // only without CVSS vectors, which determine the severity otherwise
	CvssV3Vector    string
	CvssV4Vector    string
	CweIDs          []string
	Vulnerabilities []*advisory_model.Vulnerability
	Credits         []*advisory_model.Credit
	LabelIDs        []int64
	ContentVersion  int // the update is rejected if the advisory has been changed since this version
}

// ContentFromAdvisory returns the current content of an advisory with loaded attributes, for partial updates
func ContentFromAdvisory(a *advisory_model.Advisory) *ContentOptions {
	opts := &ContentOptions{
		Summary:         a.Summary,
		Description:     a.Description,
		CveID:           a.CveID,
		CvssV3Vector:    a.CvssV3Vector,
		CvssV4Vector:    a.CvssV4Vector,
		CweIDs:          a.CweIDs,
		Vulnerabilities: a.Vulnerabilities,
		Credits:         a.Credits,
		LabelIDs:        labelIDs(a.Labels),
		ContentVersion:  a.ContentVersion,
	}
	if !a.HasCVSS() {
		opts.Severity = a.Severity.String()
	}
	return opts
}

func IsValidCweID(id string) bool {
	return cwePattern.MatchString(id)
}

func labelIDs(labels []*issues_model.Label) []int64 {
	return container.FilterSlice(labels, func(l *issues_model.Label) (int64, bool) { return l.ID, true })
}

// SeverityFromRating maps a CVSS rating, a score of 0 ("none") has no own severity
func SeverityFromRating(rating string) advisory_model.Severity {
	if rating == "none" {
		return advisory_model.SeverityLow
	}
	return advisory_model.ParseSeverity(rating)
}

// applyContent validates the content into a copy so that a rejected update doesn't leave the advisory half changed
func applyContent(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, opts *ContentOptions) error {
	res := *a
	res.Summary = strings.TrimSpace(opts.Summary)
	if res.Summary == "" || utf8.RuneCountInString(res.Summary) > maxSummaryLength {
		return util.NewInvalidArgumentErrorf("summary must be between 1 and %d characters", maxSummaryLength)
	}
	res.Description = strings.TrimRight(opts.Description, " \t\r\n") // keeps an indented code block at the start
	if strings.TrimSpace(res.Description) == "" {
		return util.NewInvalidArgumentErrorf("description must not be empty")
	}
	res.CveID = strings.ToUpper(strings.TrimSpace(opts.CveID))
	if res.CveID != "" && !cvePattern.MatchString(res.CveID) {
		return util.NewInvalidArgumentErrorf("invalid CVE ID %q", opts.CveID)
	}
	if err := applySeverity(&res, opts); err != nil {
		return err
	}
	if len(opts.Vulnerabilities) > maxListLength || len(opts.Credits) > maxListLength || len(opts.CweIDs) > maxListLength {
		return util.NewInvalidArgumentErrorf("at most %d vulnerabilities, credits and CWE IDs are allowed", maxListLength)
	}

	res.CweIDs = nil
	for _, id := range opts.CweIDs {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" || slices.Contains(res.CweIDs, id) {
			continue
		}
		if !IsValidCweID(id) {
			return util.NewInvalidArgumentErrorf("invalid CWE ID %q", id)
		}
		res.CweIDs = append(res.CweIDs, id)
	}

	res.Vulnerabilities = make([]*advisory_model.Vulnerability, 0, len(opts.Vulnerabilities))
	for _, v := range opts.Vulnerabilities {
		fields := []string{strings.TrimSpace(v.PackageName), strings.TrimSpace(v.VulnerableVersionRange), strings.TrimSpace(v.PatchedVersions)}
		var functions []string
		for _, f := range v.VulnerableFunctions {
			if f = strings.TrimSpace(f); f != "" {
				functions = append(functions, f)
			}
		}
		if !slices.Contains(advisory_model.Ecosystems, v.Ecosystem) {
			return util.NewInvalidArgumentErrorf("invalid package ecosystem %q", v.Ecosystem)
		}
		if len(functions) > maxListLength || slices.ContainsFunc(append(fields, functions...), func(f string) bool { return utf8.RuneCountInString(f) > maxFieldLength }) {
			return util.NewInvalidArgumentErrorf("at most %d functions and %d characters per package field are allowed", maxListLength, maxFieldLength)
		}
		res.Vulnerabilities = append(res.Vulnerabilities, &advisory_model.Vulnerability{
			Ecosystem: v.Ecosystem, PackageName: fields[0], VulnerableVersionRange: fields[1], PatchedVersions: fields[2], VulnerableFunctions: functions,
		})
	}

	userIDs := make([]int64, 0, len(opts.Credits))
	for _, c := range opts.Credits {
		if !slices.Contains(advisory_model.CreditTypes, c.Type) {
			return util.NewInvalidArgumentErrorf("invalid credit type %q", c.Type)
		}
		userIDs = append(userIDs, c.UserID)
	}
	users, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return err
	}
	// unchanged labels are kept even if they have become invalid, e.g. by making a scope exclusive
	if !maps.Equal(container.SetOf(opts.LabelIDs...), container.SetOf(labelIDs(a.Labels)...)) {
		if res.Labels, err = validateLabels(ctx, a, opts.LabelIDs); err != nil {
			return err
		}
	}

	res.Credits = make([]*advisory_model.Credit, 0, len(opts.Credits))
	for _, c := range opts.Credits {
		if slices.ContainsFunc(res.Credits, func(e *advisory_model.Credit) bool { return e.UserID == c.UserID }) {
			continue
		}
		u := users[c.UserID]
		if u == nil || !u.IsIndividual() {
			return util.NewInvalidArgumentErrorf("only users can be credited")
		}
		// published credits are public, so others must be public users who accept contact from the doer
		alreadyCredited := slices.ContainsFunc(a.Credits, func(e *advisory_model.Credit) bool { return e.UserID == c.UserID })
		if !alreadyCredited && u.ID != doer.ID {
			if !user_model.IsUserVisibleToViewer(ctx, u, nil) {
				return errCannotCredit(u.Name)
			} else if user_model.IsUserBlockedBy(ctx, doer, u.ID) {
				return user_model.ErrBlockedUser
			}
		}
		res.Credits = append(res.Credits, &advisory_model.Credit{UserID: c.UserID, User: u, Type: c.Type})
	}
	*a = res
	return nil
}

// errCannotCredit doesn't tell missing and private users apart
func errCannotCredit(login string) error {
	return util.NewInvalidArgumentErrorf("user %q cannot be credited", login)
}

// CreditsByLogins checks the number of credits first to not look up unlimited logins, types are matched by index
func CreditsByLogins(ctx context.Context, logins, types []string) ([]*advisory_model.Credit, error) {
	if len(logins) > maxListLength {
		return nil, util.NewInvalidArgumentErrorf("at most %d credits are allowed", maxListLength)
	}
	credits := make([]*advisory_model.Credit, 0, len(logins))
	for i, login := range logins {
		if login = strings.TrimSpace(login); login == "" {
			continue
		}
		u, err := user_model.GetUserByName(ctx, login)
		if user_model.IsErrUserNotExist(err) {
			return nil, errCannotCredit(login)
		} else if err != nil {
			return nil, err
		}
		c := &advisory_model.Credit{UserID: u.ID}
		if i < len(types) {
			c.Type = types[i]
		}
		credits = append(credits, c)
	}
	return credits, nil
}

// isBlockedFromAdvisory also checks the reporter because a private report is their content
func isBlockedFromAdvisory(ctx context.Context, a *advisory_model.Advisory, u *user_model.User) bool {
	if a.IsReport {
		return user_model.IsUserBlockedBy(ctx, u, a.Repo.OwnerID, a.ReporterID)
	}
	return user_model.IsUserBlockedBy(ctx, u, a.Repo.OwnerID)
}

// RepoAndOrgLabels returns the labels which can be assigned to the advisories of a repository
func RepoAndOrgLabels(ctx context.Context, repo *repo_model.Repository) ([]*issues_model.Label, error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}
	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil || !repo.Owner.IsOrganization() {
		return labels, err
	}
	orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.OwnerID, "", db.ListOptions{})
	return append(labels, orgLabels...), err
}

// validateLabels allows the labels of the repository and its organization with at most one label per exclusive scope,
// other labels are reported as unknown to not reveal their names
func validateLabels(ctx context.Context, a *advisory_model.Advisory, ids []int64) ([]*issues_model.Label, error) {
	if len(ids) > maxListLength {
		return nil, util.NewInvalidArgumentErrorf("at most %d labels are allowed", maxListLength)
	}
	if err := a.LoadRepo(ctx); err != nil {
		return nil, err
	}
	labels, err := RepoAndOrgLabels(ctx, a.Repo)
	if err != nil {
		return nil, err
	}
	wanted := container.SetOf(ids...)
	labels = slices.DeleteFunc(labels, func(l *issues_model.Label) bool { return !wanted.Contains(l.ID) })
	if len(labels) != len(wanted) {
		return nil, util.NewInvalidArgumentErrorf("unknown label")
	}
	scopes := make(container.Set[string])
	for _, l := range labels {
		if scope := l.ExclusiveScope(); scope != "" && !scopes.Add(scope) {
			return nil, util.NewInvalidArgumentErrorf("only one label of the scope %q is allowed", scope)
		}
	}
	return labels, nil
}

func applySeverity(a *advisory_model.Advisory, opts *ContentOptions) error {
	a.CvssV3Vector, a.CvssV3ScoreTenths, a.CvssV4Vector, a.CvssV4ScoreTenths = "", 0, "", 0
	var rating string
	for _, vector := range []string{opts.CvssV3Vector, opts.CvssV4Vector} {
		if strings.TrimSpace(vector) == "" {
			continue
		}
		r, err := cvss.Parse(vector)
		if err != nil {
			return err
		}
		if len(r.Vector) > maxFieldLength {
			return util.NewInvalidArgumentErrorf("CVSS vector is too long")
		}
		switch {
		case r.Version == cvss.Version31 && a.CvssV3Vector == "":
			a.CvssV3Vector, a.CvssV3ScoreTenths = r.Vector, int(math.Round(r.Score*10))
		case r.Version == cvss.Version40 && a.CvssV4Vector == "":
			a.CvssV4Vector, a.CvssV4ScoreTenths = r.Vector, int(math.Round(r.Score*10))
		default:
			return util.NewInvalidArgumentErrorf("only one CVSS vector per version is allowed")
		}
		if r.Version == cvss.Version40 || a.CvssV4Vector == "" {
			rating = r.Rating // CVSS 4.0 takes precedence
		}
	}

	severity := strings.TrimSpace(opts.Severity)
	if a.HasCVSS() {
		if severity != "" {
			return util.NewInvalidArgumentErrorf("severity cannot be set together with a CVSS vector")
		}
		a.Severity = SeverityFromRating(rating)
		return nil
	}
	if severity == "" {
		a.Severity = advisory_model.SeverityUnknown
		return nil
	}
	if a.Severity = advisory_model.ParseSeverity(severity); a.Severity == advisory_model.SeverityUnknown {
		return util.NewInvalidArgumentErrorf("invalid severity %q", severity)
	}
	return nil
}

// CreateAdvisory creates a draft, only repository admins may call it
func CreateAdvisory(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, opts *ContentOptions) (*advisory_model.Advisory, error) {
	a := &advisory_model.Advisory{RepoID: repo.ID, Repo: repo, State: advisory_model.StateDraft, ReporterID: doer.ID, Reporter: doer}
	if err := applyContent(ctx, doer, a, opts); err != nil {
		return nil, err
	}
	if err := advisory_model.CreateAdvisory(ctx, a); err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisory(ctx, doer, a)
	return a, nil
}

func IsPrivateReportingEnabled(ctx context.Context, repo *repo_model.Repository) bool {
	if repo.IsArchived || unit.TypeSecurityAdvisories.UnitGlobalDisabled() {
		return false
	}
	ru, err := repo.GetUnit(ctx, unit.TypeSecurityAdvisories)
	return err == nil && ru.SecurityAdvisoriesConfig().PrivateVulnerabilityReporting
}

func SetPrivateReporting(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, enabled bool) error {
	ru, err := repo.GetUnit(ctx, unit.TypeSecurityAdvisories)
	if err != nil {
		return err
	}
	cfg := ru.SecurityAdvisoriesConfig()
	if cfg.PrivateVulnerabilityReporting == enabled {
		return nil
	}
	cfg.PrivateVulnerabilityReporting = enabled
	if err := repo_model.UpdateRepoUnitConfig(ctx, ru); err != nil {
		return err
	}
	audit.RecordAs(ctx, doer, audit_model.SecurityAdvisoryPrivateReporting, repo, "enabled", enabled)
	return nil
}

// ReportVulnerability creates a triage advisory from a private report, the reporter is credited and can't assign a CVE ID
func ReportVulnerability(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, opts *ContentOptions) (*advisory_model.Advisory, error) {
	if !doer.IsIndividual() || !IsPrivateReportingEnabled(ctx, repo) {
		return nil, util.NewPermissionDeniedErrorf("private vulnerability reporting is not enabled")
	}
	if user_model.IsUserBlockedBy(ctx, doer, repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}
	opts.CveID, opts.LabelIDs = "", nil
	opts.Credits = []*advisory_model.Credit{{UserID: doer.ID, Type: "reporter"}}
	a := &advisory_model.Advisory{RepoID: repo.ID, Repo: repo, State: advisory_model.StateTriage, IsReport: true, ReporterID: doer.ID, Reporter: doer}
	if err := applyContent(ctx, doer, a, opts); err != nil {
		return nil, err
	}
	if err := advisory_model.CreateAdvisory(ctx, a); err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisoryReport(ctx, doer, a)
	return a, nil
}

// UpdateAdvisory replaces the content of an advisory with loaded attributes, the caller must check Permissions.CanEdit.
// Only managers can change the CVE ID, the credits and the labels.
func UpdateAdvisory(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, opts *ContentOptions, canManage bool) error {
	if !canManage {
		if err := a.LoadRepo(ctx); err != nil {
			return err
		}
		if isBlockedFromAdvisory(ctx, a, doer) {
			return user_model.ErrBlockedUser
		}
		opts.CveID, opts.Credits, opts.LabelIDs = a.CveID, a.Credits, labelIDs(a.Labels)
	}
	if err := applyContent(ctx, doer, a, opts); err != nil {
		return err
	}
	a.ContentVersion = opts.ContentVersion
	return advisory_model.UpdateAdvisory(ctx, a, "summary", "description", "cve_id", "severity",
		"cvss_v3_vector", "cvss_v3_score_tenths", "cvss_v4_vector", "cvss_v4_score_tenths", "cwe_ids")
}

// StateOptions closing an advisory need a reason, duplicates also the original advisory
type StateOptions struct {
	State       advisory_model.State
	CloseReason advisory_model.CloseReason
	DuplicateOf *advisory_model.Advisory
}

// NewStateOptions parses the names, the original of a duplicate is looked up in the repository of the advisory
func NewStateOptions(ctx context.Context, a *advisory_model.Advisory, state, closeReason, duplicateOf string) (StateOptions, error) {
	opts := StateOptions{State: advisory_model.ParseState(state), CloseReason: advisory_model.ParseCloseReason(closeReason)}
	if closeReason != "" && opts.CloseReason == advisory_model.CloseReasonNone {
		return opts, util.NewInvalidArgumentErrorf("invalid close reason %q", closeReason)
	}
	if duplicateOf != "" {
		original, err := advisory_model.GetAdvisoryByIdentifier(ctx, a.RepoID, duplicateOf)
		if errors.Is(err, util.ErrNotExist) {
			return opts, util.NewInvalidArgumentErrorf("advisory %q does not exist", duplicateOf)
		} else if err != nil {
			return opts, err
		}
		original.Repo = a.Repo
		opts.DuplicateOf = original
	}
	return opts, validateStateOptions(a, opts)
}

func validateStateOptions(a *advisory_model.Advisory, opts StateOptions) error {
	if !a.State.CanTransitionTo(opts.State) {
		return util.NewInvalidArgumentErrorf("cannot change the state of the advisory from %s to %s", a.State, opts.State)
	}
	if opts.State != advisory_model.StateClosed {
		if opts.CloseReason != advisory_model.CloseReasonNone || opts.DuplicateOf != nil {
			return util.NewInvalidArgumentErrorf("only closed advisories have a close reason")
		}
		return nil
	}
	if opts.CloseReason == advisory_model.CloseReasonNone {
		return util.NewInvalidArgumentErrorf("a close reason is required")
	}
	if (opts.CloseReason == advisory_model.CloseReasonDuplicate) != (opts.DuplicateOf != nil) {
		return util.NewInvalidArgumentErrorf("duplicates and only duplicates need the original advisory")
	}
	if opts.DuplicateOf != nil && opts.DuplicateOf.ID == a.ID {
		return util.NewInvalidArgumentErrorf("an advisory cannot be a duplicate of itself")
	}
	return nil
}

// ChangeState publishes, closes, reopens or withdraws an advisory, only repository admins may call it.
// The reporter of a duplicate joins the original advisory to follow it.
func ChangeState(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, opts StateOptions) error {
	if err := validateStateOptions(a, opts); err != nil {
		return err
	}
	oldState, oldDuplicateOfID := a.State, a.DuplicateOfID
	a.State = opts.State
	var cols []string
	now := timeutil.TimeStampNow()
	switch opts.State {
	case advisory_model.StatePublished:
		a.PublisherID, a.Publisher, a.PublishedUnix = doer.ID, doer, now
		cols = []string{"publisher_id", "published_unix"}
	case advisory_model.StateWithdrawn:
		a.WithdrawnUnix = now
		cols = []string{"withdrawn_unix"}
	case advisory_model.StateClosed, advisory_model.StateDraft:
		a.ClosedUnix = util.Iif(opts.State == advisory_model.StateClosed, now, 0)
		a.CloseReason, a.DuplicateOf, a.DuplicateOfID = opts.CloseReason, opts.DuplicateOf, 0
		if opts.DuplicateOf != nil {
			a.DuplicateOfID = opts.DuplicateOf.ID
		}
		cols = []string{"closed_unix", "close_reason", "duplicate_of_id"}
	}
	changed, err := advisory_model.UpdateAdvisoryState(ctx, a, oldState, cols...)
	if err != nil {
		return err
	} else if !changed {
		return util.NewInvalidArgumentErrorf("the state of the advisory has been changed in the meantime")
	}
	notify_service.SecurityAdvisoryStateChanged(ctx, doer, a, oldState)

	if oldDuplicateOfID != 0 && a.IsReport {
		if err := revokeDuplicateAccess(ctx, doer, a, oldDuplicateOfID); err != nil {
			return err
		}
	}
	if opts.DuplicateOf != nil && a.IsReport && !(opts.DuplicateOf.IsReport && a.ReporterID == opts.DuplicateOf.ReporterID) {
		reporter, err := user_model.GetUserByID(ctx, a.ReporterID)
		if err == nil {
			if err = validateCollaborators(ctx, opts.DuplicateOf, []*user_model.User{reporter}, nil); err == nil {
				err = changeCollaborators(ctx, doer, opts.DuplicateOf, []collaboratorChange{{user: reporter, added: true, readOnly: true}})
			}
		}
		// the state has been changed already, the reporter just can't follow the original
		if err != nil && !errors.Is(err, util.ErrNotExist) && !errors.Is(err, util.ErrInvalidArgument) && !errors.Is(err, user_model.ErrBlockedUser) {
			log.Error("ChangeState: the reporter of %d cannot follow the original %d: %v", a.ID, opts.DuplicateOf.ID, err)
		}
	}
	return nil
}

// revokeDuplicateAccess removes the reporter of a reopened duplicate from the original unless they have been granted write access
func revokeDuplicateAccess(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, originalID int64) error {
	removed, err := advisory_model.RemoveReadOnlyCollaborator(ctx, originalID, a.ReporterID)
	if err != nil || !removed {
		return err
	}
	original, has, err := db.GetByID[advisory_model.Advisory](ctx, originalID)
	if err != nil || !has {
		return err
	}
	original.Repo = a.Repo
	_, reporter, err := user_model.GetPossibleUserByID(ctx, a.ReporterID)
	if err != nil {
		return err
	}
	notify_service.SecurityAdvisoryCollaboratorRemoved(ctx, doer, original, reporter, nil)
	return nil
}

// DeleteAdvisory deletes an advisory which has never been published, only repository admins may call it
func DeleteAdvisory(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) error {
	if err := advisory_model.DeleteAdvisory(ctx, a); err != nil {
		return err
	}
	notify_service.DeleteSecurityAdvisory(ctx, doer, a)
	return nil
}

// validateCollaborators checks that the users and teams can read the advisories unit and the teams belong to the repository owner
func validateCollaborators(ctx context.Context, a *advisory_model.Advisory, users []*user_model.User, teams []*organization.Team) error {
	if len(users)+len(teams) > maxListLength {
		return util.NewInvalidArgumentErrorf("at most %d collaborators are allowed", maxListLength)
	}
	if err := a.LoadRepo(ctx); err != nil {
		return err
	}
	if err := a.Repo.LoadOwner(ctx); err != nil {
		return err
	}
	for _, u := range users {
		if !u.IsIndividual() {
			return util.NewInvalidArgumentErrorf("only users can collaborate on advisories")
		}
		if isBlockedFromAdvisory(ctx, a, u) || user_model.IsUserBlockedBy(ctx, a.Repo.Owner, u.ID) {
			return user_model.ErrBlockedUser
		}
		perm, err := access_model.GetIndividualUserRepoPermission(ctx, a.Repo, u)
		if err != nil {
			return err
		}
		if !perm.CanRead(unit.TypeSecurityAdvisories) {
			return util.NewInvalidArgumentErrorf("user %s cannot access the advisories of the repository", u.Name)
		}
	}
	for _, t := range teams {
		if t.OrgID != a.Repo.OwnerID {
			return util.NewInvalidArgumentErrorf("team %s does not belong to the repository owner", t.Name)
		}
		if a.Repo.IsPrivate && !(organization.HasTeamRepo(ctx, t.OrgID, t.ID, a.RepoID) && t.UnitEnabled(ctx, unit.TypeSecurityAdvisories)) {
			return util.NewInvalidArgumentErrorf("team %s cannot access the advisories of the repository", t.Name)
		}
	}
	return nil
}

// ValidateNewCollaborators validates the users and teams which don't collaborate yet, for SetCollaborators,
// the collaborators of the advisory must be loaded
func ValidateNewCollaborators(ctx context.Context, a *advisory_model.Advisory, users []*user_model.User, teams []*organization.Team) error {
	users = slices.DeleteFunc(slices.Clone(users), func(u *user_model.User) bool {
		return slices.ContainsFunc(a.CollaboratorUsers, func(c *user_model.User) bool { return c.ID == u.ID })
	})
	teams = slices.DeleteFunc(slices.Clone(teams), func(t *organization.Team) bool {
		return slices.ContainsFunc(a.CollaboratorTeams, func(c *organization.Team) bool { return c.ID == t.ID })
	})
	return validateCollaborators(ctx, a, users, teams)
}

type collaboratorChange struct {
	user     *user_model.User
	team     *organization.Team
	added    bool
	readOnly bool
}

// changeCollaborators applies the changes in a transaction and only notifies about the effective ones after the commit
func changeCollaborators(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, changes []collaboratorChange) error {
	var effective []collaboratorChange
	err := db.WithTx(ctx, func(ctx context.Context) error {
		for _, c := range changes {
			var userID, teamID int64
			if c.user != nil {
				userID = c.user.ID
			} else {
				teamID = c.team.ID
			}
			var changed bool
			var err error
			if c.added {
				changed, err = advisory_model.AddCollaborator(ctx, a.ID, userID, teamID, c.readOnly)
			} else {
				changed, err = advisory_model.RemoveCollaborator(ctx, a.ID, userID, teamID)
			}
			if err != nil {
				return err
			}
			if changed {
				effective = append(effective, c)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, c := range effective {
		if c.added {
			notify_service.SecurityAdvisoryCollaboratorAdded(ctx, doer, a, c.user, c.team)
		} else {
			notify_service.SecurityAdvisoryCollaboratorRemoved(ctx, doer, a, c.user, c.team)
		}
	}
	return nil
}

// AddCollaborator grants a user or a team, exactly one of them is set, access to an advisory
func AddCollaborator(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, user *user_model.User, team *organization.Team) error {
	if err := validateCollaborators(ctx, a, util.Iif(user != nil, []*user_model.User{user}, nil), util.Iif(team != nil, []*organization.Team{team}, nil)); err != nil {
		return err
	}
	return changeCollaborators(ctx, doer, a, []collaboratorChange{{user: user, team: team, added: true}})
}

// RemoveCollaborator revokes the access of a user or a team, exactly one of them is set
func RemoveCollaborator(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, user *user_model.User, team *organization.Team) error {
	return changeCollaborators(ctx, doer, a, []collaboratorChange{{user: user, team: team}})
}

// SetCollaborators replaces the loaded collaborators, the caller must check the new ones with ValidateNewCollaborators
func SetCollaborators(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, users []*user_model.User, teams []*organization.Team) error {
	var changes []collaboratorChange
	for _, u := range a.CollaboratorUsers {
		if !slices.ContainsFunc(users, func(n *user_model.User) bool { return n.ID == u.ID }) {
			changes = append(changes, collaboratorChange{user: u})
		}
	}
	for _, t := range a.CollaboratorTeams {
		if !slices.ContainsFunc(teams, func(n *organization.Team) bool { return n.ID == t.ID }) {
			changes = append(changes, collaboratorChange{team: t})
		}
	}
	// existing ones are added too to grant write access to read-only collaborators
	for _, u := range users {
		changes = append(changes, collaboratorChange{user: u, added: true})
	}
	for _, t := range teams {
		changes = append(changes, collaboratorChange{team: t, added: true})
	}
	return changeCollaborators(ctx, doer, a, changes)
}

func checkComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, content string) error {
	if strings.TrimSpace(content) == "" {
		return util.NewInvalidArgumentErrorf("comment must not be empty")
	}
	if err := a.LoadRepo(ctx); err != nil {
		return err
	}
	if isBlockedFromAdvisory(ctx, a, doer) && !access_model.IsUserRepoAdmin(ctx, a.Repo, doer) {
		return user_model.ErrBlockedUser
	}
	return nil
}

// CreateComment adds a comment to the private discussion, the caller must check Viewer.CanSeeDiscussion
func CreateComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, content string) (*advisory_model.Comment, error) {
	if err := checkComment(ctx, doer, a, content); err != nil {
		return nil, err
	}
	c := &advisory_model.Comment{AdvisoryID: a.ID, PosterID: doer.ID, Poster: doer, Content: content}
	if err := advisory_model.CreateComment(ctx, c); err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisoryComment(ctx, doer, a, c)
	return c, nil
}

func EditComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, c *advisory_model.Comment, content string) error {
	if c.PosterID != doer.ID {
		return util.NewPermissionDeniedErrorf("only the poster can edit a comment")
	}
	if err := checkComment(ctx, doer, a, content); err != nil {
		return err
	}
	c.Content = content
	return advisory_model.UpdateCommentContent(ctx, c)
}

func DeleteComment(ctx context.Context, doer *user_model.User, c *advisory_model.Comment, canManage bool) error {
	if c.PosterID != doer.ID && !canManage {
		return util.NewPermissionDeniedErrorf("only the poster and repository admins can delete a comment")
	}
	return advisory_model.DeleteComment(ctx, c)
}
