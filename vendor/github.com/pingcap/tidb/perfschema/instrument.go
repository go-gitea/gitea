// Copyright 2016 PingCAP, Inc.
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

package perfschema

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/util/types"
)

// EnumCallerName is used as a parameter to avoid calling runtime.Caller(1) since
// it is too expensive (500ns+ per call), we don't want to invoke it repeatedly for
// each instrument.
type EnumCallerName int

const (
	// CallerNameSessionExecute is for session.go:Execute() method.
	CallerNameSessionExecute EnumCallerName = iota + 1
)

const (
	stageInstrumentPrefix       = "stage/"
	statementInstrumentPrefix   = "statement/"
	transactionInstrumentPrefix = "transaction"
)

// Flag indicators for table setup_timers.
const (
	flagStage = iota + 1
	flagStatement
	flagTransaction
)

type enumTimerName int

// Enum values for the TIMER_NAME columns.
// This enum is found in the following tables:
// - performance_schema.setup_timer (TIMER_NAME)
const (
	timerNameNone enumTimerName = iota
	timerNameNanosec
	timerNameMicrosec
	timerNameMillisec
)

var (
	callerNames = make(map[EnumCallerName]string)
)

// addInstrument is used to add an item to setup_instruments table.
func (ps *perfSchema) addInstrument(name string) (uint64, error) {
	record := types.MakeDatums(name, mysql.Enum{Name: "YES", Value: 1}, mysql.Enum{Name: "YES", Value: 1})
	tbl := ps.mTables[TableSetupInstruments]
	handle, err := tbl.AddRecord(nil, record)
	return uint64(handle), errors.Trace(err)
}

func (ps *perfSchema) getTimerName(flag int) (enumTimerName, error) {
	if flag < 0 || flag >= len(setupTimersRecords) {
		return timerNameNone, errors.Errorf("Unknown timerName flag %d", flag)
	}
	timerName := fmt.Sprintf("%s", setupTimersRecords[flag][1].GetString())
	switch timerName {
	case "NANOSECOND":
		return timerNameNanosec, nil
	case "MICROSECOND":
		return timerNameMicrosec, nil
	case "MILLISECOND":
		return timerNameMillisec, nil
	}
	return timerNameNone, nil
}
