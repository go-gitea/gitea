// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"cmp"
	"context"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/util"
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

func tooLong(maxLength int, values ...string) bool {
	return slices.ContainsFunc(values, func(s string) bool { return utf8.RuneCountInString(s) > maxLength })
}

// applyContent validates everything before changing the advisory, so that a rejected update doesn't leave it half changed
func applyContent(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, opts *ContentOptions) error {
	summary, err := normalizeSummary(opts.Summary)
	if err != nil {
		return err
	}
	description, err := normalizeDescription(opts.Description)
	if err != nil {
		return err
	}
	cveID, err := normalizeCveID(opts.CveID)
	if err != nil {
		return err
	}
	scores, err := scoreSeverity(opts)
	if err != nil {
		return err
	}
	if len(opts.Vulnerabilities) > maxListLength || len(opts.Credits) > maxListLength || len(opts.CweIDs) > maxListLength {
		return util.NewInvalidArgumentErrorf("at most %d vulnerabilities, credits and CWE IDs are allowed", maxListLength)
	}
	cweIDs, err := normalizeCweIDs(opts.CweIDs)
	if err != nil {
		return err
	}
	vulnerabilities, err := normalizeVulnerabilities(opts.Vulnerabilities)
	if err != nil {
		return err
	}
	credits, err := resolveCredits(ctx, doer, a, opts.Credits)
	if err != nil {
		return err
	}
	labels, err := resolveLabels(ctx, a, opts.LabelIDs)
	if err != nil {
		return err
	}

	a.Summary, a.Description, a.CveID = summary, description, cveID
	a.Severity = scores.severity
	a.CvssV3Vector, a.CvssV3ScoreTenths = scores.v3Vector, scores.v3ScoreTenths
	a.CvssV4Vector, a.CvssV4ScoreTenths = scores.v4Vector, scores.v4ScoreTenths
	a.CweIDs, a.Vulnerabilities, a.Credits, a.Labels = cweIDs, vulnerabilities, credits, labels
	return nil
}

func normalizeSummary(summary string) (string, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" || tooLong(maxSummaryLength, summary) {
		return "", util.NewInvalidArgumentErrorf("summary must be between 1 and %d characters", maxSummaryLength)
	}
	return summary, nil
}

func normalizeDescription(description string) (string, error) {
	description = strings.TrimRight(description, " \t\r\n") // keeps an indented code block at the start
	if strings.TrimSpace(description) == "" {
		return "", util.NewInvalidArgumentErrorf("description must not be empty")
	}
	return description, nil
}

func normalizeCveID(id string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(id))
	if normalized != "" && !cvePattern.MatchString(normalized) {
		return "", util.NewInvalidArgumentErrorf("invalid CVE ID %q", id)
	}
	return normalized, nil
}

func normalizeCweIDs(ids []string) ([]string, error) {
	var res []string
	for _, id := range ids {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" || slices.Contains(res, id) {
			continue
		}
		if !IsValidCweID(id) {
			return nil, util.NewInvalidArgumentErrorf("invalid CWE ID %q", id)
		}
		res = append(res, id)
	}
	return res, nil
}

func normalizeVulnerabilities(vulns []*advisory_model.Vulnerability) ([]*advisory_model.Vulnerability, error) {
	res := make([]*advisory_model.Vulnerability, 0, len(vulns))
	for _, v := range vulns {
		if !slices.Contains(advisory_model.Ecosystems, v.Ecosystem) {
			return nil, util.NewInvalidArgumentErrorf("invalid package ecosystem %q", v.Ecosystem)
		}
		packageName := strings.TrimSpace(v.PackageName)
		versionRange := strings.TrimSpace(v.VulnerableVersionRange)
		patchedVersions := strings.TrimSpace(v.PatchedVersions)
		var functions []string
		for _, f := range v.VulnerableFunctions {
			if f = strings.TrimSpace(f); f != "" {
				functions = append(functions, f)
			}
		}
		if len(functions) > maxListLength {
			return nil, util.NewInvalidArgumentErrorf("at most %d functions and %d characters per package field are allowed", maxListLength, maxFieldLength)
		}
		if tooLong(maxFieldLength, packageName, versionRange, patchedVersions) || tooLong(maxFieldLength, functions...) {
			return nil, util.NewInvalidArgumentErrorf("at most %d functions and %d characters per package field are allowed", maxListLength, maxFieldLength)
		}
		res = append(res, &advisory_model.Vulnerability{
			Ecosystem:              v.Ecosystem,
			PackageName:            packageName,
			VulnerableVersionRange: versionRange,
			PatchedVersions:        patchedVersions,
			VulnerableFunctions:    functions,
		})
	}
	return res, nil
}

