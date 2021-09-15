// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"xorm.io/xorm/convert"
	"xorm.io/xorm/core"
	"xorm.io/xorm/internal/utils"
	"xorm.io/xorm/schemas"
)

func init() {
	RegisterDriver("dm", &damengDriver{})
	RegisterDialect(schemas.DAMENG, func() Dialect {
		return &dameng{}
	})
}

var (
	damengReservedWords = map[string]bool{
		"ACCESS":                    true,
		"ACCOUNT":                   true,
		"ACTIVATE":                  true,
		"ADD":                       true,
		"ADMIN":                     true,
		"ADVISE":                    true,
		"AFTER":                     true,
		"ALL":                       true,
		"ALL_ROWS":                  true,
		"ALLOCATE":                  true,
		"ALTER":                     true,
		"ANALYZE":                   true,
		"AND":                       true,
		"ANY":                       true,
		"ARCHIVE":                   true,
		"ARCHIVELOG":                true,
		"ARRAY":                     true,
		"AS":                        true,
		"ASC":                       true,
		"AT":                        true,
		"AUDIT":                     true,
		"AUTHENTICATED":             true,
		"AUTHORIZATION":             true,
		"AUTOEXTEND":                true,
		"AUTOMATIC":                 true,
		"BACKUP":                    true,
		"BECOME":                    true,
		"BEFORE":                    true,
		"BEGIN":                     true,
		"BETWEEN":                   true,
		"BFILE":                     true,
		"BITMAP":                    true,
		"BLOB":                      true,
		"BLOCK":                     true,
		"BODY":                      true,
		"BY":                        true,
		"CACHE":                     true,
		"CACHE_INSTANCES":           true,
		"CANCEL":                    true,
		"CASCADE":                   true,
		"CAST":                      true,
		"CFILE":                     true,
		"CHAINED":                   true,
		"CHANGE":                    true,
		"CHAR":                      true,
		"CHAR_CS":                   true,
		"CHARACTER":                 true,
		"CHECK":                     true,
		"CHECKPOINT":                true,
		"CHOOSE":                    true,
		"CHUNK":                     true,
		"CLEAR":                     true,
		"CLOB":                      true,
		"CLONE":                     true,
		"CLOSE":                     true,
		"CLOSE_CACHED_OPEN_CURSORS": true,
		"CLUSTER":                   true,
		"COALESCE":                  true,
		"COLUMN":                    true,
		"COLUMNS":                   true,
		"COMMENT":                   true,
		"COMMIT":                    true,
		"COMMITTED":                 true,
		"COMPATIBILITY":             true,
		"COMPILE":                   true,
		"COMPLETE":                  true,
		"COMPOSITE_LIMIT":           true,
		"COMPRESS":                  true,
		"COMPUTE":                   true,
		"CONNECT":                   true,
		"CONNECT_TIME":              true,
		"CONSTRAINT":                true,
		"CONSTRAINTS":               true,
		"CONTENTS":                  true,
		"CONTINUE":                  true,
		"CONTROLFILE":               true,
		"CONVERT":                   true,
		"COST":                      true,
		"CPU_PER_CALL":              true,
		"CPU_PER_SESSION":           true,
		"CREATE":                    true,
		"CURRENT":                   true,
		"CURRENT_SCHEMA":            true,
		"CURREN_USER":               true,
		"CURSOR":                    true,
		"CYCLE":                     true,
		"DANGLING":                  true,
		"DATABASE":                  true,
		"DATAFILE":                  true,
		"DATAFILES":                 true,
		"DATAOBJNO":                 true,
		"DATE":                      true,
		"DBA":                       true,
		"DBHIGH":                    true,
		"DBLOW":                     true,
		"DBMAC":                     true,
		"DEALLOCATE":                true,
		"DEBUG":                     true,
		"DEC":                       true,
		"DECIMAL":                   true,
		"DECLARE":                   true,
		"DEFAULT":                   true,
		"DEFERRABLE":                true,
		"DEFERRED":                  true,
		"DEGREE":                    true,
		"DELETE":                    true,
		"DEREF":                     true,
		"DESC":                      true,
		"DIRECTORY":                 true,
		"DISABLE":                   true,
		"DISCONNECT":                true,
		"DISMOUNT":                  true,
		"DISTINCT":                  true,
		"DISTRIBUTED":               true,
		"DML":                       true,
		"DOUBLE":                    true,
		"DROP":                      true,
		"DUMP":                      true,
		"EACH":                      true,
		"ELSE":                      true,
		"ENABLE":                    true,
		"END":                       true,
		"ENFORCE":                   true,
		"ENTRY":                     true,
		"ESCAPE":                    true,
		"EXCEPT":                    true,
		"EXCEPTIONS":                true,
		"EXCHANGE":                  true,
		"EXCLUDING":                 true,
		"EXCLUSIVE":                 true,
		"EXECUTE":                   true,
		"EXISTS":                    true,
		"EXPIRE":                    true,
		"EXPLAIN":                   true,
		"EXTENT":                    true,
		"EXTENTS":                   true,
		"EXTERNALLY":                true,
		"FAILED_LOGIN_ATTEMPTS":     true,
		"FALSE":                     true,
		"FAST":                      true,
		"FILE":                      true,
		"FIRST_ROWS":                true,
		"FLAGGER":                   true,
		"FLOAT":                     true,
		"FLOB":                      true,
		"FLUSH":                     true,
		"FOR":                       true,
		"FORCE":                     true,
		"FOREIGN":                   true,
		"FREELIST":                  true,
		"FREELISTS":                 true,
		"FROM":                      true,
		"FULL":                      true,
		"FUNCTION":                  true,
		"GLOBAL":                    true,
		"GLOBALLY":                  true,
		"GLOBAL_NAME":               true,
		"GRANT":                     true,
		"GROUP":                     true,
		"GROUPS":                    true,
		"HASH":                      true,
		"HASHKEYS":                  true,
		"HAVING":                    true,
		"HEADER":                    true,
		"HEAP":                      true,
		"IDENTIFIED":                true,
		"IDGENERATORS":              true,
		"IDLE_TIME":                 true,
		"IF":                        true,
		"IMMEDIATE":                 true,
		"IN":                        true,
		"INCLUDING":                 true,
		"INCREMENT":                 true,
		"INDEX":                     true,
		"INDEXED":                   true,
		"INDEXES":                   true,
		"INDICATOR":                 true,
		"IND_PARTITION":             true,
		"INITIAL":                   true,
		"INITIALLY":                 true,
		"INITRANS":                  true,
		"INSERT":                    true,
		"INSTANCE":                  true,
		"INSTANCES":                 true,
		"INSTEAD":                   true,
		"INT":                       true,
		"INTEGER":                   true,
		"INTERMEDIATE":              true,
		"INTERSECT":                 true,
		"INTO":                      true,
		"IS":                        true,
		"ISOLATION":                 true,
		"ISOLATION_LEVEL":           true,
		"KEEP":                      true,
		"KEY":                       true,
		"KILL":                      true,
		"LABEL":                     true,
		"LAYER":                     true,
		"LESS":                      true,
		"LEVEL":                     true,
		"LIBRARY":                   true,
		"LIKE":                      true,
		"LIMIT":                     true,
		"LINK":                      true,
		"LIST":                      true,
		"LOB":                       true,
		"LOCAL":                     true,
		"LOCK":                      true,
		"LOCKED":                    true,
		"LOG":                       true,
		"LOGFILE":                   true,
		"LOGGING":                   true,
		"LOGICAL_READS_PER_CALL":    true,
		"LOGICAL_READS_PER_SESSION": true,
		"LONG":                      true,
		"MANAGE":                    true,
		"MASTER":                    true,
		"MAX":                       true,
		"MAXARCHLOGS":               true,
		"MAXDATAFILES":              true,
		"MAXEXTENTS":                true,
		"MAXINSTANCES":              true,
		"MAXLOGFILES":               true,
		"MAXLOGHISTORY":             true,
		"MAXLOGMEMBERS":             true,
		"MAXSIZE":                   true,
		"MAXTRANS":                  true,
		"MAXVALUE":                  true,
		"MIN":                       true,
		"MEMBER":                    true,
		"MINIMUM":                   true,
		"MINEXTENTS":                true,
		"MINUS":                     true,
		"MINVALUE":                  true,
		"MLSLABEL":                  true,
		"MLS_LABEL_FORMAT":          true,
		"MODE":                      true,
		"MODIFY":                    true,
		"MOUNT":                     true,
		"MOVE":                      true,
		"MTS_DISPATCHERS":           true,
		"MULTISET":                  true,
		"NATIONAL":                  true,
		"NCHAR":                     true,
		"NCHAR_CS":                  true,
		"NCLOB":                     true,
		"NEEDED":                    true,
		"NESTED":                    true,
		"NETWORK":                   true,
		"NEW":                       true,
		"NEXT":                      true,
		"NOARCHIVELOG":              true,
		"NOAUDIT":                   true,
		"NOCACHE":                   true,
		"NOCOMPRESS":                true,
		"NOCYCLE":                   true,
		"NOFORCE":                   true,
		"NOLOGGING":                 true,
		"NOMAXVALUE":                true,
		"NOMINVALUE":                true,
		"NONE":                      true,
		"NOORDER":                   true,
		"NOOVERRIDE":                true,
		"NOPARALLEL":                true,
		"NOREVERSE":                 true,
		"NORMAL":                    true,
		"NOSORT":                    true,
		"NOT":                       true,
		"NOTHING":                   true,
		"NOWAIT":                    true,
		"NULL":                      true,
		"NUMBER":                    true,
		"NUMERIC":                   true,
		"NVARCHAR2":                 true,
		"OBJECT":                    true,
		"OBJNO":                     true,
		"OBJNO_REUSE":               true,
		"OF":                        true,
		"OFF":                       true,
		"OFFLINE":                   true,
		"OID":                       true,
		"OIDINDEX":                  true,
		"OLD":                       true,
		"ON":                        true,
		"ONLINE":                    true,
		"ONLY":                      true,
		"OPCODE":                    true,
		"OPEN":                      true,
		"OPTIMAL":                   true,
		"OPTIMIZER_GOAL":            true,
		"OPTION":                    true,
		"OR":                        true,
		"ORDER":                     true,
		"ORGANIZATION":              true,
		"OSLABEL":                   true,
		"OVERFLOW":                  true,
		"OWN":                       true,
		"PACKAGE":                   true,
		"PARALLEL":                  true,
		"PARTITION":                 true,
		"PASSWORD":                  true,
		"PASSWORD_GRACE_TIME":       true,
		"PASSWORD_LIFE_TIME":        true,
		"PASSWORD_LOCK_TIME":        true,
		"PASSWORD_REUSE_MAX":        true,
		"PASSWORD_REUSE_TIME":       true,
		"PASSWORD_VERIFY_FUNCTION":  true,
		"PCTFREE":                   true,
		"PCTINCREASE":               true,
		"PCTTHRESHOLD":              true,
		"PCTUSED":                   true,
		"PCTVERSION":                true,
		"PERCENT":                   true,
		"PERMANENT":                 true,
		"PLAN":                      true,
		"PLSQL_DEBUG":               true,
		"POST_TRANSACTION":          true,
		"PRECISION":                 true,
		"PRESERVE":                  true,
		"PRIMARY":                   true,
		"PRIOR":                     true,
		"PRIVATE":                   true,
		"PRIVATE_SGA":               true,
		"PRIVILEGE":                 true,
		"PRIVILEGES":                true,
		"PROCEDURE":                 true,
		"PROFILE":                   true,
		"PUBLIC":                    true,
		"PURGE":                     true,
		"QUEUE":                     true,
		"QUOTA":                     true,
		"RANGE":                     true,
		"RAW":                       true,
		"RBA":                       true,
		"READ":                      true,
		"READUP":                    true,
		"REAL":                      true,
		"REBUILD":                   true,
		"RECOVER":                   true,
		"RECOVERABLE":               true,
		"RECOVERY":                  true,
		"REF":                       true,
		"REFERENCES":                true,
		"REFERENCING":               true,
		"REFRESH":                   true,
		"RENAME":                    true,
		"REPLACE":                   true,
		"RESET":                     true,
		"RESETLOGS":                 true,
		"RESIZE":                    true,
		"RESOURCE":                  true,
		"RESTRICTED":                true,
		"RETURN":                    true,
		"RETURNING":                 true,
		"REUSE":                     true,
		"REVERSE":                   true,
		"REVOKE":                    true,
		"ROLE":                      true,
		"ROLES":                     true,
		"ROLLBACK":                  true,
		"ROW":                       true,
		"ROWID":                     true,
		"ROWNUM":                    true,
		"ROWS":                      true,
		"RULE":                      true,
		"SAMPLE":                    true,
		"SAVEPOINT":                 true,
		"SB4":                       true,
		"SCAN_INSTANCES":            true,
		"SCHEMA":                    true,
		"SCN":                       true,
		"SCOPE":                     true,
		"SD_ALL":                    true,
		"SD_INHIBIT":                true,
		"SD_SHOW":                   true,
		"SEGMENT":                   true,
		"SEG_BLOCK":                 true,
		"SEG_FILE":                  true,
		"SELECT":                    true,
		"SEQUENCE":                  true,
		"SERIALIZABLE":              true,
		"SESSION":                   true,
		"SESSION_CACHED_CURSORS":    true,
		"SESSIONS_PER_USER":         true,
		"SET":                       true,
		"SHARE":                     true,
		"SHARED":                    true,
		"SHARED_POOL":               true,
		"SHRINK":                    true,
		"SIZE":                      true,
		"SKIP":                      true,
		"SKIP_UNUSABLE_INDEXES":     true,
		"SMALLINT":                  true,
		"SNAPSHOT":                  true,
		"SOME":                      true,
		"SORT":                      true,
		"SPECIFICATION":             true,
		"SPLIT":                     true,
		"SQL_TRACE":                 true,
		"STANDBY":                   true,
		"START":                     true,
		"STATEMENT_ID":              true,
		"STATISTICS":                true,
		"STOP":                      true,
		"STORAGE":                   true,
		"STORE":                     true,
		"STRUCTURE":                 true,
		"SUCCESSFUL":                true,
		"SWITCH":                    true,
		"SYS_OP_ENFORCE_NOT_NULL$":  true,
		"SYS_OP_NTCIMG$":            true,
		"SYNONYM":                   true,
		"SYSDATE":                   true,
		"SYSDBA":                    true,
		"SYSOPER":                   true,
		"SYSTEM":                    true,
		"TABLE":                     true,
		"TABLES":                    true,
		"TABLESPACE":                true,
		"TABLESPACE_NO":             true,
		"TABNO":                     true,
		"TEMPORARY":                 true,
		"THAN":                      true,
		"THE":                       true,
		"THEN":                      true,
		"THREAD":                    true,
		"TIMESTAMP":                 true,
		"TIME":                      true,
		"TO":                        true,
		"TOPLEVEL":                  true,
		"TRACE":                     true,
		"TRACING":                   true,
		"TRANSACTION":               true,
		"TRANSITIONAL":              true,
		"TRIGGER":                   true,
		"TRIGGERS":                  true,
		"TRUE":                      true,
		"TRUNCATE":                  true,
		"TX":                        true,
		"TYPE":                      true,
		"UB2":                       true,
		"UBA":                       true,
		"UID":                       true,
		"UNARCHIVED":                true,
		"UNDO":                      true,
		"UNION":                     true,
		"UNIQUE":                    true,
		"UNLIMITED":                 true,
		"UNLOCK":                    true,
		"UNRECOVERABLE":             true,
		"UNTIL":                     true,
		"UNUSABLE":                  true,
		"UNUSED":                    true,
		"UPDATABLE":                 true,
		"UPDATE":                    true,
		"USAGE":                     true,
		"USE":                       true,
		"USER":                      true,
		"USING":                     true,
		"VALIDATE":                  true,
		"VALIDATION":                true,
		"VALUE":                     true,
		"VALUES":                    true,
		"VARCHAR":                   true,
		"VARCHAR2":                  true,
		"VARYING":                   true,
		"VIEW":                      true,
		"WHEN":                      true,
		"WHENEVER":                  true,
		"WHERE":                     true,
		"WITH":                      true,
		"WITHOUT":                   true,
		"WORK":                      true,
		"WRITE":                     true,
		"WRITEDOWN":                 true,
		"WRITEUP":                   true,
		"XID":                       true,
		"YEAR":                      true,
		"ZONE":                      true,
	}

	damengQuoter = schemas.Quoter{
		Prefix:     '"',
		Suffix:     '"',
		IsReserved: schemas.AlwaysReserve,
	}
)

