// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package disclosure

import (
	"strings"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSeverity(t *testing.T) {
	for _, s := range []Severity{SeverityNone, SeverityLow, SeverityModerate, SeverityHigh, SeverityCritical} {
		parsed, err := ParseSeverity(string(s))
		require.NoError(t, err)
		assert.Equal(t, s, parsed)
	}
	_, err := ParseSeverity("bogus")
	assert.Error(t, err)
	assert.Equal(t, 5, SeverityCritical.Rank())
	assert.Greater(t, SeverityHigh.Rank(), SeverityLow.Rank())
}

func TestParseStatus(t *testing.T) {
	for _, s := range []Status{StatusDraft, StatusPublished, StatusWithdrawn} {
		parsed, err := ParseStatus(string(s))
		require.NoError(t, err)
		assert.Equal(t, s, parsed)
	}
	_, err := ParseStatus("bogus")
	assert.Error(t, err)
}

func TestValidateCVSSVector(t *testing.T) {
	assert.NoError(t, ValidateCVSSVector(""))
	assert.NoError(t, ValidateCVSSVector("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"))
	assert.NoError(t, ValidateCVSSVector("CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:H/SI:H/SA:H"))
	assert.NoError(t, ValidateCVSSVector("CVSS:3.0/AV:N"))
	assert.Error(t, ValidateCVSSVector("AV:N/AC:L"))
	assert.Error(t, ValidateCVSSVector("CVSS:2.0/AV:N"))
	assert.Equal(t, "3.1", CVSSVersion("CVSS:3.1/AV:N"))
	assert.Equal(t, "4.0", CVSSVersion("CVSS:4.0/AV:N"))
	assert.Equal(t, "", CVSSVersion("bogus"))
}

func TestGetDisclosureByRepoAndID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	d, err := GetDisclosureByRepoAndID(t.Context(), 1, 1)
	require.NoError(t, err)
	assert.Equal(t, "Buffer overflow in URL parser", d.Title)
	assert.Equal(t, SeverityCritical, d.Severity)
	assert.Equal(t, StatusPublished, d.Status)
	assert.Equal(t, "SD-0001", d.Identifier())
	assert.False(t, d.IsDraft())

	_, err = GetDisclosureByRepoAndID(t.Context(), 1, 3) // belongs to repo 2
	assert.True(t, IsErrDisclosureNotExist(err))
	_, err = GetDisclosureByRepoAndID(t.Context(), 2, 1)
	assert.True(t, IsErrDisclosureNotExist(err))
}

func TestDisclosureLoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	d, err := GetDisclosureByID(t.Context(), 1)
	require.NoError(t, err)
	require.NoError(t, d.LoadAttributes(t.Context()))
	assert.Equal(t, int64(1), d.Repo.ID)
	assert.Equal(t, "user2", d.Author.GetDisplayName())
	assert.Equal(t, "user2", d.Publisher.GetDisplayName())
	require.Len(t, d.Versions, 3)
	assert.Equal(t, VersionKindAffected, d.Versions[0].Kind)
	assert.Equal(t, VersionKindFixed, d.Versions[2].Kind)
	assert.Equal(t, `["CWE-120"]`, string(mustJSON(t, d.CweIds)))
}

func mustJSON(t *testing.T, l StringList) []byte {
	t.Helper()
	bs, err := l.ToDB()
	require.NoError(t, err)
	return bs
}

func TestCreateDisclosure(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	d := &Disclosure{
		RepoID:   1,
		AuthorID: 2,
		Title:    "New disclosure",
		Severity: SeverityModerate,
		Status:   StatusDraft,
	}
	versions := []*Version{
		{Kind: VersionKindAffected, Version: ">= 3.0.0, < 3.1.0"},
		{Kind: VersionKindFixed, Version: "3.1.0"},
	}
	require.NoError(t, CreateDisclosure(t.Context(), d, versions))
	assert.Equal(t, int64(3), d.Num) // fixtures have num 1 and 2 in repo 1
	assert.Equal(t, "SD-0003", d.Identifier())
	assert.Positive(t, d.ID)

	got, err := GetDisclosureByRepoAndID(t.Context(), 1, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "New disclosure", got.Title)
	versionsGot, err := GetVersionsByDisclosureID(t.Context(), d.ID)
	require.NoError(t, err)
	assert.Len(t, versionsGot, 2)
}

func TestUpdateDisclosure(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	d, err := GetDisclosureByRepoAndID(t.Context(), 1, 2)
	require.NoError(t, err)
	d.Title = "Updated title"
	d.Status = StatusPublished
	d.PublishedByID = 2
	require.NoError(t, UpdateDisclosure(t.Context(), d))

	got, err := GetDisclosureByRepoAndID(t.Context(), 1, 2)
	require.NoError(t, err)
	assert.Equal(t, "Updated title", got.Title)
	assert.Equal(t, StatusPublished, got.Status)

	assert.Error(t, UpdateDisclosure(t.Context(), &Disclosure{ID: 9999, RepoID: 1}))
}

func TestDeleteDisclosureByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	require.NoError(t, DeleteDisclosureByID(t.Context(), 1, 2))
	_, err := GetDisclosureByRepoAndID(t.Context(), 1, 2)
	assert.True(t, IsErrDisclosureNotExist(err))
	versions, err := GetVersionsByDisclosureID(t.Context(), 2)
	require.NoError(t, err)
	assert.Empty(t, versions)

	assert.True(t, IsErrDisclosureNotExist(DeleteDisclosureByID(t.Context(), 1, 2)))
	// wrong repo must not delete
	assert.True(t, IsErrDisclosureNotExist(DeleteDisclosureByID(t.Context(), 2, 1)))
}

func TestListDisclosures(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// repo 1 has 2 disclosures, drafts excluded by default
	list, err := ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "Buffer overflow in URL parser", list[0].Title)
	assert.NotNil(t, list[0].Repo)
	assert.NotNil(t, list[0].Author)
	assert.Len(t, list[0].Versions, 3)

	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, IncludeDrafts: true, ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// keyword search is case-insensitive and covers title/summary/content
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, IncludeDrafts: true, Keyword: "BUFFER", ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, IncludeDrafts: true, Keyword: "draft content", ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(2), list[0].ID)

	// severity filter
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, IncludeDrafts: true, Severities: []Severity{SeverityHigh}, ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, SeverityHigh, list[0].Severity)

	// status filter
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, Status: StatusPublished, ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// count matches list
	count, err := CountDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, IncludeDrafts: true})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestListDisclosuresVersionFilter(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// semver range match: disclosure 1 affects >= 1.0.0, < 1.4.0
	list, err := ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, Version: "1.2.0", ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(1), list[0].ID)

	// outside the range
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, Version: "1.4.0", ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Empty(t, list)

	// non-semver entry matches by exact string equality
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, Version: "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3", ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(1), list[0].ID)

	// disclosure 2 (draft) is filtered out by default even if it matches
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, Version: "2.5.0", ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	assert.Empty(t, list)
	list, err = ListDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, Version: "2.5.0", IncludeDrafts: true, ListOptions: db.ListOptionsAll})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(2), list[0].ID)

	// count respects the version filter too
	count, err := CountDisclosures(t.Context(), ListDisclosuresOptions{RepoID: 1, Version: "1.2.0"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestAffectedVersionMatches(t *testing.T) {
	assert.True(t, AffectedVersionMatches(">= 1.0.0, < 1.4.0", "1.2.0"))
	assert.False(t, AffectedVersionMatches(">= 1.0.0, < 1.4.0", "1.4.0"))
	assert.True(t, AffectedVersionMatches(">= 1.0.0, < 1.4.0", "1.2")) // go-version pads segments
	assert.True(t, AffectedVersionMatches("1.4.0", "1.4.0"))           // bare version acts as exact constraint
	assert.False(t, AffectedVersionMatches("1.4.0", "1.4.1"))
	assert.True(t, AffectedVersionMatches("a94a8fe5", "a94a8fe5")) // non-semver falls back to exact match
	assert.False(t, AffectedVersionMatches("a94a8fe5", "1.0.0"))
}

func TestValidateVersions(t *testing.T) {
	assert.NoError(t, ValidateAffectedVersion(">= 1.0.0, < 1.4.0"))
	assert.NoError(t, ValidateAffectedVersion("1.4.0"))
	assert.NoError(t, ValidateAffectedVersion("a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")) // non-semver literal is fine
	assert.Error(t, ValidateAffectedVersion(">= 1.0.0 < 1.4.0"))                           // missing comma is not a valid constraint
	assert.Error(t, ValidateAffectedVersion(""))
	assert.Error(t, ValidateAffectedVersion(strings.Repeat("a", 300)))

	assert.NoError(t, ValidateFixedVersion("1.4.0"))
	assert.NoError(t, ValidateFixedVersion("release-2026-01"))
	assert.Error(t, ValidateFixedVersion(">= 1.0.0"))
	assert.Error(t, ValidateFixedVersion(""))
}

func TestSetDisclosureVersions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	require.NoError(t, SetDisclosureVersions(t.Context(), 1,
		[]string{">= 2.0.0, < 2.5.0", "  ", ">= 2.0.0, < 2.5.0"}, // dedupe + skip empty
		[]string{"2.5.0"},
	))
	versions, err := GetVersionsByDisclosureID(t.Context(), 1)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, ">= 2.0.0, < 2.5.0", versions[0].Version)
	assert.Equal(t, VersionKindAffected, versions[0].Kind)
	assert.Equal(t, "2.5.0", versions[1].Version)
	assert.Equal(t, VersionKindFixed, versions[1].Kind)

	// invalid entries abort the whole replacement
	assert.Error(t, SetDisclosureVersions(t.Context(), 1, []string{">= broken"}, nil))
	versions, err = GetVersionsByDisclosureID(t.Context(), 1)
	require.NoError(t, err)
	assert.Len(t, versions, 2) // unchanged
}

func TestListDisclosuresOrdering(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	opts := ListDisclosuresOptions{RepoID: 1, IncludeDrafts: true, ListOptions: db.ListOptionsAll}
	opts.OrderBy = "severity"
	list, err := ListDisclosures(t.Context(), opts)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, SeverityCritical, list[0].Severity) // critical before high

	opts.OrderBy = "oldest"
	list, err = ListDisclosures(t.Context(), opts)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, int64(1), list[0].ID)
}
