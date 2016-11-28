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

package tidb

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx/variable"
)

const (
	// CreateUserTable is the SQL statement creates User table in system db.
	CreateUserTable = `CREATE TABLE if not exists mysql.user (
		Host			CHAR(64),
		User			CHAR(16),
		Password		CHAR(41),
		Select_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Insert_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Update_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Delete_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Create_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Drop_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Grant_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Alter_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Show_db_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Execute_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Index_priv		ENUM('N','Y') NOT NULL  DEFAULT 'N',
		Create_user_priv	ENUM('N','Y') NOT NULL  DEFAULT 'N',
		PRIMARY KEY (Host, User));`
	// CreateDBPrivTable is the SQL statement creates DB scope privilege table in system db.
	CreateDBPrivTable = `CREATE TABLE if not exists mysql.db (
		Host		CHAR(60),
		DB		CHAR(64),
		User		CHAR(16),
		Select_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Insert_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Update_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Delete_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Create_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Drop_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Grant_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Index_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Alter_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		Execute_priv	ENUM('N','Y') Not Null  DEFAULT 'N',
		PRIMARY KEY (Host, DB, User));`
	// CreateTablePrivTable is the SQL statement creates table scope privilege table in system db.
	CreateTablePrivTable = `CREATE TABLE if not exists mysql.tables_priv (
		Host		CHAR(60),
		DB		CHAR(64),
		User		CHAR(16),
		Table_name	CHAR(64),
		Grantor		CHAR(77),
		Timestamp	Timestamp DEFAULT CURRENT_TIMESTAMP,
		Table_priv	SET('Select','Insert','Update','Delete','Create','Drop','Grant', 'Index','Alter'),
		Column_priv	SET('Select','Insert','Update'),
		PRIMARY KEY (Host, DB, User, Table_name));`
	// CreateColumnPrivTable is the SQL statement creates column scope privilege table in system db.
	CreateColumnPrivTable = `CREATE TABLE if not exists mysql.columns_priv(
		Host		CHAR(60),
		DB		CHAR(64),
		User		CHAR(16),
		Table_name	CHAR(64),
		Column_name	CHAR(64),
		Timestamp	Timestamp DEFAULT CURRENT_TIMESTAMP,
		Column_priv	SET('Select','Insert','Update'),
		PRIMARY KEY (Host, DB, User, Table_name, Column_name));`
	// CreateGloablVariablesTable is the SQL statement creates global variable table in system db.
	// TODO: MySQL puts GLOBAL_VARIABLES table in INFORMATION_SCHEMA db.
	// INFORMATION_SCHEMA is a virtual db in TiDB. So we put this table in system db.
	// Maybe we will put it back to INFORMATION_SCHEMA.
	CreateGloablVariablesTable = `CREATE TABLE if not exists mysql.GLOBAL_VARIABLES(
		VARIABLE_NAME  VARCHAR(64) Not Null PRIMARY KEY,
		VARIABLE_VALUE VARCHAR(1024) DEFAULT Null);`
	// CreateTiDBTable is the SQL statement creates a table in system db.
	// This table is a key-value struct contains some information used by TiDB.
	// Currently we only put bootstrapped in it which indicates if the system is already bootstrapped.
	CreateTiDBTable = `CREATE TABLE if not exists mysql.tidb(
		VARIABLE_NAME  VARCHAR(64) Not Null PRIMARY KEY,
		VARIABLE_VALUE VARCHAR(1024) DEFAULT Null,
		COMMENT VARCHAR(1024));`
)

// Bootstrap initiates system DB for a store.
func bootstrap(s Session) {
	b, err := checkBootstrapped(s)
	if err != nil {
		log.Fatal(err)
	}
	if b {
		return
	}
	doDDLWorks(s)
	doDMLWorks(s)
}

const (
	bootstrappedVar     = "bootstrapped"
	bootstrappedVarTrue = "True"
)

func checkBootstrapped(s Session) (bool, error) {
	//  Check if system db exists.
	_, err := s.Execute(fmt.Sprintf("USE %s;", mysql.SystemDB))
	if err != nil && infoschema.DatabaseNotExists.NotEqual(err) {
		log.Fatal(err)
	}
	// Check bootstrapped variable value in TiDB table.
	v, err := checkBootstrappedVar(s)
	if err != nil {
		return false, errors.Trace(err)
	}
	return v, nil
}

func checkBootstrappedVar(s Session) (bool, error) {
	sql := fmt.Sprintf(`SELECT VARIABLE_VALUE FROM %s.%s WHERE VARIABLE_NAME="%s"`,
		mysql.SystemDB, mysql.TiDBTable, bootstrappedVar)
	rs, err := s.Execute(sql)
	if err != nil {
		if infoschema.TableNotExists.Equal(err) {
			return false, nil
		}
		return false, errors.Trace(err)
	}

	if len(rs) != 1 {
		return false, errors.New("Wrong number of Recordset")
	}
	r := rs[0]
	row, err := r.Next()
	if err != nil || row == nil {
		return false, errors.Trace(err)
	}

	isBootstrapped := row.Data[0].GetString() == bootstrappedVarTrue
	if isBootstrapped {
		// Make sure that doesn't affect the following operations.

		if err = s.FinishTxn(false); err != nil {
			return false, errors.Trace(err)
		}
	}

	return isBootstrapped, nil
}

// Execute DDL statements in bootstrap stage.
func doDDLWorks(s Session) {
	// Create a test database.
	mustExecute(s, "CREATE DATABASE IF NOT EXISTS test")
	// Create system db.
	mustExecute(s, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", mysql.SystemDB))
	// Create user table.
	mustExecute(s, CreateUserTable)
	// Create privilege tables.
	mustExecute(s, CreateDBPrivTable)
	mustExecute(s, CreateTablePrivTable)
	mustExecute(s, CreateColumnPrivTable)
	// Create global systemt variable table.
	mustExecute(s, CreateGloablVariablesTable)
	// Create TiDB table.
	mustExecute(s, CreateTiDBTable)
}

// Execute DML statements in bootstrap stage.
// All the statements run in a single transaction.
func doDMLWorks(s Session) {
	mustExecute(s, "BEGIN")

	// Insert a default user with empty password.
	mustExecute(s, `INSERT INTO mysql.user VALUES
		("%", "root", "", "Y", "Y", "Y", "Y", "Y", "Y", "Y", "Y", "Y", "Y", "Y", "Y")`)

	// Init global system variables table.
	values := make([]string, 0, len(variable.SysVars))
	for k, v := range variable.SysVars {
		value := fmt.Sprintf(`("%s", "%s")`, strings.ToLower(k), v.Value)
		values = append(values, value)
	}
	sql := fmt.Sprintf("INSERT INTO %s.%s VALUES %s;", mysql.SystemDB, mysql.GlobalVariablesTable,
		strings.Join(values, ", "))
	mustExecute(s, sql)

	sql = fmt.Sprintf(`INSERT INTO %s.%s VALUES("%s", "%s", "Bootstrap flag. Do not delete.")
		ON DUPLICATE KEY UPDATE VARIABLE_VALUE="%s"`,
		mysql.SystemDB, mysql.TiDBTable, bootstrappedVar, bootstrappedVarTrue, bootstrappedVarTrue)
	mustExecute(s, sql)
	mustExecute(s, "COMMIT")
}

func mustExecute(s Session, sql string) {
	_, err := s.Execute(sql)
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
	}
}