type dameng struct {
	Base
}

func (db *dameng) Init(uri *URI) error {
	db.quoter = damengQuoter
	return db.Base.Init(db, uri)
}

func (db *dameng) Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT * FROM V$VERSION") // select id_code
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var version string
	if !rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return nil, errors.New("unknow version")
	}

	if err := rows.Scan(&version); err != nil {
		return nil, err
	}
	return &schemas.Version{
		Number: version,
	}, nil
}

func (db *dameng) Features() *DialectFeatures {
	return &DialectFeatures{
		AutoincrMode: SequenceAutoincrMode,
	}
}

// DropIndexSQL returns a SQL to drop index
func (db *dameng) DropIndexSQL(tableName string, index *schemas.Index) string {
	quote := db.dialect.Quoter().Quote
	var name string
	if index.IsRegular {
		name = index.XName(tableName)
	} else {
		name = index.Name
	}
	return fmt.Sprintf("DROP INDEX %v", quote(name))
}

func (db *dameng) SQLType(c *schemas.Column) string {
	var res string
	switch t := c.SQLType.Name; t {
	case schemas.TinyInt, "BYTE":
		return "TINYINT"
	case schemas.SmallInt, schemas.MediumInt, schemas.Int, schemas.Integer, schemas.UnsignedTinyInt:
		return "INTEGER"
	case schemas.BigInt,
		schemas.UnsignedBigInt, schemas.UnsignedBit, schemas.UnsignedInt,
		schemas.Serial, schemas.BigSerial:
		return "BIGINT"
	case schemas.Bit, schemas.Bool, schemas.Boolean:
		return schemas.Bit
	case schemas.Uuid:
		res = schemas.Varchar
		c.Length = 40
	case schemas.Binary:
		if c.Length == 0 {
			return schemas.Binary + "(MAX)"
		}
	case schemas.VarBinary, schemas.Blob, schemas.TinyBlob, schemas.MediumBlob, schemas.LongBlob, schemas.Bytea:
		return schemas.VarBinary
	case schemas.Date:
		return schemas.Date
	case schemas.Time:
		if c.Length > 0 {
			return fmt.Sprintf("%s(%d)", schemas.Time, c.Length)
		}
		return schemas.Time
	case schemas.DateTime, schemas.TimeStamp:
		res = schemas.TimeStamp
	case schemas.TimeStampz:
		if c.Length > 0 {
			return fmt.Sprintf("TIMESTAMP(%d) WITH TIME ZONE", c.Length)
		}
		return "TIMESTAMP WITH TIME ZONE"
	case schemas.Float:
		res = "FLOAT"
	case schemas.Real, schemas.Double:
		res = "REAL"
	case schemas.Numeric, schemas.Decimal, "NUMBER":
		res = "NUMERIC"
	case schemas.Text, schemas.Json:
		return "TEXT"
	case schemas.MediumText, schemas.LongText:
		res = "CLOB"
	case schemas.Char, schemas.Varchar, schemas.TinyText:
		res = "VARCHAR2"
	default:
		res = t
	}

	hasLen1 := (c.Length > 0)
	hasLen2 := (c.Length2 > 0)

	if hasLen2 {
		res += "(" + strconv.Itoa(c.Length) + "," + strconv.Itoa(c.Length2) + ")"
	} else if hasLen1 {
		res += "(" + strconv.Itoa(c.Length) + ")"
	}
	return res
}

