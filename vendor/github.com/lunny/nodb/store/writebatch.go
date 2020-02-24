package store

import (
	"github.com/lunny/nodb/store/driver"
)

type WriteBatch interface {
	driver.IWriteBatch
}
