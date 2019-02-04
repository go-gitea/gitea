/*
A thin wrapper for the lmdb C library. These are low-level bindings for the C
API. The C documentation should be used as a reference while developing
(http://symas.com/mdb/doc/group__mdb.html).

Errors

The errors returned by the package API will with few exceptions be of type
Errno or syscall.Errno.  The only errors of type Errno returned are those
defined in lmdb.h.  Other errno values like EINVAL will by of type
syscall.Errno.
*/
package mdb