func (db *dameng) ColumnTypeKind(t string) int {
	switch strings.ToUpper(t) {
	case "DATE":
		return schemas.TIME_TYPE
	case "CHAR", "NCHAR", "VARCHAR", "VARCHAR2", "NVARCHAR2", "LONG", "CLOB", "NCLOB":
		return schemas.TEXT_TYPE
	case "NUMBER":
		return schemas.NUMERIC_TYPE
	case "BLOB":
		return schemas.BLOB_TYPE
	default:
		return schemas.UNKNOW_TYPE
	}
}

func (db *dameng) AutoIncrStr() string {
	return "IDENTITY"
}

func (db *dameng) IsReserved(name string) bool {
	_, ok := damengReservedWords[strings.ToUpper(name)]
	return ok
}

func (db *dameng) DropTableSQL(tableName string) (string, bool) {
	return fmt.Sprintf("DROP TABLE %s", db.quoter.Quote(tableName)), false
}

// ModifyColumnSQL returns a SQL to modify SQL
func (db *dameng) ModifyColumnSQL(tableName string, col *schemas.Column) string {
	s, _ := ColumnString(db.dialect, col, false)
	return fmt.Sprintf("ALTER TABLE %s MODIFY %s", db.quoter.Quote(tableName), s)
}

func (db *dameng) CreateTableSQL(ctx context.Context, queryer core.Queryer, table *schemas.Table, tableName string) (string, bool, error) {
	if tableName == "" {
		tableName = table.Name
	}

	quoter := db.Quoter()
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	quoter.QuoteTo(&b, tableName)
	b.WriteString(" (")

	pkList := table.PrimaryKeys

	for i, colName := range table.ColumnsSeq() {
		col := table.GetColumn(colName)
		if col.SQLType.IsBool() && !col.DefaultIsEmpty {
			if col.Default == "true" {
				col.Default = "1"
			} else if col.Default == "false" {
				col.Default = "0"
			}
		}

		s, _ := ColumnString(db, col, false)
		b.WriteString(s)
		if i != len(table.ColumnsSeq())-1 {
			b.WriteString(", ")
		}
	}

	if len(pkList) > 0 {
		if len(table.ColumnsSeq()) > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("CONSTRAINT PK_%s PRIMARY KEY (", tableName))
		quoter.JoinWrite(&b, pkList, ",")
		b.WriteString(")")
	}
	b.WriteString(")")

	return b.String(), false, nil
}

