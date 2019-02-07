package version

import (
	"strings"
)

type Constraint struct {
	operator string
	version  string
}

// Return a new Constrain and sets operator and version to compare
func NewConstrain(operator, version string) *Constraint {
	constraint := new(Constraint)
	constraint.SetOperator(operator)
	constraint.SetVersion(version)

	return constraint
}

// Sets operator to compare
func (self *Constraint) SetOperator(operator string) {
	self.operator = operator
}

// Get operator to compare
func (self *Constraint) GetOperator() string {
	return self.operator
}

// Sets version to compare
func (self *Constraint) SetVersion(version string) {
	self.version = version
}

// Get version to compare
func (self *Constraint) GetVersion() string {
	return self.version
}

// Match a given version againts the constraint
func (self *Constraint) Match(version string) bool {
	return Compare(version, self.version, self.operator)
}

// Return a string representation
func (self *Constraint) String() string {
	return strings.Trim(self.operator+" "+self.version, " ")
}
