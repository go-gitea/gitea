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
	"github.com/juju/errors"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/terror"
)

func (d *ddl) onCreateSchema(t *meta.Meta, job *model.Job) error {
	schemaID := job.SchemaID
	dbInfo := &model.DBInfo{}
	if err := job.DecodeArgs(dbInfo); err != nil {
		// arg error, cancel this job.
		job.State = model.JobCancelled
		return errors.Trace(err)
	}

	dbInfo.ID = schemaID
	dbInfo.State = model.StateNone

	dbs, err := t.ListDatabases()
	if err != nil {
		return errors.Trace(err)
	}

	for _, db := range dbs {
		if db.Name.L == dbInfo.Name.L {
			if db.ID != schemaID {
				// database exists, can't create, we should cancel this job now.
				job.State = model.JobCancelled
				return errors.Trace(infoschema.DatabaseExists)
			}
			dbInfo = db
		}
	}

	_, err = t.GenSchemaVersion()
	if err != nil {
		return errors.Trace(err)
	}

	switch dbInfo.State {
	case model.StateNone:
		// none -> public
		job.SchemaState = model.StatePublic
		dbInfo.State = model.StatePublic
		err = t.CreateDatabase(dbInfo)
		if err != nil {
			return errors.Trace(err)
		}
		// finish this job
		job.State = model.JobDone
		return nil
	default:
		// we can't enter here.
		return errors.Errorf("invalid db state %v", dbInfo.State)
	}
}

func (d *ddl) delReorgSchema(t *meta.Meta, job *model.Job) error {
	dbInfo := &model.DBInfo{}
	if err := job.DecodeArgs(dbInfo); err != nil {
		// arg error, cancel this job.
		job.State = model.JobCancelled
		return errors.Trace(err)
	}

	tables, err := t.ListTables(dbInfo.ID)
	if terror.ErrorEqual(meta.ErrDBNotExists, err) {
		job.State = model.JobDone
		return nil
	}
	if err != nil {
		return errors.Trace(err)
	}

	if err = d.dropSchemaData(dbInfo, tables); err != nil {
		return errors.Trace(err)
	}

	// finish this background job
	job.SchemaState = model.StateNone
	job.State = model.JobDone

	return nil
}

func (d *ddl) onDropSchema(t *meta.Meta, job *model.Job) error {
	dbInfo, err := t.GetDatabase(job.SchemaID)
	if err != nil {
		return errors.Trace(err)
	}
	if dbInfo == nil {
		job.State = model.JobCancelled
		return errors.Trace(infoschema.DatabaseNotExists)
	}

	_, err = t.GenSchemaVersion()
	if err != nil {
		return errors.Trace(err)
	}

	switch dbInfo.State {
	case model.StatePublic:
		// public -> write only
		job.SchemaState = model.StateWriteOnly
		dbInfo.State = model.StateWriteOnly
		err = t.UpdateDatabase(dbInfo)
	case model.StateWriteOnly:
		// write only -> delete only
		job.SchemaState = model.StateDeleteOnly
		dbInfo.State = model.StateDeleteOnly
		err = t.UpdateDatabase(dbInfo)
	case model.StateDeleteOnly:
		dbInfo.State = model.StateDeleteReorganization
		err = t.UpdateDatabase(dbInfo)
		if err = t.DropDatabase(dbInfo.ID); err != nil {
			break
		}
		// finish this job
		job.Args = []interface{}{dbInfo}
		job.State = model.JobDone
		job.SchemaState = model.StateNone
	default:
		// we can't enter here.
		err = errors.Errorf("invalid db state %v", dbInfo.State)
	}

	return errors.Trace(err)
}

func (d *ddl) dropSchemaData(dbInfo *model.DBInfo, tables []*model.TableInfo) error {
	for _, tblInfo := range tables {
		alloc := autoid.NewAllocator(d.store, dbInfo.ID)
		t, err := table.TableFromMeta(alloc, tblInfo)
		if err != nil {
			return errors.Trace(err)
		}

		err = d.dropTableData(t)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
