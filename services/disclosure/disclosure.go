// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package disclosure

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	disclosure_model "gitea.dev/models/disclosure"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

// Input carries the editable fields of a disclosure, shared by the web and API handlers
type Input struct {
	Title            string
	Summary          string
	Content          string
	Severity         disclosure_model.Severity
	CVSSVector       string
	CweIds           []string
	CveID            string
	References       []string
	AffectedVersions []string
	FixedVersions    []string
	// Credits are the raw credit entries; UserName refers to a local user, otherwise
	// Name/URL describe an external person
	Credits []CreditInput
	// KeepCredits leaves the existing credits unchanged (API edit: credits not sent)
	KeepCredits bool
	// CreditsResolved is set by validateInput from Credits
	CreditsResolved disclosure_model.CreditList
	Status          disclosure_model.Status
	WithdrawReason  string
}

// CreditInput is one raw credit entry from a form or API request
type CreditInput struct {
	Role     string
	UserName string // local Gitea user name; when set, Name/URL are ignored
	Name     string // external person's display name
	URL      string // external person's optional link
}

// InputFromDisclosure builds an input carrying the disclosure's current field values,
// e.g. for a status change that must not alter the content
func InputFromDisclosure(d *disclosure_model.Disclosure) *Input {
	return &Input{
		Title:            d.Title,
		Summary:          d.Summary,
		Content:          d.Content,
		Severity:         d.Severity,
		CVSSVector:       d.CVSSVector,
		CweIds:           d.CweIds,
		CveID:            d.CveID,
		References:       d.References,
		AffectedVersions: d.AffectedVersions(),
		FixedVersions:    d.FixedVersions(),
	}
}

const (
	// TitleMaxLength is the maximum length of a disclosure title
	TitleMaxLength = 255
	// SummaryMaxLength is the maximum length of a disclosure summary
	SummaryMaxLength = 4096
	// CveIDMaxLength is the maximum length of the optional CVE ID reference
	CveIDMaxLength = 64
)

// SplitLines splits a multi-line form field into trimmed, non-empty entries
func SplitLines(s string) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// SplitCommaSeparated splits a comma-separated form field into trimmed, non-empty entries
func SplitCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateInput(ctx context.Context, in *Input) error {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return util.NewInvalidArgumentErrorf("title is required")
	}
	if len(in.Title) > TitleMaxLength {
		return util.NewInvalidArgumentErrorf("title too long")
	}
	if len(in.Summary) > SummaryMaxLength {
		return util.NewInvalidArgumentErrorf("summary too long")
	}
	// severity and CVSS vector are mutually exclusive: when a CVSS vector is set,
	// the severity is derived from its base score server-side
	if in.CVSSVector != "" {
		vec, err := cvss.Parse(in.CVSSVector)
		if err != nil {
			return util.NewInvalidArgumentErrorf("invalid CVSS vector: %s", err.Error())
		}
		in.Severity = disclosure_model.SeverityFromCVSS(vec.Severity())
	} else {
		if in.Severity == "" {
			return util.NewInvalidArgumentErrorf("severity is required when no CVSS vector is set")
		}
		if _, err := disclosure_model.ParseSeverity(string(in.Severity)); err != nil {
			return err
		}
	}
	if len(in.CveID) > CveIDMaxLength {
		return util.NewInvalidArgumentErrorf("CVE ID too long")
	}
	if in.Status != "" {
		if _, err := disclosure_model.ParseStatus(string(in.Status)); err != nil {
			return err
		}
	}
	for _, s := range in.AffectedVersions {
		if err := disclosure_model.ValidateAffectedVersion(s); err != nil {
			return err
		}
	}
	for _, s := range in.FixedVersions {
		if err := disclosure_model.ValidateFixedVersion(s); err != nil {
			return err
		}
	}
	if !in.KeepCredits {
		resolved, err := resolveCredits(ctx, in.Credits)
		if err != nil {
			return err
		}
		in.CreditsResolved = resolved
	}
	return nil
}

// resolveCredits validates raw credit entries and resolves local user names to user IDs.
// The resolved credits are stored back on the input so applyTo can persist them.
func resolveCredits(ctx context.Context, inputs []CreditInput) (disclosure_model.CreditList, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	credits := make(disclosure_model.CreditList, 0, len(inputs))
	for _, in := range inputs {
		role, err := disclosure_model.ParseCreditRole(strings.TrimSpace(in.Role))
		if err != nil {
			return nil, err
		}
		c := &disclosure_model.Credit{Role: role}
		if userName := strings.TrimSpace(in.UserName); userName != "" {
			user, err := user_model.GetUserByName(ctx, userName)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					return nil, util.NewInvalidArgumentErrorf("credit user %q does not exist", userName)
				}
				return nil, err
			}
			c.UserID = user.ID
			c.Name = user.GetDisplayName()
		} else {
			c.Name = strings.TrimSpace(in.Name)
			c.URL = strings.TrimSpace(in.URL)
		}
		credits = append(credits, c)
	}
	if err := disclosure_model.ValidateCredits(credits); err != nil {
		return nil, err
	}
	return credits, nil
}