func (db *dameng) SetQuotePolicy(quotePolicy QuotePolicy) {
	switch quotePolicy {
	case QuotePolicyNone:
		var q = damengQuoter
		q.IsReserved = schemas.AlwaysNoReserve
		db.quoter = q
	case QuotePolicyReserved:
		var q = damengQuoter
		q.IsReserved = db.IsReserved
		db.quoter = q
	case QuotePolicyAlways:
		fallthrough
	default:
		db.quoter = damengQuoter
	}
}

func (db *dameng) IndexCheckSQL(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{tableName, idxName}
	return `SELECT INDEX_NAME FROM USER_INDEXES ` +
		`WHERE TABLE_NAME = ? AND INDEX_NAME = ?`, args
}

func (db *dameng) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) {
	return db.HasRecords(queryer, ctx, `SELECT table_name FROM user_tables WHERE table_name = ?`, tableName)
}

func (db *dameng) IsSequenceExist(ctx context.Context, queryer core.Queryer, seqName string) (bool, error) {
	var cnt int
	rows, err := queryer.QueryContext(ctx, "SELECT COUNT(*) FROM user_sequences WHERE sequence_name = ?", seqName)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if rows.Err() != nil {
			return false, rows.Err()
		}
		return false, errors.New("query sequence failed")
	}

	if err := rows.Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (db *dameng) IsColumnExist(queryer core.Queryer, ctx context.Context, tableName, colName string) (bool, error) {
	args := []interface{}{tableName, colName}
	query := "SELECT column_name FROM USER_TAB_COLUMNS WHERE table_name = ?" +
		" AND column_name = ?"
	return db.HasRecords(queryer, ctx, query, args...)
}

