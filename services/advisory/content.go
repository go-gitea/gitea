// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
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

type ContentOptions struct {
	Summary         string
	Description     string
	CveID           string
	Severity        string
	CvssVector      string
	CweIDs          []string
	Vulnerabilities []*advisory_model.Vulnerability
	Credits         []*advisory_model.Credit
	LabelIDs        []int64
	ContentVersion  int
}

func ContentFromAdvisory(advisory *advisory_model.Advisory) *ContentOptions {
	opts := &ContentOptions{
		Summary:         advisory.Summary,
		Description:     advisory.Description,
		CveID:           advisory.CveID,
		CvssVector:      advisory.CvssVector,
		CweIDs:          advisory.CweIDs,
		Vulnerabilities: advisory.Vulnerabilities,
		Credits:         advisory.Credits,
		LabelIDs:        labelIDs(advisory.Labels),
		ContentVersion:  advisory.ContentVersion,
	}
	if advisory.CvssVector == "" {
		opts.Severity = advisory.Severity.String()
	}
	return opts
}

func labelIDs(labels []*issues_model.Label) []int64 {
	return container.FilterSlice(labels, func(label *issues_model.Label) (int64, bool) { return label.ID, true })
}

func fieldsTooLong(values ...string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return utf8.RuneCountInString(value) > maxFieldLength })
}

func applyContent(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, opts *ContentOptions) error {
	summary := strings.TrimSpace(opts.Summary)
	if summary == "" || utf8.RuneCountInString(summary) > maxSummaryLength {
		return util.NewInvalidArgumentErrorf("summary must be between 1 and %d characters", maxSummaryLength)
	}
	description := strings.TrimRight(opts.Description, " \t\r\n") // keeps an indented code block at the start
	if strings.TrimSpace(description) == "" {
		return util.NewInvalidArgumentErrorf("description must not be empty")
	}
	cveID := strings.ToUpper(strings.TrimSpace(opts.CveID))
	if cveID != "" && !cvePattern.MatchString(cveID) {
		return util.NewInvalidArgumentErrorf("invalid CVE ID %q", opts.CveID)
	}
	severity, vector, scoreTenths, err := scoreSeverity(opts.Severity, opts.CvssVector)
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
	credits, err := resolveCredits(ctx, doer, advisory, opts.Credits)
	if err != nil {
		return err
	}
	labels, err := resolveLabels(ctx, advisory, opts.LabelIDs)
	if err != nil {
		return err
	}

	advisory.Summary, advisory.Description, advisory.CveID = summary, description, cveID
	advisory.Severity, advisory.CvssVector, advisory.CvssScoreTenths = severity, vector, scoreTenths
	advisory.CweIDs, advisory.Vulnerabilities, advisory.Credits, advisory.Labels = cweIDs, vulnerabilities, credits, labels
	return nil
}

func normalizeCweIDs(ids []string) ([]string, error) {
	var res []string
	for _, id := range ids {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" || slices.Contains(res, id) {
			continue
		}
		if !cwePattern.MatchString(id) {
			return nil, util.NewInvalidArgumentErrorf("invalid CWE ID %q", id)
		}
		res = append(res, id)
	}
	return res, nil
}

func normalizeVulnerabilities(vulnerabilities []*advisory_model.Vulnerability) ([]*advisory_model.Vulnerability, error) {
	normalized := make([]*advisory_model.Vulnerability, 0, len(vulnerabilities))
	for _, vulnerability := range vulnerabilities {
		if !slices.Contains(advisory_model.Ecosystems, vulnerability.Ecosystem) {
			return nil, util.NewInvalidArgumentErrorf("invalid package ecosystem %q", vulnerability.Ecosystem)
		}
		entry := &advisory_model.Vulnerability{
			Ecosystem:              vulnerability.Ecosystem,
			PackageName:            strings.TrimSpace(vulnerability.PackageName),
			VulnerableVersionRange: strings.TrimSpace(vulnerability.VulnerableVersionRange),
			PatchedVersions:        strings.TrimSpace(vulnerability.PatchedVersions),
		}
		for _, function := range vulnerability.VulnerableFunctions {
			if function = strings.TrimSpace(function); function != "" {
				entry.VulnerableFunctions = append(entry.VulnerableFunctions, function)
			}
		}
		if len(entry.VulnerableFunctions) > maxListLength || fieldsTooLong(entry.PackageName, entry.VulnerableVersionRange, entry.PatchedVersions) || fieldsTooLong(entry.VulnerableFunctions...) {
			return nil, util.NewInvalidArgumentErrorf("at most %d functions and %d characters per package field are allowed", maxListLength, maxFieldLength)
		}
		normalized = append(normalized, entry)
	}
	return normalized, nil
}

