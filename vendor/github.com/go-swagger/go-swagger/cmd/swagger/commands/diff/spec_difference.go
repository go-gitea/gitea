package diff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// SpecDifference encapsulates the details of an individual diff in part of a spec
type SpecDifference struct {
	DifferenceLocation DifferenceLocation `json:"location"`
	Code               SpecChangeCode     `json:"code"`
	Compatibility      Compatibility      `json:"compatibility"`
	DiffInfo           string             `json:"info,omitempty"`
}

// SpecDifferences list of differences
type SpecDifferences []SpecDifference

// Matches returns true if the diff matches another
func (sd SpecDifference) Matches(other SpecDifference) bool {
	return sd.Code == other.Code &&
		sd.Compatibility == other.Compatibility &&
		sd.DiffInfo == other.DiffInfo &&
		equalLocations(sd.DifferenceLocation, other.DifferenceLocation)
}

func equalLocations(a, b DifferenceLocation) bool {
	return a.Method == b.Method &&
		a.Response == b.Response &&
		a.URL == b.URL &&
		equalNodes(a.Node, b.Node)
}

func equalNodes(a, b *Node) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Field == b.Field &&
		a.IsArray == b.IsArray &&
		a.TypeName == b.TypeName &&
		equalNodes(a.ChildNode, b.ChildNode)

}

// BreakingChangeCount Calculates the breaking change count
func (sd SpecDifferences) BreakingChangeCount() int {
	count := 0
	for _, eachDiff := range sd {
		if eachDiff.Compatibility == Breaking {
			count++
		}
	}
	return count
}

// FilterIgnores returns a copy of the list without the items in the specified ignore list
func (sd SpecDifferences) FilterIgnores(ignores SpecDifferences) SpecDifferences {
	newDiffs := SpecDifferences{}
	for _, eachDiff := range sd {
		if !ignores.Contains(eachDiff) {
			newDiffs = newDiffs.addDiff(eachDiff)
		}
	}
	return newDiffs
}

// Contains Returns true if the item contains the specified item
func (sd SpecDifferences) Contains(diff SpecDifference) bool {
	for _, eachDiff := range sd {
		if eachDiff.Matches(diff) {
			return true
		}
	}
	return false
}

// String std string renderer
func (sd SpecDifference) String() string {
	isResponse := sd.DifferenceLocation.Response > 0
	hasMethod := len(sd.DifferenceLocation.Method) > 0
	hasURL := len(sd.DifferenceLocation.URL) > 0

	prefix := ""
	direction := ""

	if hasMethod {
		if hasURL {
			prefix = fmt.Sprintf("%s:%s", sd.DifferenceLocation.URL, sd.DifferenceLocation.Method)
		}
		if isResponse {
			prefix += fmt.Sprintf(" -> %d", sd.DifferenceLocation.Response)
			direction = "Response"
		} else {
			direction = "Request"
		}
	} else {
		prefix = sd.DifferenceLocation.URL
	}

	paramOrPropertyLocation := ""
	if sd.DifferenceLocation.Node != nil {
		paramOrPropertyLocation = sd.DifferenceLocation.Node.String()
	}
	optionalInfo := ""
	if sd.DiffInfo != "" {
		optionalInfo = sd.DiffInfo
	}

	items := []string{}
	for _, item := range []string{prefix, direction, paramOrPropertyLocation, sd.Code.Description(), optionalInfo} {
		if item != "" {
			items = append(items, item)
		}
	}
	return strings.Join(items, " - ")
	// return fmt.Sprintf("%s%s%s - %s%s", prefix, direction, paramOrPropertyLocation, sd.Code.Description(), optionalInfo)
}

func (sd SpecDifferences) addDiff(diff SpecDifference) SpecDifferences {
	context := Request
	if diff.DifferenceLocation.Response > 0 {
		context = Response
	}
	diff.Compatibility = getCompatibilityForChange(diff.Code, context)

	return append(sd, diff)
}

// ReportCompatibility lists and spec
func (sd *SpecDifferences) ReportCompatibility() (io.Reader, error, error) {
	var out bytes.Buffer
	breakingCount := sd.BreakingChangeCount()
	if breakingCount > 0 {
		fmt.Fprintln(&out, "\nBREAKING CHANGES:\n=================")
		_, _ = out.ReadFrom(sd.reportChanges(Breaking))
		msg := fmt.Sprintf("compatibility test FAILED: %d breaking changes detected", breakingCount)
		fmt.Fprintln(&out, msg)
		return &out, nil, errors.New(msg)
	}
	fmt.Fprintf(&out, "compatibility test OK. No breaking changes identified.")
	return &out, nil, nil
}

func (sd SpecDifferences) reportChanges(compat Compatibility) io.Reader {
	toReportList := []string{}
	var out bytes.Buffer

	for _, diff := range sd {
		if diff.Compatibility == compat {
			toReportList = append(toReportList, diff.String())
		}
	}

	sort.Slice(toReportList, func(i, j int) bool {
		return toReportList[i] < toReportList[j]
	})

	for _, eachDiff := range toReportList {
		fmt.Fprintln(&out, eachDiff)
	}
	return &out
}

// ReportAllDiffs lists all the diffs between two specs
func (sd SpecDifferences) ReportAllDiffs(fmtJSON bool) (io.Reader, error, error) {
	if fmtJSON {
		b, err := JSONMarshal(sd)
		if err != nil {
			return nil, fmt.Errorf("couldn't print results: %v", err), nil
		}
		out, err := prettyprint(b)
		return out, err, nil
	}
	numDiffs := len(sd)
	if numDiffs == 0 {
		return bytes.NewBuffer([]byte("No changes identified")), nil, nil
	}

	var out bytes.Buffer
	if numDiffs != sd.BreakingChangeCount() {
		fmt.Fprintln(&out, "NON-BREAKING CHANGES:\n=====================")
		_, _ = out.ReadFrom(sd.reportChanges(NonBreaking))
	}

	more, err, warn := sd.ReportCompatibility()
	if err != nil {
		return nil, err, warn
	}
	_, _ = out.ReadFrom(more)
	return &out, nil, warn
}
