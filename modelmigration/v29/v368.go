// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"slices"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
)

type SecurityAdvisory struct {
	ID                int64              `xorm:"pk autoincr"`
	RepoID            int64              `xorm:"UNIQUE(s) NOT NULL"`
	Identifier        string             `xorm:"VARCHAR(14) UNIQUE(s) NOT NULL"`
	Index             int64              `xorm:"INDEX NOT NULL DEFAULT 0"`
	CveID             string             `xorm:"VARCHAR(32)"`
	Summary           string             `xorm:"VARCHAR(1024) NOT NULL"`
	Description       string             `xorm:"LONGTEXT"`
	ReportDescription string             `xorm:"LONGTEXT"`
	Severity          int                `xorm:"NOT NULL DEFAULT 0"`
	CvssV3Vector      string             `xorm:"VARCHAR(255)"`
	CvssV3ScoreTenths int                `xorm:"NOT NULL DEFAULT 0"`
	CvssV4Vector      string             `xorm:"VARCHAR(255)"`
	CvssV4ScoreTenths int                `xorm:"NOT NULL DEFAULT 0"`
	ContentVersion    int                `xorm:"NOT NULL DEFAULT 0"`
	CweIDs            []string           `xorm:"'cwe_ids' JSON TEXT"`
	State             int                `xorm:"INDEX NOT NULL"`
	IsReport          bool               `xorm:"NOT NULL DEFAULT false"`
	ReporterID        int64              `xorm:"INDEX NOT NULL"`
	PublisherID       int64              `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix       timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"INDEX updated"`
	PublishedUnix     timeutil.TimeStamp `xorm:"INDEX NOT NULL DEFAULT 0"`
	ClosedUnix        timeutil.TimeStamp `xorm:"NOT NULL DEFAULT 0"`
	WithdrawnUnix     timeutil.TimeStamp `xorm:"NOT NULL DEFAULT 0"`
	CloseReason       int                `xorm:"NOT NULL DEFAULT 0"`
	DuplicateOfID     int64              `xorm:"NOT NULL DEFAULT 0"`
}

type SecurityAdvisoryIndex struct {
	GroupID  int64 `xorm:"pk"`
	MaxIndex int64 `xorm:"index"`
}

type SecurityAdvisoryVulnerability struct {
	ID                     int64    `xorm:"pk autoincr"`
	AdvisoryID             int64    `xorm:"INDEX NOT NULL"`
	Ecosystem              string   `xorm:"VARCHAR(32) NOT NULL"`
	PackageName            string   `xorm:"VARCHAR(255)"`
	VulnerableVersionRange string   `xorm:"VARCHAR(255)"`
	PatchedVersions        string   `xorm:"VARCHAR(255)"`
	VulnerableFunctions    []string `xorm:"JSON LONGTEXT"`
}

type SecurityAdvisoryCredit struct {
	ID         int64  `xorm:"pk autoincr"`
	AdvisoryID int64  `xorm:"UNIQUE(s) NOT NULL"`
	UserID     int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Type       string `xorm:"VARCHAR(32) NOT NULL"`
}

type SecurityAdvisoryCollaborator struct {
	ID         int64 `xorm:"pk autoincr"`
	AdvisoryID int64 `xorm:"UNIQUE(s) NOT NULL"`
	UserID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	TeamID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	ReadOnly   bool  `xorm:"NOT NULL DEFAULT false"`
}

type SecurityAdvisoryLabel struct {
	ID         int64 `xorm:"pk autoincr"`
	AdvisoryID int64 `xorm:"UNIQUE(s) NOT NULL"`
	LabelID    int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
}

type SecurityAdvisoryComment struct {
	ID          int64              `xorm:"pk autoincr"`
	AdvisoryID  int64              `xorm:"INDEX NOT NULL"`
	PosterID    int64              `xorm:"INDEX NOT NULL"`
	Content     string             `xorm:"LONGTEXT"`
	IsInternal  bool               `xorm:"NOT NULL DEFAULT false"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

type Team struct {
	IsSecurityTeam bool `xorm:"NOT NULL DEFAULT false"`
}

// AddSecurityAdvisoryTables adds the advisory tables and enables the advisories unit for existing repositories and teams
func AddSecurityAdvisoryTables(_ context.Context, x base.EngineMigration) error {
	if err := x.Sync(new(SecurityAdvisory), new(SecurityAdvisoryIndex), new(SecurityAdvisoryVulnerability), new(SecurityAdvisoryCredit),
		new(SecurityAdvisoryCollaborator), new(SecurityAdvisoryLabel), new(SecurityAdvisoryComment), new(Team)); err != nil {
		return err
	}
	defaultUnits := setting.Repository.DefaultRepoUnits
	if len(defaultUnits) > 0 && !slices.Contains(defaultUnits, "repo.security_advisories") {
		return nil // the instance doesn't want the unit by default
	}
	if err := enableAdvisoriesUnitForRepos(x); err != nil {
		return err
	}
	return grantAdvisoriesReadToCodeTeams(x)
}

func enableAdvisoriesUnitForRepos(x base.EngineMigration) error {
	const typeSecurityAdvisories = 11
	_, err := x.Exec(`INSERT INTO repo_unit (repo_id, type, config, created_unix, anonymous_access_mode, everyone_access_mode)
SELECT id, ?, '{}', ?, 0, 0 FROM repository
WHERE is_fork = ? AND is_mirror = ? AND NOT EXISTS (SELECT 1 FROM repo_unit ru WHERE ru.repo_id = repository.id AND ru.type = ?)`,
		typeSecurityAdvisories, timeutil.TimeStampNow(), false, false, typeSecurityAdvisories)
	return err
}

// grantAdvisoriesReadToCodeTeams only changes teams with per-unit permissions, a general permission already covers new units
func grantAdvisoriesReadToCodeTeams(x base.EngineMigration) error {
	const (
		typeCode               = 1
		typeSecurityAdvisories = 11
		accessModeRead         = 1
	)
	_, err := x.Exec(`INSERT INTO team_unit (org_id, team_id, type, access_mode)
SELECT org_id, id, ?, ? FROM team
WHERE authorize = 0
AND EXISTS (SELECT 1 FROM team_unit tu WHERE tu.team_id = team.id AND tu.type = ? AND tu.access_mode > 0)
AND NOT EXISTS (SELECT 1 FROM team_unit tu WHERE tu.team_id = team.id AND tu.type = ?)`,
		typeSecurityAdvisories, accessModeRead, typeCode, typeSecurityAdvisories)
	return err
}