func resolveCredits(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, credits []*advisory_model.Credit) ([]*advisory_model.Credit, error) {
	userIDs := make([]int64, 0, len(credits))
	for _, credit := range credits {
		if !slices.Contains(advisory_model.CreditTypes, credit.Type) {
			return nil, util.NewInvalidArgumentErrorf("invalid credit type %q", credit.Type)
		}
		userIDs = append(userIDs, credit.UserID)
	}
	users, err := user_model.GetUsersMapByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	res := make([]*advisory_model.Credit, 0, len(credits))
	for _, credit := range credits {
		if slices.ContainsFunc(res, func(existing *advisory_model.Credit) bool { return existing.UserID == credit.UserID }) {
			continue
		}
		user := users[credit.UserID]
		if user == nil || !user.IsIndividual() {
			return nil, util.NewInvalidArgumentErrorf("only users can be credited")
		}
		if user.ID != doer.ID && !slices.ContainsFunc(advisory.Credits, func(existing *advisory_model.Credit) bool { return existing.UserID == credit.UserID }) {
			if !user_model.IsUserVisibleToViewer(ctx, user, nil) { // Published credits are public.
				return nil, errCannotCredit(user.Name)
			}
			if user_model.IsUserBlockedBy(ctx, doer, user.ID) {
				return nil, user_model.ErrBlockedUser
			}
		}
		res = append(res, &advisory_model.Credit{UserID: credit.UserID, User: user, Type: credit.Type})
	}
	return res, nil
}

func errCannotCredit(login string) error {
	return util.NewInvalidArgumentErrorf("user %q cannot be credited", login)
}

func CreditsByLogins(ctx context.Context, logins, types []string) ([]*advisory_model.Credit, error) {
	if len(logins) > maxListLength {
		return nil, util.NewInvalidArgumentErrorf("at most %d credits are allowed", maxListLength)
	}
	credits := make([]*advisory_model.Credit, 0, len(logins))
	for i, login := range logins {
		if login = strings.TrimSpace(login); login == "" {
			continue
		}
		user, err := user_model.GetUserByName(ctx, login)
		if user_model.IsErrUserNotExist(err) {
			return nil, errCannotCredit(login)
		} else if err != nil {
			return nil, err
		}
		credit := &advisory_model.Credit{UserID: user.ID}
		if i < len(types) {
			credit.Type = types[i]
		}
		credits = append(credits, credit)
	}
	return credits, nil
}

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

func resolveLabels(ctx context.Context, advisory *advisory_model.Advisory, ids []int64) ([]*issues_model.Label, error) {
	wanted := container.SetOf(ids...)
	if maps.Equal(wanted, container.SetOf(labelIDs(advisory.Labels)...)) {
		return advisory.Labels, nil // A label scope can become exclusive after labels were assigned.
	}
	if len(ids) > maxListLength {
		return nil, util.NewInvalidArgumentErrorf("at most %d labels are allowed", maxListLength)
	}
	if err := advisory.LoadRepo(ctx); err != nil {
		return nil, err
	}
	labels, err := RepoAndOrgLabels(ctx, advisory.Repo)
	if err != nil {
		return nil, err
	}
	labels = slices.DeleteFunc(labels, func(label *issues_model.Label) bool { return !wanted.Contains(label.ID) })
	if len(labels) != len(wanted) {
		return nil, util.NewInvalidArgumentErrorf("unknown label")
	}
	scopes := make(container.Set[string])
	for _, label := range labels {
		if scope := label.ExclusiveScope(); scope != "" && !scopes.Add(scope) {
			return nil, util.NewInvalidArgumentErrorf("only one label of the scope %q is allowed", scope)
		}
	}
	return labels, nil
}

func LabelIDsByNames(ctx context.Context, repo *repo_model.Repository, names []string) (ids []int64, allFound bool, err error) {
	labels, err := RepoAndOrgLabels(ctx, repo)
	if err != nil {
		return nil, false, err
	}
	byName := make(map[string]int64, len(labels))
	for _, label := range labels {
		if _, ok := byName[label.Name]; !ok { // the repository labels come first
			byName[label.Name] = label.ID
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

func scoreSeverity(manualSeverity, vector string) (advisory_model.Severity, string, int, error) {
	manualSeverity, vector = strings.TrimSpace(manualSeverity), strings.TrimSpace(vector)
	if vector == "" {
		severity := advisory_model.ParseSeverity(manualSeverity)
		if manualSeverity != "" && severity == advisory_model.SeverityUnknown {
			return 0, "", 0, util.NewInvalidArgumentErrorf("invalid severity %q", manualSeverity)
		}
		return severity, "", 0, nil
	}
	if manualSeverity != "" {
		return 0, "", 0, util.NewInvalidArgumentErrorf("severity cannot be set together with a CVSS vector")
	}
	if len(vector) > maxFieldLength {
		return 0, "", 0, util.NewInvalidArgumentErrorf("CVSS vector is too long")
	}
	score, err := cvss.Parse(vector)
	if err != nil {
		return 0, "", 0, err
	}
	return advisory_model.ParseSeverity(score.Rating), score.Vector, int(math.Round(score.Score * 10)), nil // Match GitHub's empty severity for a score of zero.
}
