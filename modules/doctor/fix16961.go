// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// #16831 revealed that the dump command that was broken in 1.14.3-1.14.6 and 1.15.0 (#15885).
// This led to repo_unit and login_source cfg not being converted to JSON in the dump
// Unfortunately although it was hoped that there were only a few users affected it
// appears that many users are affected.

// We therefore need to provide a doctor command to fix this repeated issue #16961

func parseBool16961(bs []byte) (bool, error) {
	if bytes.EqualFold(bs, []byte("%!s(bool=false)")) {
		return false, nil
	}

	if bytes.EqualFold(bs, []byte("%!s(bool=true)")) {
		return true, nil
	}

	return false, fmt.Errorf("unexpected bool format: %s", string(bs))
}

func fixUnitConfig16961(bs []byte, cfg *repo_model.UnitConfig) (fixed bool, err error) {
	err = json.UnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return false, nil
	}

	// Handle #16961
	if string(bs) != "&{}" && len(bs) != 0 {
		return false, err
	}

	return true, nil
}

func fixExternalWikiConfig16961(bs []byte, cfg *repo_model.ExternalWikiConfig) (fixed bool, err error) {
	err = json.UnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return false, nil
	}

	if len(bs) < 3 {
		return false, err
	}
	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return false, err
	}
	cfg.ExternalWikiURL = string(bs[2 : len(bs)-1])
	return true, nil
}

func fixExternalTrackerConfig16961(bs []byte, cfg *repo_model.ExternalTrackerConfig) (fixed bool, err error) {
	err = json.UnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return false, nil
	}
	// Handle #16961
	if len(bs) < 3 {
		return false, err
	}

	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return false, err
	}

	parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
	if len(parts) != 3 {
		return false, err
	}

	cfg.ExternalTrackerURL = string(bytes.Join(parts[:len(parts)-2], []byte{' '}))
	cfg.ExternalTrackerFormat = string(parts[len(parts)-2])
	cfg.ExternalTrackerStyle = string(parts[len(parts)-1])
	return true, nil
}

func fixPullRequestsConfig16961(bs []byte, cfg *repo_model.PullRequestsConfig) (fixed bool, err error) {
	err = json.UnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return false, nil
	}

	// Handle #16961
	if len(bs) < 3 {
		return false, err
	}

	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return false, err
	}

	// PullRequestsConfig was the following in 1.14
	// type PullRequestsConfig struct {
	// 	IgnoreWhitespaceConflicts bool
	// 	AllowMerge                bool
	// 	AllowRebase               bool
	// 	AllowRebaseMerge          bool
	// 	AllowSquash               bool
	// 	AllowManualMerge          bool
	// 	AutodetectManualMerge     bool
	// }
	//
	// 1.15 added in addition:
	// DefaultDeleteBranchAfterMerge bool
	// DefaultMergeStyle             MergeStyle
	parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
	if len(parts) < 7 {
		return false, err
	}

	var parseErr error
	cfg.IgnoreWhitespaceConflicts, parseErr = parseBool16961(parts[0])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.AllowMerge, parseErr = parseBool16961(parts[1])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.AllowRebase, parseErr = parseBool16961(parts[2])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.AllowRebaseMerge, parseErr = parseBool16961(parts[3])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.AllowSquash, parseErr = parseBool16961(parts[4])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.AllowManualMerge, parseErr = parseBool16961(parts[5])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.AutodetectManualMerge, parseErr = parseBool16961(parts[6])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}

	// 1.14 unit
	if len(parts) == 7 {
		return true, nil
	}

	if len(parts) < 9 {
		return false, err
	}

	cfg.DefaultDeleteBranchAfterMerge, parseErr = parseBool16961(parts[7])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}

	cfg.DefaultMergeStyle = repo_model.MergeStyle(string(bytes.Join(parts[8:], []byte{' '})))
	return true, nil
}

func fixIssuesConfig16961(bs []byte, cfg *repo_model.IssuesConfig) (fixed bool, err error) {
	err = json.UnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return false, nil
	}

	// Handle #16961
	if len(bs) < 3 {
		return false, err
	}

	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return false, err
	}

	parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
	if len(parts) != 3 {
		return false, err
	}
	var parseErr error
	cfg.EnableTimetracker, parseErr = parseBool16961(parts[0])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.AllowOnlyContributorsToTrackTime, parseErr = parseBool16961(parts[1])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	cfg.EnableDependencies, parseErr = parseBool16961(parts[2])
	if parseErr != nil {
		return false, errors.Join(err, parseErr)
	}
	return true, nil
}

func fixBrokenRepoUnit16961(repoUnit *repo_model.RepoUnit, bs []byte) (fixed bool, err error) {
	// Shortcut empty or null values
	if len(bs) == 0 {
		return false, nil
	}

	switch repoUnit.Type {
	case unit.TypeCode, unit.TypeReleases, unit.TypeWiki, unit.TypeProjects:
		cfg := &repo_model.UnitConfig{}
		repoUnit.Config = cfg
		if fixed, err := fixUnitConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypeExternalWiki:
		cfg := &repo_model.ExternalWikiConfig{}
		repoUnit.Config = cfg

		if fixed, err := fixExternalWikiConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypeExternalTracker:
		cfg := &repo_model.ExternalTrackerConfig{}
		repoUnit.Config = cfg
		if fixed, err := fixExternalTrackerConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypePullRequests:
		cfg := &repo_model.PullRequestsConfig{}
		repoUnit.Config = cfg

		if fixed, err := fixPullRequestsConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypeIssues:
		cfg := &repo_model.IssuesConfig{}
		repoUnit.Config = cfg
		if fixed, err := fixIssuesConfig16961(bs, cfg); !fixed {
			return false, err
		}
	default:
		panic(fmt.Sprintf("unrecognized repo unit type: %v", repoUnit.Type))
	}
	return true, nil
}

func fixBrokenRepoUnits16961(ctx context.Context, logger log.Logger, autofix bool) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64
		Type        unit.Type
		Config      []byte
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
	}

	count := 0

	err := db.Iterate(
		ctx,
		builder.Gt{
			"id": 0,
		},
		func(ctx context.Context, unit *RepoUnit) error {
			bs := unit.Config
			repoUnit := &repo_model.RepoUnit{
				ID:          unit.ID,
				RepoID:      unit.RepoID,
				Type:        unit.Type,
				CreatedUnix: unit.CreatedUnix,
			}

			if fixed, err := fixBrokenRepoUnit16961(repoUnit, bs); !fixed {
				return err
			}

			count++
			if !autofix {
				return nil
			}

			return repo_model.UpdateRepoUnit(ctx, repoUnit)
		},
	)
	if err != nil {
		logger.Critical("Unable to iterate across repounits to fix the broken units: Error %v", err)
		return err
	}

	if !autofix {
		if count == 0 {
			logger.Info("Found no broken repo_units")
		} else {
			logger.Warn("Found %d broken repo_units", count)
		}
		return nil
	}
	logger.Info("Fixed %d broken repo_units", count)

	return nil
}

func init() {
	Register(&Check{
		Title:     "Check for incorrectly dumped repo_units (See #16961)",
		Name:      "fix-broken-repo-units",
		IsDefault: false,
		Run:       fixBrokenRepoUnits16961,
		Priority:  7,
	})
}
