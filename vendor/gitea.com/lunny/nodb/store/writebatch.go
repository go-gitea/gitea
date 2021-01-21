package store

import (
	"gitea.com/lunny/nodb/store/driver"
)

type WriteBatch interface {
	driver.IWriteBatch
}
