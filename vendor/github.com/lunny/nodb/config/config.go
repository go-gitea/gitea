package config

import (
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

type Size int

const (
	DefaultAddr     string = "127.0.0.1:6380"
	DefaultHttpAddr string = "127.0.0.1:11181"

	DefaultDBName string = "goleveldb"

	DefaultDataDir string = "./data"
)

const (
	MaxBinLogFileSize int = 1024 * 1024 * 1024
	MaxBinLogFileNum  int = 10000

	DefaultBinLogFileSize int = MaxBinLogFileSize
	DefaultBinLogFileNum  int = 10
)

type LevelDBConfig struct {
	Compression     bool `toml:"compression"`
	BlockSize       int  `toml:"block_size"`
	WriteBufferSize int  `toml:"write_buffer_size"`
	CacheSize       int  `toml:"cache_size"`
	MaxOpenFiles    int  `toml:"max_open_files"`
}

type LMDBConfig struct {
	MapSize int  `toml:"map_size"`
	NoSync  bool `toml:"nosync"`
}

type BinLogConfig struct {
	MaxFileSize int `toml:"max_file_size"`
	MaxFileNum  int `toml:"max_file_num"`
}

type Config struct {
	DataDir string `toml:"data_dir"`

	DBName string `toml:"db_name"`

	LevelDB LevelDBConfig `toml:"leveldb"`

	LMDB LMDBConfig `toml:"lmdb"`

	BinLog BinLogConfig `toml:"binlog"`

	SlaveOf string `toml:"slaveof"`

	AccessLog string `toml:"access_log"`
}

func NewConfigWithFile(fileName string) (*Config, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	return NewConfigWithData(data)
}

func NewConfigWithData(data []byte) (*Config, error) {
	cfg := NewConfigDefault()

	_, err := toml.Decode(string(data), cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func NewConfigDefault() *Config {
	cfg := new(Config)

	cfg.DataDir = DefaultDataDir

	cfg.DBName = DefaultDBName

	// disable binlog
	cfg.BinLog.MaxFileNum = 0
	cfg.BinLog.MaxFileSize = 0

	// disable replication
	cfg.SlaveOf = ""

	// disable access log
	cfg.AccessLog = ""

	cfg.LMDB.MapSize = 20 * 1024 * 1024
	cfg.LMDB.NoSync = true

	return cfg
}

func (cfg *LevelDBConfig) Adjust() {
	if cfg.CacheSize <= 0 {
		cfg.CacheSize = 4 * 1024 * 1024
	}

	if cfg.BlockSize <= 0 {
		cfg.BlockSize = 4 * 1024
	}

	if cfg.WriteBufferSize <= 0 {
		cfg.WriteBufferSize = 4 * 1024 * 1024
	}

	if cfg.MaxOpenFiles < 1024 {
		cfg.MaxOpenFiles = 1024
	}
}

func (cfg *BinLogConfig) Adjust() {
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = DefaultBinLogFileSize
	} else if cfg.MaxFileSize > MaxBinLogFileSize {
		cfg.MaxFileSize = MaxBinLogFileSize
	}

	if cfg.MaxFileNum <= 0 {
		cfg.MaxFileNum = DefaultBinLogFileNum
	} else if cfg.MaxFileNum > MaxBinLogFileNum {
		cfg.MaxFileNum = MaxBinLogFileNum
	}
}
