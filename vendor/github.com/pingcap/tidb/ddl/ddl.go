// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package ddl

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/evaluator"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/types"
	"github.com/twinj/uuid"
)

// DDL is responsible for updating schema in data store and maintaining in-memory InfoSchema cache.
type DDL interface {
	CreateSchema(ctx context.Context, name model.CIStr, charsetInfo *ast.CharsetOpt) error
	DropSchema(ctx context.Context, schema model.CIStr) error
	CreateTable(ctx context.Context, ident ast.Ident, cols []*ast.ColumnDef,
		constrs []*ast.Constraint, options []*ast.TableOption) error
	DropTable(ctx context.Context, tableIdent ast.Ident) (err error)
	CreateIndex(ctx context.Context, tableIdent ast.Ident, unique bool, indexName model.CIStr,
		columnNames []*ast.IndexColName) error
	DropIndex(ctx context.Context, tableIdent ast.Ident, indexName model.CIStr) error
	GetInformationSchema() infoschema.InfoSchema
	AlterTable(ctx context.Context, tableIdent ast.Ident, spec []*ast.AlterTableSpec) error
	// SetLease will reset the lease time for online DDL change,
	// it's a very dangerous function and you must guarantee that all servers have the same lease time.
	SetLease(lease time.Duration)
	// GetLease returns current schema lease time.
	GetLease() time.Duration
	// Stats returns the DDL statistics.
	Stats() (map[string]interface{}, error)
	// GetScope gets the status variables scope.
	GetScope(status string) variable.ScopeFlag
	// Stop stops DDL worker.
	Stop() error
	// Start starts DDL worker.
	Start() error
}

type ddl struct {
	m sync.RWMutex

	infoHandle *infoschema.Handle
	hook       Callback
	store      kv.Storage
	// schema lease seconds.
	lease        time.Duration
	uuid         string
	ddlJobCh     chan struct{}
	ddlJobDoneCh chan struct{}
	// drop database/table job runs in the background.
	bgJobCh chan struct{}
	// reorgDoneCh is for reorganization, if the reorganization job is done,
	// we will use this channel to notify outer.
	// TODO: now we use goroutine to simulate reorganization jobs, later we may
	// use a persistent job list.
	reorgDoneCh chan error

	quitCh chan struct{}
	wait   sync.WaitGroup
}

// NewDDL creates a new DDL.
func NewDDL(store kv.Storage, infoHandle *infoschema.Handle, hook Callback, lease time.Duration) DDL {
	return newDDL(store, infoHandle, hook, lease)
}

func newDDL(store kv.Storage, infoHandle *infoschema.Handle, hook Callback, lease time.Duration) *ddl {
	if hook == nil {
		hook = &BaseCallback{}
	}

	d := &ddl{
		infoHandle:   infoHandle,
		hook:         hook,
		store:        store,
		lease:        lease,
		uuid:         uuid.NewV4().String(),
		ddlJobCh:     make(chan struct{}, 1),
		ddlJobDoneCh: make(chan struct{}, 1),
		bgJobCh:      make(chan struct{}, 1),
	}

	d.start()

	variable.RegisterStatistics(d)

	return d
}

func (d *ddl) Stop() error {
	d.m.Lock()
	defer d.m.Unlock()

	d.close()

	err := kv.RunInNewTxn(d.store, true, func(txn kv.Transaction) error {
		t := meta.NewMeta(txn)
		owner, err1 := t.GetDDLJobOwner()
		if err1 != nil {
			return errors.Trace(err1)
		}
		if owner == nil || owner.OwnerID != d.uuid {
			return nil
		}

		// ddl job's owner is me, clean it so other servers can compete for it quickly.
		return t.SetDDLJobOwner(&model.Owner{})
	})
	if err != nil {
		return errors.Trace(err)
	}

	err = kv.RunInNewTxn(d.store, true, func(txn kv.Transaction) error {
		t := meta.NewMeta(txn)
		owner, err1 := t.GetBgJobOwner()
		if err1 != nil {
			return errors.Trace(err1)
		}
		if owner == nil || owner.OwnerID != d.uuid {
			return nil
		}

		// background job's owner is me, clean it so other servers can compete for it quickly.
		return t.SetBgJobOwner(&model.Owner{})
	})

	return errors.Trace(err)
}