var _ sql.Scanner = &dmClobScanner{}

type dmClobScanner struct {
	valid bool
	data  string
}

type dmClobObject interface {
	GetLength() (int64, error)
	ReadString(int, int) (string, error)
}

//var _ dmClobObject = &dm.DmClob{}

func (d *dmClobScanner) Scan(data interface{}) error {
	if data == nil {
		return nil
	}

	switch t := data.(type) {
	case dmClobObject: // *dm.DmClob
		if t == nil {
			return nil
		}
		l, err := t.GetLength()
		if err != nil {
			return err
		}
		if l == 0 {
			d.valid = true
			return nil
		}
		d.data, err = t.ReadString(1, int(l))
		if err != nil {
			return err
		}
		d.valid = true
		return nil
	case []byte:
		if t == nil {
			return nil
		}
		d.data = string(t)
		d.valid = true
		return nil
	default:
		return fmt.Errorf("cannot convert %T as dmClobScanner", data)
	}
}

func addSingleQuote(name string) string {
	if len(name) < 2 {
		return name
	}
	if name[0] == '\'' && name[len(name)-1] == '\'' {
		return name
	}
	return fmt.Sprintf("'%s'", name)
}

func (db *dameng) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) {
	s := `select   column_name   from   user_cons_columns   
  where   constraint_name   =   (select   constraint_name   from   user_constraints   
			  where   table_name   =   ?  and   constraint_type   ='P')`
	rows, err := queryer.QueryContext(ctx, s, tableName)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var pkNames []string
	for rows.Next() {
		var pkName string
		err = rows.Scan(&pkName)
		if err != nil {
			return nil, nil, err
		}
		pkNames = append(pkNames, pkName)
	}
	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}
	rows.Close()

	s = `SELECT USER_TAB_COLS.COLUMN_NAME, USER_TAB_COLS.DATA_DEFAULT, USER_TAB_COLS.DATA_TYPE, USER_TAB_COLS.DATA_LENGTH, 
		USER_TAB_COLS.data_precision, USER_TAB_COLS.data_scale, USER_TAB_COLS.NULLABLE,
		user_col_comments.comments
		FROM USER_TAB_COLS 
		LEFT JOIN user_col_comments on user_col_comments.TABLE_NAME=USER_TAB_COLS.TABLE_NAME 
		AND user_col_comments.COLUMN_NAME=USER_TAB_COLS.COLUMN_NAME
		WHERE USER_TAB_COLS.table_name = ?`
	rows, err = queryer.QueryContext(ctx, s, tableName)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := make(map[string]*schemas.Column)
	colSeq := make([]string, 0)
	for rows.Next() {
		col := new(schemas.Column)
		col.Indexes = make(map[string]int)

		var colDefault dmClobScanner
		var colName, nullable, dataType, dataPrecision, comment sql.NullString
		var dataScale, dataLen sql.NullInt64

		err = rows.Scan(&colName, &colDefault, &dataType, &dataLen, &dataPrecision,
			&dataScale, &nullable, &comment)
		if err != nil {
			return nil, nil, err
		}

		if !colName.Valid {
			return nil, nil, errors.New("column name is nil")
		}

		col.Name = strings.Trim(colName.String, `" `)
		if colDefault.valid {
			col.Default = colDefault.data
		} else {
			col.DefaultIsEmpty = true
		}

		if nullable.String == "Y" {
			col.Nullable = true
		} else {
			col.Nullable = false
		}

		if !comment.Valid {
			col.Comment = comment.String
		}
		if utils.IndexSlice(pkNames, col.Name) > -1 {
			col.IsPrimaryKey = true
			has, err := db.HasRecords(queryer, ctx, "SELECT * FROM USER_SEQUENCES WHERE SEQUENCE_NAME = ?", utils.SeqName(tableName))
			if err != nil {
				return nil, nil, err
			}
			if has {
				col.IsAutoIncrement = true
			}
		}

		var (
			ignore     bool
			dt         string
			len1, len2 int
		)

		dts := strings.Split(dataType.String, "(")
		dt = dts[0]
		if len(dts) > 1 {
			lens := strings.Split(dts[1][:len(dts[1])-1], ",")
			if len(lens) > 1 {
				len1, _ = strconv.Atoi(lens[0])
				len2, _ = strconv.Atoi(lens[1])
			} else {
				len1, _ = strconv.Atoi(lens[0])
			}
		}

		switch dt {
		case "VARCHAR2":
			col.SQLType = schemas.SQLType{Name: "VARCHAR2", DefaultLength: len1, DefaultLength2: len2}
		case "VARCHAR":
			col.SQLType = schemas.SQLType{Name: schemas.Varchar, DefaultLength: len1, DefaultLength2: len2}
		case "TIMESTAMP WITH TIME ZONE":
			col.SQLType = schemas.SQLType{Name: schemas.TimeStampz, DefaultLength: 0, DefaultLength2: 0}
		case "NUMBER":
			col.SQLType = schemas.SQLType{Name: "NUMBER", DefaultLength: len1, DefaultLength2: len2}
		case "LONG", "LONG RAW", "NCLOB", "CLOB", "TEXT":
			col.SQLType = schemas.SQLType{Name: schemas.Text, DefaultLength: 0, DefaultLength2: 0}
		case "RAW":
			col.SQLType = schemas.SQLType{Name: schemas.Binary, DefaultLength: 0, DefaultLength2: 0}
		case "ROWID":
			col.SQLType = schemas.SQLType{Name: schemas.Varchar, DefaultLength: 18, DefaultLength2: 0}
		case "AQ$_SUBSCRIBERS":
			ignore = true
		default:
			col.SQLType = schemas.SQLType{Name: strings.ToUpper(dt), DefaultLength: len1, DefaultLength2: len2}
		}

		if ignore {
			continue
		}

		if _, ok := schemas.SqlTypes[col.SQLType.Name]; !ok {
			return nil, nil, fmt.Errorf("unknown colType %v %v", dataType.String, col.SQLType)
		}

		if col.SQLType.Name == "TIMESTAMP" {
			col.Length = int(dataScale.Int64)
		} else {
			col.Length = int(dataLen.Int64)
		}

		if col.SQLType.IsTime() {
			if !col.DefaultIsEmpty && !strings.EqualFold(col.Default, "CURRENT_TIMESTAMP") {
				col.Default = addSingleQuote(col.Default)
			}
		}
		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}
	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}

	return colSeq, cols, nil
}

