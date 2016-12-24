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

package mysql

// Version informations.
const (
	MinProtocolVersion byte   = 10
	MaxPayloadLen      int    = 1<<24 - 1
	ServerVersion      string = "5.5.31-TiDB-1.0"
)

// Header informations.
const (
	OKHeader          byte = 0x00
	ErrHeader         byte = 0xff
	EOFHeader         byte = 0xfe
	LocalInFileHeader byte = 0xfb
)

// Server informations.
const (
	ServerStatusInTrans            uint16 = 0x0001
	ServerStatusAutocommit         uint16 = 0x0002
	ServerMoreResultsExists        uint16 = 0x0008
	ServerStatusNoGoodIndexUsed    uint16 = 0x0010
	ServerStatusNoIndexUsed        uint16 = 0x0020
	ServerStatusCursorExists       uint16 = 0x0040
	ServerStatusLastRowSend        uint16 = 0x0080
	ServerStatusDBDropped          uint16 = 0x0100
	ServerStatusNoBackslashEscaped uint16 = 0x0200
	ServerStatusMetadataChanged    uint16 = 0x0400
	ServerStatusWasSlow            uint16 = 0x0800
	ServerPSOutParams              uint16 = 0x1000
)

// Command informations.
const (
	ComSleep byte = iota
	ComQuit
	ComInitDB
	ComQuery
	ComFieldList
	ComCreateDB
	ComDropDB
	ComRefresh
	ComShutdown
	ComStatistics
	ComProcessInfo
	ComConnect
	ComProcessKill
	ComDebug
	ComPing
	ComTime
	ComDelayedInsert
	ComChangeUser
	ComBinlogDump
	ComTableDump
	ComConnectOut
	ComRegisterSlave
	ComStmtPrepare
	ComStmtExecute
	ComStmtSendLongData
	ComStmtClose
	ComStmtReset
	ComSetOption
	ComStmtFetch
	ComDaemon
	ComBinlogDumpGtid
	ComResetConnection
)

// Client informations.
const (
	ClientLongPassword uint32 = 1 << iota
	ClientFoundRows
	ClientLongFlag
	ClientConnectWithDB
	ClientNoSchema
	ClientCompress
	ClientODBC
	ClientLocalFiles
	ClientIgnoreSpace
	ClientProtocol41
	ClientInteractive
	ClientSSL
	ClientIgnoreSigpipe
	ClientTransactions
	ClientReserved
	ClientSecureConnection
	ClientMultiStatements
	ClientMultiResults
	ClientPSMultiResults
	ClientPluginAuth
	ClientConnectAtts
	ClientPluginAuthLenencClientData
)

// Cache type informations.
const (
	TypeNoCache byte = 0xff
)

// Auth name informations.
const (
	AuthName = "mysql_native_password"
)

// MySQL database and tables.
const (
	// SystemDB is the name of system database.
	SystemDB = "mysql"
	// UserTable is the table in system db contains user info.
	UserTable = "User"
	// DBTable is the table in system db contains db scope privilege info.
	DBTable = "DB"
	// TablePrivTable is the table in system db contains table scope privilege info.
	TablePrivTable = "Tables_priv"
	// ColumnPrivTable is the table in system db contains column scope privilege info.
	ColumnPrivTable = "Columns_priv"
	// GlobalVariablesTable is the table contains global system variables.
	GlobalVariablesTable = "GLOBAL_VARIABLES"
	// GlobalStatusTable is the table contains global status variables.
	GlobalStatusTable = "GLOBAL_STATUS"
	// TiDBTable is the table contains tidb info.
	TiDBTable = "tidb"
)

// PrivilegeType  privilege
type PrivilegeType uint32

const (
	_ PrivilegeType = 1 << iota
	// CreatePriv is the privilege to create schema/table.
	CreatePriv
	// SelectPriv is the privilege to read from table.
	SelectPriv
	// InsertPriv is the privilege to insert data into table.
	InsertPriv
	// UpdatePriv is the privilege to update data in table.
	UpdatePriv
	// DeletePriv is the privilege to delete data from table.
	DeletePriv
	// ShowDBPriv is the privilege to run show databases statement.
	ShowDBPriv
	// CreateUserPriv is the privilege to create user.
	CreateUserPriv
	// DropPriv is the privilege to drop schema/table.
	DropPriv
	// GrantPriv is the privilege to grant privilege to user.
	GrantPriv
	// AlterPriv is the privilege to run alter statement.
	AlterPriv
	// ExecutePriv is the privilege to run execute statement.
	ExecutePriv
	// IndexPriv is the privilege to create/drop index.
	IndexPriv
	// AllPriv is the privilege for all actions.
	AllPriv
)

