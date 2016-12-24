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

package model

import (
	"encoding/json"
	"fmt"

	"github.com/juju/errors"
)

// ActionType is the type for DDL action.
type ActionType byte

// List DDL actions.
const (
	ActionNone ActionType = iota
	ActionCreateSchema
	ActionDropSchema
	ActionCreateTable
	ActionDropTable
	ActionAddColumn
	ActionDropColumn
	ActionAddIndex
	ActionDropIndex
)

func (action ActionType) String() string {
	switch action {
	case ActionCreateSchema:
		return "create schema"
	case ActionDropSchema:
		return "drop schema"
	case ActionCreateTable:
		return "create table"
	case ActionDropTable:
		return "drop table"
	case ActionAddColumn:
		return "add column"
	case ActionDropColumn:
		return "drop column"
	case ActionAddIndex:
		return "add index"
	case ActionDropIndex:
		return "drop index"
	default:
		return "none"
	}
}

// Job is for a DDL operation.
type Job struct {
	ID       int64      `json:"id"`
	Type     ActionType `json:"type"`
	SchemaID int64      `json:"schema_id"`
	TableID  int64      `json:"table_id"`
	State    JobState   `json:"state"`
	Error    string     `json:"err"`
	// every time we meet an error when running job, we will increase it
	ErrorCount int64         `json:"err_count"`
	Args       []interface{} `json:"-"`
	// we must use json raw message for delay parsing special args.
	RawArgs     json.RawMessage `json:"raw_args"`
	SchemaState SchemaState     `json:"schema_state"`
	// snapshot version for this job.
	SnapshotVer uint64 `json:"snapshot_ver"`
	// unix nano seconds
	// TODO: use timestamp allocated by TSO
	LastUpdateTS int64 `json:"last_update_ts"`
}

// Encode encodes job with json format.
func (job *Job) Encode() ([]byte, error) {
	var err error
	job.RawArgs, err = json.Marshal(job.Args)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var b []byte
	b, err = json.Marshal(job)
	return b, errors.Trace(err)
}

// Decode decodes job from the json buffer, we must use DecodeArgs later to
// decode special args for this job.
func (job *Job) Decode(b []byte) error {
	err := json.Unmarshal(b, job)
	return errors.Trace(err)
}

// DecodeArgs decodes job args.
func (job *Job) DecodeArgs(args ...interface{}) error {
	job.Args = args
	err := json.Unmarshal(job.RawArgs, &job.Args)
	return errors.Trace(err)
}

// String implements fmt.Stringer interface.
func (job *Job) String() string {
	return fmt.Sprintf("ID:%d, Type:%s, State:%s, SchemaState:%s, SchemaID:%d, TableID:%d, Args:%s",
		job.ID, job.Type, job.State, job.SchemaState, job.SchemaID, job.TableID, job.RawArgs)
}

// IsFinished returns whether job is finished or not.
// If the job state is Done or Cancelled, it is finished.
func (job *Job) IsFinished() bool {
	return job.State == JobDone || job.State == JobCancelled
}

// IsRunning returns whether job is still running or not.
func (job *Job) IsRunning() bool {
	return job.State == JobRunning
}

// JobState is for job state.
type JobState byte

// List job states.
const (
	JobNone JobState = iota
	JobRunning
	JobDone
	JobCancelled
)

// String implements fmt.Stringer interface.
func (s JobState) String() string {
	switch s {
	case JobRunning:
		return "running"
	case JobDone:
		return "done"
	case JobCancelled:
		return "cancelled"
	default:
		return "none"
	}
}

// Owner is for DDL Owner.
type Owner struct {
	OwnerID string `json:"owner_id"`
	// unix nano seconds
	// TODO: use timestamp allocated by TSO
	LastUpdateTS int64 `json:"last_update_ts"`
}

// String implements fmt.Stringer interface.
func (o *Owner) String() string {
	return fmt.Sprintf("ID:%s, LastUpdateTS:%d", o.OwnerID, o.LastUpdateTS)
}
