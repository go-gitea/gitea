// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cvss

import (
	"math"
	"strconv"
	"strings"
)

// The CVSS v4.0 base score is not computed by a closed formula: the vector is
// reduced to a 6-digit MacroVector, looked up in macroVectorScores, and then
// interpolated within its MacroVector based on the distance to the most severe
// vectors of each equivalence group. See the CVSS v4.0 specification, section 8.

// highest severity vectors (MAX_COMPOSED) per equivalence group level
var maxComposedEq1 = map[string][]string{
	"0": {"AV:N/PR:N/UI:N/"},
	"1": {"AV:A/PR:N/UI:N/", "AV:N/PR:L/UI:N/", "AV:N/PR:N/UI:P/"},
	"2": {"AV:P/PR:N/UI:N/", "AV:A/PR:L/UI:P/"},
}

var maxComposedEq2 = map[string][]string{
	"0": {"AC:L/AT:N/"},
	"1": {"AC:H/AT:N/", "AC:L/AT:P/"},
}

var maxComposedEq3 = map[string]map[string][]string{
	"0": {
		"0": {"VC:H/VI:H/VA:H/CR:H/IR:H/AR:H/"},
		"1": {"VC:H/VI:H/VA:L/CR:M/IR:M/AR:H/", "VC:H/VI:H/VA:H/CR:M/IR:M/AR:M/"},
	},
	"1": {
		"0": {"VC:L/VI:H/VA:H/CR:H/IR:H/AR:H/", "VC:H/VI:L/VA:H/CR:H/IR:H/AR:H/"},
		"1": {"VC:L/VI:H/VA:L/CR:H/IR:M/AR:H/", "VC:L/VI:H/VA:H/CR:H/IR:M/AR:M/", "VC:H/VI:L/VA:H/CR:M/IR:H/AR:M/", "VC:H/VI:L/VA:L/CR:M/IR:H/AR:H/", "VC:L/VI:L/VA:H/CR:H/IR:H/AR:M/"},
	},
	"2": {
		"1": {"VC:L/VI:L/VA:L/CR:H/IR:H/AR:H/"},
	},
}

var maxComposedEq4 = map[string][]string{
	"0": {"SC:H/SI:S/SA:S/"},
	"1": {"SC:H/SI:H/SA:H/"},
	"2": {"SC:L/SI:L/SA:L/"},
}

var maxComposedEq5 = map[string][]string{
	"0": {"E:A/"},
	"1": {"E:P/"},
	"2": {"E:U/"},
}

// maximum severity distance (MAX_SEVERITY) per equivalence group level, in metric-value steps
var (
	maxSeverityEq1    = map[int]float64{0: 1, 1: 4, 2: 5}
	maxSeverityEq2    = map[int]float64{0: 1, 1: 2}
	maxSeverityEq3eq6 = map[int]map[int]float64{0: {0: 7, 1: 6}, 1: {0: 8, 1: 8}, 2: {1: 10}}
	maxSeverityEq4    = map[int]float64{0: 6, 1: 5, 2: 4}
)

// severityLevelsByMetric maps each metric to the severity distance weights of its
// values: 0.0 is the most severe value
var severityLevelsByMetric = map[string]map[string]float64{
	"AV": {"N": 0.0, "A": 0.1, "L": 0.2, "P": 0.3},
	"PR": {"N": 0.0, "L": 0.1, "H": 0.2},
	"UI": {"N": 0.0, "P": 0.1, "A": 0.2},
	"AC": {"L": 0.0, "H": 0.1},
	"AT": {"N": 0.0, "P": 0.1},
	"VC": {"H": 0.0, "L": 0.1, "N": 0.2},
	"VI": {"H": 0.0, "L": 0.1, "N": 0.2},
	"VA": {"H": 0.0, "L": 0.1, "N": 0.2},
	"SC": {"H": 0.1, "L": 0.2, "N": 0.3},
	"SI": {"S": 0.0, "H": 0.1, "L": 0.2, "N": 0.3},
	"SA": {"S": 0.0, "H": 0.1, "L": 0.2, "N": 0.3},
	"CR": {"H": 0.0, "M": 0.1, "L": 0.2},
	"IR": {"H": 0.0, "M": 0.1, "L": 0.2},
	"AR": {"H": 0.0, "M": 0.1, "L": 0.2},
}

