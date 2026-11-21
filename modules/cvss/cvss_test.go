// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cvss

import (
	"math"
	"testing"
)

func TestParseAndScore(t *testing.T) {
	tests := []struct {
		vector   string
		score    float64
		severity Severity
	}{
		// CVSS v3.0 / v3.1 (scores verified against the FIRST v3.1 specification examples)
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8, SeverityCritical},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", 10.0, SeverityCritical},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N", 7.5, SeverityHigh},
		{"CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H", 8.8, SeverityHigh},
		{"CVSS:3.1/AV:N/AC:H/PR:N/UI:R/S:U/C:L/I:L/A:N", 4.2, SeverityMedium},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:C/C:L/I:L/A:N", 5.6, SeverityMedium},
		{"CVSS:3.1/AV:P/AC:H/PR:H/UI:R/S:U/C:L/I:L/A:N", 2.7, SeverityLow},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N", 0.0, SeverityNone},
		{"CVSS:3.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8, SeverityCritical},
		// CVSS v4.0 (scores from the official FIRST v4.0 examples)
		{"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:H/SI:H/SA:H", 10.0, SeverityCritical},
		{"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N", 9.3, SeverityCritical},
		{"CVSS:4.0/AV:N/AC:L/AT:P/PR:N/UI:P/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N", 7.7, SeverityHigh},
		{"CVSS:4.0/AV:N/AC:L/AT:P/PR:N/UI:P/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N/E:U", 5.2, SeverityMedium},
		{"CVSS:4.0/AV:L/AC:L/AT:P/PR:L/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N", 7.3, SeverityHigh},
		{"CVSS:4.0/AV:N/AC:L/AT:P/PR:N/UI:N/VC:H/VI:L/VA:L/SC:N/SI:N/SA:N", 8.3, SeverityHigh},
		{"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:N/VI:N/VA:N/SC:L/SI:L/SA:N", 6.9, SeverityMedium},
		{"CVSS:4.0/AV:L/AC:L/AT:N/PR:H/UI:N/VC:N/VI:N/VA:N/SC:H/SI:N/SA:N", 5.9, SeverityMedium},
		{"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:A/VC:N/VI:N/VA:N/SC:L/SI:L/SA:N", 5.1, SeverityMedium},
		{"CVSS:4.0/AV:L/AC:L/AT:N/PR:N/UI:A/VC:L/VI:N/VA:N/SC:N/SI:N/SA:N", 4.6, SeverityMedium},
		{"CVSS:4.0/AV:P/AC:H/AT:P/PR:H/UI:A/VC:N/VI:N/VA:N/SC:N/SI:N/SA:N", 0.0, SeverityNone},
		{"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:N/VI:N/VA:N/SC:N/SI:N/SA:N", 0.0, SeverityNone},
	}
	for _, tt := range tests {
		v, err := Parse(tt.vector)
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tt.vector, err)
			continue
		}
		score := v.BaseScore()
		if math.Abs(score-tt.score) > 0.001 {
			t.Errorf("Parse(%q).BaseScore() = %v, want %v (macro %s)", tt.vector, score, tt.score, v.macroVectorSafe())
		}
		if sev := v.Severity(); sev != tt.severity {
			t.Errorf("Parse(%q).Severity() = %v, want %v", tt.vector, sev, tt.severity)
		}
	}
}

func (v *Vector) macroVectorSafe() string {
	if v.Version != "4.0" {
		return "-"
	}
	return v.macroVector()
}

func TestParseCanonicalizes(t *testing.T) {
	// fields out of order are accepted and rendered in canonical order
	v, err := Parse("CVSS:3.1/A:N/AC:L/C:H/AV:N/I:H/PR:N/S:U/UI:N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:N"
	if v.String() != want {
		t.Errorf("String() = %q, want %q", v.String(), want)
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{
		"",
		"CVSS:3.1/AV:N",
		"CVSS:2.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		"AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H",                              // missing A
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:X",                          // invalid value
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H/AV:L",                     // duplicate metric
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H/XX:N",                     // unknown metric
		"CVSS:4.0/AV:N/AC:L/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",            // missing AT
		"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N/MAV:N", // modified metrics not allowed
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H/",
	} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", s)
		}
	}
}
