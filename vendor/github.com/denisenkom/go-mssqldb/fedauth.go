package mssql

import (
	"context"
	"errors"
)

// Federated authentication library affects the login data structure and message sequence.
const (
	// fedAuthLibraryLiveIDCompactToken specifies the Microsoft Live ID Compact Token authentication scheme
	fedAuthLibraryLiveIDCompactToken = 0x00

	// fedAuthLibrarySecurityToken specifies a token-based authentication where the token is available
	// without additional information provided during the login sequence.
	fedAuthLibrarySecurityToken = 0x01

	// fedAuthLibraryADAL specifies a token-based authentication where a token is obtained during the
	// login sequence using the server SPN and STS URL provided by the server during login.
	fedAuthLibraryADAL = 0x02

	// fedAuthLibraryReserved is used to indicate that no federated authentication scheme applies.
	fedAuthLibraryReserved = 0x7F
)

// Federated authentication ADAL workflow affects the mechanism used to authenticate.
const (
	// fedAuthADALWorkflowPassword uses a username/password to obtain a token from Active Directory
	fedAuthADALWorkflowPassword = 0x01

	// fedAuthADALWorkflowPassword uses the Windows identity to obtain a token from Active Directory
	fedAuthADALWorkflowIntegrated = 0x02

	// fedAuthADALWorkflowMSI uses the managed identity service to obtain a token
	fedAuthADALWorkflowMSI = 0x03
)

// newSecurityTokenConnector creates a new connector from a DSN and a token provider.
// When invoked, token provider implementations should contact the security token
// service specified and obtain the appropriate token, or return an error
// to indicate why a token is not available.
// The returned connector may be used with sql.OpenDB.
func newSecurityTokenConnector(dsn string, tokenProvider func(ctx context.Context) (string, error)) (*Connector, error) {
	if tokenProvider == nil {
		return nil, errors.New("mssql: tokenProvider cannot be nil")
	}

	conn, err := NewConnector(dsn)
	if err != nil {
		return nil, err
	}

	conn.params.fedAuthLibrary = fedAuthLibrarySecurityToken
	conn.securityTokenProvider = tokenProvider

	return conn, nil
}

// newADALTokenConnector creates a new connector from a DSN and a Active Directory token provider.
// Token provider implementations are called during federated
// authentication login sequences where the server provides a service
// principal name and security token service endpoint that should be used
// to obtain the token. Implementations should contact the security token
// service specified and obtain the appropriate token, or return an error
// to indicate why a token is not available.
//
// The returned connector may be used with sql.OpenDB.
func newActiveDirectoryTokenConnector(dsn string, adalWorkflow byte, tokenProvider func(ctx context.Context, serverSPN, stsURL string) (string, error)) (*Connector, error) {
	if tokenProvider == nil {
		return nil, errors.New("mssql: tokenProvider cannot be nil")
	}

	conn, err := NewConnector(dsn)
	if err != nil {
		return nil, err
	}

	conn.params.fedAuthLibrary = fedAuthLibraryADAL
	conn.params.fedAuthADALWorkflow = adalWorkflow
	conn.adalTokenProvider = tokenProvider

	return conn, nil
}