func (d *ddl) Start() error {
	d.m.Lock()
	defer d.m.Unlock()

	if !d.isClosed() {
		return nil
	}

	d.start()

	return nil
}

func (d *ddl) start() {
	d.quitCh = make(chan struct{})
	d.wait.Add(2)
	go d.onBackgroundWorker()
	go d.onDDLWorker()
	// for every start, we will send a fake job to let worker
	// check owner first and try to find whether a job exists and run.
	asyncNotify(d.ddlJobCh)
	asyncNotify(d.bgJobCh)
}

func (d *ddl) close() {
	if d.isClosed() {
		return
	}

	close(d.quitCh)

	d.wait.Wait()
}

func (d *ddl) isClosed() bool {
	select {
	case <-d.quitCh:
		return true
	default:
		return false
	}
}

func (d *ddl) SetLease(lease time.Duration) {
	d.m.Lock()
	defer d.m.Unlock()

	if lease == d.lease {
		return
	}

	log.Warnf("[ddl] change schema lease %s -> %s", d.lease, lease)

	if d.isClosed() {
		// if already closed, just set lease and return
		d.lease = lease
		return
	}

	// close the running worker and start again
	d.close()
	d.lease = lease
	d.start()
}

func (d *ddl) GetLease() time.Duration {
	d.m.RLock()
	lease := d.lease
	d.m.RUnlock()
	return lease
}

func (d *ddl) GetInformationSchema() infoschema.InfoSchema {
	return d.infoHandle.Get()
}

func (d *ddl) genGlobalID() (int64, error) {
	var globalID int64
	err := kv.RunInNewTxn(d.store, true, func(txn kv.Transaction) error {
		var err error
		globalID, err = meta.NewMeta(txn).GenGlobalID()
		return errors.Trace(err)
	})

	return globalID, errors.Trace(err)
}

