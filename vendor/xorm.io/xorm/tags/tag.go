// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tags

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"xorm.io/xorm/schemas"
)

func splitTag(tag string) (tags []string) {
	tag = strings.TrimSpace(tag)
	var hasQuote = false
	var lastIdx = 0
	for i, t := range tag {
		if t == '\'' {
			hasQuote = !hasQuote
		} else if t == ' ' {
			if lastIdx < i && !hasQuote {
				tags = append(tags, strings.TrimSpace(tag[lastIdx:i]))
				lastIdx = i + 1
			}
		}
	}
	if lastIdx < len(tag) {
		tags = append(tags, strings.TrimSpace(tag[lastIdx:]))
	}
	return
}

// Context represents a context for xorm tag parse.
type Context struct {
	tagName         string
	params          []string
	preTag, nextTag string
	table           *schemas.Table
	col             *schemas.Column
	fieldValue      reflect.Value
	isIndex         bool
	isUnique        bool
	indexNames      map[string]int
	parser          *Parser
	hasCacheTag     bool
	hasNoCacheTag   bool
	ignoreNext      bool
}

// Handler describes tag handler for XORM
type Handler func(ctx *Context) error

var (
	// defaultTagHandlers enumerates all the default tag handler
	defaultTagHandlers = map[string]Handler{
		"<-":       OnlyFromDBTagHandler,
		"->":       OnlyToDBTagHandler,
		"PK":       PKTagHandler,
		"NULL":     NULLTagHandler,
		"NOT":      IgnoreTagHandler,
		"AUTOINCR": AutoIncrTagHandler,
		"DEFAULT":  DefaultTagHandler,
		"CREATED":  CreatedTagHandler,
		"UPDATED":  UpdatedTagHandler,
		"DELETED":  DeletedTagHandler,
		"VERSION":  VersionTagHandler,
		"UTC":      UTCTagHandler,
		"LOCAL":    LocalTagHandler,
		"NOTNULL":  NotNullTagHandler,
		"INDEX":    IndexTagHandler,
		"UNIQUE":   UniqueTagHandler,
		"CACHE":    CacheTagHandler,
		"NOCACHE":  NoCacheTagHandler,
		"COMMENT":  CommentTagHandler,
	}
)

func init() {
	for k := range schemas.SqlTypes {
		defaultTagHandlers[k] = SQLTypeTagHandler
	}
}

// IgnoreTagHandler describes ignored tag handler
func IgnoreTagHandler(ctx *Context) error {
	return nil
}

// OnlyFromDBTagHandler describes mapping direction tag handler
func OnlyFromDBTagHandler(ctx *Context) error {
	ctx.col.MapType = schemas.ONLYFROMDB
	return nil
}

// OnlyToDBTagHandler describes mapping direction tag handler
func OnlyToDBTagHandler(ctx *Context) error {
	ctx.col.MapType = schemas.ONLYTODB
	return nil
}

// PKTagHandler describes primary key tag handler
func PKTagHandler(ctx *Context) error {
	ctx.col.IsPrimaryKey = true
	ctx.col.Nullable = false
	return nil
}

// NULLTagHandler describes null tag handler
func NULLTagHandler(ctx *Context) error {
	ctx.col.Nullable = (strings.ToUpper(ctx.preTag) != "NOT")
	return nil
}

// NotNullTagHandler describes notnull tag handler
func NotNullTagHandler(ctx *Context) error {
	ctx.col.Nullable = false
	return nil
}

// AutoIncrTagHandler describes autoincr tag handler
func AutoIncrTagHandler(ctx *Context) error {
	ctx.col.IsAutoIncrement = true
	/*
		if len(ctx.params) > 0 {
			autoStartInt, err := strconv.Atoi(ctx.params[0])
			if err != nil {
				return err
			}
			ctx.col.AutoIncrStart = autoStartInt
		} else {
			ctx.col.AutoIncrStart = 1
		}
	*/
	return nil
}

// DefaultTagHandler describes default tag handler
func DefaultTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.col.Default = ctx.params[0]
	} else {
		ctx.col.Default = ctx.nextTag
		ctx.ignoreNext = true
	}
	ctx.col.DefaultIsEmpty = false
	return nil
}

// CreatedTagHandler describes created tag handler
func CreatedTagHandler(ctx *Context) error {
	ctx.col.IsCreated = true
	return nil
}

// VersionTagHandler describes version tag handler
func VersionTagHandler(ctx *Context) error {
	ctx.col.IsVersion = true
	ctx.col.Default = "1"
	return nil
}

