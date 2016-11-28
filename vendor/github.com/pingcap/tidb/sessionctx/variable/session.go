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

package variable

import (
	"github.com/pingcap/tidb/context"
)

// SessionVars is to handle user-defined or global variables in current session.
type SessionVars struct {
	// user-defined variables
	Users map[string]string
	// system variables
	Systems map[string]string
	// prepared statement
	PreparedStmts map[uint32]interface{}

	PreparedStmtNameToID map[string]uint32
	// prepared statement auto increment id
	preparedStmtID uint32

	// following variables are special for current session
	Status       uint16
	LastInsertID uint64
	AffectedRows uint64

	// Client capability
	ClientCapability uint32

	// Connection ID
	ConnectionID uint64

	// Found rows
	FoundRows uint64

	// Current user
	User string
}

// sessionVarsKeyType is a dummy type to avoid naming collision in context.
type sessionVarsKeyType int

// String defines a Stringer function for debugging and pretty printing.
func (k sessionVarsKeyType) String() string {
	return "session_vars"
}

const sessionVarsKey sessionVarsKeyType = 0

// BindSessionVars creates a session vars object and binds it to context.
func BindSessionVars(ctx context.Context) {
	v := &SessionVars{
		Users:                make(map[string]string),
		Systems:              make(map[string]string),
		PreparedStmts:        make(map[uint32]interface{}),
		PreparedStmtNameToID: make(map[string]uint32),
	}

	ctx.SetValue(sessionVarsKey, v)
}

// GetSessionVars gets the session vars from context.
func GetSessionVars(ctx context.Context) *SessionVars {
	v, ok := ctx.Value(sessionVarsKey).(*SessionVars)
	if !ok {
		return nil
	}
	return v
}

const (
	characterSetConnection = "character_set_connection"
	collationConnection    = "collation_connection"
)

// GetCharsetInfo gets charset and collation for current context.
// What character set should the server translate a statement to after receiving it?
// For this, the server uses the character_set_connection and collation_connection system variables.
// It converts statements sent by the client from character_set_client to character_set_connection
// (except for string literals that have an introducer such as _latin1 or _utf8).
// collation_connection is important for comparisons of literal strings.
// For comparisons of strings with column values, collation_connection does not matter because columns
// have their own collation, which has a higher collation precedence.
// See: https://dev.mysql.com/doc/refman/5.7/en/charset-connection.html
func GetCharsetInfo(ctx context.Context) (charset, collation string) {
	sessionVars := GetSessionVars(ctx)
	charset = sessionVars.Systems[characterSetConnection]
	collation = sessionVars.Systems[collationConnection]
	return
}

// SetLastInsertID saves the last insert id to the session context.
// TODO: we may store the result for last_insert_id sys var later.
func (s *SessionVars) SetLastInsertID(insertID uint64) {
	s.LastInsertID = insertID
}

// SetAffectedRows saves the affected rows to the session context.
func (s *SessionVars) SetAffectedRows(affectedRows uint64) {
	s.AffectedRows = affectedRows
}

// AddAffectedRows adds affected rows with the argument rows.
func (s *SessionVars) AddAffectedRows(rows uint64) {
	s.AffectedRows += rows
}

// AddFoundRows adds found rows with the argument rows.
func (s *SessionVars) AddFoundRows(rows uint64) {
	s.FoundRows += rows
}

// SetStatusFlag sets the session server status variable.
// If on is ture sets the flag in session status,
// otherwise removes the flag.
func (s *SessionVars) SetStatusFlag(flag uint16, on bool) {
	if on {
		s.Status |= flag
		return
	}
	s.Status &= (^flag)
}

// GetNextPreparedStmtID generates and returns the next session scope prepared statement id.
func (s *SessionVars) GetNextPreparedStmtID() uint32 {
	s.preparedStmtID++
	return s.preparedStmtID
}

// SetCurrentUser saves the current user to the session context.
func (s *SessionVars) SetCurrentUser(user string) {
	s.User = user
}
