tidb driver and dialect for github.com/go-xorm/xorm
========

Currently, we can support tidb for allmost all the operations.

# How to use

Just like other supports of xorm, but you should import the three packages:

```Go
import (
    _ "github.com/pingcap/tidb"
    _ "github.com/go-xorm/tidb"
    "github.com/go-xorm/xorm"
)

//The formate of DataSource name is store://uri/dbname
// for goleveldb as store
xorm.NewEngine("tidb", "goleveldb://./tidb/tidb")
// for memory as store
xorm.NewEngine("tidb", "memory://tidb/tidb")
// for boltdb as store
xorm.NewEngine("tidb", "boltdb://./tidb/tidb")
```