func (db *dameng) GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error) {
	s := "SELECT table_name FROM user_tables WHERE temporary = 'N' AND table_name NOT LIKE ?"
	args := []interface{}{strings.ToUpper(db.uri.User), "%$%"}

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*schemas.Table, 0)
	for rows.Next() {
		table := schemas.NewEmptyTable()
		err = rows.Scan(&table.Name)
		if err != nil {
			return nil, err
		}

		tables = append(tables, table)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return tables, nil
}

func (db *dameng) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) {
	args := []interface{}{tableName, tableName}
	s := "SELECT t.column_name,i.uniqueness,i.index_name FROM user_ind_columns t,user_indexes i " +
		"WHERE t.index_name = i.index_name and t.table_name = i.table_name and t.table_name =?" +
		" AND t.index_name not in (SELECT index_name FROM ALL_CONSTRAINTS WHERE CONSTRAINT_TYPE='P' AND table_name = ?)"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*schemas.Index)
	for rows.Next() {
		var indexType int
		var indexName, colName, uniqueness string

		err = rows.Scan(&colName, &uniqueness, &indexName)
		if err != nil {
			return nil, err
		}

		indexName = strings.Trim(indexName, `" `)

		var isRegular bool
		if strings.HasPrefix(indexName, "IDX_"+tableName) || strings.HasPrefix(indexName, "UQE_"+tableName) {
			indexName = indexName[5+len(tableName):]
			isRegular = true
		}

		if uniqueness == "UNIQUE" {
			indexType = schemas.UniqueType
		} else {
			indexType = schemas.IndexType
		}

		var index *schemas.Index
		var ok bool
		if index, ok = indexes[indexName]; !ok {
			index = new(schemas.Index)
			index.Type = indexType
			index.Name = indexName
			index.IsRegular = isRegular
			indexes[indexName] = index
		}
		index.AddColumn(colName)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return indexes, nil
}

