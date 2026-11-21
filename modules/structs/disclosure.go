// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// SecurityDisclosureCVSS is the optional CVSS vector of a security disclosure,
// with the CVSS version encoded in the vector prefix
type SecurityDisclosureCVSS struct {
	// The CVSS version: "3.0", "3.1" or "4.0"
	Version string `json:"version"`
	// The CVSS vector string, e.g. "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
	Vector string `json:"vector"`
}

// SecurityDisclosure represents a repository security disclosure
type SecurityDisclosure struct {
	// The unique identifier of the disclosure
	ID int64 `json:"id"`
	// The short per-repository identifier, e.g. "SD-0001"
	Identifier string `json:"identifier"`
	// The title of the disclosure
	Title string `json:"title"`
	// A short plain-text summary
	Summary string `json:"summary"`
	// The full description, in markdown
	Description string `json:"description"`
	// The severity: "informative", "low", "moderate", "high" or "critical"
	Severity string `json:"severity"`
	// The optional CVSS vector
	CVSS *SecurityDisclosureCVSS `json:"cvss,omitempty"`
	// CWE identifiers, e.g. ["CWE-79"]
	CweIds []string `json:"cwe_ids"`
	// An optional reference to an existing CVE record
	CveID string `json:"cve_id,omitempty"`
	// External references (URLs)
	References []string `json:"references"`
	// Affected version ranges, e.g. [">= 1.2.3, < 1.4.0"]
	AffectedVersions []string `json:"affected_versions"`
	// Fixed versions, e.g. ["1.4.0"]
	FixedVersions []string `json:"fixed_versions"`
	// The status: "draft", "published" or "withdrawn"
	Status string `json:"status"`
	// The reason for withdrawing the disclosure, required when status is "withdrawn"
	WithdrawReason string `json:"withdraw_reason,omitempty"`
	// The user who created the disclosure
	Author *User `json:"author,omitempty"`
	// The user who published the disclosure
	PublishedBy *User `json:"published_by,omitempty"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	PublishedAt *time.Time `json:"published_at,omitempty"`
	// The API URL of the disclosure
	URL string `json:"url"`
	// The HTML URL of the disclosure
	HTMLURL string `json:"html_url"`
}

// CreateSecurityDisclosureOption options for creating a security disclosure
type CreateSecurityDisclosureOption struct {
	// required: true
	Title string `json:"title" binding:"Required"`
	// A short plain-text summary
	Summary string `json:"summary"`
	// The full description, in markdown
	Description string `json:"description"`
	// required: true
	// enum: informative,low,moderate,high,critical
	Severity string `json:"severity" binding:"Required"`
	// Optional CVSS vector, must start with CVSS:3.0/, CVSS:3.1/ or CVSS:4.0/
	CVSSVector string `json:"cvss_vector"`
	// CWE identifiers, e.g. ["CWE-79"]
	CweIds []string `json:"cwe_ids"`
	// An optional reference to an existing CVE record
	CveID string `json:"cve_id"`
	// External references (URLs)
	References []string `json:"references"`
	// Affected version ranges, e.g. [">= 1.2.3, < 1.4.0"]
	AffectedVersions []string `json:"affected_versions"`
	// Fixed versions, e.g. ["1.4.0"]
	FixedVersions []string `json:"fixed_versions"`
	// Publish immediately instead of saving as draft
	Publish bool `json:"publish"`
}

// EditSecurityDisclosureOption options for editing a security disclosure; empty fields are left unchanged
type EditSecurityDisclosureOption struct {
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	Description      string   `json:"description"`
	Severity         string   `json:"severity"` // enum: informative,low,moderate,high,critical
	CVSSVector       string   `json:"cvss_vector"`
	CweIds           []string `json:"cwe_ids"`
	CveID            string   `json:"cve_id"`
	References       []string `json:"references"`
	AffectedVersions []string `json:"affected_versions"` // null keeps, [] clears
	FixedVersions    []string `json:"fixed_versions"`    // null keeps, [] clears
	// enum: draft,published,withdrawn
	Status string `json:"status"`
	// The reason for withdrawing the disclosure, required when status is set to "withdrawn"
	WithdrawReason string `json:"withdraw_reason"`
}
