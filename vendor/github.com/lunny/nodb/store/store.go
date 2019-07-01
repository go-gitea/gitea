package store

import (
	"fmt"
	"os"
	"path"
	"github.com/lunny/nodb/config"
	"github.com/lunny/nodb/store/driver"

	_ "github.com/lunny/nodb/store/goleveldb"
)

func getStorePath(cfg *config.Config) string {
	return path.Join(cfg.DataDir, fmt.Sprintf("%s_data", cfg.DBName))
}

func Open(cfg *config.Config) (*DB, error) {
	s, err := driver.GetStore(cfg)
	if err != nil {
		return nil, err
	}

	path := getStorePath(cfg)

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}

	idb, err := s.Open(path, cfg)
	if err != nil {
		return nil, err
	}

	db := &DB{idb}

	return db, nil
}

func Repair(cfg *config.Config) error {
	s, err := driver.GetStore(cfg)
	if err != nil {
		return err
	}

	path := getStorePath(cfg)

	return s.Repair(path, cfg)
}

func init() {
}