// UTCTagHandler describes utc tag handler
func UTCTagHandler(ctx *Context) error {
	ctx.col.TimeZone = time.UTC
	return nil
}

// LocalTagHandler describes local tag handler
func LocalTagHandler(ctx *Context) error {
	if len(ctx.params) == 0 {
		ctx.col.TimeZone = time.Local
	} else {
		var err error
		ctx.col.TimeZone, err = time.LoadLocation(ctx.params[0])
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdatedTagHandler describes updated tag handler
func UpdatedTagHandler(ctx *Context) error {
	ctx.col.IsUpdated = true
	return nil
}

// DeletedTagHandler describes deleted tag handler
func DeletedTagHandler(ctx *Context) error {
	ctx.col.IsDeleted = true
	return nil
}

// IndexTagHandler describes index tag handler
func IndexTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.indexNames[ctx.params[0]] = schemas.IndexType
	} else {
		ctx.isIndex = true
	}
	return nil
}

// UniqueTagHandler describes unique tag handler
func UniqueTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.indexNames[ctx.params[0]] = schemas.UniqueType
	} else {
		ctx.isUnique = true
	}
	return nil
}

// CommentTagHandler add comment to column
func CommentTagHandler(ctx *Context) error {
	if len(ctx.params) > 0 {
		ctx.col.Comment = strings.Trim(ctx.params[0], "' ")
	}
	return nil
}

// SQLTypeTagHandler describes SQL Type tag handler
func SQLTypeTagHandler(ctx *Context) error {
	ctx.col.SQLType = schemas.SQLType{Name: ctx.tagName}
	if len(ctx.params) > 0 {
		if ctx.tagName == schemas.Enum {
			ctx.col.EnumOptions = make(map[string]int)
			for k, v := range ctx.params {
				v = strings.TrimSpace(v)
				v = strings.Trim(v, "'")
				ctx.col.EnumOptions[v] = k
			}
		} else if ctx.tagName == schemas.Set {
			ctx.col.SetOptions = make(map[string]int)
			for k, v := range ctx.params {
				v = strings.TrimSpace(v)
				v = strings.Trim(v, "'")
				ctx.col.SetOptions[v] = k
			}
		} else {
			var err error
			if len(ctx.params) == 2 {
				ctx.col.Length, err = strconv.Atoi(ctx.params[0])
				if err != nil {
					return err
				}
				ctx.col.Length2, err = strconv.Atoi(ctx.params[1])
				if err != nil {
					return err
				}
			} else if len(ctx.params) == 1 {
				ctx.col.Length, err = strconv.Atoi(ctx.params[0])
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ExtendsTagHandler describes extends tag handler
func ExtendsTagHandler(ctx *Context) error {
	var fieldValue = ctx.fieldValue
	var isPtr = false
	switch fieldValue.Kind() {
	case reflect.Ptr:
		f := fieldValue.Type().Elem()
		if f.Kind() == reflect.Struct {
			fieldPtr := fieldValue
			fieldValue = fieldValue.Elem()
			if !fieldValue.IsValid() || fieldPtr.IsNil() {
				fieldValue = reflect.New(f).Elem()
			}
		}
		isPtr = true
		fallthrough
	case reflect.Struct:
		parentTable, err := ctx.parser.Parse(fieldValue)
		if err != nil {
			return err
		}
		for _, col := range parentTable.Columns() {
			col.FieldName = fmt.Sprintf("%v.%v", ctx.col.FieldName, col.FieldName)

			var tagPrefix = ctx.col.FieldName
			if len(ctx.params) > 0 {
				col.Nullable = isPtr
				tagPrefix = ctx.params[0]
				if col.IsPrimaryKey {
					col.Name = ctx.col.FieldName
					col.IsPrimaryKey = false
				} else {
					col.Name = fmt.Sprintf("%v%v", tagPrefix, col.Name)
				}
			}

			if col.Nullable {
				col.IsAutoIncrement = false
				col.IsPrimaryKey = false
			}

			ctx.table.AddColumn(col)
			for indexName, indexType := range col.Indexes {
				addIndex(indexName, ctx.table, col, indexType)
			}
		}
	default:
		//TODO: warning
	}
	return nil
}

// CacheTagHandler describes cache tag handler
func CacheTagHandler(ctx *Context) error {
	if !ctx.hasCacheTag {
		ctx.hasCacheTag = true
	}
	return nil
}

// NoCacheTagHandler describes nocache tag handler
func NoCacheTagHandler(ctx *Context) error {
	if !ctx.hasNoCacheTag {
		ctx.hasNoCacheTag = true
	}
	return nil
}