// worstCaseDefaults are the values assumed for absent optional metrics
// (the base score assumes the worst case for threat/environmental metrics)
var worstCaseDefaults = map[string]string{"E": "A", "CR": "H", "IR": "H", "AR": "H"}

// m returns the effective value of a metric, applying the worst-case default when absent
func (v *Vector) m4(metric string) string {
	value := v.Metrics[metric]
	if value == "" || value == "X" {
		if def, ok := worstCaseDefaults[metric]; ok {
			return def
		}
	}
	return value
}

func (v *Vector) macroVector() string {
	m := v.m4
	av, pr, ui := m("AV"), m("PR"), m("UI")
	eq1 := "2"
	switch {
	case av == "N" && pr == "N" && ui == "N":
		eq1 = "0"
	case (av == "N" || pr == "N" || ui == "N") && av != "P":
		eq1 = "1"
	}
	eq2 := "1"
	if m("AC") == "L" && m("AT") == "N" {
		eq2 = "0"
	}
	eq3 := "2"
	if m("VC") == "H" && m("VI") == "H" {
		eq3 = "0"
	} else if m("VC") == "H" || m("VI") == "H" || m("VA") == "H" {
		eq3 = "1"
	}
	eq4 := "2"
	switch {
	case m("SI") == "S" || m("SA") == "S":
		eq4 = "0"
	case m("SC") == "H" || m("SI") == "H" || m("SA") == "H":
		eq4 = "1"
	}
	eq5 := map[string]string{"A": "0", "P": "1", "U": "2"}[m("E")]
	eq6 := "1"
	if (m("CR") == "H" && m("VC") == "H") || (m("IR") == "H" && m("VI") == "H") || (m("AR") == "H" && m("VA") == "H") {
		eq6 = "0"
	}
	return eq1 + eq2 + eq3 + eq4 + eq5 + eq6
}

// extractValueMetric extracts the value of a metric from a highest-severity vector fragment
func extractValueMetric(metric, s string) string {
	i := strings.Index(s, metric+":")
	if i < 0 {
		return ""
	}
	rest := s[i+len(metric)+1:]
	value, _, _ := strings.Cut(rest, "/")
	return value
}

// severityDistance returns the severity distance of the vector to a highest-severity
// (max) vector, per metric; ok is false if the vector exceeds the max vector in any metric
func (v *Vector) severityDistance(maxVector string) (dist map[string]float64, ok bool) {
	m := v.m4
	dist = make(map[string]float64, len(severityLevelsByMetric))
	for metric, levels := range severityLevelsByMetric {
		d := levels[m(metric)] - levels[extractValueMetric(metric, maxVector)]
		if d < 0 {
			return nil, false
		}
		dist[metric] = d
	}
	return dist, true
}