// Priv2UserCol is the privilege to mysql.user table column name.
var Priv2UserCol = map[PrivilegeType]string{
	CreatePriv:     "Create_priv",
	SelectPriv:     "Select_priv",
	InsertPriv:     "Insert_priv",
	UpdatePriv:     "Update_priv",
	DeletePriv:     "Delete_priv",
	ShowDBPriv:     "Show_db_priv",
	CreateUserPriv: "Create_user_priv",
	DropPriv:       "Drop_priv",
	GrantPriv:      "Grant_priv",
	AlterPriv:      "Alter_priv",
	ExecutePriv:    "Execute_priv",
	IndexPriv:      "Index_priv",
}

// Col2PrivType is the privilege tables column name to privilege type.
var Col2PrivType = map[string]PrivilegeType{
	"Create_priv":      CreatePriv,
	"Select_priv":      SelectPriv,
	"Insert_priv":      InsertPriv,
	"Update_priv":      UpdatePriv,
	"Delete_priv":      DeletePriv,
	"Show_db_priv":     ShowDBPriv,
	"Create_user_priv": CreateUserPriv,
	"Drop_priv":        DropPriv,
	"Grant_priv":       GrantPriv,
	"Alter_priv":       AlterPriv,
	"Execute_priv":     ExecutePriv,
	"Index_priv":       IndexPriv,
}

// AllGlobalPrivs is all the privileges in global scope.
var AllGlobalPrivs = []PrivilegeType{SelectPriv, InsertPriv, UpdatePriv, DeletePriv, CreatePriv, DropPriv, GrantPriv, AlterPriv, ShowDBPriv, ExecutePriv, IndexPriv, CreateUserPriv}

// Priv2Str is the map for privilege to string.
var Priv2Str = map[PrivilegeType]string{
	CreatePriv:     "Create",
	SelectPriv:     "Select",
	InsertPriv:     "Insert",
	UpdatePriv:     "Update",
	DeletePriv:     "Delete",
	ShowDBPriv:     "Show Databases",
	CreateUserPriv: "Create User",
	DropPriv:       "Drop",
	GrantPriv:      "Grant Option",
	AlterPriv:      "Alter",
	ExecutePriv:    "Execute",
	IndexPriv:      "Index",
}

// Priv2SetStr is the map for privilege to string.
var Priv2SetStr = map[PrivilegeType]string{
	CreatePriv:  "Create",
	SelectPriv:  "Select",
	InsertPriv:  "Insert",
	UpdatePriv:  "Update",
	DeletePriv:  "Delete",
	DropPriv:    "Drop",
	GrantPriv:   "Grant",
	AlterPriv:   "Alter",
	ExecutePriv: "Execute",
	IndexPriv:   "Index",
}

// SetStr2Priv is the map for privilege set string to privilege type.
var SetStr2Priv = map[string]PrivilegeType{
	"Create":  CreatePriv,
	"Select":  SelectPriv,
	"Insert":  InsertPriv,
	"Update":  UpdatePriv,
	"Delete":  DeletePriv,
	"Drop":    DropPriv,
	"Grant":   GrantPriv,
	"Alter":   AlterPriv,
	"Execute": ExecutePriv,
	"Index":   IndexPriv,
}

// AllDBPrivs is all the privileges in database scope.
var AllDBPrivs = []PrivilegeType{SelectPriv, InsertPriv, UpdatePriv, DeletePriv, CreatePriv, DropPriv, GrantPriv, AlterPriv, ExecutePriv, IndexPriv}

// AllTablePrivs is all the privileges in table scope.
var AllTablePrivs = []PrivilegeType{SelectPriv, InsertPriv, UpdatePriv, DeletePriv, CreatePriv, DropPriv, GrantPriv, AlterPriv, IndexPriv}

// AllColumnPrivs is all the privileges in column scope.
var AllColumnPrivs = []PrivilegeType{SelectPriv, InsertPriv, UpdatePriv}

// AllPrivilegeLiteral is the string literal for All Privilege.
const AllPrivilegeLiteral = "ALL PRIVILEGES"
