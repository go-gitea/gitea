// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"
	"strings"

	"xorm.io/xorm/core"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/schemas"
)

var firebirdQuoter = schemas.Quoter{
	Prefix:     '"',
	Suffix:     '"',
	IsReserved: schemas.AlwaysReserve,
	// Firebird folds unquoted identifiers to upper case and treats quoted ones
	// literally, so identifiers are created and referenced in upper case to let
	// the unquoted SQL Gitea already emits resolve to the same objects.
	Normalize: strings.ToUpper,
}

type firebird struct {
	dialects.Base
	quoter schemas.Quoter
}

func (db *firebird) Init(uri *dialects.URI) error {
	db.quoter = firebirdQuoter
	return db.Base.Init(db, uri)
}

// Base.Quoter returns a field Base cannot populate from outside its own package,
// so the quoter must be served from here or identifiers stay unquoted.
func (db *firebird) Quoter() schemas.Quoter { return db.quoter }

// upperIdent normalizes a name for comparison against rdb$ system tables, which
// store identifiers as they were created. Schema-qualified names keep their dot.
func upperIdent(name string) string {
	schema, ident, found := strings.Cut(name, ".")
	if found {
		return strings.ToUpper(schema) + "." + strings.ToUpper(ident)
	}
	return strings.ToUpper(name)
}

func (db *firebird) Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT rdb$get_context('SYSTEM','ENGINE_VERSION') FROM rdb$database")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	v := &schemas.Version{}
	if rows.Next() {
		if err := rows.Scan(&v.Number); err != nil {
			return nil, err
		}
	}
	return v, rows.Err()
}

func (db *firebird) Features() *dialects.DialectFeatures {
	return &dialects.DialectFeatures{AutoincrMode: dialects.IncrAutoincrMode}
}

func (db *firebird) Filters() []dialects.Filter { return []dialects.Filter{} }

func (db *firebird) IsReserved(name string) bool { return false }

func (db *firebird) SetQuotePolicy(quotePolicy dialects.QuotePolicy) {
	q := firebirdQuoter
	switch quotePolicy {
	case dialects.QuotePolicyNone:
		q.IsReserved = schemas.AlwaysNoReserve
	case dialects.QuotePolicyReserved:
		q.IsReserved = db.IsReserved
	case dialects.QuotePolicyAlways:
	}
	db.quoter = q
}

// Firebird rejects the IDENTITY clause after PRIMARY KEY, so CreateTableSQL
// emits it in the right position instead. See CreateTableSQL.
func (db *firebird) AutoIncrStr() string { return "" }

func (db *firebird) SQLType(c *schemas.Column) string {
	switch t := c.SQLType.Name; t {
	case schemas.Bool:
		return "BOOLEAN"
	case schemas.BigInt:
		return "BIGINT"
	case schemas.SmallInt, schemas.TinyInt:
		return "SMALLINT"
	case schemas.Int, schemas.MediumInt:
		return "INTEGER"
	case schemas.UnsignedBit, schemas.UnsignedTinyInt, schemas.UnsignedSmallInt, schemas.UnsignedMediumInt, schemas.UnsignedInt:
		// Firebird has no unsigned integers, so use the next wider signed type
		return "BIGINT"
	case schemas.UnsignedBigInt:
		return "NUMERIC(20,0)"
	case schemas.UnsignedFloat:
		return "REAL"
	case schemas.Double:
		return "DOUBLE PRECISION"
	case schemas.Float:
		return "REAL"
	case schemas.TinyText, schemas.Text, schemas.NText, schemas.MediumText, schemas.LongText:
		// Firebird 5 removed TEXT as a column type, BLOB SUB_TYPE TEXT is the supported equivalent
		return "BLOB SUB_TYPE TEXT"
	case schemas.Char:
		return fmt.Sprintf("CHAR(%d)", c.Length)
	case schemas.Varchar:
		if c.Length == 0 || c.Length > 32767 {
			return "BLOB SUB_TYPE TEXT"
		}
		return fmt.Sprintf("VARCHAR(%d)", c.Length)
	case schemas.Date:
		return "DATE"
	case schemas.DateTime, schemas.SmallDateTime:
		return "TIMESTAMP"
	case schemas.Time:
		return "TIME"
	case schemas.Binary, schemas.Blob:
		return "BLOB"
	case schemas.Uuid:
		// Firebird's native UUID is a 16-byte binary type, but Gitea stores UUIDs as
		// their textual form, so keep that representation as MySQL does
		return "VARCHAR(40)"
	case schemas.VarBinary:
		// Firebird cannot index a BLOB, and a length is required to get a
		// fixed-size binary column, so use the octets charset of a VARCHAR
		if c.Length > 0 && c.Length <= 32765 {
			return fmt.Sprintf("VARCHAR(%d) CHARACTER SET OCTETS", c.Length)
		}
		return "BLOB"
	case schemas.MediumBlob, schemas.LongBlob, schemas.TinyBlob:
		// MySQL-style binary types, Firebird stores all of them as BLOB
		return "BLOB"
	case schemas.Decimal:
		// precision and scale are mandatory for NUMERIC, omitting them truncates stored values to integers
		precision, scale := c.Length, c.Length2
		if precision <= 0 {
			precision = 10
		}
		if scale <= 0 || scale >= precision {
			scale = 0
		}
		return fmt.Sprintf("NUMERIC(%d,%d)", precision, scale)
	}
	return strings.ToUpper(c.SQLType.Name)
}

