// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type CheckCollationsResult struct {
	ExpectedCollation        string
	AvailableCollation       container.Set[string]
	DatabaseCollation        string
	IsCollationCaseSensitive func(s string) bool
	CollationEquals          func(a, b string) bool
	ExistingTableNumber      int

	InconsistentCollationColumns []string
}

func findAvailableCollationsMySQL(x *xorm.Engine) (ret container.Set[string], err error) {
	var res []struct {
		Collation string
	}
	if err = x.SQL("SHOW COLLATION WHERE (Collation = 'utf8mb4_bin') OR (Collation LIKE '%\\_as\\_cs%')").Find(&res); err != nil {
		return nil, err
	}
	ret = make(container.Set[string], len(res))
	for _, r := range res {
		ret.Add(r.Collation)
	}
	return ret, nil
}

func findAvailableCollationsMSSQL(x *xorm.Engine) (ret container.Set[string], err error) {
	var res []struct {
		Name string
	}
	if err = x.SQL("SELECT * FROM sys.fn_helpcollations() WHERE name LIKE '%[_]CS[_]AS%'").Find(&res); err != nil {
		return nil, err
	}
	ret = make(container.Set[string], len(res))
	for _, r := range res {
		ret.Add(r.Name)
	}
	return ret, nil
}

func CheckCollations(x *xorm.Engine) (*CheckCollationsResult, error) {
	dbTables, err := x.DBMetas()
	if err != nil {
		return nil, err
	}

	res := &CheckCollationsResult{
		ExistingTableNumber: len(dbTables),
		CollationEquals:     func(a, b string) bool { return a == b },
	}

	var candidateCollations []string
	if x.Dialect().URI().DBType == schemas.MYSQL {
		_, err = x.SQL("SELECT DEFAULT_COLLATION_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", setting.Database.Name).Get(&res.DatabaseCollation)
		if err != nil {
			return nil, err
		}
		res.IsCollationCaseSensitive = func(s string) bool {
			return s == "utf8mb4_bin" || strings.HasSuffix(s, "_as_cs")
		}
		candidateCollations = []string{"utf8mb4_0900_as_cs", "uca1400_as_cs", "utf8mb4_bin"}
		res.AvailableCollation, err = findAvailableCollationsMySQL(x)
		if err != nil {
			return nil, err
		}
		res.CollationEquals = func(a, b string) bool {
			// MariaDB adds the "utf8mb4_" prefix, eg: "utf8mb4_uca1400_as_cs", but not the name "uca1400_as_cs" in "SHOW COLLATION"
			// At the moment, it's safe to ignore the database difference, just trim the prefix and compare. It could be fixed easily if there is any problem in the future.
			return a == b || strings.TrimPrefix(a, "utf8mb4_") == strings.TrimPrefix(b, "utf8mb4_")
		}
	} else if x.Dialect().URI().DBType == schemas.MSSQL {
		if _, err = x.SQL("SELECT DATABASEPROPERTYEX(DB_NAME(), 'Collation')").Get(&res.DatabaseCollation); err != nil {
			return nil, err
		}
		res.IsCollationCaseSensitive = func(s string) bool {
			return strings.HasSuffix(s, "_CS_AS")
		}
		candidateCollations = []string{"Latin1_General_CS_AS"}
		res.AvailableCollation, err = findAvailableCollationsMSSQL(x)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, nil
	}

	if res.DatabaseCollation == "" {
		return nil, errors.New("unable to get collation for current database")
	}

	res.ExpectedCollation = setting.Database.CharsetCollation
	if res.ExpectedCollation == "" {
		for _, collation := range candidateCollations {
			if res.AvailableCollation.Contains(collation) {
				res.ExpectedCollation = collation
				break
			}
		}
	}

	if res.ExpectedCollation == "" {
		return nil, errors.New("unable to find a suitable collation for current database")
	}

	allColumnsMatchExpected := true
	allColumnsMatchDatabase := true
	for _, table := range dbTables {
		for _, col := range table.Columns() {
			if col.Collation != "" {
				allColumnsMatchExpected = allColumnsMatchExpected && res.CollationEquals(col.Collation, res.ExpectedCollation)
				allColumnsMatchDatabase = allColumnsMatchDatabase && res.CollationEquals(col.Collation, res.DatabaseCollation)
				if !res.IsCollationCaseSensitive(col.Collation) || !res.CollationEquals(col.Collation, res.DatabaseCollation) {
					res.InconsistentCollationColumns = append(res.InconsistentCollationColumns, fmt.Sprintf("%s.%s", table.Name, col.Name))
				}
			}
		}
	}
	// if all columns match expected collation or all match database collation, then it could also be considered as "consistent"
	if allColumnsMatchExpected || allColumnsMatchDatabase {
		res.InconsistentCollationColumns = nil
	}
	return res, nil
}

func CheckCollationsDefaultEngine() (*CheckCollationsResult, error) {
	return CheckCollations(xormEngine)
}

func alterDatabaseCollation(x *xorm.Engine, collation string) error {
	if x.Dialect().URI().DBType == schemas.MYSQL {
		_, err := x.Exec("ALTER DATABASE CHARACTER SET utf8mb4 COLLATE " + collation)
		return err
	} else if x.Dialect().URI().DBType == schemas.MSSQL {
		// TODO: MSSQL has many limitations on changing database collation, it could fail in many cases.
		_, err := x.Exec("ALTER DATABASE CURRENT COLLATE " + collation)
		return err
	}
	return errors.New("unsupported database type")
}

// preprocessDatabaseCollation checks database & table column collation, and alter the database collation if needed
func preprocessDatabaseCollation(x *xorm.Engine) {
	r, err := CheckCollations(x)
	if err != nil {
		log.Error("Failed to check database collation: %v", err)
	}
	if r == nil {
		return // no check result means the database doesn't need to do such check/process (at the moment ....)
	}

	// try to alter database collation to expected if the database is empty, it might fail in some cases (and it isn't necessary to succeed)
	// at the moment, there is no "altering" solution for MSSQL, site admin should manually change the database collation
	if !r.CollationEquals(r.DatabaseCollation, r.ExpectedCollation) && r.ExistingTableNumber == 0 {
		if err = alterDatabaseCollation(x, r.ExpectedCollation); err != nil {
			log.Error("Failed to change database collation to %q: %v", r.ExpectedCollation, err)
		} else {
			_, _ = x.Exec("SELECT 1") // after "altering", MSSQL's session becomes invalid, so make a simple query to "refresh" the session
			if r, err = CheckCollations(x); err != nil {
				log.Error("Failed to check database collation again after altering: %v", err) // impossible case
				return
			}
			log.Warn("Current database has been altered to use collation %q", r.DatabaseCollation)
		}
	}

	// check column collation, and show warning/error to end users -- no need to fatal, do not block the startup
	if !r.IsCollationCaseSensitive(r.DatabaseCollation) {
		log.Warn("Current database is using a case-insensitive collation %q, although Gitea could work with it, there might be some rare cases which don't work as expected.", r.DatabaseCollation)
	}

	if len(r.InconsistentCollationColumns) > 0 {
		log.Error("There are %d table columns using inconsistent collation, they should use %q. Please go to admin panel Self Check page", len(r.InconsistentCollationColumns), r.DatabaseCollation)
	}
}
