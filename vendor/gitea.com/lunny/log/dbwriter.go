package log

import (
	"database/sql"
	"time"
)

type DBWriter struct {
	db      *sql.DB
	stmt    *sql.Stmt
	content chan []byte
}

func NewDBWriter(db *sql.DB) (*DBWriter, error) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS log (id int, content text, created datetime)")
	if err != nil {
		return nil, err
	}
	stmt, err := db.Prepare("INSERT INTO log (content, created) values (?, ?)")
	if err != nil {
		return nil, err
	}
	return &DBWriter{db, stmt, make(chan []byte, 1000)}, nil
}

func (w *DBWriter) Write(p []byte) (n int, err error) {
	_, err = w.stmt.Exec(string(p), time.Now())
	if err == nil {
		n = len(p)
	}
	return
}

func (w *DBWriter) Close() {
	w.stmt.Close()
}
