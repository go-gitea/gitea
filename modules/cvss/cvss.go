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
	Rating  string // none, low, medium, high or critical
}

// DetectVersion returns "" for unsupported vectors
func DetectVersion(vector string) Version {
	switch {
	case strings.HasPrefix(vector, "CVSS:3.1/"):
		return Version31
	case strings.HasPrefix(vector, "CVSS:4.0/"):
		return Version40
	}
	return ""
}

// Parse validates a vector and computes the base score for 3.1 and the single defined score for 4.0
func Parse(vector string) (*Result, error) {
	vector = strings.TrimSpace(vector)
	res := &Result{Version: DetectVersion(vector), Vector: vector}
	var err error
	switch res.Version {
	case Version31:
		var v *gocvss31.CVSS31
		if v, err = gocvss31.ParseVector(vector); err == nil {
			res.Score = v.BaseScore()
			res.Rating, err = gocvss31.Rating(res.Score)
		}
	case Version40:
		var v *gocvss40.CVSS40
		if v, err = gocvss40.ParseVector(vector); err == nil {
			res.Score = v.Score()
			res.Rating, err = gocvss40.Rating(res.Score)
		}
	default:
		return nil, util.NewInvalidArgumentErrorf("unsupported CVSS vector, only 3.1 and 4.0 are supported")
	}
	if err != nil {
		return nil, util.NewInvalidArgumentErrorf("invalid CVSS %s vector: %v", res.Version, err)
	}
	res.Rating = strings.ToLower(res.Rating)
	return res, nil
}

// Metric is a base metric, used to render the calculator
type Metric struct {
	Key    string
	Values []string
}

var baseMetrics = map[Version][]Metric{
	Version31: {
		{Key: "AV", Values: []string{"N", "A", "L", "P"}},
		{Key: "AC", Values: []string{"L", "H"}},
		{Key: "PR", Values: []string{"N", "L", "H"}},
		{Key: "UI", Values: []string{"N", "R"}},
		{Key: "S", Values: []string{"U", "C"}},
		{Key: "C", Values: []string{"N", "L", "H"}},
		{Key: "I", Values: []string{"N", "L", "H"}},
		{Key: "A", Values: []string{"N", "L", "H"}},
	},
	Version40: {
		{Key: "AV", Values: []string{"N", "A", "L", "P"}},
		{Key: "AC", Values: []string{"L", "H"}},
		{Key: "AT", Values: []string{"N", "P"}},
		{Key: "PR", Values: []string{"N", "L", "H"}},
		{Key: "UI", Values: []string{"N", "P", "A"}},
		{Key: "VC", Values: []string{"N", "L", "H"}},
		{Key: "VI", Values: []string{"N", "L", "H"}},
		{Key: "VA", Values: []string{"N", "L", "H"}},
		{Key: "SC", Values: []string{"N", "L", "H"}},
		{Key: "SI", Values: []string{"N", "L", "H"}},
		{Key: "SA", Values: []string{"N", "L", "H"}},
	},
}

// BaseMetrics returns the base metrics in vector order
func BaseMetrics(version Version) []Metric {
	return baseMetrics[version]
}
