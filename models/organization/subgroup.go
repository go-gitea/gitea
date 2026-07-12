package organization

// SubGroup represents a nested group under a parent organization.
type SubGroup struct {
	ID       int64  `xorm:"pk autoincr"`
	ParentID int64  `xorm:"INDEX NOT NULL"`
	Name     string `xorm:"UNIQUE(s) NOT NULL"`
	LowerName string `xorm:"UNIQUE(s) NOT NULL"`
}

// TableName returns the table name for SubGroup.
func (SubGroup) TableName() string {
	return "sub_group"
}