func (db *firebird) ColumnTypeKind(t string) int {
	// Firebird reports internal type names (LONG, INT64, VARYING, ...), not DDL spellings
	switch strings.ToUpper(t) {
	case "TEXT", "BLOB", "VARCHAR", "CHAR", "VARYING":
		return schemas.TEXT_TYPE
	case "DATE", "TIME", "TIMESTAMP":
		return schemas.TIME_TYPE
	case "BOOLEAN", "INTEGER", "SMALLINT", "BIGINT", "LONG", "INT64", "SHORT",
		"REAL", "FLOAT", "DOUBLE PRECISION", "NUMERIC", "DECIMAL", "D_FLOAT":
		return schemas.NUMERIC_TYPE
	}
	return schemas.UNKNOW_TYPE
}

func (db *firebird) CreateTableSQL(ctx context.Context, queryer core.Queryer, table *schemas.Table, tableName string) (string, bool, error) {
	if tableName == "" {
		tableName = table.Name
	}
	quoter := db.Quoter()
	var b strings.Builder
	// Firebird has no CREATE TABLE IF NOT EXISTS, existence is the caller's precondition via IsTableExist
	b.WriteString("CREATE TABLE ")
	b.WriteString(quoter.Quote(tableName))
	b.WriteString(" (")

	cols := table.ColumnsSeq()
	for i, colName := range cols {
		col := table.GetColumn(colName)
		db.writeColumnDef(&b, col, col.IsPrimaryKey && len(table.PrimaryKeys) == 1)
		if i != len(cols)-1 {
			b.WriteString(", ")
		}
	}
	b.WriteString(")")
	return b.String(), false, nil
}

func (db *firebird) AddColumnSQL(tableName string, col *schemas.Column) string {
	var b strings.Builder
	b.WriteString("ALTER TABLE ")
	b.WriteString(db.Quoter().Quote(tableName))
	b.WriteString(" ADD ")
	db.writeColumnDef(&b, col, col.IsPrimaryKey)
	return b.String()
}

// writeColumnDef builds a column definition itself: Base's ColumnString suffixes
// nullable columns with a bare NULL, which Firebird rejects as a token.
func (db *firebird) writeColumnDef(b *strings.Builder, col *schemas.Column, includePrimaryKey bool) {
	b.WriteString(db.Quoter().Quote(col.Name))
	b.WriteByte(' ')
	b.WriteString(db.columnTypeSQL(col))
	// Firebird's grammar is "<type> GENERATED ... AS IDENTITY <constraints>", so NOT NULL
	// and PRIMARY KEY must follow the identity clause, not precede it
	if col.IsAutoIncrement {
		b.WriteString(" GENERATED BY DEFAULT AS IDENTITY")
	}
	// Gitea marks columns that fixtures omit with a default, so the DDL has to carry
	// it or an INSERT that leaves the column out fails the NOT NULL check
	if !col.DefaultIsEmpty {
		b.WriteString(" DEFAULT ")
		if col.Default == "" {
			b.WriteString("''")
		} else {
			b.WriteString(col.Default)
		}
	}
	if !col.Nullable {
		b.WriteString(" NOT NULL")
	}
	if includePrimaryKey {
		b.WriteString(" PRIMARY KEY")
	}
}

func (db *firebird) columnTypeSQL(col *schemas.Column) string {
	if col.IsAutoIncrement {
		if col.SQLType.Name == schemas.Int {
			return "INTEGER"
		}
		return "BIGINT"
	}
	return db.SQLType(col)
}

func (db *firebird) DropTableSQL(tableName string) (string, bool) {
	// checkIfExist=false so xorm gates on IsTableExist, Firebird has no DROP TABLE IF EXISTS
	return "DROP TABLE " + db.Quoter().Quote(tableName), false
}

func (db *firebird) ModifyColumnSQL(tableName string, col *schemas.Column) string {
	return fmt.Sprintf("ALTER TABLE %s ALTER %s TYPE %s",
		db.Quoter().Quote(tableName), db.Quoter().Quote(col.Name), db.SQLType(col))
}