func (d *ddl) CreateSchema(ctx context.Context, schema model.CIStr, charsetInfo *ast.CharsetOpt) (err error) {
	is := d.GetInformationSchema()
	_, ok := is.SchemaByName(schema)
	if ok {
		return errors.Trace(infoschema.DatabaseExists)
	}

	schemaID, err := d.genGlobalID()
	if err != nil {
		return errors.Trace(err)
	}
	dbInfo := &model.DBInfo{
		Name: schema,
	}
	if charsetInfo != nil {
		dbInfo.Charset = charsetInfo.Chs
		dbInfo.Collate = charsetInfo.Col
	} else {
		dbInfo.Charset, dbInfo.Collate = getDefaultCharsetAndCollate()
	}

	job := &model.Job{
		SchemaID: schemaID,
		Type:     model.ActionCreateSchema,
		Args:     []interface{}{dbInfo},
	}

	err = d.startDDLJob(ctx, job)
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

func (d *ddl) DropSchema(ctx context.Context, schema model.CIStr) (err error) {
	is := d.GetInformationSchema()
	old, ok := is.SchemaByName(schema)
	if !ok {
		return errors.Trace(infoschema.DatabaseNotExists)
	}

	job := &model.Job{
		SchemaID: old.ID,
		Type:     model.ActionDropSchema,
	}

	err = d.startDDLJob(ctx, job)
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

func getDefaultCharsetAndCollate() (string, string) {
	// TODO: TableDefaultCharset-->DatabaseDefaultCharset-->SystemDefaultCharset.
	// TODO: change TableOption parser to parse collate.
	// This is a tmp solution.
	return "utf8", "utf8_unicode_ci"
}

func setColumnFlagWithConstraint(colMap map[string]*column.Col, v *ast.Constraint) {
	switch v.Tp {
	case ast.ConstraintPrimaryKey:
		for _, key := range v.Keys {
			c, ok := colMap[key.Column.Name.L]
			if !ok {
				// TODO: table constraint on unknown column.
				continue
			}
			c.Flag |= mysql.PriKeyFlag
			// Primary key can not be NULL.
			c.Flag |= mysql.NotNullFlag
		}
	case ast.ConstraintUniq, ast.ConstraintUniqIndex, ast.ConstraintUniqKey:
		for i, key := range v.Keys {
			c, ok := colMap[key.Column.Name.L]
			if !ok {
				// TODO: table constraint on unknown column.
				continue
			}
			if i == 0 {
				// Only the first column can be set
				// if unique index has multi columns,
				// the flag should be MultipleKeyFlag.
				// See: https://dev.mysql.com/doc/refman/5.7/en/show-columns.html
				if len(v.Keys) > 1 {
					c.Flag |= mysql.MultipleKeyFlag
				} else {
					c.Flag |= mysql.UniqueKeyFlag
				}
			}
		}
	case ast.ConstraintKey, ast.ConstraintIndex:
		for i, key := range v.Keys {
			c, ok := colMap[key.Column.Name.L]
			if !ok {
				// TODO: table constraint on unknown column.
				continue
			}
			if i == 0 {
				// Only the first column can be set.
				c.Flag |= mysql.MultipleKeyFlag
			}
		}
	}
}

func (d *ddl) buildColumnsAndConstraints(ctx context.Context, colDefs []*ast.ColumnDef,
	constraints []*ast.Constraint) ([]*column.Col, []*ast.Constraint, error) {
	var cols []*column.Col
	colMap := map[string]*column.Col{}
	for i, colDef := range colDefs {
		col, cts, err := d.buildColumnAndConstraint(ctx, i, colDef)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		col.State = model.StatePublic
		constraints = append(constraints, cts...)
		cols = append(cols, col)
		colMap[colDef.Name.Name.L] = col
	}
	// traverse table Constraints and set col.flag
	for _, v := range constraints {
		setColumnFlagWithConstraint(colMap, v)
	}
	return cols, constraints, nil
}

func (d *ddl) buildColumnAndConstraint(ctx context.Context, offset int,
	colDef *ast.ColumnDef) (*column.Col, []*ast.Constraint, error) {
	// Set charset.
	if len(colDef.Tp.Charset) == 0 {
		switch colDef.Tp.Tp {
		case mysql.TypeString, mysql.TypeVarchar, mysql.TypeVarString, mysql.TypeBlob, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeLongBlob:
			colDef.Tp.Charset, colDef.Tp.Collate = getDefaultCharsetAndCollate()
		default:
			colDef.Tp.Charset = charset.CharsetBin
			colDef.Tp.Collate = charset.CharsetBin
		}
	}

	col, cts, err := columnDefToCol(ctx, offset, colDef)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	col.ID, err = d.genGlobalID()
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	return col, cts, nil
}

// columnDefToCol converts ColumnDef to Col and TableConstraints.
func columnDefToCol(ctx context.Context, offset int, colDef *ast.ColumnDef) (*column.Col, []*ast.Constraint, error) {
	constraints := []*ast.Constraint{}
	col := &column.Col{
		ColumnInfo: model.ColumnInfo{
			Offset:    offset,
			Name:      colDef.Name.Name,
			FieldType: *colDef.Tp,
		},
	}

	// Check and set TimestampFlag and OnUpdateNowFlag.
	if col.Tp == mysql.TypeTimestamp {
		col.Flag |= mysql.TimestampFlag
		col.Flag |= mysql.OnUpdateNowFlag
		col.Flag |= mysql.NotNullFlag
	}

	// If flen is not assigned, assigned it by type.
	if col.Flen == types.UnspecifiedLength {
		col.Flen = mysql.GetDefaultFieldLength(col.Tp)
	}
	if col.Decimal == types.UnspecifiedLength {
		col.Decimal = mysql.GetDefaultDecimal(col.Tp)
	}

	setOnUpdateNow := false
	hasDefaultValue := false
	if colDef.Options != nil {
		keys := []*ast.IndexColName{
			{
				Column: colDef.Name,
				Length: colDef.Tp.Flen,
			},
		}
		for _, v := range colDef.Options {
			switch v.Tp {
			case ast.ColumnOptionNotNull:
				col.Flag |= mysql.NotNullFlag
			case ast.ColumnOptionNull:
				col.Flag &= ^uint(mysql.NotNullFlag)
				removeOnUpdateNowFlag(col)
			case ast.ColumnOptionAutoIncrement:
				col.Flag |= mysql.AutoIncrementFlag
			case ast.ColumnOptionPrimaryKey:
				constraint := &ast.Constraint{Tp: ast.ConstraintPrimaryKey, Keys: keys}
				constraints = append(constraints, constraint)
				col.Flag |= mysql.PriKeyFlag
			case ast.ColumnOptionUniq:
				constraint := &ast.Constraint{Tp: ast.ConstraintUniq, Name: colDef.Name.Name.O, Keys: keys}
				constraints = append(constraints, constraint)
				col.Flag |= mysql.UniqueKeyFlag
			case ast.ColumnOptionIndex:
				constraint := &ast.Constraint{Tp: ast.ConstraintIndex, Name: colDef.Name.Name.O, Keys: keys}
				constraints = append(constraints, constraint)
			case ast.ColumnOptionUniqIndex:
				constraint := &ast.Constraint{Tp: ast.ConstraintUniqIndex, Name: colDef.Name.Name.O, Keys: keys}
				constraints = append(constraints, constraint)
				col.Flag |= mysql.UniqueKeyFlag
			case ast.ColumnOptionKey:
				constraint := &ast.Constraint{Tp: ast.ConstraintKey, Name: colDef.Name.Name.O, Keys: keys}
				constraints = append(constraints, constraint)
			case ast.ColumnOptionUniqKey:
				constraint := &ast.Constraint{Tp: ast.ConstraintUniqKey, Name: colDef.Name.Name.O, Keys: keys}
				constraints = append(constraints, constraint)
				col.Flag |= mysql.UniqueKeyFlag
			case ast.ColumnOptionDefaultValue:
				value, err := getDefaultValue(ctx, v, colDef.Tp.Tp, colDef.Tp.Decimal)
				if err != nil {
					return nil, nil, errors.Errorf("invalid default value - %s", errors.Trace(err))
				}
				col.DefaultValue = value
				hasDefaultValue = true
				removeOnUpdateNowFlag(col)
			case ast.ColumnOptionOnUpdate:
				if !evaluator.IsCurrentTimeExpr(v.Expr) {
					return nil, nil, errors.Errorf("invalid ON UPDATE for - %s", col.Name)
				}

				col.Flag |= mysql.OnUpdateNowFlag
				setOnUpdateNow = true
			case ast.ColumnOptionFulltext, ast.ColumnOptionComment:
				// Do nothing.
			}
		}
	}

	setTimestampDefaultValue(col, hasDefaultValue, setOnUpdateNow)

	// Set `NoDefaultValueFlag` if this field doesn't have a default value and
	// it is `not null` and not an `AUTO_INCREMENT` field or `TIMESTAMP` field.
	setNoDefaultValueFlag(col, hasDefaultValue)

	err := checkDefaultValue(col, hasDefaultValue)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	if col.Charset == charset.CharsetBin {
		col.Flag |= mysql.BinaryFlag
	}
	return col, constraints, nil
}

func getDefaultValue(ctx context.Context, c *ast.ColumnOption, tp byte, fsp int) (interface{}, error) {
	if tp == mysql.TypeTimestamp || tp == mysql.TypeDatetime {
		value, err := evaluator.GetTimeValue(ctx, c.Expr, tp, fsp)
		if err != nil {
			return nil, errors.Trace(err)
		}

		// Value is nil means `default null`.
		if value == nil {
			return nil, nil
		}

		// If value is mysql.Time, convert it to string.
		if vv, ok := value.(mysql.Time); ok {
			return vv.String(), nil
		}

		return value, nil
	}
	v, err := evaluator.Eval(ctx, c.Expr)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return v, nil
}

func removeOnUpdateNowFlag(c *column.Col) {
	// For timestamp Col, if it is set null or default value,
	// OnUpdateNowFlag should be removed.
	if mysql.HasTimestampFlag(c.Flag) {
		c.Flag &= ^uint(mysql.OnUpdateNowFlag)
	}
}

func setTimestampDefaultValue(c *column.Col, hasDefaultValue bool, setOnUpdateNow bool) {
	if hasDefaultValue {
		return
	}

	// For timestamp Col, if is not set default value or not set null, use current timestamp.
	if mysql.HasTimestampFlag(c.Flag) && mysql.HasNotNullFlag(c.Flag) {
		if setOnUpdateNow {
			c.DefaultValue = evaluator.ZeroTimestamp
		} else {
			c.DefaultValue = evaluator.CurrentTimestamp
		}
	}
}

func setNoDefaultValueFlag(c *column.Col, hasDefaultValue bool) {
	if hasDefaultValue {
		return
	}

	if !mysql.HasNotNullFlag(c.Flag) {
		return
	}

	// Check if it is an `AUTO_INCREMENT` field or `TIMESTAMP` field.
	if !mysql.HasAutoIncrementFlag(c.Flag) && !mysql.HasTimestampFlag(c.Flag) {
		c.Flag |= mysql.NoDefaultValueFlag
	}
}

func checkDefaultValue(c *column.Col, hasDefaultValue bool) error {
	if !hasDefaultValue {
		return nil
	}

	if c.DefaultValue != nil {
		return nil
	}

	// Set not null but default null is invalid.
	if mysql.HasNotNullFlag(c.Flag) {
		return errors.Errorf("invalid default value for %s", c.Name)
	}

	return nil
}

func checkDuplicateColumn(colDefs []*ast.ColumnDef) error {
	colNames := map[string]bool{}
	for _, colDef := range colDefs {
		nameLower := colDef.Name.Name.O
		if colNames[nameLower] {
			return errors.Errorf("CREATE TABLE: duplicate column %s", colDef.Name)
		}
		colNames[nameLower] = true
	}
	return nil
}

func checkConstraintNames(constraints []*ast.Constraint) error {
	constrNames := map[string]bool{}

	// Check not empty constraint name whether is duplicated.
	for _, constr := range constraints {
		if constr.Tp == ast.ConstraintForeignKey {
			// Ignore foreign key.
			continue
		}
		if constr.Name != "" {
			nameLower := strings.ToLower(constr.Name)
			if constrNames[nameLower] {
				return errors.Errorf("CREATE TABLE: duplicate key %s", constr.Name)
			}
			constrNames[nameLower] = true
		}
	}

	// Set empty constraint names.
	for _, constr := range constraints {
		if constr.Name == "" && len(constr.Keys) > 0 {
			colName := constr.Keys[0].Column.Name.O
			constrName := colName
			i := 2
			for constrNames[strings.ToLower(constrName)] {
				// We loop forever until we find constrName that haven't been used.
				constrName = fmt.Sprintf("%s_%d", colName, i)
				i++
			}
			constr.Name = constrName
			constrNames[constrName] = true
		}
	}
	return nil
}

func (d *ddl) buildTableInfo(tableName model.CIStr, cols []*column.Col, constraints []*ast.Constraint) (tbInfo *model.TableInfo, err error) {
	tbInfo = &model.TableInfo{
		Name: tableName,
	}
	tbInfo.ID, err = d.genGlobalID()
	if err != nil {
		return nil, errors.Trace(err)
	}
	for _, v := range cols {
		tbInfo.Columns = append(tbInfo.Columns, &v.ColumnInfo)
	}
	for _, constr := range constraints {
		if constr.Tp == ast.ConstraintPrimaryKey {
			if len(constr.Keys) == 1 {
				key := constr.Keys[0]
				col := column.FindCol(cols, key.Column.Name.O)
				if col == nil {
					return nil, errors.Errorf("No such column: %v", key)
				}
				switch col.Tp {
				case mysql.TypeLong, mysql.TypeLonglong:
					tbInfo.PKIsHandle = true
					// Avoid creating index for PK handle column.
					continue
				}
			}
		}

		// 1. check if the column is exists
		// 2. add index
		indexColumns := make([]*model.IndexColumn, 0, len(constr.Keys))
		for _, key := range constr.Keys {
			col := column.FindCol(cols, key.Column.Name.O)
			if col == nil {
				return nil, errors.Errorf("No such column: %v", key)
			}
			indexColumns = append(indexColumns, &model.IndexColumn{
				Name:   key.Column.Name,
				Offset: col.Offset,
				Length: key.Length,
			})
		}
		idxInfo := &model.IndexInfo{
			Name:    model.NewCIStr(constr.Name),
			Columns: indexColumns,
			State:   model.StatePublic,
		}
		switch constr.Tp {
		case ast.ConstraintPrimaryKey:
			idxInfo.Unique = true
			idxInfo.Primary = true
			idxInfo.Name = model.NewCIStr(column.PrimaryKeyName)
		case ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex:
			idxInfo.Unique = true
		}
		if constr.Option != nil {
			idxInfo.Comment = constr.Option.Comment
			idxInfo.Tp = constr.Option.Tp
		} else {
			// Use btree as default index type.
			idxInfo.Tp = model.IndexTypeBtree
		}
		idxInfo.ID, err = d.genGlobalID()
		if err != nil {
			return nil, errors.Trace(err)
		}
		tbInfo.Indices = append(tbInfo.Indices, idxInfo)
	}
	return
}

func (d *ddl) CreateTable(ctx context.Context, ident ast.Ident, colDefs []*ast.ColumnDef,
	constraints []*ast.Constraint, options []*ast.TableOption) (err error) {
	is := d.GetInformationSchema()
	schema, ok := is.SchemaByName(ident.Schema)
	if !ok {
		return infoschema.DatabaseNotExists.Gen("database %s not exists", ident.Schema)
	}
	if is.TableExists(ident.Schema, ident.Name) {
		return errors.Trace(infoschema.TableExists)
	}
	if err = checkDuplicateColumn(colDefs); err != nil {
		return errors.Trace(err)
	}

	cols, newConstraints, err := d.buildColumnsAndConstraints(ctx, colDefs, constraints)
	if err != nil {
		return errors.Trace(err)
	}

	err = checkConstraintNames(newConstraints)
	if err != nil {
		return errors.Trace(err)
	}

	tbInfo, err := d.buildTableInfo(ident.Name, cols, newConstraints)
	if err != nil {
		return errors.Trace(err)
	}

	job := &model.Job{
		SchemaID: schema.ID,
		TableID:  tbInfo.ID,
		Type:     model.ActionCreateTable,
		Args:     []interface{}{tbInfo},
	}

	err = d.startDDLJob(ctx, job)
	if err == nil {
		err = d.handleTableOptions(options, tbInfo, schema.ID)
	}
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

func (d *ddl) handleTableOptions(options []*ast.TableOption, tbInfo *model.TableInfo, schemaID int64) error {
	for _, op := range options {
		if op.Tp == ast.TableOptionAutoIncrement {
			alloc := autoid.NewAllocator(d.store, schemaID)
			tbInfo.State = model.StatePublic
			tb, err := table.TableFromMeta(alloc, tbInfo)
			if err != nil {
				return errors.Trace(err)
			}
			// The operation of the minus 1 to make sure that the current value doesn't be used,
			// the next Alloc operation will get this value.
			// Its behavior is consistent with MySQL.
			if err = tb.RebaseAutoID(int64(op.UintValue-1), false); err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

func (d *ddl) AlterTable(ctx context.Context, ident ast.Ident, specs []*ast.AlterTableSpec) (err error) {
	// now we only allow one schema changes at the same time.
	if len(specs) != 1 {
		return errors.New("can't run multi schema changes in one DDL")
	}

	for _, spec := range specs {
		switch spec.Tp {
		case ast.AlterTableAddColumn:
			err = d.AddColumn(ctx, ident, spec)
		case ast.AlterTableDropColumn:
			err = d.DropColumn(ctx, ident, spec.DropColumn.Name)
		case ast.AlterTableDropIndex:
			err = d.DropIndex(ctx, ident, model.NewCIStr(spec.Name))
		case ast.AlterTableAddConstraint:
			constr := spec.Constraint
			switch spec.Constraint.Tp {
			case ast.ConstraintKey, ast.ConstraintIndex:
				err = d.CreateIndex(ctx, ident, false, model.NewCIStr(constr.Name), spec.Constraint.Keys)
			case ast.ConstraintUniq, ast.ConstraintUniqIndex, ast.ConstraintUniqKey:
				err = d.CreateIndex(ctx, ident, true, model.NewCIStr(constr.Name), spec.Constraint.Keys)
			default:
				// nothing to do now.
			}
		default:
			// nothing to do now.
		}

		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func checkColumnConstraint(constraints []*ast.ColumnOption) error {
	for _, constraint := range constraints {
		switch constraint.Tp {
		case ast.ColumnOptionAutoIncrement, ast.ColumnOptionPrimaryKey, ast.ColumnOptionUniq, ast.ColumnOptionUniqKey:
			return errors.Errorf("unsupported add column constraint - %v", constraint.Tp)
		}
	}

	return nil
}

// AddColumn will add a new column to the table.
func (d *ddl) AddColumn(ctx context.Context, ti ast.Ident, spec *ast.AlterTableSpec) error {
	// Check whether the added column constraints are supported.
	err := checkColumnConstraint(spec.Column.Options)
	if err != nil {
		return errors.Trace(err)
	}

	is := d.infoHandle.Get()
	schema, ok := is.SchemaByName(ti.Schema)
	if !ok {
		return errors.Trace(infoschema.DatabaseNotExists)
	}

	t, err := is.TableByName(ti.Schema, ti.Name)
	if err != nil {
		return errors.Trace(infoschema.TableNotExists)
	}

	// Check whether added column has existed.
	colName := spec.Column.Name.Name.O
	col := column.FindCol(t.Cols(), colName)
	if col != nil {
		return errors.Errorf("column %s already exists", colName)
	}

	// ingore table constraints now, maybe return error later
	// we use length(t.Cols()) as the default offset first, later we will change the
	// column's offset later.
	col, _, err = d.buildColumnAndConstraint(ctx, len(t.Cols()), spec.Column)
	if err != nil {
		return errors.Trace(err)
	}

	job := &model.Job{
		SchemaID: schema.ID,
		TableID:  t.Meta().ID,
		Type:     model.ActionAddColumn,
		Args:     []interface{}{&col.ColumnInfo, spec.Position, 0},
	}

	err = d.startDDLJob(ctx, job)
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

// DropColumn will drop a column from the table, now we don't support drop the column with index covered.
func (d *ddl) DropColumn(ctx context.Context, ti ast.Ident, colName model.CIStr) error {
	is := d.infoHandle.Get()
	schema, ok := is.SchemaByName(ti.Schema)
	if !ok {
		return errors.Trace(infoschema.DatabaseNotExists)
	}

	t, err := is.TableByName(ti.Schema, ti.Name)
	if err != nil {
		return errors.Trace(infoschema.TableNotExists)
	}

	// Check whether dropped column has existed.
	col := column.FindCol(t.Cols(), colName.L)
	if col == nil {
		return errors.Errorf("column %s doesn’t exist", colName.L)
	}

	job := &model.Job{
		SchemaID: schema.ID,
		TableID:  t.Meta().ID,
		Type:     model.ActionDropColumn,
		Args:     []interface{}{colName},
	}

	err = d.startDDLJob(ctx, job)
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

// DropTable will proceed even if some table in the list does not exists.
func (d *ddl) DropTable(ctx context.Context, ti ast.Ident) (err error) {
	is := d.GetInformationSchema()
	schema, ok := is.SchemaByName(ti.Schema)
	if !ok {
		return infoschema.DatabaseNotExists.Gen("database %s not exists", ti.Schema)
	}

	tb, err := is.TableByName(ti.Schema, ti.Name)
	if err != nil {
		return errors.Trace(infoschema.TableNotExists)
	}

	job := &model.Job{
		SchemaID: schema.ID,
		TableID:  tb.Meta().ID,
		Type:     model.ActionDropTable,
	}

	err = d.startDDLJob(ctx, job)
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

func (d *ddl) CreateIndex(ctx context.Context, ti ast.Ident, unique bool, indexName model.CIStr, idxColNames []*ast.IndexColName) error {
	is := d.infoHandle.Get()
	schema, ok := is.SchemaByName(ti.Schema)
	if !ok {
		return infoschema.DatabaseNotExists.Gen("database %s not exists", ti.Schema)
	}

	t, err := is.TableByName(ti.Schema, ti.Name)
	if err != nil {
		return errors.Trace(infoschema.TableNotExists)
	}
	indexID, err := d.genGlobalID()
	if err != nil {
		return errors.Trace(err)
	}

	job := &model.Job{
		SchemaID: schema.ID,
		TableID:  t.Meta().ID,
		Type:     model.ActionAddIndex,
		Args:     []interface{}{unique, indexName, indexID, idxColNames},
	}

	err = d.startDDLJob(ctx, job)
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

func (d *ddl) DropIndex(ctx context.Context, ti ast.Ident, indexName model.CIStr) error {
	is := d.infoHandle.Get()
	schema, ok := is.SchemaByName(ti.Schema)
	if !ok {
		return errors.Trace(infoschema.DatabaseNotExists)
	}

	t, err := is.TableByName(ti.Schema, ti.Name)
	if err != nil {
		return errors.Trace(infoschema.TableNotExists)
	}

	job := &model.Job{
		SchemaID: schema.ID,
		TableID:  t.Meta().ID,
		Type:     model.ActionDropIndex,
		Args:     []interface{}{indexName},
	}

	err = d.startDDLJob(ctx, job)
	err = d.hook.OnChanged(err)
	return errors.Trace(err)
}

// findCol finds column in cols by name.
func findCol(cols []*model.ColumnInfo, name string) *model.ColumnInfo {
	name = strings.ToLower(name)
	for _, col := range cols {
		if col.Name.L == name {
			return col
		}
	}

	return nil
}
