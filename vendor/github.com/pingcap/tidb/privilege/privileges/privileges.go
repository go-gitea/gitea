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

package privileges

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/privilege"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/util/sqlexec"
	"github.com/pingcap/tidb/util/types"
)

var _ privilege.Checker = (*UserPrivileges)(nil)

type privileges struct {
	Level ast.GrantLevelType
	privs map[mysql.PrivilegeType]bool
}

func (ps *privileges) contain(p mysql.PrivilegeType) bool {
	if ps.privs == nil {
		return false
	}
	_, ok := ps.privs[p]
	return ok
}

func (ps *privileges) add(p mysql.PrivilegeType) {
	if ps.privs == nil {
		ps.privs = make(map[mysql.PrivilegeType]bool)
	}
	ps.privs[p] = true
}

func (ps *privileges) String() string {
	switch ps.Level {
	case ast.GrantLevelGlobal:
		return ps.globalPrivToString()
	case ast.GrantLevelDB:
		return ps.dbPrivToString()
	case ast.GrantLevelTable:
		return ps.tablePrivToString()
	}
	return ""
}

func (ps *privileges) globalPrivToString() string {
	if len(ps.privs) == len(mysql.AllGlobalPrivs) {
		return mysql.AllPrivilegeLiteral
	}
	pstrs := make([]string, 0, len(ps.privs))
	// Iterate AllGlobalPrivs to get stable order result.
	for _, p := range mysql.AllGlobalPrivs {
		_, ok := ps.privs[p]
		if !ok {
			continue
		}
		s, _ := mysql.Priv2Str[p]
		pstrs = append(pstrs, s)
	}
	return strings.Join(pstrs, ",")
}

func (ps *privileges) dbPrivToString() string {
	if len(ps.privs) == len(mysql.AllDBPrivs) {
		return mysql.AllPrivilegeLiteral
	}
	pstrs := make([]string, 0, len(ps.privs))
	// Iterate AllDBPrivs to get stable order result.
	for _, p := range mysql.AllDBPrivs {
		_, ok := ps.privs[p]
		if !ok {
			continue
		}
		s, _ := mysql.Priv2SetStr[p]
		pstrs = append(pstrs, s)
	}
	return strings.Join(pstrs, ",")
}

func (ps *privileges) tablePrivToString() string {
	if len(ps.privs) == len(mysql.AllTablePrivs) {
		return mysql.AllPrivilegeLiteral
	}
	pstrs := make([]string, 0, len(ps.privs))
	// Iterate AllTablePrivs to get stable order result.
	for _, p := range mysql.AllTablePrivs {
		_, ok := ps.privs[p]
		if !ok {
			continue
		}
		s, _ := mysql.Priv2Str[p]
		pstrs = append(pstrs, s)
	}
	return strings.Join(pstrs, ",")
}

type userPrivileges struct {
	User string
	Host string
	// Global privileges
	GlobalPrivs *privileges
	// DBName-privileges
	DBPrivs map[string]*privileges
	// DBName-TableName-privileges
	TablePrivs map[string]map[string]*privileges
}

func (ps *userPrivileges) ShowGrants() []string {
	gs := []string{}
	// Show global grants
	g := ps.GlobalPrivs.String()
	if len(g) > 0 {
		s := fmt.Sprintf(`GRANT %s ON *.* TO '%s'@'%s'`, g, ps.User, ps.Host)
		gs = append(gs, s)
	}
	// Show db scope grants
	for d, p := range ps.DBPrivs {
		g := p.String()
		if len(g) > 0 {
			s := fmt.Sprintf(`GRANT %s ON %s.* TO '%s'@'%s'`, g, d, ps.User, ps.Host)
			gs = append(gs, s)
		}
	}
	// Show table scope grants
	for d, dps := range ps.TablePrivs {
		for t, p := range dps {
			g := p.String()
			if len(g) > 0 {
				s := fmt.Sprintf(`GRANT %s ON %s.%s TO '%s'@'%s'`, g, d, t, ps.User, ps.Host)
				gs = append(gs, s)
			}
		}
	}
	return gs
}

// UserPrivileges implements privilege.Checker interface.
// This is used to check privilege for the current user.
type UserPrivileges struct {
	User  string
	privs *userPrivileges
}

// Check implements Checker.Check interface.
func (p *UserPrivileges) Check(ctx context.Context, db *model.DBInfo, tbl *model.TableInfo, privilege mysql.PrivilegeType) (bool, error) {
	if p.privs == nil {
		// Lazy load
		if len(p.User) == 0 {
			// User current user
			p.User = variable.GetSessionVars(ctx).User
			if len(p.User) == 0 {
				// In embedded db mode, user does not need to login. So we do not have username.
				// TODO: remove this check latter.
				return true, nil
			}
		}
		err := p.loadPrivileges(ctx)
		if err != nil {
			return false, errors.Trace(err)
		}
	}
	// Check global scope privileges.
	ok := p.privs.GlobalPrivs.contain(privilege)
	if ok {
		return true, nil
	}
	// Check db scope privileges.
	dbp, ok := p.privs.DBPrivs[db.Name.O]
	if ok {
		ok = dbp.contain(privilege)
		if ok {
			return true, nil
		}
	}
	if tbl == nil {
		return false, nil
	}
	// Check table scope privileges.
	dbTbl, ok := p.privs.TablePrivs[db.Name.O]
	if !ok {
		return false, nil
	}
	tblp, ok := dbTbl[tbl.Name.O]
	if !ok {
		return false, nil
	}
	return tblp.contain(privilege), nil
}