func (db *firebird) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) { //nolint:revive // context-after-queryer matches the xorm dialects.Dialect signature
	return db.hasRecords(queryer, ctx,
		"SELECT COUNT(*) FROM rdb$relations WHERE TRIM(rdb$relation_name) = ? AND rdb$view_blr IS NULL",
		upperIdent(tableName))
}

func (db *firebird) IsColumnExist(queryer core.Queryer, ctx context.Context, tableName, colName string) (bool, error) { //nolint:revive // context-after-queryer matches the xorm dialects.Dialect signature
	return db.hasRecords(queryer, ctx,
		"SELECT COUNT(*) FROM rdb$relation_fields WHERE rdb$relation_name = ? AND TRIM(rdb$field_name) = ?",
		upperIdent(tableName), upperIdent(colName))
}

func (db *firebird) GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error) { //nolint:revive // context-after-queryer matches the xorm dialects.Dialect signature
	rows, err := queryer.QueryContext(ctx, `SELECT TRIM(rdb$relation_name) FROM rdb$relations
		WHERE rdb$view_blr IS NULL AND (rdb$system_flag IS NULL OR rdb$system_flag = 0)
		ORDER BY rdb$relation_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*schemas.Table, 0)
	for rows.Next() {
		t := schemas.NewEmptyTable()
		if err := rows.Scan(&t.Name); err != nil {
			return nil, err
		}
		// objects are created unquoted and fold to upper case, while the model metadata
		// is lower case, so the names are reported in the model's case like the other
		// dialects do, otherwise exact-name lookups miss every table
		t.Name = strings.ToLower(t.Name)
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (db *firebird) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) { //nolint:revive // context-after-queryer matches the xorm dialects.Dialect signature
	// Firebird 4+ has no RDB$RELATION_FIELDS.RDB$FIELD_TYPE: the type lives in RDB$FIELDS,
	// reached through RDB$FIELD_SOURCE. Sync derives the current column type through
	// Dialect.SQLType, so the internal codes are reconstructed into xorm type names first,
	// otherwise the internal spellings (INT64, VARYING) mismatch every declared type.
	// Identity is signalled by RDB$GENERATOR_NAME, there is no RDB$IDENTITY_TYPE.
	// RDB$DEFAULT_SOURCE is a text BLOB, it is cast to VARCHAR to scan it as a plain string.
	rows, err := queryer.QueryContext(ctx, `SELECT TRIM(rf.rdb$field_name), TRIM(tp.rdb$type_name),
		fd.rdb$field_type, fd.rdb$field_sub_type, fd.rdb$field_scale, fd.rdb$field_precision,
		fd.rdb$character_length, rf.rdb$null_flag, rf.rdb$generator_name,
		CAST(rf.rdb$default_source AS VARCHAR(256))
		FROM rdb$relation_fields rf
		JOIN rdb$fields fd ON fd.rdb$field_name = rf.rdb$field_source
		JOIN rdb$types tp ON tp.rdb$type = fd.rdb$field_type AND tp.rdb$field_name = 'RDB$FIELD_TYPE'
		WHERE rf.rdb$relation_name = ?
		ORDER BY rf.rdb$field_position`, upperIdent(tableName))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	colsSeq := make([]string, 0)
	colsMap := make(map[string]*schemas.Column)
	for rows.Next() {
		var name, rawType string
		var fieldType int
		var subType, scale, precision, charLength, nullFlag *int
		var generatorName, defaultSource *string
		if err := rows.Scan(&name, &rawType, &fieldType, &subType, &scale, &precision, &charLength, &nullFlag, &generatorName, &defaultSource); err != nil {
			return nil, nil, err
		}
		// column names fold to upper case like table names, the model's case is reported
		name = strings.ToLower(name)
		sqlType, length, length2 := firebirdColumnType(fieldType, subType, scale, precision, charLength, rawType)
		col := schemas.NewColumn(name, name, sqlType, length, length2, nullFlag == nil)
		col.IsAutoIncrement = generatorName != nil
		if defaultSource != nil {
			// the stored form is the literal DDL text "DEFAULT <value>"
			if def, ok := strings.CutPrefix(strings.TrimSpace(*defaultSource), "DEFAULT"); ok {
				col.Default = strings.TrimSpace(def)
				col.DefaultIsEmpty = false
			}
		}
		colsSeq = append(colsSeq, name)
		colsMap[name] = col
	}
	return colsSeq, colsMap, rows.Err()
}

// firebirdColumnType reconstructs the xorm type name, length and scale from Firebird's
// internal field codes, so that Dialect.SQLType(SQLType) reproduces the declared spelling.
func firebirdColumnType(fieldType int, subType, scale, precision, charLength *int, rawType string) (schemas.SQLType, int64, int64) {
	const (
		firebirdSubNumeric  = 1
		firebirdSubDecimal  = 2
		firebirdSubBlobText = 1
	)
	switch fieldType {
	case 7: // SHORT
		return schemas.SQLType{Name: schemas.SmallInt}, 0, 0
	case 8: // LONG
		return schemas.SQLType{Name: schemas.Int}, 0, 0
	case 10: // FLOAT
		return schemas.SQLType{Name: schemas.Float}, 0, 0
	case 12: // DATE
		return schemas.SQLType{Name: schemas.Date}, 0, 0
	case 13: // TIME
		return schemas.SQLType{Name: schemas.Time}, 0, 0
	case 14: // TEXT, a fixed-length character field
		return schemas.SQLType{Name: schemas.Char}, int64Value(charLength), 0
	case 35: // TIMESTAMP
		return schemas.SQLType{Name: schemas.DateTime}, 0, 0
	case 37: // VARYING
		return schemas.SQLType{Name: schemas.Varchar}, int64Value(charLength), 0
	case 23: // BOOLEAN
		return schemas.SQLType{Name: schemas.Bool}, 0, 0
	case 16, 27: // INT64 and DOUBLE, both carry the NUMERIC/DECIMAL sub code
		if subType != nil && (*subType == firebirdSubNumeric || *subType == firebirdSubDecimal) {
			return schemas.SQLType{Name: schemas.Decimal}, int64Value(precision), -int64Value(scale)
		}
		if fieldType == 16 {
			return schemas.SQLType{Name: schemas.BigInt}, 0, 0
		}
		return schemas.SQLType{Name: schemas.Double}, 0, 0
	case 261: // BLOB
		if subType != nil && *subType == firebirdSubBlobText {
			return schemas.SQLType{Name: schemas.Text}, 0, 0
		}
		return schemas.SQLType{Name: schemas.Blob}, 0, 0
	}
	// exotic internal codes (QUAD, DECFLOAT, INT128, ...) have no xorm spelling, keep the raw name
	return schemas.SQLType{Name: rawType}, 0, 0
}

func int64Value(v *int) int64 {
	if v == nil {
		return 0
	}
	return int64(*v)
}

func (db *firebird) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) { //nolint:revive // context-after-queryer matches the xorm dialects.Dialect signature
	rows, err := queryer.QueryContext(ctx, `SELECT TRIM(i.rdb$index_name), i.rdb$unique_flag, TRIM(s.rdb$field_name)
		FROM rdb$indices i
		JOIN rdb$index_segments s ON s.rdb$index_name = i.rdb$index_name
		WHERE i.rdb$relation_name = ?
		ORDER BY i.rdb$index_name, s.rdb$field_position`, upperIdent(tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*schemas.Index)
	for rows.Next() {
		var name, colName string
		var uniqueFlag int16
		if err := rows.Scan(&name, &uniqueFlag, &colName); err != nil {
			return nil, err
		}
		if strings.HasPrefix(name, "RDB$") { // skip constraint-backed system indexes
			continue
		}
		idxType := schemas.IndexType
		if uniqueFlag == 1 {
			idxType = schemas.UniqueType
		}
		// index names fold to upper case like table names, and the generated prefix is
		// stripped like the other dialects do so the model's exterior name is reported
		var isRegular bool
		if upperName := upperIdent(name); strings.HasPrefix(upperName, "IDX_"+upperIdent(tableName)) || strings.HasPrefix(upperName, "UQE_"+upperIdent(tableName)) {
			if remainder := name[5+len(tableName):]; remainder != "" {
				name = strings.ToLower(remainder)
				isRegular = true
			}
		}
		idx, ok := indexes[name]
		if !ok {
			idx = schemas.NewIndex(name, idxType)
			idx.Name = name
			idx.IsRegular = isRegular
			indexes[name] = idx
		}
		idx.Cols = append(idx.Cols, strings.ToLower(colName))
	}
	return indexes, rows.Err()
}

func (db *firebird) IndexCheckSQL(_, idxName string) (string, []any) {
	return "SELECT COUNT(*) FROM rdb$indices WHERE rdb$index_name = ?", []any{upperIdent(idxName)}
}

func (db *firebird) DropIndexSQL(tableName string, index *schemas.Index) string {
	name := index.Name
	if index.IsRegular {
		name = index.XName(tableName)
	}
	return "DROP INDEX " + db.Quoter().Quote(name)
}

func (db *firebird) hasRecords(queryer core.Queryer, ctx context.Context, query string, args ...any) (bool, error) { //nolint:revive // context-after-queryer matches the xorm dialects.Dialect signature
	rows, err := queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var n int64
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return false, err
		}
	}
	return n > 0, rows.Err()
}
