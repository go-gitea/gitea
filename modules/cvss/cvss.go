// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cvss

import (
	"strings"

	"gitea.dev/modules/util"

	gocvss31 "github.com/pandatix/go-cvss/31"
	gocvss40 "github.com/pandatix/go-cvss/40"
)

type Version string

const (
	Version31 Version = "3.1"
	Version40 Version = "4.0"
)

type Result struct {
	Version Version
	Vector  string
	Score   float64
	Rating  string
}

func DetectVersion(vector string) Version {
	switch {
	case strings.HasPrefix(vector, "CVSS:3.1/"):
		return Version31
	case strings.HasPrefix(vector, "CVSS:4.0/"):
		return Version40
	}
	return ""
}

// Parse uses the base score for 3.1, which defines several scores.
func Parse(vector string) (*Result, error) {
	vector = strings.TrimSpace(vector)
	result := &Result{Version: DetectVersion(vector), Vector: vector}
	var err error
	switch result.Version {
	case Version31:
		var parsed *gocvss31.CVSS31
		if parsed, err = gocvss31.ParseVector(vector); err == nil {
			result.Score = parsed.BaseScore()
			result.Rating, err = gocvss31.Rating(result.Score)
		}
	case Version40:
		var parsed *gocvss40.CVSS40
		if parsed, err = gocvss40.ParseVector(vector); err == nil {
			result.Score = parsed.Score()
			result.Rating, err = gocvss40.Rating(result.Score)
		}
	default:
		return nil, util.NewInvalidArgumentErrorf("unsupported CVSS vector, only 3.1 and 4.0 are supported")
	}
	if err != nil {
		return nil, util.NewInvalidArgumentErrorf("invalid CVSS %s vector: %v", result.Version, err)
	}
	result.Rating = strings.ToLower(result.Rating)
	return result, nil
}

type Metric struct {
	Key    string
	Name   string
	Values []MetricValue
}

type MetricValue struct {
	Key  string
	Name string
}

var (
	noneLowHigh        = []MetricValue{{"N", "None"}, {"L", "Low"}, {"H", "High"}}
	attackVector       = Metric{Key: "AV", Name: "Attack Vector", Values: []MetricValue{{"N", "Network"}, {"A", "Adjacent"}, {"L", "Local"}, {"P", "Physical"}}}
	attackComplexity   = Metric{Key: "AC", Name: "Attack Complexity", Values: []MetricValue{{"L", "Low"}, {"H", "High"}}}
	privilegesRequired = Metric{Key: "PR", Name: "Privileges Required", Values: noneLowHigh}
)

var baseMetrics = map[Version][]Metric{
	Version31: {
		attackVector,
		attackComplexity,
		privilegesRequired,
		{Key: "UI", Name: "User Interaction", Values: []MetricValue{{"N", "None"}, {"R", "Required"}}},
		{Key: "S", Name: "Scope", Values: []MetricValue{{"U", "Unchanged"}, {"C", "Changed"}}},
		{Key: "C", Name: "Confidentiality", Values: noneLowHigh},
		{Key: "I", Name: "Integrity", Values: noneLowHigh},
		{Key: "A", Name: "Availability", Values: noneLowHigh},
	},
	Version40: {
		attackVector,
		attackComplexity,
		{Key: "AT", Name: "Attack Requirements", Values: []MetricValue{{"N", "None"}, {"P", "Present"}}},
		privilegesRequired,
		{Key: "UI", Name: "User Interaction", Values: []MetricValue{{"N", "None"}, {"P", "Passive"}, {"A", "Active"}}},
		{Key: "VC", Name: "Vulnerable System Confidentiality", Values: noneLowHigh},
		{Key: "VI", Name: "Vulnerable System Integrity", Values: noneLowHigh},
		{Key: "VA", Name: "Vulnerable System Availability", Values: noneLowHigh},
		{Key: "SC", Name: "Subsequent System Confidentiality", Values: noneLowHigh},
		{Key: "SI", Name: "Subsequent System Integrity", Values: noneLowHigh},
		{Key: "SA", Name: "Subsequent System Availability", Values: noneLowHigh},
	},
}

// BaseMetrics returns the base metrics in vector order
func BaseMetrics(version Version) []Metric {
	return baseMetrics[version]
}