func (db *dameng) Filters() []Filter {
	return []Filter{}
}

type damengDriver struct {
	baseDriver
}

// Features return features
func (d *damengDriver) Features() *DriverFeatures {
	return &DriverFeatures{
		SupportReturnInsertedID: false,
	}
}

// Parse parse the datasource
// dm://userName:password@ip:port
func (d *damengDriver) Parse(driverName, dataSourceName string) (*URI, error) {
	u, err := url.Parse(dataSourceName)
	if err != nil {
		return nil, err
	}

	if u.User == nil {
		return nil, errors.New("user/password needed")
	}

	passwd, _ := u.User.Password()
	return &URI{
		DBType: schemas.DAMENG,
		Proto:  u.Scheme,
		Host:   u.Hostname(),
		Port:   u.Port(),
		DBName: u.User.Username(),
		User:   u.User.Username(),
		Passwd: passwd,
	}, nil
}

func (d *damengDriver) GenScanResult(colType string) (interface{}, error) {
	switch colType {
	case "CHAR", "NCHAR", "VARCHAR", "VARCHAR2", "NVARCHAR2", "LONG", "CLOB", "NCLOB":
		var s sql.NullString
		return &s, nil
	case "NUMBER":
		var s sql.NullString
		return &s, nil
	case "BIGINT":
		var s sql.NullInt64
		return &s, nil
	case "INTEGER":
		var s sql.NullInt32
		return &s, nil
	case "DATE", "TIMESTAMP":
		var s sql.NullString
		return &s, nil
	case "BLOB":
		var r sql.RawBytes
		return &r, nil
	case "FLOAT":
		var s sql.NullFloat64
		return &s, nil
	default:
		var r sql.RawBytes
		return &r, nil
	}
}