func (v *Vector) baseScore4() float64 {
	m := v.m4
	if m("VC") == "N" && m("VI") == "N" && m("VA") == "N" && m("SC") == "N" && m("SI") == "N" && m("SA") == "N" {
		return 0
	}
	mv := v.macroVector()
	value, ok := macroVectorScores[mv]
	if !ok {
		// cannot happen for a valid vector, but never return an unvalidated score
		return 0
	}
	var eq [6]int
	for i := range eq {
		eq[i], _ = strconv.Atoi(string(mv[i]))
	}
	join := func(e [6]int) string {
		var sb strings.Builder
		for _, d := range e {
			sb.WriteString(strconv.Itoa(d))
		}
		return sb.String()
	}
	with := func(idx, val int) string {
		e := eq
		e[idx] = val
		return join(e)
	}
	lookup := func(macro string) float64 {
		if s, ok := macroVectorScores[macro]; ok {
			return s
		}
		return math.NaN()
	}
	scoreEq1Next := lookup(with(0, eq[0]+1))
	scoreEq2Next := lookup(with(1, eq[1]+1))
	scoreEq4Next := lookup(with(3, eq[3]+1))
	scoreEq5Next := lookup(with(4, eq[4]+1))
	var scoreEq3eq6Next float64
	switch {
	case eq[2] == 1 && eq[5] == 1, eq[2] == 0 && eq[5] == 1:
		scoreEq3eq6Next = lookup(with(2, eq[2]+1))
	case eq[2] == 1 && eq[5] == 0:
		scoreEq3eq6Next = lookup(with(5, eq[5]+1))
	case eq[2] == 0 && eq[5] == 0:
		scoreEq3eq6Next = math.Max(lookup(with(5, eq[5]+1)), lookup(with(2, eq[2]+1)))
	default: // eq3 == 2: the next lower MacroVector increments both EQ3 and EQ6
		e := eq
		e[2]++
		e[5]++
		scoreEq3eq6Next = lookup(join(e))
	}
	// find the first highest-severity vector of this MacroVector that the vector does not exceed
	maxVectors := []string{}
	for _, eq1Max := range maxComposedEq1[strconv.Itoa(eq[0])] {
		for _, eq2Max := range maxComposedEq2[strconv.Itoa(eq[1])] {
			for _, eq3eq6Max := range maxComposedEq3[strconv.Itoa(eq[2])][strconv.Itoa(eq[5])] {
				for _, eq4Max := range maxComposedEq4[strconv.Itoa(eq[3])] {
					for _, eq5Max := range maxComposedEq5[strconv.Itoa(eq[4])] {
						maxVectors = append(maxVectors, eq1Max+eq2Max+eq3eq6Max+eq4Max+eq5Max)
					}
				}
			}
		}
	}
	var distEq1, distEq2, distEq3eq6, distEq4 float64
	found := false
	for _, maxVector := range maxVectors {
		dist, ok := v.severityDistance(maxVector)
		if !ok {
			continue
		}
		distEq1 = dist["AV"] + dist["PR"] + dist["UI"]
		distEq2 = dist["AC"] + dist["AT"]
		distEq3eq6 = dist["VC"] + dist["VI"] + dist["VA"] + dist["CR"] + dist["IR"] + dist["AR"]
		distEq4 = dist["SC"] + dist["SI"] + dist["SA"]
		found = true
		break
	}
	if !found {
		return roundScore4(value)
	}
	const step = 0.1
	availableEq1 := value - scoreEq1Next
	availableEq2 := value - scoreEq2Next
	availableEq3eq6 := value - scoreEq3eq6Next
	availableEq4 := value - scoreEq4Next
	availableEq5 := value - scoreEq5Next
	sum, count := 0.0, 0
	for _, avail := range []struct {
		available float64
		current   float64
		max       float64
	}{{
		availableEq1, distEq1, maxSeverityEq1[eq[0]] * step,
	}, {
		availableEq2, distEq2, maxSeverityEq2[eq[1]] * step,
	}, {
		availableEq3eq6, distEq3eq6, maxSeverityEq3eq6[eq[2]][eq[5]] * step,
	}, {
		availableEq4, distEq4, maxSeverityEq4[eq[3]] * step,
	}, {
		availableEq5, 0, 1 * step, // EQ5 does not contribute severity distance
	}} {
		if !math.IsNaN(avail.available) && avail.available >= 0 && avail.max > 0 {
			sum += avail.available * (avail.current / avail.max)
			count++
		}
	}
	if count > 0 {
		value -= sum / float64(count)
	}
	value = math.Max(0.0, math.Min(10.0, value))
	return roundScore4(value)
}

// roundScore4 rounds to one decimal place, half away from zero, with an epsilon
// to compensate floating point inaccuracies (per the reference implementation)
func roundScore4(x float64) float64 {
	return math.Floor((x+1e-6)*10+0.5) / 10
}
