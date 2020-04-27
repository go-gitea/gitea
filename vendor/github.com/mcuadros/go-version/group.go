package version

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type ConstraintGroup struct {
	constraints []*Constraint
}

// Return a new NewConstrainGroup
func NewConstrainGroup() *ConstraintGroup {
	group := new(ConstraintGroup)

	return group
}

// Return a new NewConstrainGroup and create the constraints based on a string
//
// Version constraints can be specified in a few different ways:
//
// Exact version: You can specify the exact version of a package, for 
// example 1.0.2.
//
// Range: By using comparison operators you can specify ranges of valid versions. 
// Valid operators are >, >=, <, <=, !=. An example range would be >=1.0. You can 
// define multiple ranges, separated by a comma: >=1.0,<2.0.
// 
// Wildcard: You can specify a pattern with a * wildcard. 1.0.* is the equivalent 
// of >=1.0,<1.1.
//
// Next Significant Release (Tilde Operator): The ~ operator is best explained by 
// example: ~1.2 is equivalent to >=1.2,<2.0, while ~1.2.3 is equivalent to 
// >=1.2.3,<1.3. As you can see it is mostly useful for projects respecting 
// semantic versioning. A common usage would be to mark the minimum minor 
// version you depend on, like ~1.2 (which allows anything up to, but not 
// including, 2.0). Since in theory there should be no backwards compatibility 
// breaks until 2.0, that works well. Another way of looking at it is that 
// using ~ specifies a minimum version, but allows the last digit specified 
// to go up.
//
// By default only stable releases are taken into consideration. If you would like 
// to also get RC, beta, alpha or dev versions of your dependencies you can do so 
// using stability flags. To change that for all packages instead of doing per 
// dependency you can also use the minimum-stability setting.
// 
// From: http://getcomposer.org/doc/01-basic-usage.md#package-versions
func NewConstrainGroupFromString(name string) *ConstraintGroup {
	group := new(ConstraintGroup)
	group.fromString(name)

	return group
}

// Adds a Contraint to the group
func (self *ConstraintGroup) AddConstraint(constraint ...*Constraint) {
	if self.constraints == nil {
		self.constraints = make([]*Constraint, 0)
	}

	self.constraints = append(self.constraints, constraint...)
}

// Return all the constraints
func (self *ConstraintGroup) GetConstraints() []*Constraint {
	return self.constraints
}

// Match a given version againts the group
//
// Usage
//     c := version.NewConstrainGroupFromString(">2.0,<=3.0")
//     c.Match("2.5.0beta")
//     Returns: true
//
//     c := version.NewConstrainGroupFromString("~1.2.3")
//     c.Match("1.2.3.5")
//     Returns: true
func (self *ConstraintGroup) Match(version string) bool {
	for _, constraint := range self.constraints {
		if constraint.Match(version) == false {
			return false
		}
	}

	return true
}

func (self *ConstraintGroup) fromString(constraint string) bool {
	result := RegFind(`(?i)^([^,\s]*?)@(stable|RC|beta|alpha|dev)$`, constraint)
	if result != nil {
		constraint = result[1]
		if constraint == "" {
			constraint = "*"
		}
	}

	result = RegFind(`(?i)^(dev-[^,\s@]+?|[^,\s@]+?\.x-dev)#.+$`, constraint)
	if result != nil {
		if result[1] != "" {
			constraint = result[1]
		}
	}

	constraints := RegSplit(`\s*,\s*`, strings.Trim(constraint, " "))

	if len(constraints) > 1 {
		for _, part := range constraints {
			self.AddConstraint(self.parseConstraint(part)...)
		}

		return true
	}

	self.AddConstraint(self.parseConstraint(constraints[0])...)

	return true
}