func (p *UserPrivileges) loadPrivileges(ctx context.Context) error {
	strs := strings.Split(p.User, "@")
	if len(strs) != 2 {
		return errors.Errorf("Wrong username format: %s", p.User)
	}
	username, host := strs[0], strs[1]
	p.privs = &userPrivileges{
		User: username,
		Host: host,
	}
	// Load privileges from mysql.User/DB/Table_privs/Column_privs table
	err := p.loadGlobalPrivileges(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	err = p.loadDBScopePrivileges(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	err = p.loadTableScopePrivileges(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	// TODO: consider column scope privilege latter.
	return nil
}

// mysql.User/mysql.DB table privilege columns start from index 3.
// See: booststrap.go CreateUserTable/CreateDBPrivTable
const userTablePrivColumnStartIndex = 3
const dbTablePrivColumnStartIndex = 3

func (p *UserPrivileges) loadGlobalPrivileges(ctx context.Context) error {
	sql := fmt.Sprintf(`SELECT * FROM %s.%s WHERE User="%s" AND (Host="%s" OR Host="%%");`,
		mysql.SystemDB, mysql.UserTable, p.privs.User, p.privs.Host)
	rs, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	if err != nil {
		return errors.Trace(err)
	}
	defer rs.Close()
	ps := &privileges{Level: ast.GrantLevelGlobal}
	fs, err := rs.Fields()
	if err != nil {
		return errors.Trace(err)
	}
	for {
		row, err := rs.Next()
		if err != nil {
			return errors.Trace(err)
		}
		if row == nil {
			break
		}
		for i := userTablePrivColumnStartIndex; i < len(fs); i++ {
			d := row.Data[i]
			if d.Kind() != types.KindMysqlEnum {
				return errors.Errorf("Privilege should be mysql.Enum: %v(%T)", d, d)
			}
			ed := d.GetMysqlEnum()
			if ed.String() != "Y" {
				continue
			}
			f := fs[i]
			p, ok := mysql.Col2PrivType[f.ColumnAsName.O]
			if !ok {
				return errors.New("Unknown Privilege Type!")
			}
			ps.add(p)
		}
	}
	p.privs.GlobalPrivs = ps
	return nil
}

func (p *UserPrivileges) loadDBScopePrivileges(ctx context.Context) error {
	sql := fmt.Sprintf(`SELECT * FROM %s.%s WHERE User="%s" AND (Host="%s" OR Host="%%");`,
		mysql.SystemDB, mysql.DBTable, p.privs.User, p.privs.Host)
	rs, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	if err != nil {
		return errors.Trace(err)
	}
	defer rs.Close()
	ps := make(map[string]*privileges)
	fs, err := rs.Fields()
	if err != nil {
		return errors.Trace(err)
	}
	for {
		row, err := rs.Next()
		if err != nil {
			return errors.Trace(err)
		}
		if row == nil {
			break
		}
		// DB
		dbStr := row.Data[1].GetString()
		ps[dbStr] = &privileges{Level: ast.GrantLevelDB}
		for i := dbTablePrivColumnStartIndex; i < len(fs); i++ {
			d := row.Data[i]
			if d.Kind() != types.KindMysqlEnum {
				return errors.Errorf("Privilege should be mysql.Enum: %v(%T)", d, d)
			}
			ed := d.GetMysqlEnum()
			if ed.String() != "Y" {
				continue
			}
			f := fs[i]
			p, ok := mysql.Col2PrivType[f.ColumnAsName.O]
			if !ok {
				return errors.New("Unknown Privilege Type!")
			}
			ps[dbStr].add(p)
		}
	}
	p.privs.DBPrivs = ps
	return nil
}

func (p *UserPrivileges) loadTableScopePrivileges(ctx context.Context) error {
	sql := fmt.Sprintf(`SELECT * FROM %s.%s WHERE User="%s" AND (Host="%s" OR Host="%%");`,
		mysql.SystemDB, mysql.TablePrivTable, p.privs.User, p.privs.Host)
	rs, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, sql)
	if err != nil {
		return errors.Trace(err)
	}
	defer rs.Close()
	ps := make(map[string]map[string]*privileges)
	for {
		row, err := rs.Next()
		if err != nil {
			return errors.Trace(err)
		}
		if row == nil {
			break
		}
		// DB
		dbStr := row.Data[1].GetString()
		// Table_name
		tblStr := row.Data[3].GetString()
		_, ok := ps[dbStr]
		if !ok {
			ps[dbStr] = make(map[string]*privileges)
		}
		ps[dbStr][tblStr] = &privileges{Level: ast.GrantLevelTable}
		// Table_priv
		tblPrivs := row.Data[6].GetMysqlSet()
		pvs := strings.Split(tblPrivs.Name, ",")
		for _, d := range pvs {
			p, ok := mysql.SetStr2Priv[d]
			if !ok {
				return errors.New("Unknown Privilege Type!")
			}
			ps[dbStr][tblStr].add(p)
		}
	}
	p.privs.TablePrivs = ps
	return nil
}

// ShowGrants implements privilege.Checker ShowGrants interface.
func (p *UserPrivileges) ShowGrants(ctx context.Context, user string) ([]string, error) {
	// If user is current user
	if user == p.User {
		return p.privs.ShowGrants(), nil
	}
	userp := &UserPrivileges{User: user}
	err := userp.loadPrivileges(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return userp.privs.ShowGrants(), nil
}
