package gorm

import (
	"bytes"
	"strings"
)

// Namer is a function type which is given a string and return a string
type Namer func(string) string

// NamingStrategy represents naming strategies
type NamingStrategy struct {
	DB     Namer
	Table  Namer
	Column Namer
}

// TheNamingStrategy is being initialized with defaultNamingStrategy
var TheNamingStrategy = &NamingStrategy{
	DB:     defaultNamer,
	Table:  defaultNamer,
	Column: defaultNamer,
}

// AddNamingStrategy sets the naming strategy
func AddNamingStrategy(ns *NamingStrategy) {
	if ns.DB == nil {
		ns.DB = defaultNamer
	}
	if ns.Table == nil {
		ns.Table = defaultNamer
	}
	if ns.Column == nil {
		ns.Column = defaultNamer
	}
	TheNamingStrategy = ns
}

// DBName alters the given name by DB
func (ns *NamingStrategy) DBName(name string) string {
	return ns.DB(name)
}

// TableName alters the given name by Table
func (ns *NamingStrategy) TableName(name string) string {
	return ns.Table(name)
}

// ColumnName alters the given name by Column
func (ns *NamingStrategy) ColumnName(name string) string {
	return ns.Column(name)
}

// ToDBName convert string to db name
func ToDBName(name string) string {
	return TheNamingStrategy.DBName(name)
}

// ToTableName convert string to table name
func ToTableName(name string) string {
	return TheNamingStrategy.TableName(name)
}

// ToColumnName convert string to db name
func ToColumnName(name string) string {
	return TheNamingStrategy.ColumnName(name)
}

var smap = newSafeMap()

func defaultNamer(name string) string {
	const (
		lower = false
		upper = true
	)

	if v := smap.Get(name); v != "" {
		return v
	}

	if name == "" {
		return ""
	}

	var (
		value                                    = commonInitialismsReplacer.Replace(name)
		buf                                      = bytes.NewBufferString("")
		lastCase, currCase, nextCase, nextNumber bool
	)

	for i, v := range value[:len(value)-1] {
		nextCase = bool(value[i+1] >= 'A' && value[i+1] <= 'Z')
		nextNumber = bool(value[i+1] >= '0' && value[i+1] <= '9')

		if i > 0 {
			if currCase == upper {
				if lastCase == upper && (nextCase == upper || nextNumber == upper) {
					buf.WriteRune(v)
				} else {
					if value[i-1] != '_' && value[i+1] != '_' {
						buf.WriteRune('_')
					}
					buf.WriteRune(v)
				}
			} else {
				buf.WriteRune(v)
				if i == len(value)-2 && (nextCase == upper && nextNumber == lower) {
					buf.WriteRune('_')
				}
			}
		} else {
			currCase = upper
			buf.WriteRune(v)
		}
		lastCase = currCase
		currCase = nextCase
	}

	buf.WriteByte(value[len(value)-1])

	s := strings.ToLower(buf.String())
	smap.Set(name, s)
	return s
}