// resolveCredits only checks the newly credited users, existing credits stay valid
func resolveCredits(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, credits []*advisory_model.Credit) ([]*advisory_model.Credit, error) {
	userIDs := make([]int64, 0, len(credits))
	for _, c := range credits {
		if !slices.Contains(advisory_model.CreditTypes, c.Type) {
			return nil, util.NewInvalidArgumentErrorf("invalid credit type %q", c.Type)
		}
		userIDs = append(userIDs, c.UserID)
	}
	users, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	res := make([]*advisory_model.Credit, 0, len(credits))
	for _, c := range credits {
		if slices.ContainsFunc(res, func(e *advisory_model.Credit) bool { return e.UserID == c.UserID }) {
			continue
		}
		u := users[c.UserID]
		if u == nil || !u.IsIndividual() {
			return nil, util.NewInvalidArgumentErrorf("only users can be credited")
		}
		alreadyCredited := slices.ContainsFunc(a.Credits, func(e *advisory_model.Credit) bool { return e.UserID == c.UserID })
		if !alreadyCredited && u.ID != doer.ID {
			if err := checkCreditable(ctx, doer, u); err != nil {
				return nil, err
			}
		}
		res = append(res, &advisory_model.Credit{UserID: c.UserID, User: u, Type: c.Type})
	}
	return res, nil
}

// checkCreditable requires public users who accept contact from the doer because published credits are public
func checkCreditable(ctx context.Context, doer, u *user_model.User) error {
	if !user_model.IsUserVisibleToViewer(ctx, u, nil) {
		return errCannotCredit(u.Name)
	}
	if user_model.IsUserBlockedBy(ctx, doer, u.ID) {
		return user_model.ErrBlockedUser
	}
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

// resolveLabels keeps unchanged labels even if they have become invalid, e.g. by making a scope exclusive
func resolveLabels(ctx context.Context, a *advisory_model.Advisory, ids []int64) ([]*issues_model.Label, error) {
	if maps.Equal(container.SetOf(ids...), container.SetOf(labelIDs(a.Labels)...)) {
		return a.Labels, nil
	}
	return validateLabels(ctx, a, ids)
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

// LabelIDsByNames prefers repository labels over organization labels of the same name
func LabelIDsByNames(ctx context.Context, repo *repo_model.Repository, names []string) (ids []int64, allFound bool, err error) {
	labels, err := RepoAndOrgLabels(ctx, repo)
	if err != nil {
		return nil, false, err
	}
	byName := make(map[string]int64, len(labels))
	for _, l := range labels {
		if _, ok := byName[l.Name]; !ok { // the repository labels come first
			byName[l.Name] = l.ID
		}
	}
	for _, name := range names {
		id, ok := byName[strings.TrimSpace(name)]
		if !ok {
			return nil, false, nil
		}
		ids = append(ids, id)
	}
	return ids, true, nil
}

type cvssScores struct {
	severity      advisory_model.Severity
	v3Vector      string
	v3ScoreTenths int
	v4Vector      string
	v4ScoreTenths int
}

// parseCVSS returns nil for an empty vector
func parseCVSS(vector string, version cvss.Version) (*cvss.Result, error) {
	if strings.TrimSpace(vector) == "" {
		return nil, nil //nolint:nilnil // no vector of this version
	}
	r, err := cvss.Parse(vector)
	if err != nil {
		return nil, err
	}
	if r.Version != version {
		return nil, util.NewInvalidArgumentErrorf("only one CVSS vector per version is allowed")
	}
	if len(r.Vector) > maxFieldLength {
		return nil, util.NewInvalidArgumentErrorf("CVSS vector is too long")
	}
	return r, nil
}

// scoreSeverity derives the severity from the CVSS vectors, CVSS 4.0 takes precedence, a manual severity is only allowed without vectors
func scoreSeverity(opts *ContentOptions) (res cvssScores, err error) {
	v3, err := parseCVSS(opts.CvssV3Vector, cvss.Version31)
	if err != nil {
		return res, err
	}
	v4, err := parseCVSS(opts.CvssV4Vector, cvss.Version40)
	if err != nil {
		return res, err
	}
	var v3Rating, v4Rating string
	if v3 != nil {
		res.v3Vector, res.v3ScoreTenths, v3Rating = v3.Vector, int(math.Round(v3.Score*10)), v3.Rating
	}
	if v4 != nil {
		res.v4Vector, res.v4ScoreTenths, v4Rating = v4.Vector, int(math.Round(v4.Score*10)), v4.Rating
	}

	severity := strings.TrimSpace(opts.Severity)
	if rating := cmp.Or(v4Rating, v3Rating); rating != "" {
		if severity != "" {
			return res, util.NewInvalidArgumentErrorf("severity cannot be set together with a CVSS vector")
		}
		res.severity = SeverityFromRating(rating)
		return res, nil
	}
	if severity == "" {
		return res, nil
	}
	if res.severity = advisory_model.ParseSeverity(severity); res.severity == advisory_model.SeverityUnknown {
		return res, util.NewInvalidArgumentErrorf("invalid severity %q", severity)
	}
	return res, nil
}
