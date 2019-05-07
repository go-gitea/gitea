# A pure Go MSSQL driver for Go's database/sql package

## Install

    go get github.com/denisenkom/go-mssqldb

## Tests

`go test` is used for testing. A running instance of MSSQL server is required.
Environment variables are used to pass login information.

Example:

    env HOST=localhost SQLUSER=sa SQLPASSWORD=sa DATABASE=test go test

## Connection Parameters

* "server" - host or host\instance (default localhost)
* "port" - used only when there is no instance in server (default 1433)
* "failoverpartner" - host or host\instance (default is no partner). 
* "failoverport" - used only when there is no instance in failoverpartner (default 1433)
* "user id" - enter the SQL Server Authentication user id or the Windows Authentication user id in the DOMAIN\User format. On Windows, if user id is empty or missing Single-Sign-On is used.
* "password"
* "database"
* "connection timeout" - in seconds (default is 30)
* "dial timeout" - in seconds (default is 5)
* "keepAlive" - in seconds; 0 to disable (default is 0)
* "log" - logging flags (default 0/no logging, 63 for full logging)
  *  1 log errors
  *  2 log messages
  *  4 log rows affected
  *  8 trace sql statements
  * 16 log statement parameters
  * 32 log transaction begin/end
* "encrypt"
  * disable - Data send between client and server is not encrypted.
  * false - Data sent between client and server is not encrypted beyond the login packet. (Default)
  * true - Data sent between client and server is encrypted.
* "TrustServerCertificate"
  * false - Server certificate is checked. Default is false if encypt is specified.
  * true - Server certificate is not checked. Default is true if encrypt is not specified. If trust server certificate is true, driver accepts any certificate presented by the server and any host name in that certificate. In this mode, TLS is susceptible to man-in-the-middle attacks. This should be used only for testing.
* "certificate" - The file that contains the public key certificate of the CA that signed the SQL Server certificate. The specified certificate overrides the go platform specific CA certificates.
* "hostNameInCertificate" - Specifies the Common Name (CN) in the server certificate. Default value is the server host.
* "ServerSPN" - The kerberos SPN (Service Principal Name) for the server. Default is MSSQLSvc/host:port.
* "Workstation ID" - The workstation name (default is the host name)
* "app name" - The application name (default is go-mssqldb)
* "ApplicationIntent" - Can be given the value "ReadOnly" to initiate a read-only connection to an Availability Group listener.

Example:

```go
    db, err := sql.Open("mssql", "server=localhost;user id=sa")
```

## Statement Parameters

In the SQL statement text, literals may be replaced by a parameter that matches one of the following:

* ?
* ?nnn
* :nnn
* $nnn

where nnn represents an integer that specifies a 1-indexed positional parameter. Ex:

```go
db.Query("SELECT * FROM t WHERE a = ?3, b = ?2, c = ?1", "x", "y", "z")
```

will expand to roughly

```sql
SELECT * FROM t WHERE a = 'z', b = 'y', c = 'x'
```


## Features

* Can be used with SQL Server 2005 or newer
* Can be used with Microsoft Azure SQL Database
* Can be used on all go supported platforms (e.g. Linux, Mac OS X and Windows)
* Supports new date/time types: date, time, datetime2, datetimeoffset
* Supports string parameters longer than 8000 characters
* Supports encryption using SSL/TLS
* Supports SQL Server and Windows Authentication
* Supports Single-Sign-On on Windows
* Supports connections to AlwaysOn Availability Group listeners, including re-direction to read-only replicas.
* Supports query notifications

## Known Issues

* SQL Server 2008 and 2008 R2 engine cannot handle login records when SSL encryption is not disabled.
To fix SQL Server 2008 R2 issue, install SQL Server 2008 R2 Service Pack 2.
To fix SQL Server 2008 issue, install Microsoft SQL Server 2008 Service Pack 3 and Cumulative update package 3 for SQL Server 2008 SP3.
More information: http://support.microsoft.com/kb/2653857
