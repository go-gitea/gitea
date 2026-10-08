// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// RepositoryAdvisory represents a security advisory of a repository
type RepositoryAdvisory struct {
	// The identifier of the advisory, unique within its repository
	Identifier string `json:"identifier"`
	// The CVE ID assigned to the advisory
	CveID string `json:"cve_id"`
	// The API URL of the advisory
	URL string `json:"url"`
	// The HTML URL to view the advisory
	HTMLURL string `json:"html_url"`
	// A short summary of the advisory
	Summary string `json:"summary"`
	// A detailed description of the vulnerability in Markdown
	Description string `json:"description"`
	// The severity of the advisory
	// enum: low,medium,high,critical
	Severity *string `json:"severity"`
	// The user who reported or created the advisory, only visible to repository admins and participants
	Author *User `json:"author"`
	// The user who published the advisory
	Publisher *User `json:"publisher"`
	// The identifiers of the advisory like its CVE ID
	Identifiers []*RepositoryAdvisoryIdentifier `json:"identifiers"`
	// The state of the advisory
	// enum: triage,draft,published,closed,withdrawn
	State string `json:"state"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	PublishedAt *time.Time `json:"published_at"`
	// swagger:strfmt date-time
	ClosedAt *time.Time `json:"closed_at"`
	// swagger:strfmt date-time
	WithdrawnAt *time.Time `json:"withdrawn_at"`
	// Whether the advisory was reported privately and if the report was accepted
	Submission *RepositoryAdvisorySubmission `json:"submission"`
	// The packages affected by the vulnerability
	Vulnerabilities []*RepositoryAdvisoryVulnerability `json:"vulnerabilities"`
	// The CVSS 3.1 score if set
	CVSS *RepositoryAdvisoryCVSS `json:"cvss"`
	// The CVSS scores of the advisory
	CVSSSeverities *RepositoryAdvisoryCVSSSeverities `json:"cvss_severities"`
	// The CWE IDs of the advisory
	CweIDs []string `json:"cwe_ids"`
	// The users credited for the advisory
	Credits []*RepositoryAdvisoryCredit `json:"credits"`
	// The users credited for the advisory with their details
	CreditsDetailed []*RepositoryAdvisoryCreditDetailed `json:"credits_detailed"`
	// The users collaborating on the advisory, only visible to repository admins and participants
	CollaboratingUsers []*User `json:"collaborating_users"`
	// The teams collaborating on the advisory, only visible to repository admins and participants
	CollaboratingTeams []*Team `json:"collaborating_teams"`
	// The labels of the advisory
	Labels []*Label `json:"labels"`
	// Why the advisory was closed
	// enum: invalid,duplicate
	CloseReason string `json:"close_reason,omitempty"`
	// The identifier of the original advisory if this one was closed as its duplicate
	DuplicateOf string `json:"duplicate_of,omitempty"`
}

// RepositoryAdvisoryIdentifier is an identifier of an advisory
type RepositoryAdvisoryIdentifier struct {
	// enum: CVE
	Type  string `json:"type"`
	Value string `json:"value"`
}

// RepositoryAdvisorySubmission describes a private vulnerability report
type RepositoryAdvisorySubmission struct {
	// Whether the maintainers accepted the report
	Accepted bool `json:"accepted"`
}

// RepositoryAdvisoryPackage is a package of an ecosystem
type RepositoryAdvisoryPackage struct {
	// The package ecosystem
	// enum: actions,composer,erlang,go,maven,npm,nuget,other,pip,pub,rubygems,rust,swift
	Ecosystem string `json:"ecosystem" binding:"Required"`
	// The package name
	Name string `json:"name"`
}

// RepositoryAdvisoryVulnerability is a package affected by an advisory
type RepositoryAdvisoryVulnerability struct {
	Package *RepositoryAdvisoryPackage `json:"package" binding:"Required"`
	// The range of affected versions, e.g. "< 1.2.3"
	VulnerableVersionRange string `json:"vulnerable_version_range"`
	// The versions containing the fix, e.g. "1.2.3"
	PatchedVersions string `json:"patched_versions"`
	// The affected functions
	VulnerableFunctions []string `json:"vulnerable_functions"`
}

// RepositoryAdvisoryCVSS is a CVSS vector with its score
type RepositoryAdvisoryCVSS struct {
	VectorString string  `json:"vector_string"`
	Score        float64 `json:"score"`
}

// RepositoryAdvisoryCVSSSeverities are the CVSS 3.1 and 4.0 scores of an advisory
type RepositoryAdvisoryCVSSSeverities struct {
	CvssV3 *RepositoryAdvisoryCVSS `json:"cvss_v3"`
	CvssV4 *RepositoryAdvisoryCVSS `json:"cvss_v4"`
}

// RepositoryAdvisoryCredit credits a user for an advisory
type RepositoryAdvisoryCredit struct {
	// The login of the credited user
	Login string `json:"login" binding:"Required"`
	// The type of the contribution
	// enum: analyst,finder,reporter,coordinator,remediation_developer,remediation_reviewer,remediation_verifier,tool,sponsor,other
	Type string `json:"type" binding:"Required"`
}

// RepositoryAdvisoryCreditDetailed credits a user for an advisory
type RepositoryAdvisoryCreditDetailed struct {
	User *User `json:"user"`
	// enum: analyst,finder,reporter,coordinator,remediation_developer,remediation_reviewer,remediation_verifier,tool,sponsor,other
	Type string `json:"type"`
	// enum: accepted
	State string `json:"state"`
}

// CreateRepositoryAdvisoryOption options for creating a draft advisory
type CreateRepositoryAdvisoryOption struct {
	// required: true
	Summary string `json:"summary" binding:"Required"`
	// required: true
	Description string `json:"description" binding:"Required"`
	CveID       string `json:"cve_id"`
	// The packages affected by the vulnerability
	Vulnerabilities []*RepositoryAdvisoryVulnerability `json:"vulnerabilities"`
	CweIDs          []string                           `json:"cwe_ids"`
	Credits         []*RepositoryAdvisoryCredit        `json:"credits"`
	// The IDs of the repository or organization labels
	Labels []int64 `json:"labels"`
	// The severity, cannot be set together with cvss_vector_string
	// enum: low,medium,high,critical
	Severity string `json:"severity"`
	// A CVSS 3.1 or 4.0 vector, the severity is derived from it
	CVSSVectorString string `json:"cvss_vector_string"`
}

// CreatePrivateVulnerabilityReportOption options for reporting a vulnerability privately
type CreatePrivateVulnerabilityReportOption struct {
	// required: true
	Summary string `json:"summary" binding:"Required"`
	// required: true
	Description string `json:"description" binding:"Required"`
	// The packages affected by the vulnerability
	Vulnerabilities []*RepositoryAdvisoryVulnerability `json:"vulnerabilities"`
	CweIDs          []string                           `json:"cwe_ids"`
	// The severity, cannot be set together with cvss_vector_string
	// enum: low,medium,high,critical
	Severity string `json:"severity"`
	// A CVSS 3.1 or 4.0 vector, the severity is derived from it
	CVSSVectorString string `json:"cvss_vector_string"`
}

// EditRepositoryAdvisoryOption options for updating an advisory, only set fields are changed
type EditRepositoryAdvisoryOption struct {
	Summary     *string `json:"summary"`
	Description *string `json:"description"`
	CveID       *string `json:"cve_id"`
	// The packages affected by the vulnerability, replaces all existing ones
	Vulnerabilities []*RepositoryAdvisoryVulnerability `json:"vulnerabilities"`
	CweIDs          []string                           `json:"cwe_ids"`
	// Replaces all existing credits, only repository admins can change them
	Credits []*RepositoryAdvisoryCredit `json:"credits"`
	// Replaces all labels with the repository or organization labels of these IDs, only repository admins can change them
	Labels []int64 `json:"labels"`
	// The severity, cannot be set together with cvss_vector_string
	// enum: low,medium,high,critical
	Severity *string `json:"severity"`
	// A CVSS 3.1 or 4.0 vector, the severity is derived from it. An empty string removes the vectors.
	CVSSVectorString *string `json:"cvss_vector_string"`
	// The new state, only repository admins can change it
	// enum: draft,published,closed,withdrawn
	State *string `json:"state"`
	// Required when closing the advisory
	// enum: invalid,duplicate
	CloseReason *string `json:"close_reason"`
	// The identifier of the original advisory when closing as duplicate, its reporter gets access to the original
	DuplicateOf *string `json:"duplicate_of"`
	// The logins of the users collaborating on the advisory, replaces all existing ones, only repository admins can change them
	CollaboratingUsers []string `json:"collaborating_users"`
	// The names of the teams collaborating on the advisory, replaces all existing ones, only repository admins can change them
	CollaboratingTeams []string `json:"collaborating_teams"`
}

// PrivateVulnerabilityReporting describes whether private vulnerability reporting is enabled
type PrivateVulnerabilityReporting struct {
	Enabled bool `json:"enabled"`
}

// RepositoryAdvisorySeverity is the severity of an advisory
//
// swagger:enum RepositoryAdvisorySeverity
type RepositoryAdvisorySeverity string

const (
	RepositoryAdvisorySeverityLow      RepositoryAdvisorySeverity = "low"
	RepositoryAdvisorySeverityMedium   RepositoryAdvisorySeverity = "medium"
	RepositoryAdvisorySeverityHigh     RepositoryAdvisorySeverity = "high"
	RepositoryAdvisorySeverityCritical RepositoryAdvisorySeverity = "critical"
)

// RepositoryAdvisoryCreditType is the kind of contribution of a credited user
//
// swagger:enum RepositoryAdvisoryCreditType
type RepositoryAdvisoryCreditType string

const (
	RepositoryAdvisoryCreditTypeAnalyst              RepositoryAdvisoryCreditType = "analyst"
	RepositoryAdvisoryCreditTypeFinder               RepositoryAdvisoryCreditType = "finder"
	RepositoryAdvisoryCreditTypeReporter             RepositoryAdvisoryCreditType = "reporter"
	RepositoryAdvisoryCreditTypeCoordinator          RepositoryAdvisoryCreditType = "coordinator"
	RepositoryAdvisoryCreditTypeRemediationDeveloper RepositoryAdvisoryCreditType = "remediation_developer"
	RepositoryAdvisoryCreditTypeRemediationReviewer  RepositoryAdvisoryCreditType = "remediation_reviewer"
	RepositoryAdvisoryCreditTypeRemediationVerifier  RepositoryAdvisoryCreditType = "remediation_verifier"
	RepositoryAdvisoryCreditTypeTool                 RepositoryAdvisoryCreditType = "tool"
	RepositoryAdvisoryCreditTypeSponsor              RepositoryAdvisoryCreditType = "sponsor"
	RepositoryAdvisoryCreditTypeOther                RepositoryAdvisoryCreditType = "other"
)

// RepositoryAdvisoryComment is a comment of the private discussion of an advisory
type RepositoryAdvisoryComment struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"html_url"`
	User    *User  `json:"user"`
	Body    string `json:"body"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	UpdatedAt time.Time `json:"updated_at"`
}

// RepositoryAdvisoryCommentOption options for creating or editing an advisory comment
type RepositoryAdvisoryCommentOption struct {
	// required: true
	Body string `json:"body" binding:"Required"`
}

// RepositoryAdvisoryCloseReason tells wrong reports and duplicates apart
//
// swagger:enum RepositoryAdvisoryCloseReason
type RepositoryAdvisoryCloseReason string

const (
	RepositoryAdvisoryCloseReasonInvalid   RepositoryAdvisoryCloseReason = "invalid"
	RepositoryAdvisoryCloseReasonDuplicate RepositoryAdvisoryCloseReason = "duplicate"
)
