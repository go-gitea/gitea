package models

// SubGroup represents a nested group under a parent organization or group.
type SubGroup struct {
	ID          int64  `xorm:"pk autoincr"`
	ParentID    int64  `xorm:"INDEX NOT NULL"`
	LowerName   string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string `xorm:"UNIQUE(s) NOT NULL"`
	Description string
	CreatedUnix int64  `xorm:"INDEX created"`
	UpdatedUnix int64  `xorm:"INDEX updated"`
}

// TableName returns the database table name for SubGroup.
func (SubGroup) TableName() string {
	return "subgroup"
}