func (in *Input) applyTo(d *disclosure_model.Disclosure) {
	d.Title = in.Title
	d.Summary = strings.TrimSpace(in.Summary)
	d.Content = in.Content
	d.Severity = in.Severity
	d.CVSSVector = strings.TrimSpace(in.CVSSVector)
	d.CweIds = disclosure_model.SanitizeVersions(in.CweIds)
	d.CveID = strings.TrimSpace(in.CveID)
	d.References = disclosure_model.SanitizeVersions(in.References)
	if !in.KeepCredits {
		d.Credits = in.CreditsResolved
	}
}

// CreateDisclosure creates a new disclosure in the repo. Only callers with maintainer
// permission should invoke this; the permission check happens in the routers.
func CreateDisclosure(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, in *Input) (*disclosure_model.Disclosure, error) {
	if err := validateInput(ctx, in); err != nil {
		return nil, err
	}
	d := &disclosure_model.Disclosure{
		RepoID:   repo.ID,
		AuthorID: doer.ID,
		Status:   disclosure_model.StatusDraft,
	}
	in.applyTo(d)
	if in.Status != "" {
		d.Status = in.Status
	}
	if d.Status == disclosure_model.StatusPublished {
		d.PublishedAt = timeutil.TimeStampNow()
		d.PublishedByID = doer.ID
	}
	versions := make([]*disclosure_model.Version, 0, len(in.AffectedVersions)+len(in.FixedVersions))
	for _, s := range disclosure_model.SanitizeVersions(in.AffectedVersions) {
		versions = append(versions, &disclosure_model.Version{Kind: disclosure_model.VersionKindAffected, Version: s})
	}
	for _, s := range disclosure_model.SanitizeVersions(in.FixedVersions) {
		versions = append(versions, &disclosure_model.Version{Kind: disclosure_model.VersionKindFixed, Version: s})
	}
	if err := disclosure_model.CreateDisclosure(ctx, d, versions); err != nil {
		return nil, err
	}
	d.Repo = repo
	d.Author = doer
	if d.PublishedByID == doer.ID {
		d.Publisher = doer
	}
	d.Versions = versions
	return d, nil
}

// UpdateDisclosure updates an existing disclosure, including its status and versions
func UpdateDisclosure(ctx context.Context, d *disclosure_model.Disclosure, doer *user_model.User, in *Input) error {
	if err := validateInput(ctx, in); err != nil {
		return err
	}
	in.applyTo(d)
	if in.Status != "" {
		if in.Status == disclosure_model.StatusPublished && d.Status != disclosure_model.StatusPublished {
			d.PublishedAt = timeutil.TimeStampNow()
			d.PublishedByID = doer.ID
		}
		if in.Status == disclosure_model.StatusWithdrawn && d.Status != disclosure_model.StatusWithdrawn {
			// withdrawing always requires a reason, which is displayed on the disclosure
			if strings.TrimSpace(in.WithdrawReason) == "" {
				return util.NewInvalidArgumentErrorf("a reason is required when withdrawing a disclosure")
			}
			d.WithdrawReason = strings.TrimSpace(in.WithdrawReason)
		} else if in.Status != disclosure_model.StatusWithdrawn {
			d.WithdrawReason = ""
		}
		d.Status = in.Status
	}
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := disclosure_model.UpdateDisclosure(ctx, d); err != nil {
			return err
		}
		return disclosure_model.SetDisclosureVersions(ctx, d.ID, in.AffectedVersions, in.FixedVersions)
	}); err != nil {
		return err
	}
	// refresh the in-memory versions, LoadAttributes would skip them otherwise
	versions, err := disclosure_model.GetVersionsByDisclosureID(ctx, d.ID)
	if err != nil {
		return err
	}
	d.Versions = versions
	return nil
}

// DeleteDisclosure deletes a disclosure of the repo. Only drafts can be deleted;
// published disclosures must be withdrawn instead.
func DeleteDisclosure(ctx context.Context, repoID, id int64) error {
	d, err := disclosure_model.GetDisclosureByRepoAndID(ctx, repoID, id)
	if err != nil {
		return err
	}
	if !d.IsDraft() {
		return util.NewInvalidArgumentErrorf("only draft disclosures can be deleted, withdraw it instead")
	}
	return disclosure_model.DeleteDisclosureByID(ctx, repoID, id)
}

// ListDisclosures lists the disclosures of the repo visible to the doer:
// drafts are only included for maintainers
func ListDisclosures(ctx context.Context, repo *repo_model.Repository, includeDrafts bool, opts disclosure_model.ListDisclosuresOptions) ([]*disclosure_model.Disclosure, int64, error) {
	opts.RepoID = repo.ID
	opts.IncludeDrafts = includeDrafts
	list, err := disclosure_model.ListDisclosures(ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	count, err := disclosure_model.CountDisclosures(ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	return list, count, nil
}