func (d *damengDriver) Scan(ctx *ScanContext, rows *core.Rows, types []*sql.ColumnType, vv ...interface{}) error {
	var scanResults = make([]interface{}, 0, len(types))
	var replaces = make([]bool, 0, len(types))
	var err error
	for i, v := range vv {
		var replaced bool
		var scanResult interface{}
		switch types[i].DatabaseTypeName() {
		case "CLOB", "TEXT":
			scanResult = &dmClobScanner{}
			replaced = true
		case "TIMESTAMP":
			scanResult = &sql.NullString{}
			replaced = true
		default:
			scanResult = v
		}

		scanResults = append(scanResults, scanResult)
		replaces = append(replaces, replaced)
	}

	if err = rows.Scan(scanResults...); err != nil {
		return err
	}

	for i, replaced := range replaces {
		if replaced {
			switch t := scanResults[i].(type) {
			case *dmClobScanner:
				var d interface{}
				if t.valid {
					d = t.data
				} else {
					d = nil
				}
				if err := convert.Assign(vv[i], d, ctx.DBLocation, ctx.UserLocation); err != nil {
					return err
				}
			default:
				switch types[i].DatabaseTypeName() {
				case "TIMESTAMP":
					ns := t.(*sql.NullString)
					if !ns.Valid {
						return nil
					}
					s := ns.String
					fields := strings.Split(s, "+")
					if err := convert.Assign(vv[i], strings.Replace(fields[0], "T", " ", -1), ctx.DBLocation, ctx.UserLocation); err != nil {
						return err
					}
				default:
					return fmt.Errorf("don't support convert %T to %T", t, vv[i])
				}
			}
		}
	}

	return nil
}
