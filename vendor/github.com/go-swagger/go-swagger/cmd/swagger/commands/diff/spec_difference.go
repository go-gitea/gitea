package diff

import (
	"fmt"
	"log"
	"sort"
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

	if isResponse {
		direction = " Response"
		if hasURL {
			if hasMethod {
				prefix = fmt.Sprintf("%s:%s -> %d", sd.DifferenceLocation.URL, sd.DifferenceLocation.Method, sd.DifferenceLocation.Response)
			} else {
				prefix = fmt.Sprintf("%s ", sd.DifferenceLocation.URL)
			}
		}
	} else {
		if hasURL {
			if hasMethod {
				direction = " Request"
				prefix = fmt.Sprintf("%s:%s", sd.DifferenceLocation.URL, sd.DifferenceLocation.Method)
			} else {
				prefix = fmt.Sprintf("%s ", sd.DifferenceLocation.URL)
			}
		} else {
			prefix = " Metadata"
		}
	}

	paramOrPropertyLocation := ""
	if sd.DifferenceLocation.Node != nil {
		paramOrPropertyLocation = " - " + sd.DifferenceLocation.Node.String() + " "
	}
	optionalInfo := ""
	if sd.DiffInfo != "" {
		optionalInfo = fmt.Sprintf(" <%s>", sd.DiffInfo)
	}
	return fmt.Sprintf("%s%s%s- %s%s", prefix, direction, paramOrPropertyLocation, sd.Code.Description(), optionalInfo)
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
func (sd *SpecDifferences) ReportCompatibility() error {
	breakingCount := sd.BreakingChangeCount()
	if breakingCount > 0 {
		fmt.Printf("\nBREAKING CHANGES:\n=================\n")
		sd.reportChanges(Breaking)
		return fmt.Errorf("compatibility Test FAILED: %d Breaking changes detected", breakingCount)
	}
	log.Printf("Compatibility test OK. No breaking changes identified.")
	return nil
}

func (sd SpecDifferences) reportChanges(compat Compatibility) {
	toReportList := []string{}

	for _, diff := range sd {
		if diff.Compatibility == compat {
			toReportList = append(toReportList, diff.String())
		}
	}

	sort.Slice(toReportList, func(i, j int) bool {
		return toReportList[i] < toReportList[j]
	})

	for _, eachDiff := range toReportList {
		fmt.Println(eachDiff)
	}
}

// ReportAllDiffs lists all the diffs between two specs
func (sd SpecDifferences) ReportAllDiffs(fmtJSON bool) error {
	if fmtJSON {

		b, err := JSONMarshal(sd)
		if err != nil {
			log.Fatalf("Couldn't print results: %v", err)
		}
		pretty, err := prettyprint(b)
		if err != nil {
			log.Fatalf("Couldn't print results: %v", err)
		}
		fmt.Println(string(pretty))
		return nil
	}
	numDiffs := len(sd)
	if numDiffs == 0 {
		fmt.Println("No changes identified")
		return nil
	}

	if numDiffs != sd.BreakingChangeCount() {
		fmt.Println("NON-BREAKING CHANGES:\n=====================")
		sd.reportChanges(NonBreaking)
	}

	return sd.ReportCompatibility()
}
