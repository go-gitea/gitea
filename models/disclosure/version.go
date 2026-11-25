// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package disclosure

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/util"

	"github.com/hashicorp/go-version"
	"xorm.io/builder"
)

// VersionKind describes whether a version entry is an affected range or a fixed version
type VersionKind string

// Enumerate all version kinds
const (
	VersionKindAffected VersionKind = "affected"
	VersionKindFixed    VersionKind = "fixed"
)

// MaxVersionLength is the maximum length of a version entry
const MaxVersionLength = 255

// Version is a single version entry of a disclosure: either an affected
// version range (e.g. ">= 1.2.3, < 1.4.0") or a fixed version (e.g. "1.4.0").
// The raw text is the source of truth; it is parsed with semver semantics when possible.
type Version struct {
	ID           int64       `xorm:"pk autoincr"`
	DisclosureID int64       `xorm:"INDEX UNIQUE(dv)"`
	Kind         VersionKind `xorm:"VARCHAR(16) UNIQUE(dv)"`
	Version      string      `xorm:"VARCHAR(255) UNIQUE(dv)"`
}

func init() {
	db.RegisterModel(new(Version))
}

// TableName keeps the table name stable regardless of the type name
func (Version) TableName() string {
	return "disclosure_version"
}

// VersionsOfKind returns the version texts of the given kind, in insertion order
func (d *Disclosure) VersionsOfKind(kind VersionKind) []string {
	out := make([]string, 0, len(d.Versions))
	for _, v := range d.Versions {
		if v.Kind == kind {
			out = append(out, v.Version)
		}
	}
	return out
}

// AffectedVersions returns the affected version range texts of the disclosure
func (d *Disclosure) AffectedVersions() []string {
	return d.VersionsOfKind(VersionKindAffected)
}

// FixedVersions returns the fixed version texts of the disclosure
func (d *Disclosure) FixedVersions() []string {
	return d.VersionsOfKind(VersionKindFixed)
}

// GetVersionsByDisclosureID returns the version entries of a disclosure, ordered by ID
func GetVersionsByDisclosureID(ctx context.Context, disclosureID int64) ([]*Version, error) {
	versions := make([]*Version, 0, 4)
	return versions, db.GetEngine(ctx).Where(builder.Eq{"disclosure_id": disclosureID}).OrderBy("id ASC").Find(&versions)
}

// GetVersionsByDisclosureIDs returns the version entries of the given disclosures, grouped by disclosure ID
func GetVersionsByDisclosureIDs(ctx context.Context, disclosureIDs []int64) (map[int64][]*Version, error) {
	result := make(map[int64][]*Version, len(disclosureIDs))
	if len(disclosureIDs) == 0 {
		return result, nil
	}
	versions := make([]*Version, 0, len(disclosureIDs)*2)
	err := db.GetEngine(ctx).Where(builder.In("disclosure_id", disclosureIDs)).OrderBy("id ASC").Find(&versions)
	if err != nil {
		return nil, err
	}
	for _, v := range versions {
		result[v.DisclosureID] = append(result[v.DisclosureID], v)
	}
	return result, nil
}

// rangeOperatorChars mark a string as an intended semver range rather than a literal version
const rangeOperatorChars = "<>=~^,"

// validateVersionLength checks the common length rules of a version entry
func validateVersionLength(s, kind string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return util.NewInvalidArgumentErrorf("%s version is empty", kind)
	}
	if len(s) > MaxVersionLength {
		return util.NewInvalidArgumentErrorf("%s version too long", kind)
	}
	return nil
}

// ValidateAffectedVersion checks an affected version entry. Entries containing constraint
// operators must be valid semver constraints; anything else is kept as a literal string
// (exact match only), so non-semver versions like tags or commit SHAs are supported.
func ValidateAffectedVersion(s string) error {
	if err := validateVersionLength(s, "affected"); err != nil {
		return err
	}
	if strings.ContainsAny(s, rangeOperatorChars) {
		if _, err := version.NewConstraint(s); err != nil {
			return util.NewInvalidArgumentErrorf("invalid affected version range %q", s)
		}
	}
	return nil
}

// ValidateFixedVersion checks a fixed version entry: it must be a single version, not a range
func ValidateFixedVersion(s string) error {
	if err := validateVersionLength(s, "fixed"); err != nil {
		return err
	}
	if strings.ContainsAny(s, rangeOperatorChars) {
		return util.NewInvalidArgumentErrorf("fixed version %q must be an exact version, not a range", s)
	}
	return nil
}

// AffectedVersionMatches reports whether an affected version entry matches the given version.
// If the entry parses as a semver constraint and the version parses as a version, the
// constraint is evaluated; otherwise the entry is compared by exact string equality.
func AffectedVersionMatches(entry, v string) bool {
	if c, err := version.NewConstraint(entry); err == nil {
		if ver, err := version.NewVersion(v); err == nil {
			return c.Check(ver)
		}
	}
	return entry == v
}

// SanitizeVersions trims, dedupes and drops empty entries
func SanitizeVersions(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// SetDisclosureVersions replaces the version entries of a disclosure in a transaction
func SetDisclosureVersions(ctx context.Context, disclosureID int64, affected, fixed []string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where(builder.Eq{"disclosure_id": disclosureID}).Delete(new(Version)); err != nil {
			return err
		}
		setVersions := func(kind VersionKind, entries []string, validate func(string) error) error {
			for _, s := range SanitizeVersions(entries) {
				if err := validate(s); err != nil {
					return err
				}
				if err := db.Insert(ctx, &Version{DisclosureID: disclosureID, Kind: kind, Version: s}); err != nil {
					return err
				}
			}
			return nil
		}
		if err := setVersions(VersionKindAffected, affected, ValidateAffectedVersion); err != nil {
			return err
		}
		return setVersions(VersionKindFixed, fixed, ValidateFixedVersion)
	})
}
