# go-themis
[![Build Status](https://travis-ci.org/pingcap/go-themis.svg?branch=master)](https://travis-ci.org/pingcap/go-themis)

go-themis is a Go client for [pingcap/themis](https://github.com/pingcap/themis).

Themis provides cross-row/cross-table transaction on HBase based on [google's Percolator](http://research.google.com/pubs/pub36726.html).

go-themis is depends on [pingcap/go-hbase](https://github.com/pingcap/go-hbase).

Install:

```
go get -u github.com/pingcap/go-themis
```

Example:

```
tx := themis.NewTxn(c, oracles.NewLocalOracle())
put := hbase.NewPut([]byte("Row1"))
put.AddValue([]byte("cf"), []byte("q"), []byte("value"))

put2 := hbase.NewPut([]byte("Row2"))
put2.AddValue([]byte("cf"), []byte("q"), []byte("value"))

tx.Put(tblName, put)
tx.Put(tblName, put2)

tx.Commit()
```
