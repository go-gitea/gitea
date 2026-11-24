// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package cvss implements parsing and base score calculation for CVSS
// vector strings (versions 3.0, 3.1 and 4.0), following the FIRST
// specifications. It is used to derive the severity of security disclosures.
package cvss

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Severity is the qualitative severity rating of a CVSS base score
type Severity string

// Qualitative severity ratings, per the CVSS specifications
const (
	SeverityNone     Severity = "none"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// SeverityFromScore maps a base score to its qualitative severity rating
func SeverityFromScore(score float64) Severity {
	switch {
	case score == 0:
		return SeverityNone
	case score <= 3.9:
		return SeverityLow
	case score <= 6.9:
		return SeverityMedium
	case score <= 8.9:
		return SeverityHigh
	default:
		return SeverityCritical
	}
}

// Vector is a parsed CVSS vector string
type Vector struct {
	// Version is "3.0", "3.1" or "4.0"
	Version string
	// Metrics maps metric abbreviations (e.g. "AV") to their values (e.g. "N")
	Metrics map[string]string

	raw string
}

var metricValues = map[string]map[string][]string{
	"3.0": cvss3MetricValues,
	"3.1": cvss3MetricValues,
	"4.0": {
		"AV": {"N", "A", "L", "P"},
		"AC": {"L", "H"},
		"AT": {"N", "P"},
		"PR": {"N", "L", "H"},
		"UI": {"N", "P", "A"},
		"VC": {"H", "L", "N"},
		"VI": {"H", "L", "N"},
		"VA": {"H", "L", "N"},
		"SC": {"H", "L", "N"},
		"SI": {"H", "L", "N"},
		"SA": {"H", "L", "N"},
		// optional metrics that affect the base score (worst-case defaults apply when absent)
		"E":  {"X", "A", "P", "U"},
		"CR": {"X", "H", "M", "L"},
		"IR": {"X", "H", "M", "L"},
		"AR": {"X", "H", "M", "L"},
	},
}

var cvss3MetricValues = map[string][]string{
	"AV": {"N", "A", "L", "P"},
	"AC": {"L", "H"},
	"PR": {"N", "L", "H"},
	"UI": {"N", "R"},
	"S":  {"U", "C"},
	"C":  {"H", "L", "N"},
	"I":  {"H", "L", "N"},
	"A":  {"H", "L", "N"},
}

// cvss3MetricOrder is the metric list of CVSS v3.0/v3.1 vectors (identical for both versions)
var cvss3MetricOrder = []string{"AV", "AC", "PR", "UI", "S", "C", "I", "A"}

// metricOrder is the canonical metric order used when rendering a vector string
var metricOrder = map[string][]string{
	"3.0": cvss3MetricOrder,
	"3.1": cvss3MetricOrder,
	"4.0": {"AV", "AC", "AT", "PR", "UI", "VC", "VI", "VA", "SC", "SI", "SA", "E", "CR", "IR", "AR"},
}

// mandatoryMetrics are the base metrics required in a vector
var mandatoryMetrics = map[string][]string{
	"3.0": cvss3MetricOrder,
	"3.1": cvss3MetricOrder,
	"4.0": {"AV", "AC", "AT", "PR", "UI", "VC", "VI", "VA", "SC", "SI", "SA"},
}

// Parse parses and validates a CVSS vector string such as
// "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
func Parse(s string) (*Vector, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed CVSS vector %q", s)
	}
	version := strings.TrimPrefix(parts[0], "CVSS:")
	allowed, ok := metricValues[version]
	if !ok || parts[0] != "CVSS:"+version {
		return nil, fmt.Errorf("unsupported CVSS version in vector %q (want CVSS:3.0/, CVSS:3.1/ or CVSS:4.0/)", s)
	}
	v := &Vector{Version: version, Metrics: map[string]string{}, raw: s}
	for _, field := range parts[1:] {
		kv := strings.SplitN(field, ":", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("malformed field %q in CVSS vector %q", field, s)
		}
		metric, value := kv[0], kv[1]
		values, ok := allowed[metric]
		if !ok {
			return nil, fmt.Errorf("unknown metric %q in CVSS %s vector", metric, version)
		}
		if _, ok := v.Metrics[metric]; ok {
			return nil, fmt.Errorf("duplicate metric %q in CVSS vector %q", metric, s)
		}
		if !slices.Contains(values, value) {
			return nil, fmt.Errorf("invalid value %q for metric %q in CVSS %s vector", value, metric, version)
		}
		v.Metrics[metric] = value
	}
	for _, metric := range mandatoryMetrics[version] {
		if _, ok := v.Metrics[metric]; !ok {
			return nil, fmt.Errorf("missing mandatory metric %q in CVSS %s vector %q", metric, version, s)
		}
	}
	return v, nil
}

// String returns the canonical vector string with metrics in the official order
func (v *Vector) String() string {
	var sb strings.Builder
	sb.WriteString("CVSS:")
	sb.WriteString(v.Version)
	for _, metric := range metricOrder[v.Version] {
		if value, ok := v.Metrics[metric]; ok && value != "X" {
			sb.WriteString("/")
			sb.WriteString(metric)
			sb.WriteString(":")
			sb.WriteString(value)
		}
	}
	return sb.String()
}

// BaseScore computes the CVSS base score (0.0 - 10.0) of the vector
func (v *Vector) BaseScore() float64 {
	if v.Version == "4.0" {
		return v.baseScore4()
	}
	return v.baseScore3()
}

// Severity returns the qualitative severity rating of the base score
func (v *Vector) Severity() Severity {
	return SeverityFromScore(v.BaseScore())
}

var (
	impactWeights = map[string]float64{"H": 0.56, "L": 0.22, "N": 0.0}
	avWeights     = map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2}
	acWeights     = map[string]float64{"L": 0.77, "H": 0.44}
	prWeights     = map[string]map[string]float64{ // PR weights depend on the scope
		"U": {"N": 0.85, "L": 0.62, "H": 0.27},
		"C": {"N": 0.85, "L": 0.68, "H": 0.5},
	}
	uiWeights = map[string]float64{"N": 0.85, "R": 0.62}
)

// baseScore3 implements the CVSS v3.0/v3.1 base score formula (identical for both versions)
func (v *Vector) baseScore3() float64 {
	m := v.Metrics
	iss := 1 - (1-impactWeights[m["C"]])*(1-impactWeights[m["I"]])*(1-impactWeights[m["A"]])
	var impact float64
	if m["S"] == "U" {
		impact = 6.42 * iss
	} else {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	}
	if impact <= 0 {
		return 0
	}
	exploitability := 8.22 * avWeights[m["AV"]] * acWeights[m["AC"]] * prWeights[m["S"]][m["PR"]] * uiWeights[m["UI"]]
	return roundScore(min(impact+exploitability, 10), v.Version == "3.1")
}

// roundScore rounds to one decimal place: v3.1 uses the spec's Roundup function,
// v3.0 uses plain round-half-up
func roundScore(score float64, roundup bool) float64 {
	if !roundup {
		return math.Floor(score*10+0.5) / 10
	}
	intInput := math.Round(score * 100000)
	if math.Mod(intInput, 10000) == 0 {
		return intInput / 100000
	}
	return (math.Floor(intInput/10000) + 1) / 10
}
