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
	"github.com/pingcap/tidb/inspectkv"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx/variable"
)

var (
	serverID             = "server_id"
	ddlSchemaVersion     = "ddl_schema_version"
	ddlOwnerID           = "ddl_owner_id"
	ddlOwnerLastUpdateTS = "ddl_owner_last_update_ts"
	ddlJobID             = "ddl_job_id"
	ddlJobAction         = "ddl_job_action"
	ddlJobLastUpdateTS   = "ddl_job_last_update_ts"
	ddlJobState          = "ddl_job_state"
	ddlJobError          = "ddl_job_error"
	ddlJobSchemaState    = "ddl_job_schema_state"
	ddlJobSchemaID       = "ddl_job_schema_id"
	ddlJobTableID        = "ddl_job_table_id"
	ddlJobSnapshotVer    = "ddl_job_snapshot_ver"
	ddlJobReorgHandle    = "ddl_job_reorg_handle"
	ddlJobArgs           = "ddl_job_args"
	bgSchemaVersion      = "bg_schema_version"
	bgOwnerID            = "bg_owner_id"
	bgOwnerLastUpdateTS  = "bg_owner_last_update_ts"
	bgJobID              = "bg_job_id"
	bgJobAction          = "bg_job_action"
	bgJobLastUpdateTS    = "bg_job_last_update_ts"
	bgJobState           = "bg_job_state"
	bgJobError           = "bg_job_error"
	bgJobSchemaState     = "bg_job_schema_state"
	bgJobSchemaID        = "bg_job_schema_id"
	bgJobTableID         = "bg_job_table_id"
	bgJobSnapshotVer     = "bg_job_snapshot_ver"
	bgJobReorgHandle     = "bg_job_reorg_handle"
	bgJobArgs            = "bg_job_args"
)

// GetScope gets the status variables scope.
func (d *ddl) GetScope(status string) variable.ScopeFlag {
	// Now ddl status variables scope are all default scope.
	return variable.DefaultScopeFlag
}

// Stat returns the DDL statistics.
func (d *ddl) Stats() (map[string]interface{}, error) {
	m := make(map[string]interface{})
	m[serverID] = d.uuid
	var ddlInfo, bgInfo *inspectkv.DDLInfo

	err := kv.RunInNewTxn(d.store, false, func(txn kv.Transaction) error {
		var err1 error
		ddlInfo, err1 = inspectkv.GetDDLInfo(txn)
		if err1 != nil {
			return errors.Trace(err1)
		}
		bgInfo, err1 = inspectkv.GetBgDDLInfo(txn)

		return errors.Trace(err1)
	})
	if err != nil {
		return nil, errors.Trace(err)
	}

	m[ddlSchemaVersion] = ddlInfo.SchemaVer
	if ddlInfo.Owner != nil {
		m[ddlOwnerID] = ddlInfo.Owner.OwnerID
		// LastUpdateTS uses nanosecond.
		m[ddlOwnerLastUpdateTS] = ddlInfo.Owner.LastUpdateTS / 1e9
	}
	if ddlInfo.Job != nil {
		m[ddlJobID] = ddlInfo.Job.ID
		m[ddlJobAction] = ddlInfo.Job.Type.String()
		m[ddlJobLastUpdateTS] = ddlInfo.Job.LastUpdateTS / 1e9
		m[ddlJobState] = ddlInfo.Job.State.String()
		m[ddlJobError] = ddlInfo.Job.Error
		m[ddlJobSchemaState] = ddlInfo.Job.SchemaState.String()
		m[ddlJobSchemaID] = ddlInfo.Job.SchemaID
		m[ddlJobTableID] = ddlInfo.Job.TableID
		m[ddlJobSnapshotVer] = ddlInfo.Job.SnapshotVer
		m[ddlJobReorgHandle] = ddlInfo.ReorgHandle
		m[ddlJobArgs] = ddlInfo.Job.Args
	}

	// background DDL info
	m[bgSchemaVersion] = bgInfo.SchemaVer
	if bgInfo.Owner != nil {
		m[bgOwnerID] = bgInfo.Owner.OwnerID
		// LastUpdateTS uses nanosecond.
		m[bgOwnerLastUpdateTS] = bgInfo.Owner.LastUpdateTS / 1e9
	}
	if bgInfo.Job != nil {
		m[bgJobID] = bgInfo.Job.ID
		m[bgJobAction] = bgInfo.Job.Type.String()
		m[bgJobLastUpdateTS] = bgInfo.Job.LastUpdateTS / 1e9
		m[bgJobState] = bgInfo.Job.State.String()
		m[bgJobError] = bgInfo.Job.Error
		m[bgJobSchemaState] = bgInfo.Job.SchemaState.String()
		m[bgJobSchemaID] = bgInfo.Job.SchemaID
		m[bgJobTableID] = bgInfo.Job.TableID
		m[bgJobSnapshotVer] = bgInfo.Job.SnapshotVer
		m[bgJobReorgHandle] = bgInfo.ReorgHandle
		m[bgJobArgs] = bgInfo.Job.Args
	}

	return m, nil
}