func (self *ConstraintGroup) parseConstraint(constraint string) []*Constraint {

	stabilityModifier := ""

	result := RegFind(`(?i)^([^,\s]+?)@(stable|RC|beta|alpha|dev)$`, constraint)
	if result != nil {
		constraint = result[1]
		if result[2] != "stable" {
			stabilityModifier = result[2]
		}
	}

	result = RegFind(`^[x*](\.[x*])*$`, constraint)
	if result != nil {
		return make([]*Constraint, 0)
	}

	highVersion := ""
	lowVersion := ""

	result = RegFind(`(?i)^~(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:\.(\d+))?`+modifierRegex+`?$`, constraint)
	if result != nil {
		if len(result) > 4 && result[4] != "" {
			last, _ := strconv.Atoi(result[3])
			highVersion = result[1] + "." + result[2] + "." + strconv.Itoa(last+1) + ".0-dev"
			lowVersion = result[1] + "." + result[2] + "." + result[3] + "." + result[4]
		} else if len(result) > 3 && result[3] != "" {
			last, _ := strconv.Atoi(result[2])
			highVersion = result[1] + "." + strconv.Itoa(last+1) + ".0.0-dev"
			lowVersion = result[1] + "." + result[2] + "." + result[3] + ".0"
		} else {
			last, _ := strconv.Atoi(result[1])
			highVersion = strconv.Itoa(last+1) + ".0.0.0-dev"
			if len(result) > 2 && result[2] != "" {
				lowVersion = result[1] + "." + result[2] + ".0.0"
			} else {
				lowVersion = result[1] + ".0.0.0"
			}
		}

		if len(result) > 5 && result[5] != "" {
			lowVersion = lowVersion + "-" + expandStability(result[5])

		}

		if len(result) > 6 && result[6] != "" {
			lowVersion = lowVersion + result[6]
		}

		if len(result) > 7 && result[7] != "" {
			lowVersion = lowVersion + "-dev"
		}

		return []*Constraint{
			{">=", lowVersion},
			{"<", highVersion},
		}
	}

	result = RegFind(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?\.[x*]$`, constraint)
	if result != nil {
		if len(result) > 3 && result[3] != "" {
			highVersion = result[1] + "." + result[2] + "." + result[3] + ".9999999"
			if result[3] == "0" {
				last, _ := strconv.Atoi(result[2])
				lowVersion = result[1] + "." + strconv.Itoa(last-1) + ".9999999.9999999"
			} else {
				last, _ := strconv.Atoi(result[3])
				lowVersion = result[1] + "." + result[2] + "." + strconv.Itoa(last-1) + ".9999999"
			}

		} else if len(result) > 2 && result[2] != "" {
			highVersion = result[1] + "." + result[2] + ".9999999.9999999"
			if result[2] == "0" {
				last, _ := strconv.Atoi(result[1])
				lowVersion = strconv.Itoa(last-1) + ".9999999.9999999.9999999"
			} else {
				last, _ := strconv.Atoi(result[2])
				lowVersion = result[1] + "." + strconv.Itoa(last-1) + ".9999999.9999999"
			}

		} else {
			highVersion = result[1] + ".9999999.9999999.9999999"
			if result[1] == "0" {
				return []*Constraint{{"<", highVersion}}
			} else {
				last, _ := strconv.Atoi(result[1])
				lowVersion = strconv.Itoa(last-1) + ".9999999.9999999.9999999"
			}
		}

		return []*Constraint{
			{">", lowVersion},
			{"<", highVersion},
		}
	}

	// match operators constraints
	result = RegFind(`^(<>|!=|>=?|<=?|==?)?\s*(.*)`, constraint)
	if result != nil {
		version := Normalize(result[2])

		if stabilityModifier != "" && parseStability(version) == "stable" {
			version = version + "-" + stabilityModifier
		} else if result[1] == "<" {
			match := RegFind(`(?i)-stable$`, result[2])
			if match == nil {
				version = version + "-dev"
			}
		}

		if len(result) > 1 && result[1] != "" {
			return []*Constraint{{result[1], version}}
		} else {
			return []*Constraint{{"=", version}}

		}
	}

	return []*Constraint{{constraint, stabilityModifier}}
}

// PCRegMap : PreCompiled Regex Map
type PCRegMap struct {
	sync.RWMutex
	m map[string]*regexp.Regexp
}

// MustCompile : to replace regexp.MustCompile in RegFind.
func (p *PCRegMap) MustCompile(pattern string) *regexp.Regexp {
	p.RLock()
	ret, exist := p.m[pattern]
	p.RUnlock()
	if exist {
		return ret
	}
	ret = regexp.MustCompile(pattern)
	p.Lock()
	p.m[pattern] = ret
	p.Unlock()
	return ret
}

var (
	regexpCache *PCRegMap
)

func init() {
	regexpCache = new(PCRegMap)
	regexpCache.m = make(map[string]*regexp.Regexp)
}

func RegFind(pattern, subject string) []string {
	reg := regexpCache.MustCompile(pattern)
	matched := reg.FindAllStringSubmatch(subject, -1)

	if matched != nil {
		return matched[0]
	}

	return nil
}

func RegSplit(pattern, subject string) []string {
	reg := regexp.MustCompile(pattern)
	indexes := reg.FindAllStringIndex(subject, -1)

	laststart := 0
	result := make([]string, len(indexes)+1)

	for i, element := range indexes {
		result[i] = subject[laststart:element[0]]
		laststart = element[1]
	}

	result[len(indexes)] = subject[laststart:len(subject)]
	return result
}
